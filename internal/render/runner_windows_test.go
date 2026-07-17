package render

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gabrielctavares/cs2sj-highlights/internal/media"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

type configGuardFunc func(context.Context, string) (func() error, error)

func (function configGuardFunc) Protect(ctx context.Context, path string) (func() error, error) {
	return function(ctx, path)
}

func TestRunnerDoesNotLaunchWhenConfigProtectionFails(t *testing.T) {
	runner, pass, _ := successfulRunner(t, true)
	runner.Guard = configGuardFunc(func(context.Context, string) (func() error, error) {
		return nil, errors.New("cannot protect")
	})
	launched := false
	runner.Run = func(context.Context, string, ...string) ([]byte, error) {
		launched = true
		return nil, nil
	}
	if _, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64); err == nil || !strings.Contains(err.Error(), "proteger") {
		t.Fatalf("unexpected error: %v", err)
	}
	if launched {
		t.Fatal("HLAE launched without config protection")
	}
}

func TestRunnerRestoresConfigAfterSuccessfulPass(t *testing.T) {
	runner, pass, _ := successfulRunner(t, true)
	restored := false
	runner.Guard = configGuardFunc(func(context.Context, string) (func() error, error) {
		return func() error { restored = true; return nil }, nil
	})
	runner.Run = func(context.Context, string, ...string) ([]byte, error) { return nil, nil }
	if _, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64); err != nil {
		t.Fatal(err)
	}
	if !restored {
		t.Fatal("config was not restored")
	}
}

func TestRunnerWritesCFGLaunchesAndRemovesOnSuccess(t *testing.T) {
	runner, pass, cfgDir := successfulRunner(t, true)
	var launchedArgs []string
	var capturedCFG string
	runner.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != runner.HLAEPath {
			t.Fatalf("unexpected command %q", name)
		}
		launchedArgs = append([]string(nil), args...)
		matches, _ := filepath.Glob(filepath.Join(cfgDir, "*.cfg"))
		if len(matches) != 2 {
			t.Fatalf("expected main and clip cfg during launch, got %#v", matches)
		}
		mainPath := filepath.Join(cfgDir, "final-1700000000000000000-pass-01.cfg")
		data, err := os.ReadFile(mainPath)
		if err != nil {
			t.Fatal(err)
		}
		capturedCFG = string(data)
		clipData, err := os.ReadFile(filepath.Join(cfgDir, "pass-01-clip-start.cfg"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(clipData), `mirv_streams record name "`+filepath.Dir(pass.Clips[0].MasterPath)+`"`) {
			t.Fatalf("unexpected clip cfg: %s", clipData)
		}
		return nil, nil
	}
	assets, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64)
	if err != nil {
		t.Fatal(err)
	}
	if assets["clip"].VideoPath != pass.Clips[0].MasterPath || assets["clip"].AudioPath != "" {
		t.Fatalf("unexpected assets: %#v", assets)
	}
	if !strings.Contains(capturedCFG, "exec cs2-highlights/pass-01-clip-start.cfg") {
		t.Fatalf("unexpected cfg: %s", capturedCFG)
	}
	matches, _ := filepath.Glob(filepath.Join(cfgDir, "*.cfg"))
	if len(matches) != 0 {
		t.Fatalf("cfg leaked after success: %#v", matches)
	}
	cfgName := filepath.ToSlash(filepath.Join("cs2-highlights", "final-1700000000000000000-pass-01.cfg"))
	wantArgs := CustomLoaderArgs(runner.HookDLL, runner.CS2Path, cfgName)
	if !reflect.DeepEqual(launchedArgs, wantArgs) {
		t.Fatalf("got %#v want %#v", launchedArgs, wantArgs)
	}
}

func TestRunnerKeepsAssetsWhenCFGCleanupFails(t *testing.T) {
	runner, pass, _ := successfulRunner(t, true)
	var logs bytes.Buffer
	runner.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	runner.Remove = func(string) error { return errors.New("access denied") }
	runner.Run = func(context.Context, string, ...string) ([]byte, error) { return nil, nil }

	assets, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64)
	if err != nil || assets["clip"].VideoPath == "" {
		t.Fatalf("capture discarded after cleanup error: assets=%#v err=%v", assets, err)
	}
	if !strings.Contains(logs.String(), "cfg.cleanup_failed") {
		t.Fatalf("missing cleanup warning: %s", logs.String())
	}
}

func TestRunnerAcceptsSeparateWAV(t *testing.T) {
	runner, pass, _ := successfulRunner(t, false)
	audioPath := filepath.Join(filepath.Dir(pass.Clips[0].MasterPath), "audio.wav")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner.Run = func(context.Context, string, ...string) ([]byte, error) { return nil, nil }
	assets, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64)
	if err != nil {
		t.Fatal(err)
	}
	if assets["clip"].AudioPath != audioPath {
		t.Fatalf("unexpected assets: %#v", assets)
	}
}

