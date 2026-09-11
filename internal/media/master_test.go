package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This characterizes the legacy duration-based alignment, including its known
// ambiguity: trailing audio is mistaken for an early audio start. A passing
// trailing-surplus case documents that limitation, not correct synchronization.
func TestUnifiedMasterTimingFFmpegIntegration(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}
	for _, tc := range []struct {
		name                       string
		duration, lead, wantOffset float64
	}{
		{"equal", 2, 0, 0},
		{"leading_surplus", 2.5, .5, 0},
		{"trailing_surplus_known_limitation", 2.5, 0, -.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := clipFixture(t, true)
			run := func(args ...string) []byte {
				t.Helper()
				cmd := exec.Command(ffmpeg, args...)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				out, err := cmd.Output()
				if err != nil {
					t.Fatalf("ffmpeg: %v: %s", err, stderr.String())
				}
				return out
			}
			markers := []float64{.6, 1, 1.6}
			var audioTerms, flashes []string
			for _, at := range markers {
				audioTerms = append(audioTerms, fmt.Sprintf("between(t,%.6f,%.6f)", at+tc.lead, at+tc.lead+.03))
				flashes = append(flashes, fmt.Sprintf("between(t,%.6f,%.6f)", at, at+.03))
			}
			run("-v", "error", "-y", "-f", "lavfi", "-i", "color=c=black:s=1920x1080:d=2:r=60", "-vf", "drawbox=color=white:t=fill:enable='"+strings.Join(flashes, "+")+"'", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", h.MasterPath)
			run("-v", "error", "-y", "-f", "lavfi", "-i", fmt.Sprintf("aevalsrc=exprs='0.8*sin(2*PI*1000*t)*(%s)':s=48000:d=%.6f", strings.Join(audioTerms, "+"), tc.duration), "-c:a", "pcm_s16le", h.MasterAudioPath)
			originalVideo, err := os.ReadFile(h.MasterPath)
			if err != nil {
				t.Fatal(err)
			}
			originalAudio, err := os.ReadFile(h.MasterAudioPath)
			if err != nil {
				t.Fatal(err)
			}
			b := ClipBuilder{FFmpegPath: ffmpeg, Probe: (Prober{Path: ffprobe}).Probe}
			outputs, err := b.Build(context.Background(), h)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{filepath.Join(filepath.Dir(h.MasterPath), "master-av.mkv"), outputs.Horizontal} {
				pcm := run("-v", "error", "-i", path, "-map", "0:a:0", "-ac", "1", "-ar", "48000", "-f", "s16le", "-")
				var onsets []float64
				last := -1.0
				for i := 0; i+1 < len(pcm); i += 2 {
					at := float64(i/2) / 48000
					if math.Abs(float64(int16(binary.LittleEndian.Uint16(pcm[i:])))) > 8000 {
						if at-last > .1 {
							onsets = append(onsets, at)
						}
						last = at
					}
				}
				if len(onsets) != len(markers) {
					t.Fatalf("%s: onsets %v", path, onsets)
				}
				for i, at := range onsets {
					offset := at - markers[i]
					if math.Abs(offset-tc.wantOffset) > .01 {
						t.Fatalf("%s marker %d: offset %.6f, want %.6f", path, i, offset, tc.wantOffset)
					}
				}
				t.Logf("%s: measured audio markers %v; expected visual markers %v; offset %.3fs", filepath.Base(path), onsets, markers, tc.wantOffset)
			}
			for path, before := range map[string][]byte{h.MasterPath: originalVideo, h.MasterAudioPath: originalAudio} {
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("source changed: %s (%v)", path, err)
				}
			}
		})
	}
}

func TestNormalizeMasterRejectsInvalidOutputAndPreservesSources(t *testing.T) {
	h := clipFixture(t, true)
	b := ClipBuilder{FFmpegPath: "ffmpeg",
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			return nil, os.WriteFile(args[len(args)-1], []byte("bad"), 0600)
		},
		Probe: func(_ context.Context, path string) (ProbeResult, error) {
			if strings.HasSuffix(path, ".mkv") {
				return ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264"}, nil
			}
			return ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264", HasAudio: true, AudioCodec: "pcm_s16le"}, nil
		},
	}
	if _, err := b.Build(context.Background(), h); err == nil {
		t.Fatal("missing audio accepted")
	}
	for _, path := range []string{h.MasterPath, h.MasterAudioPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(h.Outputs.Horizontal); !os.IsNotExist(err) {
		t.Fatal("invalid master produced final output")
	}
}

func TestUnifiedMasterFFmpegIntegration(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}
	for _, duration := range []string{"2", "1.5", "2.5"} {
		t.Run(duration, func(t *testing.T) {
			h := clipFixture(t, true)
			run := func(args ...string) []byte {
				t.Helper()
				out, err := exec.Command(ffmpeg, args...).CombinedOutput()
				if err != nil {
					t.Fatalf("ffmpeg: %v: %s", err, out)
				}
				return out
			}
			run("-v", "error", "-y", "-f", "lavfi", "-i", "color=c=blue:s=1920x1080:d=2:r=60", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", h.MasterPath)
			run("-v", "error", "-y", "-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration="+duration, "-c:a", "pcm_s16le", h.MasterAudioPath)
			prober := Prober{Path: ffprobe}
			b := ClipBuilder{FFmpegPath: ffmpeg, Probe: prober.Probe}
			outputs, err := b.Build(context.Background(), h)
			if err != nil {
				t.Fatal(err)
			}
			master := filepath.Join(filepath.Dir(h.MasterPath), "master-av.mkv")
			// Stream-copy must preserve decoded frames exactly.
			originalHash := run("-v", "error", "-i", h.MasterPath, "-map", "0:v:0", "-f", "hash", "-")
			masterHash := run("-v", "error", "-i", master, "-map", "0:v:0", "-f", "hash", "-")
			if string(originalHash) != string(masterHash) {
				t.Fatalf("video changed: %s / %s", originalHash, masterHash)
			}
			result, err := prober.Probe(context.Background(), outputs.Horizontal)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateFinal(result, 1920, 1080); err != nil {
				t.Fatal(err)
			}
			if result.Duration < 1.95 || result.Duration > 2.05 {
				t.Fatalf("duration changed: %f", result.Duration)
			}
		})
	}
}
