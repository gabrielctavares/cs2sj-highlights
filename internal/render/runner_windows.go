package render

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gabrielctavares/cs2sj-highlights/internal/media"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/subprocess"
)

type CommandRunner func(context.Context, string, ...string) ([]byte, error)

type Runner struct {
	HLAEPath string
	HookDLL  string
	CS2Path  string
	Timeout  time.Duration
	Run      CommandRunner
	Probe    func(context.Context, string) (media.ProbeResult, error)
	Now      func() time.Time
	Sleep    func(context.Context, time.Duration) error
	Guard    ConfigGuard
	HUDMode  model.HUDMode
}

type CaptureAssets struct {
	VideoPath string
	AudioPath string
}

type fileObservation struct {
	size int64
	seen bool
}

func (runner Runner) RunPass(ctx context.Context, demoPath string, pass RenderPass, tickRate float64) (result map[string]CaptureAssets, err error) {
	runner.setDefaults()
	if runner.Probe == nil {
		return nil, fmt.Errorf("media probe is not configured")
	}
	if runner.Guard != nil {
		restore, protectErr := runner.Guard.Protect(ctx, runner.CS2Path)
		if protectErr != nil {
			return nil, fmt.Errorf("proteger configurações pessoais do CS2: %w", protectErr)
		}
		defer func() {
			if restoreErr := restore(); restoreErr != nil {
				result = nil
				if err != nil {
					err = errors.Join(err, fmt.Errorf("restaurar configurações pessoais do CS2: %w", restoreErr))
				} else {
					err = fmt.Errorf("restaurar configurações pessoais do CS2: %w", restoreErr)
				}
			}
		}()
	}
	configuration, err := BuildCFG(demoPath, pass, tickRate, runner.HUDMode)
	if err != nil {
		return nil, fmt.Errorf("build HLAE CFG for pass %d: %w", pass.Index, err)
	}
	gameDir := filepath.Dir(filepath.Dir(filepath.Dir(runner.CS2Path)))
	cfgDir := filepath.Join(gameDir, "csgo", "cfg", "cs2-highlights")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return nil, fmt.Errorf("create HLAE CFG directory %q: %w", cfgDir, err)
	}
	fileName := fmt.Sprintf("%s-%d-pass-%02d.cfg", safeJobName(demoPath), runner.Now().UnixNano(), pass.Index)
	cfgPath := filepath.Join(cfgDir, fileName)
	if err := os.WriteFile(cfgPath, []byte(configuration), 0o600); err != nil {
		return nil, fmt.Errorf("write HLAE CFG %q: %w", cfgPath, err)
	}
	cfgPaths := []string{cfgPath}
	for _, clip := range pass.Clips {
		clipConfiguration, err := BuildClipCFG(clip)
		if err != nil {
			return nil, fmt.Errorf("build clip CFG %q: %w", clip.ID, err)
		}
		clipCFGPath := filepath.Join(cfgDir, clipStartCFGFileName(pass.Index, clip.ID))
		if err := os.WriteFile(clipCFGPath, []byte(clipConfiguration), 0o600); err != nil {
			return nil, fmt.Errorf("write clip CFG %q: %w", clipCFGPath, err)
		}
		cfgPaths = append(cfgPaths, clipCFGPath)
	}
	cfgName := filepath.ToSlash(filepath.Join("cs2-highlights", fileName))
	launchCtx, cancelLaunch := context.WithTimeout(ctx, runner.Timeout)
	_, launchErr := runner.Run(launchCtx, runner.HLAEPath, CustomLoaderArgs(runner.HookDLL, runner.CS2Path, cfgName)...)
	cancelLaunch()
	if launchErr != nil {
		if errors.Is(launchErr, context.Canceled) || errors.Is(launchErr, context.DeadlineExceeded) {
			return nil, runner.stopForCancellation(launchErr)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, runner.stopForCancellation(ctxErr)
		}
		return nil, fmt.Errorf("launch HLAE for pass %d (CFG retained at %q): %w", pass.Index, cfgPath, launchErr)
	}

	deadline := runner.Now().Add(runner.Timeout)
	observations := make(map[string]fileObservation)
	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, runner.stopForCancellation(ctxErr)
		}
		if !runner.Now().Before(deadline) {
			killErr := runner.killCS2()
			if killErr != nil {
				return nil, fmt.Errorf("timeout waiting for HLAE pass %d; taskkill failed: %v", pass.Index, killErr)
			}
			return nil, fmt.Errorf("timeout waiting for HLAE pass %d (CFG retained at %q)", pass.Index, cfgPath)
		}
		assets, ready := runner.collectAssets(ctx, pass, observations)
		if ready {
			for _, path := range cfgPaths {
				if err := os.Remove(path); err != nil {
					return nil, fmt.Errorf("remove successful HLAE CFG %q: %w", path, err)
				}
			}
			return assets, nil
		}
		if err := runner.Sleep(ctx, 500*time.Millisecond); err != nil {
			ctxErr := ctx.Err()
			if ctxErr == nil {
				ctxErr = err
			}
			return nil, runner.stopForCancellation(ctxErr)
		}
	}
}

