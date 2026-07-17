package gui

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/cli"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/pipeline"
)

func validStartRequest(t *testing.T) StartRequest {
	t.Helper()
	root := t.TempDir()
	cs2 := touchFile(t, filepath.Join(root, "cs2.exe"))
	demos := filepath.Join(root, "demos")
	touchFile(t, filepath.Join(demos, "match.dem"))
	hlae := touchFile(t, filepath.Join(root, "tools", "hlae", "hlae.exe"))
	hook := touchFile(t, filepath.Join(root, "tools", "hlae", "x64", "AfxHookSource2.dll"))
	logo := touchFile(t, filepath.Join(root, "assets", "cs2sj-logo.jpg"))
	return StartRequest{
		Values: FormValues{CS2Path: cs2, InputDir: demos, OutputDir: filepath.Join(root, "videos")},
		Bundle: Bundle{RootPath: filepath.Dir(hlae), HLAEPath: hlae, HookDLLPath: hook, LogoPath: logo},
	}
}

func waitFinalEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	for event := range events {
		switch event.State {
		case StateCompleted, StatePartial, StateFailed, StateCancelled:
			return event
		}
	}
	t.Fatal("event stream closed without final event")
	return Event{}
}

func TestControllerRejectsConcurrentStart(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	controller := NewController(func(context.Context, cli.Options, *slog.Logger) ([]pipeline.Result, error) {
		close(started)
		<-release
		return []pipeline.Result{{Manifest: model.Manifest{State: model.DemoCompleted}}}, nil
	})
	request := validStartRequest(t)
	if err := controller.Start(context.Background(), request, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := controller.Start(context.Background(), request, func(Event) {}); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("got %v", err)
	}
	close(release)
	controller.Wait()
}

func TestControllerPassesResolvedOptionsAndReportsCompletion(t *testing.T) {
	request := validStartRequest(t)
	request.HUDMode = model.HUDCustom
	request.SelectedHighlights = map[string][]string{filepath.Join(request.Values.InputDir, "match.dem"): {"clip"}}
	events := make(chan Event, 16)
	controller := NewController(func(_ context.Context, options cli.Options, logger *slog.Logger) ([]pipeline.Result, error) {
		if options.Command != "render" || options.InputDir != request.Values.InputDir || options.OutputDir != request.Values.OutputDir || options.CS2Path != request.Values.CS2Path || options.HLAEPath != request.Bundle.HLAEPath || options.HookDLLPath != request.Bundle.HookDLLPath || options.HUDLogoPath != request.Bundle.LogoPath || options.HUDMode != model.HUDCustom || len(options.SelectedHighlights) != 1 {
			t.Fatalf("unexpected options: %#v", options)
		}
		logger.Info("capture.started", "pass", 1)
		return []pipeline.Result{
			{Manifest: model.Manifest{State: model.DemoCompleted}},
			{Manifest: model.Manifest{State: model.DemoNoHighlights}},
		}, nil
	})
	if err := controller.Start(context.Background(), request, func(event Event) { events <- event }); err != nil {
		t.Fatal(err)
	}
	final := waitFinalEvent(t, events)
	controller.Wait()
	if final.State != StateCompleted || final.Processed != 2 || final.Completed != 2 || final.Partial != 0 || final.Failed != 0 {
		t.Fatalf("unexpected final event: %#v", final)
	}
}

func TestControllerReportsPartialAndFailedResults(t *testing.T) {
	events := make(chan Event, 16)
	controller := NewController(func(context.Context, cli.Options, *slog.Logger) ([]pipeline.Result, error) {
		return []pipeline.Result{
			{Manifest: model.Manifest{State: model.DemoCompleted}},
			{Manifest: model.Manifest{State: model.DemoPartial}},
			{Err: errors.New("demo corrupt")},
		}, nil
	})
	if err := controller.Start(context.Background(), validStartRequest(t), func(event Event) { events <- event }); err != nil {
		t.Fatal(err)
	}
	final := waitFinalEvent(t, events)
	controller.Wait()
	if final.State != StatePartial || final.Processed != 3 || final.Completed != 1 || final.Partial != 1 || final.Failed != 1 {
		t.Fatalf("unexpected final event: %#v", final)
	}
}

func TestControllerReportsRenderFailure(t *testing.T) {
	events := make(chan Event, 16)
	controller := NewController(func(context.Context, cli.Options, *slog.Logger) ([]pipeline.Result, error) {
		return nil, errors.New("preflight failed")
	})
	if err := controller.Start(context.Background(), validStartRequest(t), func(event Event) { events <- event }); err != nil {
		t.Fatal(err)
	}
	final := waitFinalEvent(t, events)
	controller.Wait()
	if final.State != StateFailed || final.Message != "preflight failed" {
		t.Fatalf("unexpected final event: %#v", final)
	}
}

func TestControllerPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan Event, 16)
	started := make(chan struct{})
	controller := NewController(func(ctx context.Context, _ cli.Options, _ *slog.Logger) ([]pipeline.Result, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err := controller.Start(ctx, validStartRequest(t), func(event Event) { events <- event }); err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()
	final := waitFinalEvent(t, events)
	controller.Wait()
	if final.State != StateCancelled {
		t.Fatalf("unexpected final event: %#v", final)
	}
}

func TestControllerRejectsInvalidBundleBeforeStarting(t *testing.T) {
	called := false
	controller := NewController(func(context.Context, cli.Options, *slog.Logger) ([]pipeline.Result, error) {
		called = true
		return nil, nil
	})
	request := validStartRequest(t)
	request.Bundle.HLAEPath = filepath.Join(t.TempDir(), "missing.exe")
	if err := controller.Start(context.Background(), request, func(Event) {}); err == nil || err.Error() != MissingHLAEMessage {
		t.Fatalf("got %v", err)
	}
	if called {
		t.Fatal("renderer was called")
	}
}
