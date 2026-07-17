package pipeline

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/render"
)

type parserFunc func(context.Context, string) (model.Timeline, error)

func (function parserFunc) Parse(ctx context.Context, path string) (model.Timeline, error) {
	return function(ctx, path)
}

type capturerFunc func(context.Context, string, render.RenderPass, float64) (map[string]render.CaptureAssets, error)

func (function capturerFunc) RunPass(ctx context.Context, demo string, pass render.RenderPass, rate float64) (map[string]render.CaptureAssets, error) {
	return function(ctx, demo, pass, rate)
}

type clipBuilderFunc func(context.Context, model.Highlight) (model.OutputPaths, error)

func (function clipBuilderFunc) Build(ctx context.Context, highlight model.Highlight) (model.OutputPaths, error) {
	return function(ctx, highlight)
}

type summaryBuilderFunc func(context.Context, []model.Highlight, string) (model.OutputPaths, error)

func (function summaryBuilderFunc) Build(ctx context.Context, highlights []model.Highlight, path string) (model.OutputPaths, error) {
	return function(ctx, highlights, path)
}

func TestDiscoverDemosCaseInsensitiveAndSorted(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"z.DEM", "a.dem", "ignore.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := DiscoverDemos(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || filepath.Base(got[0]) != "a.dem" || filepath.Base(got[1]) != "z.DEM" {
		t.Fatalf("unexpected demos: %#v", got)
	}
}

func TestProcessDirectoryContinuesAfterCorruptDemo(t *testing.T) {
	input, output := batchDirs(t, "bad.dem", "good.dem")
	pipeline := testPipeline(output)
	pipeline.Parser = parserFunc(func(_ context.Context, path string) (model.Timeline, error) {
		if strings.Contains(path, "bad") {
			return model.Timeline{}, errors.New("corrupt")
		}
		return model.Timeline{DemoPath: path, Map: "de_nuke", TickRate: 64}, nil
	})
	pipeline.Select = func(model.Timeline) []model.Highlight { return nil }
	results, err := pipeline.ProcessDirectory(context.Background(), input, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Err == nil || results[1].Manifest.State != model.DemoNoHighlights {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestRenderRetriesCaptureExactlyOnce(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	calls := 0
	pipeline.Capturer = capturerFunc(func(_ context.Context, _ string, pass render.RenderPass, _ float64) (map[string]render.CaptureAssets, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("capture failed")
		}
		return map[string]render.CaptureAssets{"clip": {VideoPath: pass.Clips[0].MasterPath}}, nil
	})
	manifest, err := pipeline.RenderDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || manifest.Highlights[0].Attempts != 2 || manifest.State != model.DemoCompleted {
		t.Fatalf("unexpected retry result: calls=%d manifest=%#v", calls, manifest)
	}
}

func TestRenderLogsCapturePassDuration(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	pipeline := testPipeline(output)
	var logs bytes.Buffer
	pipeline.Logger = slog.New(slog.NewTextHandler(&logs, nil))

	if _, err := pipeline.RenderDemo(context.Background(), filepath.Join(input, "match.dem")); err != nil {
		t.Fatal(err)
	}

	got := logs.String()
	for _, want := range []string{"capture.started", "capture.completed", "duration="} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in capture logs:\n%s", want, got)
		}
	}
}

func TestRenderSecondCaptureFailureMarksOnlyClipFailed(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	pipeline := testPipeline(output)
	calls := 0
	pipeline.Capturer = capturerFunc(func(context.Context, string, render.RenderPass, float64) (map[string]render.CaptureAssets, error) {
		calls++
		return nil, errors.New("capture failed")
	})
	manifest, err := pipeline.RenderDemo(context.Background(), filepath.Join(input, "match.dem"))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || manifest.Highlights[0].Status != model.ClipFailed || manifest.State != model.DemoFailed {
		t.Fatalf("unexpected failure result: calls=%d manifest=%#v", calls, manifest)
	}
}

