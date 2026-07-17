package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gabrielctavares/cs2sj-highlights/internal/cli"
	"github.com/gabrielctavares/cs2sj-highlights/internal/feedback"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/pipeline"
)

var ErrAlreadyRunning = errors.New("já existe um processamento em andamento")

const (
	StateRunning   = "running"
	StateCompleted = "completed"
	StatePartial   = "partial"
	StateFailed    = "failed"
	StateCancelled = "cancelled"
)

type RenderFunc func(context.Context, cli.Options, *slog.Logger) ([]pipeline.Result, error)

type StartRequest struct {
	Values             FormValues
	Bundle             Bundle
	SelectedHighlights map[string][]string
	HUDMode            model.HUDMode
	HUDThemePath       string
	Decisions          []feedback.Decision
}

type Event struct {
	State     string
	Message   string
	Processed int
	Completed int
	Partial   int
	Failed    int
}

type Controller struct {
	mu             sync.Mutex
	running        bool
	render         RenderFunc
	appendFeedback func(string, []feedback.Decision) error
	wait           sync.WaitGroup
}

func NewController(render RenderFunc) *Controller {
	return &Controller{render: render, appendFeedback: feedback.Append}
}

func (controller *Controller) Start(ctx context.Context, request StartRequest, report func(Event)) error {
	if controller.render == nil {
		return fmt.Errorf("renderizador não configurado")
	}
	if report == nil {
		report = func(Event) {}
	}
	if err := ValidateForm(request.Values); err != nil {
		return err
	}
	if !regularFile(request.Bundle.HLAEPath) || !regularFile(request.Bundle.HookDLLPath) {
		return fmt.Errorf("%s", MissingHLAEMessage)
	}

	controller.mu.Lock()
	if controller.running {
		controller.mu.Unlock()
		return ErrAlreadyRunning
	}
	controller.running = true
	controller.wait.Add(1)
	controller.mu.Unlock()

	go controller.run(ctx, request, report)
	return nil
}

func (controller *Controller) run(ctx context.Context, request StartRequest, report func(Event)) {
	defer controller.wait.Done()
	report(Event{State: StateRunning, Message: "Iniciando processamento das demos..."})
	if len(request.Decisions) > 0 && controller.appendFeedback != nil {
		feedbackPath := filepath.Join(request.Values.OutputDir, "selection-decisions.jsonl")
		if err := controller.appendFeedback(feedbackPath, request.Decisions); err != nil {
			report(Event{State: StateRunning, Message: "Aviso: não foi possível registrar o feedback local: " + err.Error()})
		}
	}
	writer := eventWriter{report: report}
	logger := slog.New(slog.NewTextHandler(writer, nil))
	results, err := controller.render(ctx, cli.Options{
		Command:            "render",
		InputDir:           request.Values.InputDir,
		OutputDir:          request.Values.OutputDir,
		HLAEPath:           request.Bundle.HLAEPath,
		CS2Path:            request.Values.CS2Path,
		HookDLLPath:        request.Bundle.HookDLLPath,
		HUDLogoPath:        request.Bundle.LogoPath,
		HUDMode:            request.HUDMode,
		HUDThemePath:       request.HUDThemePath,
		SelectedHighlights: request.SelectedHighlights,
	}, logger)

	var final Event
	switch {
	case ctx.Err() != nil:
		final = Event{State: StateCancelled, Message: "Processamento cancelado. O trabalho válido será reaproveitado na próxima execução."}
	case err != nil:
		final = Event{State: StateFailed, Message: err.Error()}
	default:
		final = summarizeResults(results)
	}

	controller.mu.Lock()
	controller.running = false
	controller.mu.Unlock()
	report(final)
}

func (controller *Controller) IsRunning() bool {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	return controller.running
}

func (controller *Controller) Wait() {
	controller.wait.Wait()
}

func summarizeResults(results []pipeline.Result) Event {
	event := Event{Processed: len(results)}
	for _, result := range results {
		if result.Err != nil {
			event.Failed++
			continue
		}
		switch result.Manifest.State {
		case model.DemoCompleted, model.DemoNoHighlights:
			event.Completed++
		case model.DemoPartial:
			event.Partial++
		default:
			event.Failed++
		}
	}

	switch {
	case event.Processed == 0:
		event.State = StateFailed
		event.Message = "Nenhuma demo foi processada."
	case event.Partial == 0 && event.Failed == 0:
		event.State = StateCompleted
		event.Message = fmt.Sprintf("Concluído: %d demo(s) processada(s).", event.Processed)
	case event.Completed+event.Partial > 0:
		event.State = StatePartial
		event.Message = fmt.Sprintf("Processamento parcial: %d concluída(s), %d parcial(is), %d com falha.", event.Completed, event.Partial, event.Failed)
	default:
		event.State = StateFailed
		event.Message = fmt.Sprintf("Falha: %d demo(s) não foram concluídas.", event.Failed)
	}
	return event
}

type eventWriter struct {
	report func(Event)
}

func (writer eventWriter) Write(data []byte) (int, error) {
	message := strings.TrimSpace(string(data))
	if message != "" {
		writer.report(Event{State: StateRunning, Message: message})
	}
	return len(data), nil
}

var _ io.Writer = eventWriter{}