func (runner *Runner) setDefaults() {
	if runner.Timeout <= 0 {
		runner.Timeout = 45 * time.Minute
	}
	if runner.Now == nil {
		runner.Now = time.Now
	}
	if runner.Sleep == nil {
		runner.Sleep = sleepContext
	}
	if runner.Run == nil {
		runner.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return subprocess.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}
}

func (runner Runner) collectAssets(ctx context.Context, pass RenderPass, observations map[string]fileObservation) (map[string]CaptureAssets, bool) {
	assets := make(map[string]CaptureAssets, len(pass.Clips))
	for _, clip := range pass.Clips {
		videoPath, ready := stableCaptureVideo(clip.MasterPath, observations)
		if !ready {
			return nil, false
		}
		video, err := runner.Probe(ctx, videoPath)
		if err != nil || media.ValidateMasterVideo(video) != nil {
			return nil, false
		}
		asset := CaptureAssets{VideoPath: videoPath}
		if !video.HasAudio {
			audio, ok := stableWAV(filepath.Dir(videoPath), observations)
			if !ok {
				return nil, false
			}
			probe, err := runner.Probe(ctx, audio)
			if err != nil || media.ValidateAudioAsset(probe) != nil {
				return nil, false
			}
			asset.AudioPath = audio
		}
		assets[clip.ID] = asset
	}
	return assets, true
}

func stableCaptureVideo(masterPath string, observations map[string]fileObservation) (string, bool) {
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(masterPath), "take*", "video.mp4"))
	if len(matches) > 0 {
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		return matches[0], stableFile(matches[0], observations)
	}
	return masterPath, stableFile(masterPath, observations)
}

func stableFile(path string, observations map[string]fileObservation) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() <= 0 {
		return false
	}
	previous := observations[path]
	observations[path] = fileObservation{size: info.Size(), seen: true}
	return previous.seen && previous.size == info.Size()
}

func stableWAV(directory string, observations map[string]fileObservation) (string, bool) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", false
	}
	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".wav") {
			matches = append(matches, filepath.Join(directory, entry.Name()))
		}
	}
	if len(matches) != 1 || !stableFile(matches[0], observations) {
		return "", false
	}
	return matches[0], true
}

func (runner Runner) stopForCancellation(cause error) error {
	if killErr := runner.killCS2(); killErr != nil {
		return fmt.Errorf("%w; taskkill failed: %v", cause, killErr)
	}
	return cause
}

func (runner Runner) killCS2() error {
	_, err := runner.Run(context.Background(), "taskkill", "/IM", "cs2.exe", "/T", "/F")
	return err
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func safeJobName(demoPath string) string {
	name := strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath))
	var builder strings.Builder
	separator := false
	for _, char := range strings.ToLower(name) {
		if char < 128 && (unicode.IsLetter(char) || unicode.IsDigit(char)) {
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(char)
			separator = false
		} else {
			separator = true
		}
	}
	if builder.Len() == 0 {
		return "demo"
	}
	return builder.String()
}
