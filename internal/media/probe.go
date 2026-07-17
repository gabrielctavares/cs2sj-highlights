package media

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gabrielctavares/cs2sj-highlights/internal/subprocess"
)

type ProbeResult struct {
	Duration   float64
	Width      int
	Height     int
	VideoCodec string
	AudioCodec string
	HasAudio   bool
}

type CommandRunner func(context.Context, string, ...string) ([]byte, error)

type Prober struct {
	Path string
	Run  CommandRunner
}

func (prober Prober) Probe(ctx context.Context, path string) (ProbeResult, error) {
	run := prober.Run
	if run == nil {
		run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return subprocess.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}
	output, err := run(ctx, prober.Path,
		"-v", "error",
		"-show_entries", "format=duration:stream=codec_type,codec_name,width,height",
		"-of", "json",
		path,
	)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("ffprobe %q: %w", path, err)
	}
	result, err := ParseProbe(output)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("parse ffprobe output for %q: %w", path, err)
	}
	return result, nil
}

func ParseProbe(raw []byte) (ProbeResult, error) {
	var document struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return ProbeResult{}, fmt.Errorf("decode JSON: %w", err)
	}
	duration, err := strconv.ParseFloat(document.Format.Duration, 64)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("parse duration %q: %w", document.Format.Duration, err)
	}
	result := ProbeResult{Duration: duration}
	for _, stream := range document.Streams {
		switch stream.CodecType {
		case "video":
			if result.VideoCodec == "" {
				result.VideoCodec = stream.CodecName
				result.Width = stream.Width
				result.Height = stream.Height
			}
		case "audio":
			if !result.HasAudio {
				result.HasAudio = true
				result.AudioCodec = stream.CodecName
			}
		}
	}
	return result, nil
}

func ValidateMasterVideo(result ProbeResult) error {
	if err := validateVideoStream(result); err != nil {
		return err
	}
	if result.Width != 1920 || result.Height != 1080 {
		return fmt.Errorf("master resolution is %dx%d, want 1920x1080", result.Width, result.Height)
	}
	return nil
}

func validateVideoStream(result ProbeResult) error {
	if result.Duration < 0.5 {
		return fmt.Errorf("duration %.3fs is below 0.5s", result.Duration)
	}
	if result.VideoCodec == "" {
		return fmt.Errorf("video stream is missing")
	}
	return nil
}

func ValidateAudioAsset(result ProbeResult) error {
	if result.Duration < 0.5 {
		return fmt.Errorf("audio duration %.3fs is below 0.5s", result.Duration)
	}
	if !result.HasAudio || result.AudioCodec == "" {
		return fmt.Errorf("audio stream is missing")
	}
	return nil
}

func ValidateFinal(result ProbeResult, width, height int) error {
	if err := validateVideoStream(result); err != nil {
		return err
	}
	if result.Width != width || result.Height != height {
		return fmt.Errorf("final resolution is %dx%d, want %dx%d", result.Width, result.Height, width, height)
	}
	if err := ValidateAudioAsset(result); err != nil {
		return err
	}
	if result.VideoCodec != "h264" {
		return fmt.Errorf("final video codec is %q, want h264", result.VideoCodec)
	}
	if result.AudioCodec != "aac" {
		return fmt.Errorf("final audio codec is %q, want aac", result.AudioCodec)
	}
	return nil
}
