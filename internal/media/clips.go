package media

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/highlights"
	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/subprocess"
)

type ClipBuilder struct {
	FFmpegPath string
	FontPath   string
	LogoPath   string
	Theme      *hudtheme.Theme
	ThemeDir   string
	Run        CommandRunner
	Probe      func(context.Context, string) (ProbeResult, error)
	HUDMode    model.HUDMode
	Logger     *slog.Logger
}

func TitleText(highlight model.Highlight) string {
	return highlight.Player.Name + " — " + highlights.PrimaryTagLabel(highlight.Tags)
}

type hudText struct {
	Event     string
	TeamA     string
	TeamB     string
	ScoreA    string
	ScoreB    string
	Map       string
	Round     string
	Player    string
	Highlight string
}

func tournamentHUDText(highlight model.Highlight) hudText {
	event := highlight.HUD.Event
	if event == "" {
		event = "CS2 SJ"
	}
	teamA, teamB := highlight.HUD.TeamA, highlight.HUD.TeamB
	if teamA == "" {
		teamA = "TIME A"
	}
	if teamB == "" {
		teamB = "TIME B"
	}
	mapName := highlight.HUD.Map
	if mapName == "" {
		mapName = "MAPA"
	}
	tag := highlights.PrimaryTagLabel(highlight.Tags)
	scoreA, scoreB := "–", "–"
	if highlight.HUD.ScoreKnown {
		scoreA = strconv.Itoa(highlight.HUD.ScoreA)
		scoreB = strconv.Itoa(highlight.HUD.ScoreB)
	}
	return hudText{
		Event: limitText(event, 48), TeamA: limitText(teamA, 24), TeamB: limitText(teamB, 24),
		ScoreA: scoreA, ScoreB: scoreB, Map: limitText(mapName, 18), Round: fmt.Sprintf("ROUND %d", highlight.Round),
		Player: limitText(highlight.Player.Name, 28), Highlight: limitText(tag, 36),
	}
}

func mapFontSize(name string) int {
	length := len([]rune(strings.TrimSpace(name)))
	if length > 11 {
		return 16
	}
	if length > 7 {
		return 19
	}
	return 22
}

func limitText(value string, maximum int) string {
	characters := []rune(strings.TrimSpace(value))
	if len(characters) <= maximum {
		return string(characters)
	}
	return string(characters[:maximum-1]) + "…"
}

func playerHUDColor(player model.Player, hud model.HUDMetadata) string {
	teamName := strings.TrimSpace(player.TeamName)
	if teamName != "" && strings.EqualFold(teamName, strings.TrimSpace(hud.TeamA)) {
		return "0x0b4f71"
	}
	return "0xc66a14"
}

