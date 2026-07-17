package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gabrielctavares/cs2sj-highlights/internal/highlights"
	manifestpkg "github.com/gabrielctavares/cs2sj-highlights/internal/manifest"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/render"
)

type DemoParser interface {
	Parse(context.Context, string) (model.Timeline, error)
}

type Capturer interface {
	RunPass(context.Context, string, render.RenderPass, float64) (map[string]render.CaptureAssets, error)
}

type ClipBuilder interface {
	Build(context.Context, model.Highlight) (model.OutputPaths, error)
}

type SummaryBuilder interface {
	Build(context.Context, []model.Highlight, string) (model.OutputPaths, error)
}

type ManifestStore interface {
	Load(string) (model.Manifest, error)
	Save(string, model.Manifest) error
}

type Pipeline struct {
	OutputDir        string
	Parser           DemoParser
	Capturer         Capturer
	Clips            ClipBuilder
	Summary          SummaryBuilder
	Store            ManifestStore
	Select           func(model.Timeline) []model.Highlight
	ValidateMaster   func(context.Context, model.Highlight) error
	ValidateOutputs  func(context.Context, model.OutputPaths) error
	IncludeHighlight func(string, model.Highlight) bool
	Logger           *slog.Logger
	HUDMode          model.HUDMode
}

type Result struct {
	DemoPath string
	Manifest model.Manifest
	Err      error
}

func DiscoverDemos(inputDir string) ([]string, error) {
	absolute, err := filepath.Abs(inputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve demo directory %q: %w", inputDir, err)
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return nil, fmt.Errorf("read demo directory %q: %w", absolute, err)
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".dem") {
			paths = append(paths, filepath.Join(absolute, entry.Name()))
		}
	}
	sort.Slice(paths, func(i, j int) bool { return strings.ToLower(paths[i]) < strings.ToLower(paths[j]) })
	return paths, nil
}

func (pipeline *Pipeline) ProcessDirectory(ctx context.Context, inputDir string, renderDemos bool) ([]Result, error) {
	demos, err := DiscoverDemos(inputDir)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(demos))
	for _, demo := range demos {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		var manifest model.Manifest
		if renderDemos {
			manifest, err = pipeline.RenderDemo(ctx, demo)
		} else {
			manifest, err = pipeline.AnalyzeDemo(ctx, demo)
		}
		results = append(results, Result{DemoPath: demo, Manifest: manifest, Err: err})
		if err != nil {
			pipeline.logger().Error("demo.failed", "demo", demo, "error", err)
		}
	}
	return results, nil
}

