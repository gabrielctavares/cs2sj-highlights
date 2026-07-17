package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/demos"
	"github.com/gabrielctavares/cs2sj-highlights/internal/highlights"
	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
	"github.com/gabrielctavares/cs2sj-highlights/internal/media"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/pipeline"
	"github.com/gabrielctavares/cs2sj-highlights/internal/preflight"
	"github.com/gabrielctavares/cs2sj-highlights/internal/render"
)

var ErrHelp = errors.New("help requested")

type Options struct {
	Command            string
	InputDir           string
	OutputDir          string
	HLAEPath           string
	CS2Path            string
	HookDLLPath        string
	FFmpegPath         string
	FFprobePath        string
	HUDLogoPath        string
	HUDThemePath       string
	HUDMode            model.HUDMode
	SelectedHighlights map[string][]string
}

type BatchFunc func(context.Context, Options) ([]pipeline.Result, error)

type App struct {
	Analyze BatchFunc
	Render  BatchFunc
}

func ParseArgs(args []string) (Options, error) {
	if len(args) == 0 {
		return Options{}, fmt.Errorf("comando ausente")
	}
	command := strings.ToLower(args[0])
	if command != "analyze" && command != "render" {
		return Options{}, fmt.Errorf("comando desconhecido %q", args[0])
	}
	if len(args) < 2 || strings.HasPrefix(args[1], "-") {
		return Options{}, fmt.Errorf("pasta de demos ausente")
	}
	options := Options{Command: command, InputDir: args[1]}
	hudMode := string(model.HUDNone)
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.OutputDir, "output", "", "pasta de saída")
	if command == "render" {
		flags.StringVar(&options.HLAEPath, "hlae", "", "caminho para hlae.exe")
		flags.StringVar(&options.CS2Path, "cs2", "", "caminho para cs2.exe")
		flags.StringVar(&options.HookDLLPath, "hook-dll", "", "caminho para AfxHookSource2.dll")
		flags.StringVar(&options.FFmpegPath, "ffmpeg", "", "caminho para ffmpeg.exe")
		flags.StringVar(&options.FFprobePath, "ffprobe", "", "caminho para ffprobe.exe")
		flags.StringVar(&hudMode, "hud", string(model.HUDNone), "none, game ou custom")
		flags.StringVar(&options.HUDThemePath, "hud-theme", "", "caminho para o hud.json externo")
	}
	if err := flags.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Options{}, ErrHelp
		}
		return Options{}, fmt.Errorf("argumentos inválidos: %w", err)
	}
	if flags.NArg() != 0 {
		return Options{}, fmt.Errorf("argumento inesperado %q", flags.Arg(0))
	}
	options.HUDMode = model.HUDMode(strings.ToLower(strings.TrimSpace(hudMode)))
	if options.OutputDir == "" {
		return Options{}, fmt.Errorf("--output é obrigatório")
	}
	if command == "render" && !options.HUDMode.Valid() {
		return Options{}, fmt.Errorf("--hud deve ser none, game ou custom")
	}
	if command == "render" && options.HUDThemePath != "" && options.HUDMode != model.HUDCustom {
		return Options{}, fmt.Errorf("--hud-theme requer --hud custom")
	}
	return options, nil
}

func Usage() string {
	return "Uso:\n" +
		"  cs2-highlights analyze INPUT_DIR --output OUTPUT_DIR\n" +
		"  cs2-highlights render  INPUT_DIR --output OUTPUT_DIR [--hud none|game|custom] [--hud-theme HUD_JSON] [--hlae HLAE_EXE] [--cs2 CS2_EXE] [--hook-dll HOOK_DLL] [--ffmpeg FFMPEG_EXE] [--ffprobe FFPROBE_EXE]\n"
}

func (app App) Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	options, err := ParseArgs(args)
	if err != nil {
		if errors.Is(err, ErrHelp) {
			fmt.Fprint(stdout, Usage())
			return 0
		}
		fmt.Fprintf(stderr, "erro: %v\n%s", err, Usage())
		return 1
	}
	run := app.Analyze
	if options.Command == "render" {
		run = app.Render
	}
	if run == nil {
		fmt.Fprintf(stderr, "erro: comando %s não configurado\n", options.Command)
		return 1
	}
	results, err := run(ctx, options)
	if err != nil {
		fmt.Fprintf(stderr, "erro: %v\n", err)
		return 1
	}
	failed := 0
	for _, result := range results {
		if result.Err != nil {
			failed++
			fmt.Fprintf(stdout, "%s: falhou: %v\n", filepath.Base(result.DemoPath), result.Err)
			continue
		}
		fmt.Fprintf(stdout, "%s: %s\n", filepath.Base(result.DemoPath), result.Manifest.State)
		if options.Command == "render" && (result.Manifest.State == model.DemoPartial || result.Manifest.State == model.DemoFailed) {
			failed++
		}
	}
	word := "demos"
	if len(results) == 1 {
		word = "demo"
	}
	fmt.Fprintf(stdout, "%d %s processados; %d com falha ou parcial\n", len(results), word, failed)
	if failed > 0 {
		return 2
	}
	return 0
}

func DefaultApp() App {
	return App{
		Analyze: func(ctx context.Context, options Options) ([]pipeline.Result, error) {
			return AnalyzeBatch(ctx, options, slog.Default())
		},
		Render: func(ctx context.Context, options Options) ([]pipeline.Result, error) {
			return RenderBatch(ctx, options, slog.Default())
		},
	}
}