func (builder ClipBuilder) Build(ctx context.Context, highlight model.Highlight) (outputs model.OutputPaths, err error) {
	if highlight.MasterPath == "" {
		return model.OutputPaths{}, fmt.Errorf("highlight %q has no master video", highlight.ID)
	}
	if highlight.Outputs.Horizontal == "" {
		return model.OutputPaths{}, fmt.Errorf("highlight %q has no horizontal output path", highlight.ID)
	}
	if builder.Probe == nil {
		return model.OutputPaths{}, fmt.Errorf("media probe is not configured")
	}
	if builder.Run == nil {
		builder.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return subprocess.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}
	customHUD := builder.HUDMode == model.HUDCustom
	if highlight.MasterAudioPath != "" {
		master, normalizeErr := builder.normalizeMaster(ctx, highlight)
		if normalizeErr != nil {
			return model.OutputPaths{}, normalizeErr
		}
		highlight.MasterPath = master
		highlight.MasterAudioPath = ""
	}
	videoSource := "[0:v]"
	audioMap := "0:a:0"
	filterPrefix := ""
	if len(highlight.ActionOffsets) > 0 {
		masterProbe, probeErr := builder.Probe(ctx, highlight.MasterPath)
		if probeErr != nil {
			return model.OutputPaths{}, fmt.Errorf("probe master for pacing %q: %w", highlight.ID, probeErr)
		}
		segments := PlanPacing(masterProbe.Duration, highlight.ActionOffsets)
		plannedDuration, accelerated, cuts := pacingStats(masterProbe.Duration, segments)
		if accelerated > 0 || cuts > 0 {
			filterPrefix = pacingFilter(segments, "0:a")
			videoSource = "[paced]"
			audioMap = "[a]"
			if builder.Logger != nil {
				builder.Logger.Info("ritmo inteligente aplicado", "highlight", highlight.ID, "duracao_original", masterProbe.Duration, "duracao_planejada", plannedDuration, "trechos_acelerados", accelerated, "cortes_de_cauda", cuts)
			}
		}
	}
	useLogo := customHUD && builder.Theme == nil && strings.TrimSpace(builder.LogoPath) != ""
	if useLogo {
		info, statErr := os.Stat(builder.LogoPath)
		if statErr != nil || !info.Mode().IsRegular() {
			return model.OutputPaths{}, fmt.Errorf("HUD logo is not a regular file: %q", builder.LogoPath)
		}
	}
	directory := filepath.Dir(highlight.Outputs.Horizontal)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return model.OutputPaths{}, fmt.Errorf("create clip output directory %q: %w", directory, err)
	}
	filterPath := filepath.Join(filepath.Dir(highlight.Outputs.Horizontal), highlight.ID+"-horizontal.ffscript")
	filter := filterPrefix + videoSource + "scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2[v]"
	textPaths := map[string]string{}
	if customHUD {
		labels := tournamentHUDText(highlight)
		playerColor := playerHUDColor(highlight.Player, highlight.HUD)
		textPaths = map[string]string{
			"event": filepath.Join(directory, highlight.ID+"-event.txt"), "team-a": filepath.Join(directory, highlight.ID+"-team-a.txt"),
			"team-b": filepath.Join(directory, highlight.ID+"-team-b.txt"), "score-a": filepath.Join(directory, highlight.ID+"-score-a.txt"),
			"score-b": filepath.Join(directory, highlight.ID+"-score-b.txt"), "map": filepath.Join(directory, highlight.ID+"-map.txt"),
			"round": filepath.Join(directory, highlight.ID+"-round.txt"), "player": filepath.Join(directory, highlight.ID+"-player.txt"),
			"highlight": filepath.Join(directory, highlight.ID+"-highlight.txt"),
		}
		textValues := map[string]string{
			"event": labels.Event, "team-a": labels.TeamA, "team-b": labels.TeamB, "score-a": labels.ScoreA,
			"score-b": labels.ScoreB, "map": labels.Map, "round": labels.Round, "player": labels.Player, "highlight": labels.Highlight,
		}
		for name, path := range textPaths {
			if err := os.WriteFile(path, []byte(textValues[name]), 0o600); err != nil {
				return model.OutputPaths{}, fmt.Errorf("write HUD text file %q: %w", path, err)
			}
		}
		filter = filterPrefix + fmt.Sprintf(
			videoSource+"scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,"+
				"drawbox=x=430:y=12:w=1060:h=30:color=0x0d1219@0.94:t=fill,"+
				"drawbox=x=350:y=42:w=1220:h=78:color=0x111923@0.94:t=fill,"+
				"drawbox=x=350:y=42:w=440:h=78:color=0x0b4f71@0.96:t=fill,"+
				"drawbox=x=1130:y=42:w=440:h=78:color=0xc66a14@0.96:t=fill,"+
				"drawbox=x=790:y=42:w=340:h=78:color=0x090d12@0.98:t=fill,"+
				"drawbox=x=710:y=970:w=500:h=74:color=0x0d1219@0.92:t=fill,"+
				"drawbox=x=710:y=970:w=8:h=74:color="+playerColor+"@1.0:t=fill,"+
				"drawtext=fontfile='%s':textfile='%s':fontcolor=white:fontsize=18:x=(w-text_w)/2:y=17,"+
				"drawtext=fontfile='%s':textfile='%s':fontcolor=white:fontsize=31:x=380:y=66,"+
				"drawtext=fontfile='%s':textfile='%s':fontcolor=white:fontsize=31:x=1540-text_w:y=66,"+
				"drawtext=fontfile='%s':textfile='%s':fontcolor=white:fontsize=36:x=750-text_w:y=60,"+
				"drawtext=fontfile='%s':textfile='%s':fontcolor=white:fontsize=36:x=1145:y=60,"+
				"drawtext=fontfile='%s':textfile='%s':fontcolor=white:fontsize=%d:x=960-text_w/2:y=54,"+
				"drawtext=fontfile='%s':textfile='%s':fontcolor=white:fontsize=16:x=960-text_w/2:y=88,"+
				"drawtext=fontfile='%s':textfile='%s':fontcolor=white:fontsize=26:x=(w-text_w)/2:y=976,"+
				"drawtext=fontfile='%s':textfile='%s':fontcolor="+playerColor+":fontsize=18:x=(w-text_w)/2:y=1012[hud]",
			escapeFilterPath(builder.FontPath), escapeFilterPath(textPaths["event"]),
			escapeFilterPath(builder.FontPath), escapeFilterPath(textPaths["team-a"]),
			escapeFilterPath(builder.FontPath), escapeFilterPath(textPaths["team-b"]),
			escapeFilterPath(builder.FontPath), escapeFilterPath(textPaths["score-a"]),
			escapeFilterPath(builder.FontPath), escapeFilterPath(textPaths["score-b"]),
			escapeFilterPath(builder.FontPath), escapeFilterPath(textPaths["map"]), mapFontSize(labels.Map),
			escapeFilterPath(builder.FontPath), escapeFilterPath(textPaths["round"]),
			escapeFilterPath(builder.FontPath), escapeFilterPath(textPaths["player"]),
			escapeFilterPath(builder.FontPath), escapeFilterPath(textPaths["highlight"]),
		)
		if useLogo {
			logoInput := 1
			filter += fmt.Sprintf(";[%d:v]scale=60:60[logo];[hud][logo]overlay=x=805:y=51:format=auto[v]", logoInput)
		} else {
			filter += ";[hud]null[v]"
		}
	}
	var themeInputs []string
	if customHUD && builder.Theme != nil {
		if strings.TrimSpace(builder.ThemeDir) == "" {
			return model.OutputPaths{}, fmt.Errorf("HUD theme directory is not configured")
		}
		plan, themeErr := BuildHUDFilter(*builder.Theme, builder.ThemeDir, hudtheme.ValuesFor(highlight), builder.FontPath)
		if themeErr != nil {
			return model.OutputPaths{}, fmt.Errorf("build HUD theme %q: %w", builder.Theme.Name, themeErr)
		}
		filter = filterPrefix + strings.Replace(plan.Filter, "[0:v]", videoSource, 1)
		themeInputs = plan.Inputs
	}
	if err := os.WriteFile(filterPath, []byte(filter), 0o600); err != nil {
		return model.OutputPaths{}, fmt.Errorf("write filter file %q: %w", filterPath, err)
	}
	completed := false
	defer func() {
		if completed {
			for _, path := range textPaths {
				if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) && err == nil {
					err = fmt.Errorf("remove HUD text file %q: %w", path, removeErr)
				}
			}
			if removeErr := os.Remove(filterPath); removeErr != nil && !os.IsNotExist(removeErr) && err == nil {
				err = fmt.Errorf("remove filter file %q: %w", filterPath, removeErr)
			}
		}
	}()

	horizontalPartial := partialPath(highlight.Outputs.Horizontal)
	horizontalArgs := []string{"-y", "-i", highlight.MasterPath}
	for _, input := range themeInputs {
		horizontalArgs = append(horizontalArgs, "-i", input)
	}
	if useLogo {
		horizontalArgs = append(horizontalArgs, "-i", builder.LogoPath)
	}
	horizontalArgs = append(horizontalArgs,
		"-/filter_complex", filterPath,
		"-map", "[v]", "-map", audioMap,
	)
	horizontalArgs = append(horizontalArgs, encodeArgs()...)
	horizontalArgs = append(horizontalArgs, horizontalPartial)
	if _, err := builder.Run(ctx, builder.FFmpegPath, horizontalArgs...); err != nil {
		return model.OutputPaths{}, fmt.Errorf("build horizontal clip %q (diagnostics retained): %w", highlight.ID, err)
	}
	probe, err := builder.Probe(ctx, horizontalPartial)
	if err != nil {
		return model.OutputPaths{}, fmt.Errorf("probe horizontal clip %q: %w", highlight.ID, err)
	}
	if err := ValidateFinal(probe, 1920, 1080); err != nil {
		return model.OutputPaths{}, fmt.Errorf("validate horizontal clip %q: %w", highlight.ID, err)
	}
	if err := os.Rename(horizontalPartial, highlight.Outputs.Horizontal); err != nil {
		return model.OutputPaths{}, fmt.Errorf("publish horizontal clip %q: %w", highlight.ID, err)
	}

	completed = true
	outputs = model.OutputPaths{Horizontal: highlight.Outputs.Horizontal}
	// A versão vertical está temporariamente desabilitada para reduzir a pós-produção.
	// A implementação abaixo foi mantida para reativação futura.
	// if err := builder.buildVertical(ctx, highlight.Outputs.Horizontal, highlight.Outputs.Vertical); err != nil { ... }
	return outputs, nil
}

