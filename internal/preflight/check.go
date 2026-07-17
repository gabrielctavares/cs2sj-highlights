package preflight

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/subprocess"
)

type CommandRunner func(context.Context, string, ...string) ([]byte, error)

type Config struct {
	InputDir     string
	OutputDir    string
	HLAEPath     string
	HookDLLPath  string
	CS2Path      string
	FFmpegPath   string
	FFprobePath  string
	FontPath     string
	MinFreeBytes uint64
	Run          CommandRunner
	FreeBytes    func(string) (uint64, error)
}

type Paths struct {
	InputDir        string
	OutputDir       string
	HLAEPath        string
	HookDLLPath     string
	CS2Path         string
	FFmpegPath      string
	FFprobePath     string
	FontPath        string
	HLAEIdentity    string
	CS2Identity     string
	FFmpegIdentity  string
	FFprobeIdentity string
}

func Check(ctx context.Context, config Config) (Paths, error) {
	if config.MinFreeBytes == 0 {
		config.MinFreeBytes = 20 << 30
	}
	if config.Run == nil {
		config.Run = runCommand
	}
	if config.FreeBytes == nil {
		config.FreeBytes = diskFreeBytes
	}
	input, err := requireDirectory("pasta de demos", config.InputDir)
	if err != nil {
		return Paths{}, err
	}
	if config.OutputDir == "" {
		return Paths{}, fmt.Errorf("pasta de saída não informada")
	}
	output, err := filepath.Abs(config.OutputDir)
	if err != nil {
		return Paths{}, fmt.Errorf("resolver pasta de saída %q: %w", config.OutputDir, err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return Paths{}, fmt.Errorf("criar pasta de saída %q: %w", output, err)
	}

	hlae, err := resolveExecutable(config.HLAEPath, "hlae.exe", nil)
	if err != nil {
		return Paths{}, fmt.Errorf("localizar hlae.exe: %w", err)
	}
	hook := config.HookDLLPath
	if hook == "" {
		hook, err = findHook(filepath.Dir(hlae))
		if err != nil {
			return Paths{}, err
		}
	}
	hook, err = requireFile("AfxHookSource2.dll", hook)
	if err != nil {
		return Paths{}, err
	}
	cs2, err := resolveExecutable(config.CS2Path, "cs2.exe", CommonCS2Paths())
	if err != nil {
		return Paths{}, fmt.Errorf("localizar cs2.exe: %w", err)
	}
	ffmpeg, err := resolveMediaTool(config.FFmpegPath, hlae, "ffmpeg.exe")
	if err != nil {
		return Paths{}, err
	}
	ffprobe, err := resolveMediaTool(config.FFprobePath, hlae, "ffprobe.exe")
	if err != nil {
		return Paths{}, err
	}
	font := config.FontPath
	if font == "" {
		font = `C:\Windows\Fonts\segoeuib.ttf`
	}
	font, err = requireFile("fonte Segoe UI Bold", font)
	if err != nil {
		return Paths{}, err
	}

	ffmpegOutput, err := config.Run(ctx, ffmpeg, "-version")
	if err != nil {
		return Paths{}, fmt.Errorf("executar ffmpeg -version: %w", err)
	}
	ffprobeOutput, err := config.Run(ctx, ffprobe, "-version")
	if err != nil {
		return Paths{}, fmt.Errorf("executar ffprobe -version: %w", err)
	}
	tasks, err := config.Run(ctx, "tasklist", "/FI", "IMAGENAME eq cs2.exe", "/NH")
	if err != nil {
		return Paths{}, fmt.Errorf("consultar processos do Windows: %w", err)
	}
	if strings.Contains(strings.ToLower(string(tasks)), "cs2.exe") {
		return Paths{}, fmt.Errorf("feche o CS2 antes de iniciar a renderização")
	}
	free, err := config.FreeBytes(output)
	if err != nil {
		return Paths{}, fmt.Errorf("consultar espaço livre em %q: %w", output, err)
	}
	if free < config.MinFreeBytes {
		return Paths{}, fmt.Errorf("espaço livre insuficiente em %q: disponível %.1f GiB, mínimo %.1f GiB", output, float64(free)/(1<<30), float64(config.MinFreeBytes)/(1<<30))
	}

	hlaeIdentity, err := FileIdentity(hlae)
	if err != nil {
		return Paths{}, fmt.Errorf("identificar HLAE: %w", err)
	}
	cs2Identity, err := FileIdentity(cs2)
	if err != nil {
		return Paths{}, fmt.Errorf("identificar CS2: %w", err)
	}
	return Paths{
		InputDir: input, OutputDir: output, HLAEPath: hlae, HookDLLPath: hook, CS2Path: cs2,
		FFmpegPath: ffmpeg, FFprobePath: ffprobe, FontPath: font,
		HLAEIdentity: hlaeIdentity, CS2Identity: cs2Identity,
		FFmpegIdentity: firstLine(ffmpegOutput), FFprobeIdentity: firstLine(ffprobeOutput),
	}, nil
}

func requireDirectory(label, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s não informada", label)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolver %s %q: %w", label, path, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("%s %q indisponível: %w", label, absolute, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s %q não é um diretório", label, absolute)
	}
	return absolute, nil
}

func requireFile(label, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s não informado", label)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolver %s %q: %w", label, path, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("%s %q indisponível: %w", label, absolute, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s %q é um diretório", label, absolute)
	}
	return absolute, nil
}

func resolveExecutable(explicit, name string, candidates []string) (string, error) {
	if explicit != "" {
		return requireFile(name, explicit)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("informe o caminho explicitamente: %w", err)
	}
	return requireFile(name, path)
}

func resolveMediaTool(explicit, hlae, name string) (string, error) {
	if explicit != "" {
		return requireFile(name, explicit)
	}
	bundled := filepath.Join(filepath.Dir(hlae), "ffmpeg", "bin", name)
	if info, err := os.Stat(bundled); err == nil && !info.IsDir() {
		return filepath.Abs(bundled)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("localizar %s: informe o caminho explicitamente: %w", name, err)
	}
	return requireFile(name, path)
}

func findHook(root string) (string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "AfxHookSource2.dll") {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("procurar AfxHookSource2.dll em %q: %w", root, err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("AfxHookSource2.dll não encontrado abaixo de %q", root)
	}
	sort.Slice(matches, func(i, j int) bool {
		depthI := strings.Count(filepath.Clean(matches[i]), string(filepath.Separator))
		depthJ := strings.Count(filepath.Clean(matches[j]), string(filepath.Separator))
		if depthI != depthJ {
			return depthI < depthJ
		}
		return strings.ToLower(matches[i]) < strings.ToLower(matches[j])
	})
	return matches[0], nil
}

func CommonCS2Paths() []string {
	const suffix = `Steam\steamapps\common\Counter-Strike Global Offensive\game\bin\win64\cs2.exe`
	return []string{
		filepath.Join(`C:\Program Files (x86)`, suffix),
		filepath.Join(`C:\Program Files`, suffix),
		filepath.Join(`D:\`, suffix),
	}
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := subprocess.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s: %w", filepath.Base(name), err)
	}
	return output, nil
}

func firstLine(output []byte) string {
	for _, line := range strings.Split(string(output), "\n") {
		if value := strings.TrimSpace(line); value != "" {
			return value
		}
	}
	return "unknown"
}
