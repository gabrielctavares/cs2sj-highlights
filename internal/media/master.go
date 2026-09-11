package media

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

// normalizeMaster joins a completed HLAE take before HUD/encoding. Raw capture
// paths remain in the manifest so a failed normalization never requires recapture.
// Rebuild each time: a file left by an older take must not become a stale cache.
func (builder ClipBuilder) normalizeMaster(ctx context.Context, highlight model.Highlight) (string, error) {
	video, err := builder.Probe(ctx, highlight.MasterPath)
	if err != nil {
		return "", fmt.Errorf("probe normalization video: %w", err)
	}
	if err := ValidateMasterVideo(video); err != nil {
		return "", err
	}
	audio, err := builder.Probe(ctx, highlight.MasterAudioPath)
	if err != nil {
		return "", fmt.Errorf("probe normalization audio: %w", err)
	}
	if err := ValidateAudioAsset(audio); err != nil {
		return "", err
	}
	if math.IsNaN(video.Duration) || math.IsInf(video.Duration, 0) || math.IsNaN(audio.Duration) || math.IsInf(audio.Duration, 0) {
		return "", fmt.Errorf("invalid capture duration")
	}
	path := strings.TrimSuffix(highlight.MasterPath, filepath.Ext(highlight.MasterPath)) + "-av.mkv"
	partial := partialPath(path)
	args := []string{"-y", "-i", highlight.MasterPath}
	// Compatibility with the v1 capture path: HLAE WAV can start before video.
	// This is an assumption, not synchronization proven from container duration.
	trim := audio.Duration - video.Duration
	if trim > 0.02 {
		args = append(args, "-ss", fmt.Sprintf("%.6f", trim))
		if builder.Logger != nil {
			builder.Logger.Warn("master.audio_alignment_assumed", "highlight", highlight.ID, "trim_seconds", trim)
		}
	}
	args = append(args, "-i", highlight.MasterAudioPath,
		"-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy",
		"-af", "aresample=48000:async=1:first_pts=0,apad", "-c:a", "flac",
		"-t", fmt.Sprintf("%.6f", video.Duration), "-shortest", partial)
	if output, err := builder.Run(ctx, builder.FFmpegPath, args...); err != nil {
		return "", fmt.Errorf("normalize master %q (partial retained): %w: %s", highlight.ID, err, output)
	}
	result, err := builder.Probe(ctx, partial)
	if err != nil {
		return "", fmt.Errorf("probe normalized master: %w", err)
	}
	if err := ValidateMasterVideo(result); err != nil {
		return "", err
	}
	if !result.HasAudio || result.AudioCodec != "flac" || result.VideoCodec != video.VideoCodec || math.IsNaN(result.Duration) || math.IsInf(result.Duration, 0) || math.Abs(result.Duration-video.Duration) > 0.05 {
		return "", fmt.Errorf("normalized master has invalid streams or duration: %+v", result)
	}
	if err := os.Rename(partial, path); err != nil {
		return "", fmt.Errorf("publish normalized master: %w", err)
	}
	return path, nil
}
