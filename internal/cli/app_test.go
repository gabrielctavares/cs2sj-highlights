package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/pipeline"
)

func TestParseRenderAcceptsInputBeforeFlags(t *testing.T) {
	got, err := ParseArgs([]string{"render", `C:\demos`, "--output", `C:\videos`, "--hlae", `C:\HLAE\HLAE.exe`, "--hud", "custom"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Command != "render" || got.InputDir != `C:\demos` || got.OutputDir != `C:\videos` || got.HLAEPath != `C:\HLAE\HLAE.exe` || got.HUDMode != model.HUDCustom {
		t.Fatalf("unexpected args: %#v", got)
	}
}

func TestParseAnalyze(t *testing.T) {
	got, err := ParseArgs([]string{"analyze", `C:\demos`, "--output", `C:\videos`})
	if err != nil {
		t.Fatal(err)
	}
	if got.Command != "analyze" || got.InputDir == "" || got.OutputDir == "" {
		t.Fatalf("unexpected args: %#v", got)
	}
}

func TestParseRejectsInvalidInvocation(t *testing.T) {
	tests := [][]string{
		nil,
		{"unknown", `C:\demos`, "--output", `C:\videos`},
		{"analyze"},
		{"analyze", `C:\demos`},
		{"analyze", `C:\demos`, "--unknown"},
		{"render", `C:\demos`, "--output", `C:\videos`, "--hud", "invalid"},
	}
	for _, args := range tests {
		if _, err := ParseArgs(args); err == nil {
			t.Errorf("expected error for %#v", args)
		}
	}
}

func TestParseHelpAndUsage(t *testing.T) {
	if _, err := ParseArgs([]string{"analyze", `C:\demos`, "--help"}); !errors.Is(err, ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
	usage := Usage()
	if !strings.Contains(usage, "cs2-highlights analyze") || !strings.Contains(usage, "cs2-highlights render") {
		t.Fatalf("unexpected usage: %s", usage)
	}
}

func TestAppExitCodesAndSummary(t *testing.T) {
	app := App{
		Analyze: func(context.Context, Options) ([]pipeline.Result, error) {
			return []pipeline.Result{{DemoPath: "a.dem", Manifest: model.Manifest{State: model.DemoPending}}}, nil
		},
		Render: func(context.Context, Options) ([]pipeline.Result, error) {
			return []pipeline.Result{{DemoPath: "a.dem", Manifest: model.Manifest{State: model.DemoPartial}}}, nil
		},
	}
	var stdout, stderr bytes.Buffer
	if code := app.Run(context.Background(), []string{"analyze", `C:\demos`, "--output", `C:\videos`}, &stdout, &stderr); code != 0 {
		t.Fatalf("analyze code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := app.Run(context.Background(), []string{"render", `C:\demos`, "--output", `C:\videos`}, &stdout, &stderr); code != 2 {
		t.Fatalf("render code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 demo") {
		t.Fatalf("missing summary: %s", stdout.String())
	}
}

func TestAppReturnsOneForGlobalFailure(t *testing.T) {
	app := App{Analyze: func(context.Context, Options) ([]pipeline.Result, error) { return nil, errors.New("global") }}
	var stdout, stderr bytes.Buffer
	if code := app.Run(context.Background(), []string{"analyze", `C:\demos`, "--output", `C:\videos`}, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d", code)
	}
}