func pacingFilter(segments []PacingSegment, audioInput string) string {
	if len(segments) == 0 {
		return ""
	}
	var filter strings.Builder
	for index, segment := range segments {
		outputDuration := (segment.End - segment.Start) / segment.Speed
		fadeOut := max(0, outputDuration-0.08)
		fmt.Fprintf(&filter, "[0:v]trim=start=%.3f:end=%.3f,setpts=(PTS-STARTPTS)/%.3f[v%d];", segment.Start, segment.End, segment.Speed, index)
		fmt.Fprintf(&filter, "[%s]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS,atempo=%.3f,afade=t=in:st=0:d=0.080,afade=t=out:st=%.3f:d=0.080[a%d];", audioInput, segment.Start, segment.End, segment.Speed, fadeOut, index)
	}
	for index := range segments {
		fmt.Fprintf(&filter, "[v%d][a%d]", index, index)
	}
	fmt.Fprintf(&filter, "concat=n=%d:v=1:a=1[paced][a];", len(segments))
	return filter.String()
}

func pacingStats(originalDuration float64, segments []PacingSegment) (plannedDuration float64, accelerated, cuts int) {
	for index, segment := range segments {
		plannedDuration += (segment.End - segment.Start) / segment.Speed
		if segment.Speed > 1 {
			accelerated++
		}
		if index > 0 && segment.Start-segments[index-1].End > 0.001 {
			cuts++
		}
	}
	if len(segments) > 0 {
		if segments[0].Start > 0.001 {
			cuts++
		}
		if originalDuration-segments[len(segments)-1].End > 0.001 {
			cuts++
		}
	}
	return plannedDuration, accelerated, cuts
}

