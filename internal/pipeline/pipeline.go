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
	OutputDir          string
	Parser             DemoParser
	Capturer           Capturer
	Clips              ClipBuilder
	Summary            SummaryBuilder
	Store              ManifestStore
	Select             func(model.Timeline) []model.Highlight
	Catalog            func(model.Timeline) ([]model.Highlight, []model.CandidateDiscard)
	SelectedHighlights map[string][]string
	ValidateMaster     func(context.Context, model.Highlight) error
	ValidateOutputs    func(context.Context, model.OutputPaths) error
	IncludeHighlight   func(string, model.Highlight) bool
	Logger             *slog.Logger
	HUDMode            model.HUDMode
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
	fingerprint, legacyFingerprint, err := captureFingerprints()
	if err != nil {
		return model.Manifest{}, err
	}
	manifestPath := pipeline.manifestPath(demoPath)
	existing, loadErr := pipeline.Store.Load(manifestPath)
	if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
		return model.Manifest{}, loadErr
	}
	compatibleExisting := loadErr == nil && (manifestpkg.DemoCompatible(existing, demoHash, fingerprint) || manifestpkg.DemoCompatible(existing, demoHash, legacyFingerprint))
	if compatibleExisting && manifestpkg.CatalogCurrent(existing) && existing.TickRate > 0 {
		changed := existing.ConfigFingerprint != fingerprint
		existing.ConfigFingerprint = fingerprint
		if pipeline.refreshPresentation(manifestPath, demoPath, &existing, false) {
			changed = true
		}
		if changed {
			if err := pipeline.Store.Save(manifestPath, existing); err != nil {
				return model.Manifest{}, err
			}
		}
		return existing, nil
	}

	timeline, err := pipeline.Parser.Parse(ctx, demoPath)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("parse demo %q: %w", demoPath, err)
	}
	timeline.DemoPath = demoPath
	catalog, discards := pipeline.buildCatalog(timeline)
	pipeline.prepareCatalog(manifestPath, demoPath, timeline, catalog)
	if compatibleExisting {
		catalog = reconcileCatalogMedia(catalog, existing.Highlights)
	}
	result := model.NewManifest(timeline, demoHash, fingerprint, catalog)
	result.DiscardedCandidates = discards
	if compatibleExisting {
		result.SelectedHighlightIDs = existing.SelectedHighlightIDs
		result.Summary = existing.Summary
		result.LastError = existing.LastError
	}
	pipeline.refreshPresentation(manifestPath, demoPath, &result, compatibleExisting)
	for _, discard := range discards {
		pipeline.logger().Warn("candidate.discarded", "demo", demoPath, "round", discard.Round, "player", discard.Player.SteamID, "code", discard.Code)
	}
	if err := pipeline.Store.Save(manifestPath, result); err != nil {
		return model.Manifest{}, err
	}
	return result, nil
}