func TestRunnerDiscoversHLAETakeDirectory(t *testing.T) {
	runner, pass, _ := successfulRunner(t, true)
	if err := os.Remove(pass.Clips[0].MasterPath); err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(filepath.Dir(pass.Clips[0].MasterPath), "take0000", "video.mp4")
	if err := os.MkdirAll(filepath.Dir(actual), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actual, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	runner.Timeout = 2 * time.Second
	runner.Now = func() time.Time { return now }
	runner.Sleep = func(context.Context, time.Duration) error { now = now.Add(time.Second); return nil }
	runner.Run = func(context.Context, string, ...string) ([]byte, error) { return nil, nil }
	assets, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64)
	if err != nil {
		t.Fatal(err)
	}
	if assets["clip"].VideoPath != actual {
		t.Fatalf("got %#v, want nested HLAE take %q", assets, actual)
	}
}

func TestRunnerRetainsCFGOnLaunchFailure(t *testing.T) {
	runner, pass, cfgDir := successfulRunner(t, true)
	runner.Run = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("exit 1") }
	if _, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64); err == nil || !strings.Contains(err.Error(), "pass 1") {
		t.Fatalf("unexpected error: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(cfgDir, "*.cfg"))
	if len(matches) != 2 {
		t.Fatalf("failed cfg should be retained: %#v", matches)
	}
}

func TestRunnerTimeoutKillsOwnedCS2(t *testing.T) {
	runner, pass, _ := successfulRunner(t, true)
	if err := os.Remove(pass.Clips[0].MasterPath); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	runner.Timeout = time.Second
	runner.Now = func() time.Time { return now }
	runner.Sleep = func(context.Context, time.Duration) error { now = now.Add(time.Second); return nil }
	var killed int
	runner.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if strings.EqualFold(name, "taskkill") {
			killed++
			want := []string{"/PID", "4312", "/T", "/F"}
			if !reflect.DeepEqual(args, want) {
				t.Fatalf("unexpected taskkill args: %#v", args)
			}
		}
		return nil, nil
	}
	if _, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("unexpected error: %v", err)
	}
	if killed != 1 {
		t.Fatalf("taskkill called %d times", killed)
	}
}

func TestNewProcessPIDSelectsOnlyProcessAbsentFromBaseline(t *testing.T) {
	baseline := map[uint32]struct{}{100: {}, 200: {}}
	current := map[uint32]struct{}{100: {}, 200: {}, 4312: {}}
	pid, ok := newProcessPID(baseline, current)
	if !ok || pid != 4312 {
		t.Fatalf("got pid=%d ok=%v", pid, ok)
	}
}

func TestParseTasklistCS2PIDs(t *testing.T) {
	output := []byte("\"cs2.exe\",\"4312\",\"Console\",\"1\",\"1,000 K\"\r\n\"steam.exe\",\"99\",\"Console\",\"1\",\"2,000 K\"\r\n")
	got, err := parseTasklistCS2PIDs(output)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got[4312]; !ok || len(got) != 1 {
		t.Fatalf("unexpected pids: %#v", got)
	}
}

func TestRunnerBoundsHLAELaunchByTimeout(t *testing.T) {
	runner, pass, _ := successfulRunner(t, true)
	runner.Timeout = time.Second
	var killed int
	runner.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if strings.EqualFold(name, "taskkill") {
			killed++
			return nil, nil
		}
		if _, ok := ctx.Deadline(); !ok {
			return nil, errors.New("HLAE launch has no deadline")
		}
		return nil, context.DeadlineExceeded
	}
	_, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64)
	if !errors.Is(err, context.DeadlineExceeded) || killed != 1 {
		t.Fatalf("expected bounded launch and cleanup, got %v, killed=%d", err, killed)
	}
}

func successfulRunner(t *testing.T, embeddedAudio bool) (Runner, RenderPass, string) {
	t.Helper()
	root := t.TempDir()
	cs2 := filepath.Join(root, "game", "bin", "win64", "cs2.exe")
	if err := os.MkdirAll(filepath.Dir(cs2), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cs2, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	master := filepath.Join(root, "output", "masters", "clip", "video.mp4")
	if err := os.MkdirAll(filepath.Dir(master), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(master, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	pass := RenderPass{Index: 1, Clips: []model.Highlight{{ID: "clip", StartTick: 100, EndTick: 200, Player: model.Player{SteamID: 76561198000000007, Slot: 7, Name: "gabi"}, MasterPath: master}}}
	runner := Runner{
		HLAEPath: filepath.Join(root, "hlae.exe"), HookDLL: filepath.Join(root, "AfxHookSource2.dll"), CS2Path: cs2,
		Timeout: 5 * time.Second, Now: func() time.Time { return time.Unix(1_700_000_000, 0) },
		Sleep: func(context.Context, time.Duration) error { return nil },
		ListCS2: func() ProcessLister {
			calls := 0
			return func(context.Context) (map[uint32]struct{}, error) {
				calls++
				if calls == 1 {
					return map[uint32]struct{}{100: {}}, nil
				}
				return map[uint32]struct{}{100: {}, 4312: {}}, nil
			}
		}(),
		Probe: func(_ context.Context, path string) (media.ProbeResult, error) {
			if strings.EqualFold(filepath.Ext(path), ".wav") {
				return media.ProbeResult{Duration: 2, AudioCodec: "pcm_s16le", HasAudio: true}, nil
			}
			return media.ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264", HasAudio: embeddedAudio, AudioCodec: map[bool]string{true: "aac"}[embeddedAudio]}, nil
		},
	}
	return runner, pass, filepath.Join(root, "game", "csgo", "cfg", "cs2-highlights")
}