func (builder ClipBuilder) buildVertical(ctx context.Context, horizontalPath, verticalPath string) error {
	verticalPartial := partialPath(verticalPath)
	const verticalFilter = "[0:v]crop=ih*9/16:ih:(iw-ow)/2:0,scale=1080:1920,setsar=1[v]"
	verticalArgs := []string{
		"-y", "-i", horizontalPath,
		"-filter_complex", verticalFilter,
		"-map", "[v]", "-map", "0:a:0",
	}
	verticalArgs = append(verticalArgs, encodeArgs()...)
	verticalArgs = append(verticalArgs, verticalPartial)
	if _, err := builder.Run(ctx, builder.FFmpegPath, verticalArgs...); err != nil {
		return fmt.Errorf("build vertical clip (diagnostics retained): %w", err)
	}
	probe, err := builder.Probe(ctx, verticalPartial)
	if err != nil {
		return fmt.Errorf("probe vertical clip: %w", err)
	}
	if err := ValidateFinal(probe, 1080, 1920); err != nil {
		return fmt.Errorf("validate vertical clip: %w", err)
	}
	if err := os.Rename(verticalPartial, verticalPath); err != nil {
		return fmt.Errorf("publish vertical clip: %w", err)
	}
	return nil
}

func encodeArgs() []string {
	return []string{
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart",
	}
}

func partialPath(path string) string {
	extension := filepath.Ext(path)
	return strings.TrimSuffix(path, extension) + ".partial" + extension
}

func escapeFilterPath(path string) string {
	path = filepath.ToSlash(path)
	path = strings.ReplaceAll(path, ":", `\:`)
	return strings.ReplaceAll(path, "'", `\'`)
}