func captureFingerprints() (current, legacy string, err error) {
	type captureConfig struct {
		RulesVersion string `json:"rules_version"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		FPS          int    `json:"fps"`
		Codec        string `json:"codec"`
		CRF          int    `json:"crf"`
		VerticalMode string `json:"vertical_mode"`
	}
	current, err = manifestpkg.Fingerprint(captureConfig{model.RulesVersion, 1920, 1080, 60, "h264", 18, "blurred-background"})
	if err != nil {
		return "", "", err
	}
	legacy, err = manifestpkg.Fingerprint(captureConfig{"rules-v2", 1920, 1080, 60, "h264", 18, "blurred-background"})
	return current, legacy, err
}

func (pipeline *Pipeline) buildCatalog(timeline model.Timeline) ([]model.Highlight, []model.CandidateDiscard) {
	if pipeline.Catalog != nil {
		return pipeline.Catalog(timeline)
	}
	if pipeline.Select != nil {
		return pipeline.Select(timeline), nil
	}
	return highlights.BuildCatalog(timeline, highlights.DefaultRules())
}

func (pipeline *Pipeline) prepareCatalog(manifestPath, demoPath string, timeline model.Timeline, catalog []model.Highlight) {
	hud := matchHUDMetadata(demoPath, timeline.Map, timeline.TeamA, timeline.TeamB)
	demoRoot := filepath.Dir(manifestPath)
	for index := range catalog {
		catalog[index].Status = model.ClipPending
		safePlayer := highlights.SafeName(catalog[index].Player.Name)
		primary := highlights.PrimaryTag(catalog[index].Tags)
		base := fmt.Sprintf("%02d-%s-%s", index+1, safePlayer, primary)
		catalog[index].Outputs = model.OutputPaths{
			Horizontal: filepath.Join(demoRoot, "clips", base+"-16x9.mp4"),
		}
		catalog[index].MasterMode = pipeline.HUDMode.CaptureMode()
		catalog[index].MasterVersion = model.MasterVersion
		catalog[index].MasterPath = masterPath(demoRoot, catalog[index].MasterMode, catalog[index].ID)
		catalog[index].OutputHUDMode = pipeline.HUDMode
		catalog[index].OutputVersion = model.OutputVersion
		catalog[index].HUD = hud
		for _, round := range timeline.Rounds {
			if round.Number == catalog[index].Round {
				catalog[index].HUD.ScoreA = round.ScoreA
				catalog[index].HUD.ScoreB = round.ScoreB
				catalog[index].HUD.ScoreKnown = round.ScoreKnown
				break
			}
		}
	}
}

func (pipeline *Pipeline) refreshPresentation(manifestPath, demoPath string, manifest *model.Manifest, migrated bool) bool {
	changed := false
	hud := matchHUDMetadata(demoPath, manifest.Map, manifest.TeamA, manifest.TeamB)
	for index := range manifest.Highlights {
		if pipeline.reconcileHUDMode(manifestPath, &manifest.Highlights[index]) {
			changed = true
		}
		desiredHUD := hud
		desiredHUD.ScoreA = manifest.Highlights[index].HUD.ScoreA
		desiredHUD.ScoreB = manifest.Highlights[index].HUD.ScoreB
		desiredHUD.ScoreKnown = manifest.Highlights[index].HUD.ScoreKnown
		hudChanged := manifest.Highlights[index].HUD != desiredHUD
		if hudChanged {
			manifest.Highlights[index].HUD = desiredHUD
			changed = true
		}
		if (migrated || hudChanged) && manifest.Highlights[index].Status == model.ClipCompleted {
			if fileReady(manifest.Highlights[index].MasterPath) {
				manifest.Highlights[index].Status = model.ClipCaptured
			} else {
				manifest.Highlights[index].Status = model.ClipPending
				manifest.Highlights[index].Attempts = 0
			}
			changed = true
		}
	}
	return changed
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
	manifest.SelectedHighlightIDs = pipeline.selectedIDs(demoPath, manifest.Highlights)
	if len(manifest.Highlights) == 0 {
		manifest.State = model.DemoNoHighlights
		return manifest, pipeline.Store.Save(manifestPath, manifest)
	}
	included := pipeline.includedCount(demoPath, manifest.Highlights)
	if included == 0 {
		manifest.State = model.DemoNoHighlights
		return manifest, pipeline.Store.Save(manifestPath, manifest)
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
	if pipeline.SelectedHighlights != nil {
		for _, id := range pipeline.selectedIDs(demoPath, []model.Highlight{highlight}) {
			if id == highlight.ID {
				return true
			}
		}
		return false
	}
	return pipeline.IncludeHighlight == nil || pipeline.IncludeHighlight(demoPath, highlight)
}

func (pipeline *Pipeline) selectedIDs(demoPath string, catalog []model.Highlight) []string {
	if pipeline.SelectedHighlights != nil {
		requested := make(map[string]struct{})
		key := strings.ToLower(filepath.Clean(demoPath))
		for path, ids := range pipeline.SelectedHighlights {
			if strings.ToLower(filepath.Clean(path)) != key {
				continue
			}
			for _, id := range ids {
				requested[id] = struct{}{}
			}
		}
		result := make([]string, 0, len(requested))
		for _, candidate := range catalog {
			if _, ok := requested[candidate.ID]; ok {
				result = append(result, candidate.ID)
			}
		}
		return result
	}
	if pipeline.IncludeHighlight != nil {
		result := make([]string, 0)
		for _, candidate := range catalog {
			if pipeline.IncludeHighlight(demoPath, candidate) {
				result = append(result, candidate.ID)
			}
		}
		return result
	}
	hasEvaluation := false
	for _, candidate := range catalog {
		hasEvaluation = hasEvaluation || candidate.Editorial.Score > 0 || candidate.Individual.Score > 0
	}
	if !hasEvaluation {
		result := make([]string, len(catalog))
		for index := range catalog {
			result[index] = catalog[index].ID
		}
		return result
	}
	view := highlights.EditorialView(catalog, highlights.BreadthBalanced)
	result := make([]string, len(view))
	for index := range view {
		result[index] = view[index].ID
	}
	return result
}

func reconcileCatalogMedia(current, previous []model.Highlight) []model.Highlight {
	for index := range current {
		for _, old := range previous {
			if !samePlayerWindow(current[index], old) {
				continue
			}
			current[index].MasterPath = old.MasterPath
			current[index].MasterAudioPath = old.MasterAudioPath
			current[index].MasterMode = old.MasterMode
			current[index].MasterVersion = old.MasterVersion
			current[index].OutputHUDMode = old.OutputHUDMode
			current[index].OutputVersion = old.OutputVersion
			current[index].Outputs = old.Outputs
			current[index].Status = old.Status
			current[index].Attempts = old.Attempts
			current[index].LastError = old.LastError
			break
		}
	}
	return current
}

func samePlayerWindow(a, b model.Highlight) bool {
	samePlayer := a.Player.SteamID != 0 && a.Player.SteamID == b.Player.SteamID ||
		a.Player.SteamID == 0 && b.Player.SteamID == 0 && a.Player.Slot != 0 && a.Player.Slot == b.Player.Slot
	return samePlayer && a.StartTick == b.StartTick && a.EndTick == b.EndTick
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
	if pipeline.Catalog == nil && pipeline.Select == nil {
		pipeline.Catalog = func(timeline model.Timeline) ([]model.Highlight, []model.CandidateDiscard) {
			return highlights.BuildCatalog(timeline, highlights.DefaultRules())
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
