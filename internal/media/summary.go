package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/subprocess"
)

type SummaryClip struct {
	ID        string
	Path      string
	StartTick int
	Priority  int
	Duration  float64
}

type SummaryBuilder struct {
	FFmpegPath string
	Run        CommandRunner
	Probe      func(context.Context, string) (ProbeResult, error)
}

func SelectForSummary(clips []SummaryClip, budget float64) []SummaryClip {
	valid := make([]SummaryClip, 0, len(clips))
	for _, clip := range clips {
		if clip.Path != "" && clip.Duration > 0 {
			valid = append(valid, clip)
		}
	}
	sort.SliceStable(valid, func(i, j int) bool {
		if valid[i].Priority != valid[j].Priority {
			return valid[i].Priority > valid[j].Priority
		}
		if valid[i].StartTick != valid[j].StartTick {
			return valid[i].StartTick < valid[j].StartTick
		}
		return valid[i].ID < valid[j].ID
	})
	chosen := make([]SummaryClip, 0, len(valid))
	total := 0.0
	for _, clip := range valid {
		if len(chosen) == 0 || total+clip.Duration <= budget {
			chosen = append(chosen, clip)
			total += clip.Duration
		}
	}
	sort.SliceStable(chosen, func(i, j int) bool {
		if chosen[i].StartTick != chosen[j].StartTick {
			return chosen[i].StartTick < chosen[j].StartTick
		}
		return chosen[i].ID < chosen[j].ID
	})
	return chosen
}

func (builder SummaryBuilder) Build(ctx context.Context, highlights []model.Highlight, outputDir string) (model.OutputPaths, error) {
	if builder.Probe == nil {
		return model.OutputPaths{}, fmt.Errorf("media probe is not configured")
	}
	if builder.Run == nil {
		builder.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return subprocess.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return model.OutputPaths{}, fmt.Errorf("create summary directory %q: %w", outputDir, err)
	}
	byID := make(map[string]model.Highlight)
	candidates := make([]SummaryClip, 0, len(highlights))
	for _, highlight := range highlights {
		if highlight.Status != model.ClipCompleted {
			continue
		}
		if highlight.Outputs.Horizontal == "" || highlight.Outputs.Vertical == "" {
			return model.OutputPaths{}, fmt.Errorf("completed highlight %q has incomplete output paths", highlight.ID)
		}
		probe, err := builder.Probe(ctx, highlight.Outputs.Horizontal)
		if err != nil {
			return model.OutputPaths{}, fmt.Errorf("probe highlight %q for summary: %w", highlight.ID, err)
		}
		if err := ValidateFinal(probe, 1920, 1080); err != nil {
			return model.OutputPaths{}, fmt.Errorf("validate highlight %q for summary: %w", highlight.ID, err)
		}
		byID[highlight.ID] = highlight
		candidates = append(candidates, SummaryClip{ID: highlight.ID, Path: highlight.Outputs.Horizontal, StartTick: highlight.StartTick, Priority: highlight.Priority, Duration: probe.Duration})
	}
	selected := SelectForSummary(candidates, 180)
	if len(selected) == 0 {
		return model.OutputPaths{}, fmt.Errorf("no completed clips available for summary")
	}
	horizontal := filepath.Join(outputDir, "resumo-16x9.mp4")
	if err := builder.buildFormat(ctx, selected, horizontal, "16x9", 1920, 1080); err != nil {
		return model.OutputPaths{}, err
	}
	verticalClips := make([]SummaryClip, len(selected))
	for i, clip := range selected {
		verticalClips[i] = clip
		verticalClips[i].Path = byID[clip.ID].Outputs.Vertical
	}
	vertical := filepath.Join(outputDir, "resumo-9x16.mp4")
	if err := builder.buildFormat(ctx, verticalClips, vertical, "9x16", 1080, 1920); err != nil {
		return model.OutputPaths{Horizontal: horizontal}, err
	}
	return model.OutputPaths{Horizontal: horizontal, Vertical: vertical}, nil
}

func (builder SummaryBuilder) buildFormat(ctx context.Context, clips []SummaryClip, outputPath, label string, width, height int) (err error) {
	directory := filepath.Dir(outputPath)
	transitions := make([]string, len(clips))
	for i := range clips {
		transitions[i] = filepath.Join(directory, fmt.Sprintf(".summary-%s-%02d.transition.mp4", label, i))
	}
	listPath := filepath.Join(directory, fmt.Sprintf(".summary-%s-concat.txt", label))
	var list strings.Builder
	for _, path := range transitions {
		fmt.Fprintf(&list, "file '%s'\n", escapeConcatPath(path))
	}
	if err := os.WriteFile(listPath, []byte(list.String()), 0o600); err != nil {
		return fmt.Errorf("write %s summary concat list: %w", label, err)
	}
	completed := false
	defer func() {
		if completed {
			for _, path := range append(slices.Clone(transitions), listPath) {
				if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) && err == nil {
					err = fmt.Errorf("remove summary temporary %q: %w", path, removeErr)
				}
			}
		}
	}()
	for i, clip := range clips {
		fadeOut := max(0, clip.Duration-0.25)
		videoFade := fmt.Sprintf("fade=t=in:st=0:d=0.25,fade=t=out:st=%.3f:d=0.25", fadeOut)
		audioFade := fmt.Sprintf("afade=t=in:st=0:d=0.25,afade=t=out:st=%.3f:d=0.25", fadeOut)
		args := []string{"-y", "-i", clip.Path, "-vf", videoFade, "-af", audioFade}
		args = append(args, encodeArgs()...)
		args = append(args, transitions[i])
		if _, err := builder.Run(ctx, builder.FFmpegPath, args...); err != nil {
			return fmt.Errorf("build %s summary transition %q (diagnostics retained): %w", label, clip.ID, err)
		}
	}
	partial := partialPath(outputPath)
	args := []string{"-y", "-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", "-movflags", "+faststart", partial}
	if _, err := builder.Run(ctx, builder.FFmpegPath, args...); err != nil {
		return fmt.Errorf("concatenate %s summary (diagnostics retained): %w", label, err)
	}
	probe, err := builder.Probe(ctx, partial)
	if err != nil {
		return fmt.Errorf("probe %s summary: %w", label, err)
	}
	if err := ValidateFinal(probe, width, height); err != nil {
		return fmt.Errorf("validate %s summary: %w", label, err)
	}
	if err := os.Rename(partial, outputPath); err != nil {
		return fmt.Errorf("publish %s summary: %w", label, err)
	}
	completed = true
	return nil
}

func escapeConcatPath(path string) string {
	path = filepath.ToSlash(path)
	return strings.ReplaceAll(path, "'", `'\''`)
}
