package media

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestParseProbeAndValidateFinal(t *testing.T) {
	raw := []byte(`{"format":{"duration":"12.50"},"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac"}]}`)
	got, err := ParseProbe(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateFinal(got, 1920, 1080); err != nil {
		t.Fatal(err)
	}
	if got.Duration != 12.5 || got.Width != 1920 || got.Height != 1080 || !got.HasAudio {
		t.Fatalf("unexpected probe: %#v", got)
	}
}

func TestValidateFinalAcceptsVerticalResolution(t *testing.T) {
	result := ProbeResult{Duration: 2, Width: 1080, Height: 1920, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}
	if err := ValidateFinal(result, 1080, 1920); err != nil {
		t.Fatal(err)
	}
}

func TestProbeValidationRejectsInvalidMedia(t *testing.T) {
	valid := ProbeResult{Duration: 1, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}
	tests := []struct {
		name  string
		value ProbeResult
	}{
		{"missing audio", func() ProbeResult { v := valid; v.HasAudio = false; v.AudioCodec = ""; return v }()},
		{"zero duration", func() ProbeResult { v := valid; v.Duration = 0; return v }()},
		{"wrong resolution", func() ProbeResult { v := valid; v.Width = 1280; return v }()},
		{"missing video", func() ProbeResult { v := valid; v.VideoCodec = ""; return v }()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateFinal(test.value, 1920, 1080); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParseProbeRejectsMalformedJSON(t *testing.T) {
	if _, err := ParseProbe([]byte("{")); err == nil {
		t.Fatal("malformed JSON must fail")
	}
	got, err := ParseProbe([]byte(`{"format":{"duration":"1"},"streams":[{"codec_type":"audio","codec_name":"pcm_s16le"}]}`))
	if err != nil || !got.HasAudio || got.VideoCodec != "" {
		t.Fatalf("audio-only assets must be representable: %#v, %v", got, err)
	}
}

func TestProberBuildsExpectedCommand(t *testing.T) {
	var gotName string
	var gotArgs []string
	prober := Prober{Path: `C:\ffmpeg\ffprobe.exe`, Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotName, gotArgs = name, append([]string(nil), args...)
		return []byte(`{"format":{"duration":"1"},"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080}]}`), nil
	}}
	if _, err := prober.Probe(context.Background(), `C:\video.mp4`); err != nil {
		t.Fatal(err)
	}
	want := []string{"-v", "error", "-show_entries", "format=duration:stream=codec_type,codec_name,width,height", "-of", "json", `C:\video.mp4`}
	if gotName != prober.Path || !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("got %q %#v", gotName, gotArgs)
	}
}

func TestProberWrapsCommandFailure(t *testing.T) {
	prober := Prober{Path: "ffprobe", Run: func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("exit 1") }}
	if _, err := prober.Probe(context.Background(), "video.mp4"); err == nil {
		t.Fatal("expected error")
	}
}
