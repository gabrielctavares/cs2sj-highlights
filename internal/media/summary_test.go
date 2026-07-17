package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestSelectForSummaryPrioritizesThenRestoresChronology(t *testing.T) {
	clips := []SummaryClip{
		{ID: "early", Path: "early.mp4", StartTick: 100, Priority: 50, Duration: 100},
		{ID: "middle", Path: "middle.mp4", StartTick: 200, Priority: 100, Duration: 100},
		{ID: "late", Path: "late.mp4", StartTick: 300, Priority: 90, Duration: 70},
	}
	got := SelectForSummary(clips, 180)
	if idsSummary(got) != "middle,late" {
		t.Fatalf("unexpected summary selection: %#v", got)
	}
}

func TestSelectForSummaryKeepsOneOverBudgetClip(t *testing.T) {
	got := SelectForSummary([]SummaryClip{{ID: "ace", Path: "ace.mp4", Duration: 200, Priority: 100}}, 180)
	if idsSummary(got) != "ace" {
		t.Fatalf("unexpected selection: %#v", got)
	}
}

func TestSelectForSummaryExcludesInvalidClipsAndBreaksTiesByTick(t *testing.T) {
	clips := []SummaryClip{
		{ID: "failed", Path: "", Duration: 10, Priority: 200},
		{ID: "zero", Path: "zero.mp4", Duration: 0, Priority: 200},
		{ID: "later", Path: "later.mp4", Duration: 10, Priority: 70, StartTick: 200},
		{ID: "earlier", Path: "earlier.mp4", Duration: 10, Priority: 70, StartTick: 100},
	}
	if got := idsSummary(SelectForSummary(clips, 180)); got != "earlier,later" {
		t.Fatalf("unexpected selection %q", got)
	}
}

func TestSummaryBuilderBuildsBothFormats(t *testing.T) {
	root := t.TempDir()
	highlights := []model.Highlight{
		summaryHighlight(t, root, "a", 100, 80, model.ClipCompleted),
		summaryHighlight(t, root, "b", 200, 100, model.ClipCompleted),
		summaryHighlight(t, root, "failed", 300, 200, model.ClipFailed),
	}
	var calls [][]string
	var concatLists []string
	builder := SummaryBuilder{FFmpegPath: "ffmpeg.exe",
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, slices.Clone(args))
			if slices.Contains(args, "concat") {
				index := slices.Index(args, "-i")
				data, err := os.ReadFile(args[index+1])
				if err != nil {
					return nil, err
				}
				concatLists = append(concatLists, string(data))
			}
			return nil, os.WriteFile(args[len(args)-1], []byte("mp4"), 0o600)
		},
		Probe: func(_ context.Context, path string) (ProbeResult, error) {
			if strings.Contains(path, "9x16") {
				return ProbeResult{Duration: 4, Width: 1080, Height: 1920, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
			}
			return ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
		},
	}
	got, err := builder.Build(context.Background(), highlights, root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got.Horizontal) != "resumo-16x9.mp4" || filepath.Base(got.Vertical) != "resumo-9x16.mp4" {
		t.Fatalf("unexpected outputs: %#v", got)
	}
	for _, path := range []string{got.Horizontal, got.Vertical} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	if len(calls) != 6 || len(concatLists) != 2 {
		t.Fatalf("unexpected calls/lists: %d %d", len(calls), len(concatLists))
	}
	joined := ""
	for _, call := range calls {
		joined += strings.Join(call, " ") + "\n"
	}
	for _, value := range []string{"fade=t=in:st=0:d=0.25", "fade=t=out:st=1.750:d=0.25", "afade=t=in:st=0:d=0.25", "-c copy"} {
		if !strings.Contains(joined, value) {
			t.Errorf("missing %q in calls:\n%s", value, joined)
		}
	}
	if strings.Contains(joined, highlights[2].Outputs.Horizontal) || strings.Contains(joined, highlights[2].Outputs.Vertical) {
		t.Fatal("failed clip entered the summary")
	}
	if !strings.Contains(concatLists[0], "summary-16x9-00") || !strings.Contains(concatLists[1], "summary-9x16-00") {
		t.Fatalf("unexpected concat lists: %#v", concatLists)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".summary-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary summary files leaked: %#v", matches)
	}
}

func TestSummaryBuilderRetainsDiagnosticsOnFailure(t *testing.T) {
	root := t.TempDir()
	highlights := []model.Highlight{summaryHighlight(t, root, "a", 100, 80, model.ClipCompleted)}
	builder := SummaryBuilder{FFmpegPath: "ffmpeg", Run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("exit 1")
	}, Probe: func(context.Context, string) (ProbeResult, error) {
		return ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
	}}
	if _, err := builder.Build(context.Background(), highlights, root); err == nil {
		t.Fatal("expected error")
	}
	matches, err := filepath.Glob(filepath.Join(root, ".summary-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("diagnostic files should be retained")
	}
}

func idsSummary(clips []SummaryClip) string {
	values := make([]string, len(clips))
	for i, clip := range clips {
		values[i] = clip.ID
	}
	return strings.Join(values, ",")
}

func summaryHighlight(t *testing.T, root, id string, tick, priority int, status model.ClipStatus) model.Highlight {
	t.Helper()
	horizontal := filepath.Join(root, id+"-16x9.mp4")
	vertical := filepath.Join(root, id+"-9x16.mp4")
	for _, path := range []string{horizontal, vertical} {
		if err := os.WriteFile(path, []byte("clip"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return model.Highlight{ID: id, StartTick: tick, Priority: priority, Status: status, Outputs: model.OutputPaths{Horizontal: horizontal, Vertical: vertical}}
}
