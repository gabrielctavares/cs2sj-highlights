package preflight

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	input := filepath.Join(root, "demos")
	output := filepath.Join(root, "videos")
	if err := os.MkdirAll(input, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	paths := make(map[string]string)
	for _, name := range []string{"hlae.exe", "AfxHookSource2.dll", "cs2.exe", "ffmpeg.exe", "ffprobe.exe", "segoeuib.ttf"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		paths[name] = path
	}
	return Config{
		InputDir: input, OutputDir: output,
		HLAEPath: paths["hlae.exe"], HookDLLPath: paths["AfxHookSource2.dll"], CS2Path: paths["cs2.exe"],
		FFmpegPath: paths["ffmpeg.exe"], FFprobePath: paths["ffprobe.exe"], FontPath: paths["segoeuib.ttf"],
		Run: func(context.Context, string, ...string) ([]byte, error) {
			return []byte("INFO: No tasks are running"), nil
		},
		FreeBytes: func(string) (uint64, error) { return 30 << 30, nil },
	}
}

func TestCheckAcceptsCompleteHost(t *testing.T) {
	cfg := validConfig(t)
	paths, err := Check(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if paths.HLAEPath != cfg.HLAEPath || paths.CS2Path != cfg.CS2Path || paths.FFmpegPath != cfg.FFmpegPath {
		t.Fatalf("unexpected paths: %#v", paths)
	}
}

func TestCheckRejectsMissingInput(t *testing.T) {
	cfg := validConfig(t)
	cfg.InputDir = filepath.Join(t.TempDir(), "missing")
	assertCheckError(t, cfg, "pasta de demos")
}

func TestCheckRejectsMissingHLAE(t *testing.T) {
	cfg := validConfig(t)
	cfg.HLAEPath += ".missing"
	assertCheckError(t, cfg, "hlae.exe")
}

func TestCheckRejectsMissingHook(t *testing.T) {
	cfg := validConfig(t)
	cfg.HookDLLPath += ".missing"
	assertCheckError(t, cfg, "AfxHookSource2.dll")
}

func TestCheckRejectsMissingCS2(t *testing.T) {
	cfg := validConfig(t)
	cfg.CS2Path += ".missing"
	assertCheckError(t, cfg, "cs2.exe")
}

func TestCheckRejectsFFmpegProbeFailure(t *testing.T) {
	cfg := validConfig(t)
	cfg.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == cfg.FFmpegPath {
			return []byte("broken"), errors.New("exit 1")
		}
		return []byte("INFO: No tasks are running"), nil
	}
	assertCheckError(t, cfg, "ffmpeg")
}

func TestCheckRejectsFFprobeProbeFailure(t *testing.T) {
	cfg := validConfig(t)
	cfg.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == cfg.FFprobePath {
			return []byte("broken"), errors.New("exit 1")
		}
		return []byte("INFO: No tasks are running"), nil
	}
	assertCheckError(t, cfg, "ffprobe")
}

func TestCheckRejectsRunningCS2(t *testing.T) {
	cfg := validConfig(t)
	cfg.Run = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("cs2.exe 1234 Console 1 1,000 K"), nil
	}
	cfg.FreeBytes = func(string) (uint64, error) { return 30 << 30, nil }
	assertCheckError(t, cfg, "feche o CS2")
}

func TestCheckRejectsLowDiskSpace(t *testing.T) {
	cfg := validConfig(t)
	cfg.FreeBytes = func(string) (uint64, error) { return 19 << 30, nil }
	assertCheckError(t, cfg, "espaço livre")
}

func TestFileIdentityFallsBackToHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.exe")
	if err := os.WriteFile(path, []byte("empty"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := FileIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "sha256:") || len(got) != len("sha256:")+12 {
		t.Fatalf("unexpected identity %q", got)
	}
}

func assertCheckError(t *testing.T, cfg Config, contains string) {
	t.Helper()
	_, err := Check(context.Background(), cfg)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(contains)) {
		t.Fatalf("expected error containing %q, got %v", contains, err)
	}
}