func TestNoHighlightsSkipsSummary(t *testing.T) {
	input, output := batchDirs(t, "empty.dem")
	pipeline := testPipeline(output)
	pipeline.Select = func(model.Timeline) []model.Highlight { return nil }
	summaryCalls := 0
	pipeline.Summary = summaryBuilderFunc(func(context.Context, []model.Highlight, string) (model.OutputPaths, error) {
		summaryCalls++
		return model.OutputPaths{}, nil
	})
	manifest, err := pipeline.RenderDemo(context.Background(), filepath.Join(input, "empty.dem"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.State != model.DemoNoHighlights || summaryCalls != 0 {
		t.Fatalf("unexpected result: %#v calls=%d", manifest, summaryCalls)
	}
}

func TestRenderSkipsVerticalAndSummary(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	pipeline := testPipeline(output)
	summaryCalls := 0
	pipeline.Summary = summaryBuilderFunc(func(context.Context, []model.Highlight, string) (model.OutputPaths, error) {
		summaryCalls++
		return model.OutputPaths{}, nil
	})

	manifest, err := pipeline.RenderDemo(context.Background(), filepath.Join(input, "match.dem"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.State != model.DemoCompleted || summaryCalls != 0 {
		t.Fatalf("expected completed render without summary: state=%s summaryCalls=%d", manifest.State, summaryCalls)
	}
	if manifest.Highlights[0].Outputs.Vertical != "" || manifest.Summary != (model.OutputPaths{}) {
		t.Fatalf("vertical or summary output was planned: %#v", manifest)
	}
}

func TestRenderOnlyIncludedHighlights(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	pipeline.Select = func(model.Timeline) []model.Highlight {
		return []model.Highlight{
			{ID: "skip", Round: 1, Player: model.Player{Name: "Ana"}, Tags: []string{"3K"}, StartTick: 100, EndTick: 200, Priority: 50},
			{ID: "keep", Round: 2, Player: model.Player{Name: "Bia"}, Tags: []string{"ACE"}, StartTick: 300, EndTick: 400, Priority: 100},
		}
	}
	pipeline.IncludeHighlight = func(_ string, highlight model.Highlight) bool { return highlight.ID == "keep" }
	var captured, built []string
	pipeline.Capturer = capturerFunc(func(_ context.Context, _ string, pass render.RenderPass, _ float64) (map[string]render.CaptureAssets, error) {
		assets := make(map[string]render.CaptureAssets, len(pass.Clips))
		for _, clip := range pass.Clips {
			captured = append(captured, clip.ID)
			assets[clip.ID] = render.CaptureAssets{VideoPath: clip.MasterPath}
		}
		return assets, nil
	})
	pipeline.Clips = clipBuilderFunc(func(_ context.Context, highlight model.Highlight) (model.OutputPaths, error) {
		built = append(built, highlight.ID)
		return highlight.Outputs, nil
	})

	manifest, err := pipeline.RenderDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(captured, ",") != "keep" || strings.Join(built, ",") != "keep" {
		t.Fatalf("wrong highlights processed: captured=%v built=%v", captured, built)
	}
	if manifest.State != model.DemoCompleted || manifest.Highlights[0].Status != model.ClipPending || manifest.Highlights[1].Status != model.ClipCompleted {
		t.Fatalf("unexpected selected render result: %#v", manifest)
	}
}

func TestPartialPostProductionReusesValidatedMaster(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	captures := 0
	pipeline.Capturer = capturerFunc(func(_ context.Context, _ string, pass render.RenderPass, _ float64) (map[string]render.CaptureAssets, error) {
		captures++
		if err := os.MkdirAll(filepath.Dir(pass.Clips[0].MasterPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(pass.Clips[0].MasterPath, []byte("master"), 0o600); err != nil {
			return nil, err
		}
		return map[string]render.CaptureAssets{"clip": {VideoPath: pass.Clips[0].MasterPath}}, nil
	})
	builds := 0
	pipeline.Clips = clipBuilderFunc(func(_ context.Context, highlight model.Highlight) (model.OutputPaths, error) {
		builds++
		if builds == 1 {
			return model.OutputPaths{}, errors.New("ffmpeg failed")
		}
		return highlight.Outputs, nil
	})
	first, err := pipeline.RenderDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	if first.State != model.DemoFailed {
		t.Fatalf("unexpected first state: %#v", first)
	}
	second, err := pipeline.RenderDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	if captures != 1 || builds != 2 || second.State != model.DemoCompleted {
		t.Fatalf("master was not reused: captures=%d builds=%d manifest=%#v", captures, builds, second)
	}
}

func TestCompatibleCompletedOutputsAreSkipped(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	parses, captures := 0, 0
	pipeline.Parser = parserFunc(func(_ context.Context, path string) (model.Timeline, error) {
		parses++
		return model.Timeline{DemoPath: path, Map: "de_nuke", TickRate: 64}, nil
	})
	pipeline.Capturer = capturerFunc(func(_ context.Context, _ string, pass render.RenderPass, _ float64) (map[string]render.CaptureAssets, error) {
		captures++
		return map[string]render.CaptureAssets{"clip": {VideoPath: pass.Clips[0].MasterPath}}, nil
	})
	if _, err := pipeline.RenderDemo(context.Background(), demo); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.RenderDemo(context.Background(), demo); err != nil {
		t.Fatal(err)
	}
	if parses != 1 || captures != 1 {
		t.Fatalf("completed work repeated: parses=%d captures=%d", parses, captures)
	}
}

func TestHUDModesPlanCleanAndGameMasters(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	pipeline.HUDMode = model.HUDNone
	clean, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	if clean.Highlights[0].MasterMode != "clean" || !strings.Contains(clean.Highlights[0].MasterPath, filepath.Join("masters", "clean")) {
		t.Fatalf("unexpected clean master: %#v", clean.Highlights[0])
	}
	pipeline.HUDMode = model.HUDGame
	game, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	if game.Highlights[0].MasterMode != "game" || !strings.Contains(game.Highlights[0].MasterPath, filepath.Join("masters", "game")) {
		t.Fatalf("unexpected game master: %#v", game.Highlights[0])
	}
}

func TestNoneToCustomReusesValidCleanMasterWithZeroAttempts(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	pipeline.HUDMode = model.HUDNone
	manifest, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	master := manifest.Highlights[0].MasterPath
	if err := os.MkdirAll(filepath.Dir(master), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(master, []byte("valid master"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest.Highlights[0].Attempts = 0
	manifest.Highlights[0].Status = model.ClipPending
	if err := pipeline.Store.Save(pipeline.manifestPath(demo), manifest); err != nil {
		t.Fatal(err)
	}
	pipeline.HUDMode = model.HUDCustom
	captures := 0
	pipeline.Capturer = capturerFunc(func(context.Context, string, render.RenderPass, float64) (map[string]render.CaptureAssets, error) {
		captures++
		return nil, errors.New("must not capture")
	})
	if _, err := pipeline.RenderDemo(context.Background(), demo); err != nil {
		t.Fatal(err)
	}
	if captures != 0 {
		t.Fatalf("clean master was recaptured %d time(s)", captures)
	}
}

func TestFullClipV1ReusesMasterAndRebuildsOnlyOutputForAudioSync(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	pipeline.HUDMode = model.HUDCustom

	manifest, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	highlight := &manifest.Highlights[0]
	if err := os.MkdirAll(filepath.Dir(highlight.MasterPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(highlight.MasterPath, []byte("valid master"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(highlight.Outputs.Horizontal), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(highlight.Outputs.Horizontal, []byte("old paced output"), 0o600); err != nil {
		t.Fatal(err)
	}
	highlight.Status = model.ClipCompleted
	highlight.OutputVersion = "full-clip-v1"
	master := highlight.MasterPath
	if err := pipeline.Store.Save(pipeline.manifestPath(demo), manifest); err != nil {
		t.Fatal(err)
	}

	got, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	updated := got.Highlights[0]
	if updated.MasterPath != master || updated.Status != model.ClipCaptured || updated.OutputVersion != currentOutputVersion {
		t.Fatalf("final output was not invalidated while preserving master: %#v", updated)
	}
}

func TestOutdatedCleanCaptureIsNotReusedAfterHUDFix(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	pipeline.HUDMode = model.HUDCustom
	manifest, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(filepath.Dir(pipeline.manifestPath(demo)), "masters", "clean", manifest.Highlights[0].ID, "video.mp4")
	manifest.Highlights[0].MasterPath = oldPath
	manifest.Highlights[0].MasterVersion = "before-hud-postload-fix"
	manifest.Highlights[0].Status = model.ClipCompleted
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("contaminated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.Store.Save(pipeline.manifestPath(demo), manifest); err != nil {
		t.Fatal(err)
	}

	got, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Highlights[0].MasterVersion != currentMasterVersion || got.Highlights[0].MasterPath == oldPath || got.Highlights[0].Status != model.ClipPending {
		t.Fatalf("outdated clean master was reused: %#v", got.Highlights[0])
	}
}

func TestCaptureV4MasterIsInvalidatedAfterCrosshairFix(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	pipeline.HUDMode = model.HUDCustom
	manifest, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	highlight := &manifest.Highlights[0]
	oldPath := filepath.Join(filepath.Dir(pipeline.manifestPath(demo)), "masters", "clean", "capture-v4-early-slowdown", highlight.ID, "take0000", "video.mp4")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("late-start master"), 0o600); err != nil {
		t.Fatal(err)
	}
	highlight.MasterVersion = "capture-v4-early-slowdown"
	highlight.MasterPath = oldPath
	highlight.Status = model.ClipCompleted
	if err := pipeline.Store.Save(pipeline.manifestPath(demo), manifest); err != nil {
		t.Fatal(err)
	}

	got, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	updated := got.Highlights[0]
	if updated.MasterVersion == "capture-v4-early-slowdown" || updated.MasterPath == oldPath || updated.Status != model.ClipPending {
		t.Fatalf("crosshair-less v4 master was reused: %#v", updated)
	}
}

func TestDemoV4MigratesRedundantFourKTag(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	pipeline.Select = func(model.Timeline) []model.Highlight {
		return []model.Highlight{{ID: "four-k", Round: 1, Player: model.Player{SteamID: 1, Name: "Ana", Team: model.TeamT}, Tags: []string{"4K"}, StartTick: 100, EndTick: 200, Priority: 80}}
	}
	manifest, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	manifest.DemoMetadata = "demo-v4"
	manifest.Highlights[0].Tags = []string{"4K", "3K"}
	if err := pipeline.Store.Save(pipeline.manifestPath(demo), manifest); err != nil {
		t.Fatal(err)
	}

	got, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Highlights[0].Tags, []string{"4K"}) || got.DemoMetadata == "demo-v4" {
		t.Fatalf("redundant tag was not migrated: %#v", got.Highlights[0].Tags)
	}
}

func TestChangedDemoHashCausesFreshAnalysis(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	parses := 0
	pipeline.Parser = parserFunc(func(_ context.Context, path string) (model.Timeline, error) {
		parses++
		return model.Timeline{DemoPath: path, Map: "de_nuke", TickRate: 64}, nil
	})
	if _, err := pipeline.AnalyzeDemo(context.Background(), demo); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(demo, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.AnalyzeDemo(context.Background(), demo); err != nil {
		t.Fatal(err)
	}
	if parses != 2 {
		t.Fatalf("expected fresh analysis, parses=%d", parses)
	}
}

func TestMatchHUDMetadataFromDemoPath(t *testing.T) {
	demo := filepath.Join(`G:\campeonatos`, "1º CAMP MONTADO CS2 SJ", "2_1_ONU_vs_G3neration_Z_de_dust2_2026-07-12.dem")
	got := matchHUDMetadata(demo, "de_dust2", "ONU da demo", "G3 da demo")
	if got.Event != "1º CAMP MONTADO CS2 SJ" || got.TeamA != "ONU da demo" || got.TeamB != "G3 da demo" || got.Map != "DUST2" {
		t.Fatalf("unexpected HUD metadata: %#v", got)
	}
}

func TestMatchHUDMetadataFallsBackToDemoFileName(t *testing.T) {
	demo := filepath.Join(`G:\campeonatos`, "evento", "2_1_ONU_vs_G3neration_Z_de_dust2_2026-07-12.dem")
	got := matchHUDMetadata(demo, "de_dust2", "", "")
	if got.TeamA != "ONU" || got.TeamB != "G3neration Z" {
		t.Fatalf("unexpected fallback HUD metadata: %#v", got)
	}
}

func TestLegacyDemoMetadataAddsPlayerTeamWithoutDiscardingMaster(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	pipeline.HUDMode = model.HUDGame
	parses := 0
	pipeline.Parser = parserFunc(func(_ context.Context, path string) (model.Timeline, error) {
		parses++
		player := model.Player{SteamID: 1, Name: "Ana", Team: model.TeamT, TeamName: "ONU"}
		return model.Timeline{DemoPath: path, Map: "de_nuke", TeamA: "ONU", TeamB: "Tedesco", TickRate: 64, Rounds: []model.Round{{Number: 1, ScoreA: 3, ScoreB: 2, ScoreKnown: true, Players: []model.Player{player}}}}, nil
	})
	pipeline.Select = func(model.Timeline) []model.Highlight {
		return []model.Highlight{{ID: "clip", Round: 1, Player: model.Player{SteamID: 1, Name: "Ana", Team: model.TeamT, TeamName: "ONU"}, Tags: []string{"ACE"}, StartTick: 100, EndTick: 200, Priority: 100, ActionOffsets: []float64{1, 6}}}
	}

	legacy, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	legacy.DemoMetadata = "demo-v2"
	legacy.Highlights[0].Player.TeamName = ""
	legacy.Highlights[0].ActionOffsets = nil
	legacy.Highlights[0].HUD.ScoreKnown = false
	legacy.Highlights[0].Attempts = 1
	legacy.Highlights[0].Status = model.ClipCompleted
	master := legacy.Highlights[0].MasterPath
	if err := os.MkdirAll(filepath.Dir(master), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(master, []byte("master"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy.Highlights[0].Outputs.Horizontal), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy.Highlights[0].Outputs.Horizontal, []byte("old-color"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.Store.Save(pipeline.manifestPath(demo), legacy); err != nil {
		t.Fatal(err)
	}

	got, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	if parses != 2 || got.DemoMetadata != "demo-v5" || got.Highlights[0].Player.TeamName != "ONU" {
		t.Fatalf("metadata was not migrated: parses=%d manifest=%#v", parses, got)
	}
	if !got.Highlights[0].HUD.ScoreKnown || got.Highlights[0].HUD.ScoreA != 3 || got.Highlights[0].HUD.ScoreB != 2 || len(got.Highlights[0].ActionOffsets) != 2 {
		t.Fatalf("score/action metadata was not migrated: %#v", got.Highlights[0])
	}
	if got.Highlights[0].MasterPath != master || got.Highlights[0].Attempts != 1 {
		t.Fatalf("master progress was discarded: %#v", got.Highlights[0])
	}
	if got.Highlights[0].Status != model.ClipCaptured {
		t.Fatalf("old-color final output was not invalidated: %#v", got.Highlights[0])
	}
}

func TestMetadataMigrationReplacesFrameBasedHighlightTicks(t *testing.T) {
	input, output := batchDirs(t, "match.dem")
	demo := filepath.Join(input, "match.dem")
	pipeline := testPipeline(output)
	player := model.Player{SteamID: 7, Name: "Ana", Team: model.TeamT, TeamName: "ONU"}
	pipeline.Select = func(model.Timeline) []model.Highlight {
		return []model.Highlight{{ID: "old-frame-id", Round: 5, Player: player, Tags: []string{"4K"}, StartTick: 100, EndTick: 200}}
	}
	legacy, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	legacy.DemoMetadata = "demo-v3"
	if err := pipeline.Store.Save(pipeline.manifestPath(demo), legacy); err != nil {
		t.Fatal(err)
	}
	pipeline.Select = func(model.Timeline) []model.Highlight {
		return []model.Highlight{{ID: "server-tick-id", Round: 5, Player: player, Tags: []string{"4K"}, StartTick: 3800, EndTick: 4100, ActionOffsets: []float64{8, 10}}}
	}

	got, err := pipeline.AnalyzeDemo(context.Background(), demo)
	if err != nil {
		t.Fatal(err)
	}
	highlight := got.Highlights[0]
	if highlight.ID != "server-tick-id" || highlight.StartTick != 3800 || highlight.EndTick != 4100 || !slices.Equal(highlight.ActionOffsets, []float64{8, 10}) {
		t.Fatalf("frame-based timing survived migration: %#v", highlight)
	}
}

func testPipeline(output string) *Pipeline {
	pipeline := &Pipeline{OutputDir: output}
	pipeline.Parser = parserFunc(func(_ context.Context, path string) (model.Timeline, error) {
		return model.Timeline{DemoPath: path, Map: "de_nuke", TickRate: 64}, nil
	})
	pipeline.Select = func(model.Timeline) []model.Highlight {
		return []model.Highlight{{ID: "clip", Round: 1, Player: model.Player{SteamID: 1, Slot: 1, Name: "Ana", Team: model.TeamT}, Tags: []string{"ACE"}, StartTick: 100, EndTick: 200, Priority: 100, Status: model.ClipPending}}
	}
	pipeline.Capturer = capturerFunc(func(_ context.Context, _ string, pass render.RenderPass, _ float64) (map[string]render.CaptureAssets, error) {
		return map[string]render.CaptureAssets{"clip": {VideoPath: pass.Clips[0].MasterPath}}, nil
	})
	pipeline.Clips = clipBuilderFunc(func(_ context.Context, highlight model.Highlight) (model.OutputPaths, error) {
		return highlight.Outputs, nil
	})
	pipeline.Summary = summaryBuilderFunc(func(_ context.Context, _ []model.Highlight, path string) (model.OutputPaths, error) {
		return model.OutputPaths{Horizontal: filepath.Join(path, "resumo-16x9.mp4"), Vertical: filepath.Join(path, "resumo-9x16.mp4")}, nil
	})
	pipeline.ValidateMaster = func(context.Context, model.Highlight) error { return nil }
	pipeline.ValidateOutputs = func(context.Context, model.OutputPaths) error { return nil }
	return pipeline
}

func batchDirs(t *testing.T, demos ...string) (string, string) {
	t.Helper()
	root := t.TempDir()
	input, output := filepath.Join(root, "demos"), filepath.Join(root, "videos")
	if err := os.MkdirAll(input, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, demo := range demos {
		if err := os.WriteFile(filepath.Join(input, demo), []byte(demo), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return input, output
}