func (pipeline *Pipeline) AnalyzeDemo(ctx context.Context, demoPath string) (model.Manifest, error) {
	pipeline.defaults()
	if pipeline.Parser == nil {
		return model.Manifest{}, fmt.Errorf("demo parser is not configured")
	}
	demoHash, err := manifestpkg.SHA256File(demoPath)
	if err != nil {
		return model.Manifest{}, err
	}
	fingerprint, err := manifestpkg.Fingerprint(struct {
		RulesVersion string `json:"rules_version"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		FPS          int    `json:"fps"`
		Codec        string `json:"codec"`
		CRF          int    `json:"crf"`
		VerticalMode string `json:"vertical_mode"`
	}{model.RulesVersion, 1920, 1080, 60, "h264", 18, "blurred-background"})
	if err != nil {
		return model.Manifest{}, err
	}
	manifestPath := pipeline.manifestPath(demoPath)
	if existing, loadErr := pipeline.Store.Load(manifestPath); loadErr == nil {
		if manifestpkg.Compatible(existing, demoHash, fingerprint) {
			changed := false
			metadataMigrated := false
			if existing.DemoMetadata != model.DemoMetadataVersion || existing.TickRate <= 0 {
				timeline, parseErr := pipeline.Parser.Parse(ctx, demoPath)
				if parseErr != nil {
					return model.Manifest{}, fmt.Errorf("parse demo team names %q: %w", demoPath, parseErr)
				}
				existing.TeamA = timeline.TeamA
				existing.TeamB = timeline.TeamB
				teamNames := timelinePlayerTeamNames(timeline)
				current := pipeline.Select(timeline)
				byID := make(map[string]model.Highlight, len(current))
				for _, highlight := range current {
					byID[highlight.ID] = highlight
				}
				for index := range existing.Highlights {
					if value := teamNames[existing.Highlights[index].Player.SteamID]; value != "" {
						existing.Highlights[index].Player.TeamName = value
					}
					value, ok := byID[existing.Highlights[index].ID]
					if !ok {
						for _, candidate := range current {
							if candidate.Round == existing.Highlights[index].Round && candidate.Player.SteamID == existing.Highlights[index].Player.SteamID {
								value, ok = candidate, true
								break
							}
						}
					}
					if ok {
						oldID := existing.Highlights[index].ID
						existing.Highlights[index].ID = value.ID
						existing.Highlights[index].StartTick = value.StartTick
						existing.Highlights[index].EndTick = value.EndTick
						existing.Highlights[index].Tags = value.Tags
						existing.Highlights[index].Priority = value.Priority
						existing.Highlights[index].ActionOffsets = value.ActionOffsets
						if oldID != value.ID {
							existing.Highlights[index].MasterVersion = ""
							existing.Highlights[index].Status = model.ClipPending
						}
					}
					for _, round := range timeline.Rounds {
						if round.Number == existing.Highlights[index].Round {
							existing.Highlights[index].HUD.ScoreA = round.ScoreA
							existing.Highlights[index].HUD.ScoreB = round.ScoreB
							existing.Highlights[index].HUD.ScoreKnown = round.ScoreKnown
							break
						}
					}
				}
				existing.TickRate = timeline.TickRate
				existing.DemoMetadata = model.DemoMetadataVersion
				changed = true
				metadataMigrated = true
			}
			hud := matchHUDMetadata(demoPath, existing.Map, existing.TeamA, existing.TeamB)
			for index := range existing.Highlights {
				if pipeline.reconcileHUDMode(manifestPath, &existing.Highlights[index]) {
					changed = true
				}
				desiredHUD := hud
				desiredHUD.ScoreA = existing.Highlights[index].HUD.ScoreA
				desiredHUD.ScoreB = existing.Highlights[index].HUD.ScoreB
				desiredHUD.ScoreKnown = existing.Highlights[index].HUD.ScoreKnown
				hudChanged := existing.Highlights[index].HUD != desiredHUD
				if hudChanged {
					existing.Highlights[index].HUD = desiredHUD
					changed = true
				}
				if (metadataMigrated || hudChanged) && existing.Highlights[index].Status == model.ClipCompleted {
					if fileReady(existing.Highlights[index].MasterPath) {
						existing.Highlights[index].Status = model.ClipCaptured
					} else {
						existing.Highlights[index].Status = model.ClipPending
						existing.Highlights[index].Attempts = 0
					}
				}
			}
			if changed {
				if err := pipeline.Store.Save(manifestPath, existing); err != nil {
					return model.Manifest{}, err
				}
			}
			return existing, nil
		}
	} else if !errors.Is(loadErr, os.ErrNotExist) {
		return model.Manifest{}, loadErr
	}
	timeline, err := pipeline.Parser.Parse(ctx, demoPath)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("parse demo %q: %w", demoPath, err)
	}
	timeline.DemoPath = demoPath
	selected := pipeline.Select(timeline)
	hud := matchHUDMetadata(demoPath, timeline.Map, timeline.TeamA, timeline.TeamB)
	demoRoot := filepath.Dir(manifestPath)
	for index := range selected {
		selected[index].Status = model.ClipPending
		safePlayer := highlights.SafeName(selected[index].Player.Name)
		primary := highlights.PrimaryTag(selected[index].Tags)
		selected[index].Outputs = model.OutputPaths{
			Horizontal: filepath.Join(demoRoot, "clips", fmt.Sprintf("%02d-%s-%s-16x9.mp4", index+1, safePlayer, primary)),
		}
		selected[index].MasterMode = pipeline.HUDMode.CaptureMode()
		selected[index].MasterVersion = model.MasterVersion
		selected[index].MasterPath = masterPath(demoRoot, selected[index].MasterMode, selected[index].ID)
		selected[index].OutputHUDMode = pipeline.HUDMode
		selected[index].OutputVersion = model.OutputVersion
		selected[index].HUD = hud
		for _, round := range timeline.Rounds {
			if round.Number == selected[index].Round {
				selected[index].HUD.ScoreA = round.ScoreA
				selected[index].HUD.ScoreB = round.ScoreB
				selected[index].HUD.ScoreKnown = round.ScoreKnown
				break
			}
		}
	}
	result := model.NewManifest(timeline, demoHash, fingerprint, selected)
	if err := pipeline.Store.Save(manifestPath, result); err != nil {
		return model.Manifest{}, err
	}
	return result, nil
}

func matchHUDMetadata(demoPath, mapName, demoTeamA, demoTeamB string) model.HUDMetadata {
	metadata := model.HUDMetadata{
		Event: displayName(filepath.Base(filepath.Dir(demoPath))),
		TeamA: "TIME A",
		TeamB: "TIME B",
		Map:   strings.ToUpper(strings.TrimPrefix(mapName, "de_")),
	}
	base := strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath))
	matchParts := strings.SplitN(base, "_vs_", 2)
	if len(matchParts) == 2 {
		left := strings.Split(matchParts[0], "_")
		if len(left) >= 3 && allDigits(left[0]) && allDigits(left[1]) {
			left = left[2:]
		}
		right := strings.SplitN(matchParts[1], "_de_", 2)[0]
		if value := displayName(strings.Join(left, "_")); value != "" {
			metadata.TeamA = value
		}
		if value := displayName(right); value != "" {
			metadata.TeamB = value
		}
	}
	if value := displayName(demoTeamA); value != "" {
		metadata.TeamA = value
	}
	if value := displayName(demoTeamB); value != "" {
		metadata.TeamB = value
	}
	if metadata.Event == "" || strings.EqualFold(metadata.Event, ".") {
		metadata.Event = "CS2 SJ"
	}
	if metadata.Map == "" {
		metadata.Map = "MAPA"
	}
	return metadata
}

func displayName(value string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(value, "_", " ")), " "))
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func timelinePlayerTeamNames(timeline model.Timeline) map[uint64]string {
	result := make(map[uint64]string)
	add := func(player model.Player) {
		if player.SteamID != 0 && strings.TrimSpace(player.TeamName) != "" {
			result[player.SteamID] = strings.TrimSpace(player.TeamName)
		}
	}
	for _, round := range timeline.Rounds {
		for _, player := range round.Players {
			add(player)
		}
		for _, kill := range round.Kills {
			add(kill.Killer)
			add(kill.Victim)
		}
	}
	return result
}

func (pipeline *Pipeline) RenderDemo(ctx context.Context, demoPath string) (model.Manifest, error) {
	pipeline.defaults()
	manifest, err := pipeline.AnalyzeDemo(ctx, demoPath)
	if err != nil {
		return model.Manifest{}, err
	}
	manifestPath := pipeline.manifestPath(demoPath)
	if len(manifest.Highlights) == 0 {
		manifest.State = model.DemoNoHighlights
		return manifest, pipeline.Store.Save(manifestPath, manifest)
	}
	included := pipeline.includedCount(demoPath, manifest.Highlights)
	if included == 0 {
		manifest.State = model.DemoNoHighlights
		return manifest, nil
	}
	if manifest.State == model.DemoCompleted && pipeline.allCompletedOutputsValid(ctx, demoPath, manifest) {
		return manifest, nil
	}
	if pipeline.Capturer == nil || pipeline.Clips == nil {
		return model.Manifest{}, fmt.Errorf("render dependencies are not configured")
	}
	manifest.State = model.DemoRendering
	if err := pipeline.Store.Save(manifestPath, manifest); err != nil {
		return model.Manifest{}, err
	}

	readyMaster := make(map[string]bool)
	toCapture := make([]model.Highlight, 0)
	for index := range manifest.Highlights {
		highlight := &manifest.Highlights[index]
		if !pipeline.includes(demoPath, *highlight) {
			continue
		}
		if highlight.Status == model.ClipCompleted && pipeline.outputsValid(ctx, highlight.Outputs) {
			continue
		}
		if pipeline.masterValid(ctx, *highlight) {
			readyMaster[highlight.ID] = true
			continue
		}
		if highlight.Attempts < 2 {
			toCapture = append(toCapture, *highlight)
		} else {
			highlight.Status = model.ClipFailed
		}
	}

	for _, pass := range render.PartitionPasses(toCapture) {
		assets, captureErr := pipeline.captureAttempt(ctx, demoPath, &manifest, manifestPath, pass)
		for _, clip := range pass.Clips {
			asset, ok := assets[clip.ID]
			if captureErr == nil && ok && asset.VideoPath != "" {
				pipeline.recordCapture(&manifest, clip.ID, asset)
				readyMaster[clip.ID] = true
				if err := pipeline.Store.Save(manifestPath, manifest); err != nil {
					return model.Manifest{}, err
				}
				continue
			}
			index := highlightIndex(manifest.Highlights, clip.ID)
			if index < 0 {
				continue
			}
			if manifest.Highlights[index].Attempts < 2 {
				retryPass := render.RenderPass{Index: pass.Index, Clips: []model.Highlight{manifest.Highlights[index]}}
				retryAssets, retryErr := pipeline.captureAttempt(ctx, demoPath, &manifest, manifestPath, retryPass)
				asset, retryOK := retryAssets[clip.ID]
				if retryErr == nil && retryOK && asset.VideoPath != "" {
					pipeline.recordCapture(&manifest, clip.ID, asset)
					readyMaster[clip.ID] = true
					if err := pipeline.Store.Save(manifestPath, manifest); err != nil {
						return model.Manifest{}, err
					}
					continue
				}
			}
			manifest.Highlights[index].Status = model.ClipFailed
			if manifest.Highlights[index].LastError == "" {
				manifest.Highlights[index].LastError = "capture failed"
			}
			if err := pipeline.Store.Save(manifestPath, manifest); err != nil {
				return model.Manifest{}, err
			}
		}
	}

	for index := range manifest.Highlights {
		highlight := &manifest.Highlights[index]
		if !pipeline.includes(demoPath, *highlight) {
			continue
		}
		if highlight.Status == model.ClipCompleted && pipeline.outputsValid(ctx, highlight.Outputs) {
			continue
		}
		if !readyMaster[highlight.ID] {
			continue
		}
		highlight.Status = model.ClipProcessing
		if err := pipeline.Store.Save(manifestPath, manifest); err != nil {
			return model.Manifest{}, err
		}
		outputs, buildErr := pipeline.Clips.Build(ctx, *highlight)
		if buildErr != nil {
			highlight.Status = model.ClipFailed
			highlight.LastError = buildErr.Error()
			pipeline.logger().Error("clip.failed", "demo", demoPath, "highlight", highlight.ID, "error", buildErr)
		} else {
			highlight.Outputs = outputs
			highlight.Status = model.ClipCompleted
			highlight.LastError = ""
		}
		if err := pipeline.Store.Save(manifestPath, manifest); err != nil {
			return model.Manifest{}, err
		}
	}

	completed := 0
	for _, highlight := range manifest.Highlights {
		if pipeline.includes(demoPath, highlight) && highlight.Status == model.ClipCompleted {
			completed++
		}
	}
	// Resumos estão temporariamente desabilitados para reduzir a pós-produção.
	// A implementação em media.SummaryBuilder foi mantida para reativação futura.
	// summary, summaryErr := pipeline.Summary.Build(ctx, manifest.Highlights, filepath.Dir(manifestPath))
	manifest.Summary = model.OutputPaths{}
	switch {
	case completed == included:
		manifest.State = model.DemoCompleted
	case completed > 0:
		manifest.State = model.DemoPartial
	default:
		manifest.State = model.DemoFailed
	}
	if err := pipeline.Store.Save(manifestPath, manifest); err != nil {
		return model.Manifest{}, err
	}
	return manifest, nil
}

func (pipeline *Pipeline) captureAttempt(ctx context.Context, demoPath string, manifest *model.Manifest, manifestPath string, pass render.RenderPass) (map[string]render.CaptureAssets, error) {
	if manifest.TickRate <= 0 {
		return nil, fmt.Errorf("manifest has invalid tick rate %.3f", manifest.TickRate)
	}
	for _, clip := range pass.Clips {
		if index := highlightIndex(manifest.Highlights, clip.ID); index >= 0 {
			manifest.Highlights[index].Attempts++
			manifest.Highlights[index].LastError = ""
		}
	}
	if err := pipeline.Store.Save(manifestPath, *manifest); err != nil {
		return nil, err
	}
	pipeline.logger().Info("capture.started", "demo", demoPath, "pass", pass.Index, "clips", len(pass.Clips))
	started := time.Now()
	assets, err := pipeline.Capturer.RunPass(ctx, demoPath, pass, manifest.TickRate)
	duration := time.Since(started)
	if err != nil {
		for _, clip := range pass.Clips {
			if index := highlightIndex(manifest.Highlights, clip.ID); index >= 0 {
				manifest.Highlights[index].LastError = err.Error()
			}
		}
		if saveErr := pipeline.Store.Save(manifestPath, *manifest); saveErr != nil {
			return nil, saveErr
		}
		pipeline.logger().Error("capture.failed", "demo", demoPath, "pass", pass.Index, "duration", duration, "error", err)
	} else {
		pipeline.logger().Info("capture.completed", "demo", demoPath, "pass", pass.Index, "clips", len(pass.Clips), "duration", duration)
	}
	return assets, err
}

func (pipeline *Pipeline) recordCapture(manifest *model.Manifest, id string, asset render.CaptureAssets) {
	if index := highlightIndex(manifest.Highlights, id); index >= 0 {
		manifest.Highlights[index].MasterPath = asset.VideoPath
		manifest.Highlights[index].MasterAudioPath = asset.AudioPath
		manifest.Highlights[index].Status = model.ClipCaptured
		manifest.Highlights[index].LastError = ""
	}
}

func (pipeline *Pipeline) allCompletedOutputsValid(ctx context.Context, demoPath string, manifest model.Manifest) bool {
	for _, highlight := range manifest.Highlights {
		if !pipeline.includes(demoPath, highlight) {
			continue
		}
		if highlight.Status != model.ClipCompleted || !pipeline.outputsValid(ctx, highlight.Outputs) {
			return false
		}
	}
	return true
}

func (pipeline *Pipeline) includes(demoPath string, highlight model.Highlight) bool {
	return pipeline.IncludeHighlight == nil || pipeline.IncludeHighlight(demoPath, highlight)
}

func (pipeline *Pipeline) includedCount(demoPath string, highlights []model.Highlight) int {
	count := 0
	for _, highlight := range highlights {
		if pipeline.includes(demoPath, highlight) {
			count++
		}
	}
	return count
}

func (pipeline *Pipeline) masterValid(ctx context.Context, highlight model.Highlight) bool {
	if !fileReady(highlight.MasterPath) || highlight.MasterAudioPath != "" && !fileReady(highlight.MasterAudioPath) {
		return false
	}
	if pipeline.ValidateMaster != nil {
		return pipeline.ValidateMaster(ctx, highlight) == nil
	}
	return fileReady(highlight.MasterPath) && (highlight.MasterAudioPath == "" || fileReady(highlight.MasterAudioPath))
}

func (pipeline *Pipeline) outputsValid(ctx context.Context, outputs model.OutputPaths) bool {
	if pipeline.ValidateOutputs != nil {
		return pipeline.ValidateOutputs(ctx, outputs) == nil
	}
	return fileReady(outputs.Horizontal)
}

func fileReady(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func highlightIndex(highlights []model.Highlight, id string) int {
	for index := range highlights {
		if highlights[index].ID == id {
			return index
		}
	}
	return -1
}

func (pipeline *Pipeline) manifestPath(demoPath string) string {
	name := strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath))
	return filepath.Join(pipeline.OutputDir, highlights.SafeName(name), "manifest.json")
}

func (pipeline *Pipeline) defaults() {
	if !pipeline.HUDMode.Valid() {
		pipeline.HUDMode = model.HUDNone
	}
	if pipeline.Store == nil {
		pipeline.Store = manifestpkg.Store{}
	}
	if pipeline.Select == nil {
		pipeline.Select = func(timeline model.Timeline) []model.Highlight {
			return highlights.Select(timeline, highlights.DefaultRules())
		}
	}
}

func (pipeline *Pipeline) reconcileHUDMode(manifestPath string, highlight *model.Highlight) bool {
	desiredMasterMode := pipeline.HUDMode.CaptureMode()
	demoRoot := filepath.Dir(manifestPath)
	changed := false
	if highlight.MasterMode == "" {
		cleanPath := strings.ToLower(filepath.ToSlash(highlight.MasterPath))
		if strings.Contains(cleanPath, "/masters/clean/") {
			highlight.MasterMode = "clean"
		} else {
			highlight.MasterMode = "game"
		}
		changed = true
	}
	if highlight.MasterMode != desiredMasterMode || highlight.MasterVersion != model.MasterVersion {
		highlight.MasterMode = desiredMasterMode
		highlight.MasterVersion = model.MasterVersion
		highlight.MasterPath = masterPath(demoRoot, desiredMasterMode, highlight.ID)
		highlight.MasterAudioPath = ""
		highlight.Attempts = 0
		if fileReady(highlight.MasterPath) {
			highlight.Status = model.ClipCaptured
		} else {
			highlight.Status = model.ClipPending
		}
		changed = true
	}
	if highlight.OutputHUDMode != pipeline.HUDMode {
		highlight.OutputHUDMode = pipeline.HUDMode
		if highlight.Status == model.ClipCompleted {
			if fileReady(highlight.MasterPath) {
				highlight.Status = model.ClipCaptured
			} else {
				highlight.Status = model.ClipPending
			}
		}
		changed = true
	}
	if highlight.OutputVersion != model.OutputVersion {
		highlight.OutputVersion = model.OutputVersion
		if highlight.Status == model.ClipCompleted {
			if fileReady(highlight.MasterPath) {
				highlight.Status = model.ClipCaptured
			} else {
				highlight.Status = model.ClipPending
				highlight.Attempts = 0
			}
		}
		changed = true
	}
	return changed
}

func masterPath(demoRoot, mode, highlightID string) string {
	return filepath.Join(demoRoot, "masters", mode, model.MasterVersion, highlightID, "video.mp4")
}

func (pipeline *Pipeline) logger() *slog.Logger {
	if pipeline.Logger == nil {
		pipeline.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return pipeline.Logger
}