func AnalyzeBatch(ctx context.Context, options Options, logger *slog.Logger) ([]pipeline.Result, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(options.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("criar pasta de saída: %w", err)
	}
	worker := &pipeline.Pipeline{OutputDir: options.OutputDir, Parser: demos.DemoParser{}, Logger: logger}
	return worker.ProcessDirectory(ctx, options.InputDir, false)
}

func RenderBatch(ctx context.Context, options Options, logger *slog.Logger) ([]pipeline.Result, error) {
	if logger == nil {
		logger = slog.Default()
	}
	paths, err := preflight.Check(ctx, preflight.Config{
		InputDir: options.InputDir, OutputDir: options.OutputDir,
		HLAEPath: options.HLAEPath, HookDLLPath: options.HookDLLPath, CS2Path: options.CS2Path,
		FFmpegPath: options.FFmpegPath, FFprobePath: options.FFprobePath,
	})
	if err != nil {
		return nil, err
	}
	var theme *hudtheme.Theme
	themeDir := ""
	if options.HUDThemePath != "" {
		loaded, loadErr := hudtheme.Load(options.HUDThemePath)
		if loadErr != nil {
			return nil, fmt.Errorf("carregar HUD externo: %w", loadErr)
		}
		theme = &loaded
		themeDir = filepath.Dir(options.HUDThemePath)
	}
	prober := media.Prober{Path: paths.FFprobePath}
	worker := &pipeline.Pipeline{
		OutputDir: paths.OutputDir,
		HUDMode:   options.HUDMode,
		Parser:    demos.DemoParser{},
		Capturer:  render.Runner{HLAEPath: paths.HLAEPath, HookDLL: paths.HookDLLPath, CS2Path: paths.CS2Path, Probe: prober.Probe, Guard: render.SteamConfigGuard{}, HUDMode: options.HUDMode, Logger: logger},
		Clips:     media.ClipBuilder{FFmpegPath: paths.FFmpegPath, FontPath: paths.FontPath, LogoPath: options.HUDLogoPath, Theme: theme, ThemeDir: themeDir, Probe: prober.Probe, HUDMode: options.HUDMode, Logger: logger},
		// Summary: media.SummaryBuilder{FFmpegPath: paths.FFmpegPath, Probe: prober.Probe}, // temporariamente desabilitado
		Logger: logger,
	}
	if options.SelectedHighlights != nil {
		selected := make(map[string]map[string]struct{}, len(options.SelectedHighlights))
		for demoPath, ids := range options.SelectedHighlights {
			key := strings.ToLower(filepath.Clean(demoPath))
			selected[key] = make(map[string]struct{}, len(ids))
			for _, id := range ids {
				selected[key][id] = struct{}{}
			}
		}
		worker.IncludeHighlight = func(demoPath string, highlight model.Highlight) bool {
			ids, ok := selected[strings.ToLower(filepath.Clean(demoPath))]
			if !ok {
				return false
			}
			_, ok = ids[highlight.ID]
			return ok
		}
	}
	worker.ValidateMaster = func(ctx context.Context, highlight model.Highlight) error {
		video, err := prober.Probe(ctx, highlight.MasterPath)
		if err != nil {
			return err
		}
		if err := media.ValidateMasterVideo(video); err != nil {
			return err
		}
		if video.HasAudio {
			return nil
		}
		if highlight.MasterAudioPath == "" {
			return fmt.Errorf("master has no embedded or separate audio")
		}
		audio, err := prober.Probe(ctx, highlight.MasterAudioPath)
		if err != nil {
			return err
		}
		return media.ValidateAudioAsset(audio)
	}
	worker.ValidateOutputs = func(ctx context.Context, outputs model.OutputPaths) error {
		if outputs.Horizontal == "" {
			return fmt.Errorf("horizontal output path is missing")
		}
		horizontal, err := prober.Probe(ctx, outputs.Horizontal)
		if err != nil {
			return err
		}
		if err := media.ValidateFinal(horizontal, 1920, 1080); err != nil {
			return err
		}
		return nil
	}
	results, err := worker.ProcessDirectory(ctx, paths.InputDir, true)
	if err != nil {
		return results, err
	}
	if err := writeRenderLogs(paths, results); err != nil {
		return results, err
	}
	return results, nil
}

func writeRenderLogs(paths preflight.Paths, results []pipeline.Result) error {
	for _, result := range results {
		name := strings.TrimSuffix(filepath.Base(result.DemoPath), filepath.Ext(result.DemoPath))
		directory := filepath.Join(paths.OutputDir, highlights.SafeName(name))
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
		path := filepath.Join(directory, "render.log")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open render log %q: %w", path, err)
		}
		logger := slog.New(slog.NewTextHandler(file, nil))
		logger.Info("host.detected", "hlae", paths.HLAEIdentity, "cs2", paths.CS2Identity, "ffmpeg", paths.FFmpegIdentity)
		if result.Err != nil {
			logger.Error("demo.failed", "demo", result.DemoPath, "error", result.Err)
		} else {
			logger.Info("demo.finished", "demo", result.DemoPath, "state", result.Manifest.State)
			for _, clip := range result.Manifest.Highlights {
				if clip.LastError != "" {
					logger.Error("clip.failed", "highlight", clip.ID, "error", clip.LastError)
				}
			}
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close render log %q: %w", path, err)
		}
	}
	return nil
}
