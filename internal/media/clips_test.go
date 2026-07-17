package media

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestTitleText(t *testing.T) {
	highlight := model.Highlight{Player: model.Player{Name: "Ana; quit"}, Tags: []string{"ACE", "CLUTCH"}}
	if got := TitleText(highlight); got != "Ana; quit — ACE · CLUTCH" {
		t.Fatalf("unexpected title %q", got)
	}
}

func TestPlayerHUDColorMatchesLogicalTeam(t *testing.T) {
	hud := model.HUDMetadata{TeamA: "ONU", TeamB: "Tedesco"}
	if got := playerHUDColor(model.Player{TeamName: "ONU", Team: model.TeamCT}, hud); got != "0x0b4f71" {
		t.Fatalf("TeamA player color = %q", got)
	}
	if got := playerHUDColor(model.Player{TeamName: "Tedesco", Team: model.TeamT}, hud); got != "0xc66a14" {
		t.Fatalf("TeamB player color = %q", got)
	}
}

func TestAncientUsesReadableMapFont(t *testing.T) {
	if got := mapFontSize("ANCIENT"); got < 18 || got > 22 {
		t.Fatalf("font size = %d", got)
	}
}

func TestClipBuilderAllowsHorizontalOnlyOutput(t *testing.T) {
	highlight := clipFixture(t, false)
	highlight.Outputs.Vertical = ""
	var calls int
	builder := ClipBuilder{
		FFmpegPath: "ffmpeg.exe",
		FontPath:   `C:\Windows\Fonts\segoeuib.ttf`,
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls++
			return nil, os.WriteFile(args[len(args)-1], []byte("mp4"), 0o600)
		},
		Probe: func(context.Context, string) (ProbeResult, error) {
			return ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
		},
	}

	got, err := builder.Build(context.Background(), highlight)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || got.Horizontal != highlight.Outputs.Horizontal || got.Vertical != "" {
		t.Fatalf("expected one horizontal encode only: calls=%d outputs=%#v", calls, got)
	}
}

func TestClipBuilderBuildsHorizontalOnly(t *testing.T) {
	highlight := clipFixture(t, false)
	highlight.HUD = model.HUDMetadata{Event: "1º CAMP MONTADO CS2 SJ", TeamA: "ONU", TeamB: "G3neration Z", Map: "ANCIENT", ScoreA: 8, ScoreB: 4, ScoreKnown: true}
	logoPath := filepath.Join(filepath.Dir(highlight.MasterPath), "cs2sj-logo.jpg")
	if err := os.WriteFile(logoPath, []byte("jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	var horizontalFilter string
	hudTexts := make(map[string]string)
	builder := ClipBuilder{
		FFmpegPath: "ffmpeg.exe", FontPath: `C:\Windows\Fonts\segoeuib.ttf`, LogoPath: logoPath, HUDMode: model.HUDCustom,
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, slices.Clone(args))
			if index := slices.Index(args, "-/filter_complex"); index >= 0 {
				data, err := os.ReadFile(args[index+1])
				if err != nil {
					return nil, err
				}
				horizontalFilter = string(data)
				for _, suffix := range []string{"-event.txt", "-team-a.txt", "-team-b.txt", "-score-a.txt", "-score-b.txt", "-map.txt", "-round.txt", "-player.txt", "-highlight.txt"} {
					data, err = os.ReadFile(filepath.Join(filepath.Dir(highlight.Outputs.Horizontal), highlight.ID+suffix))
					if err != nil {
						return nil, err
					}
					hudTexts[suffix] = string(data)
				}
			}
			return nil, os.WriteFile(args[len(args)-1], []byte("mp4"), 0o600)
		},
		Probe: func(context.Context, string) (ProbeResult, error) {
			return ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
		},
	}
	got, err := builder.Build(context.Background(), highlight)
	if err != nil {
		t.Fatal(err)
	}
	if got.Horizontal != highlight.Outputs.Horizontal || got.Vertical != "" || len(calls) != 1 {
		t.Fatalf("unexpected result/calls: %#v %#v", got, calls)
	}
	if _, err := os.Stat(got.Horizontal); err != nil {
		t.Fatal(err)
	}
	if hudTexts["-player.txt"] != highlight.Player.Name || strings.Contains(strings.Join(calls[0], " "), "Ana; quit") {
		t.Fatalf("HUD text must be isolated in text files: %#v %#v", hudTexts, calls[0])
	}
	for _, value := range []string{"scale=1920:1080", "drawtext=", "drawbox=", "0x0b4f71", "0xc66a14", "event.txt", "team-a.txt", "team-b.txt", "player.txt", "highlight.txt", "[1:v]scale=60:60", "overlay=x=805:y=51"} {
		if !strings.Contains(horizontalFilter, value) {
			t.Errorf("horizontal filter missing %q: %s", value, horizontalFilter)
		}
	}
	for suffix, want := range map[string]string{
		"-event.txt": "1º CAMP MONTADO CS2 SJ", "-team-a.txt": "ONU", "-team-b.txt": "G3neration Z", "-score-a.txt": "8", "-score-b.txt": "4",
		"-map.txt": "ANCIENT", "-round.txt": "ROUND 0", "-player.txt": "Ana; quit", "-highlight.txt": "ACE + CLUTCH",
	} {
		if hudTexts[suffix] != want {
			t.Errorf("unexpected HUD text %s: %q", suffix, hudTexts[suffix])
		}
	}
	joinedHorizontal := strings.Join(calls[0], " ")
	if !strings.Contains(joinedHorizontal, logoPath) {
		t.Fatalf("logo input missing: %#v", calls[0])
	}
	if strings.Contains(joinedHorizontal, "-filter_complex_script") {
		t.Fatalf("deprecated FFmpeg option used: %#v", calls[0])
	}
	for _, value := range []string{"libx264", "veryfast", "18", "yuv420p", "aac", "192k", "+faststart", "0:a:0"} {
		if !strings.Contains(joinedHorizontal, value) {
			t.Errorf("horizontal args missing %q: %#v", value, calls[0])
		}
	}
	for _, suffix := range []string{"-event.txt", "-team-a.txt", "-team-b.txt", "-score-a.txt", "-score-b.txt", "-map.txt", "-round.txt", "-player.txt", "-highlight.txt", "-horizontal.ffscript"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(got.Horizontal), highlight.ID+suffix)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary file leaked: %v", err)
		}
	}
}

func TestClipBuilderUsesSelectedExternalTheme(t *testing.T) {
	highlight := clipFixture(t, false)
	themeDir := t.TempDir()
	logoPath := filepath.Join(themeDir, "team-a.png")
	if err := os.WriteFile(logoPath, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	theme := hudtheme.Theme{Version: 1, Name: "Campeonato", Elements: []hudtheme.Element{
		{ID: "event", Type: hudtheme.Text, Anchor: hudtheme.TopCenter, Width: 40, Height: 5, Visible: true, Binding: hudtheme.Event, FontSize: 24, Color: "#FFFFFF"},
		{ID: "logo", Type: hudtheme.Image, Anchor: hudtheme.TopLeft, Width: 5, Height: 8, Visible: true, Asset: "team-a.png"},
	}}
	var args []string
	var filter string
	builder := ClipBuilder{FFmpegPath: "ffmpeg", FontPath: `C:\font.ttf`, HUDMode: model.HUDCustom, Theme: &theme, ThemeDir: themeDir,
		Run: func(_ context.Context, _ string, values ...string) ([]byte, error) {
			args = slices.Clone(values)
			index := slices.Index(values, "-/filter_complex")
			data, err := os.ReadFile(values[index+1])
			filter = string(data)
			if err != nil {
				return nil, err
			}
			return nil, os.WriteFile(values[len(values)-1], []byte("mp4"), 0o600)
		},
		Probe: func(context.Context, string) (ProbeResult, error) {
			return ProbeResult{Duration: 1, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
		},
	}
	if _, err := builder.Build(context.Background(), highlight); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filter, "text='CS2 SJ'") || !strings.Contains(filter, "overlay=") {
		t.Fatalf("external theme was not compiled: %s", filter)
	}
	if !slices.Contains(args, logoPath) {
		t.Fatalf("theme asset input missing: %#v", args)
	}
}

func TestClipBuilderMapsSeparateAudio(t *testing.T) {
	highlight := clipFixture(t, true)
	var first []string
	builder := ClipBuilder{FFmpegPath: "ffmpeg", FontPath: `C:\font.ttf`,
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if first == nil {
				first = slices.Clone(args)
			}
			return nil, os.WriteFile(args[len(args)-1], []byte("mp4"), 0o600)
		},
		Probe: func(_ context.Context, path string) (ProbeResult, error) {
			if path == highlight.MasterPath {
				return ProbeResult{Duration: 25.766667, Width: 1920, Height: 1080, VideoCodec: "h264"}, nil
			}
			if path == highlight.MasterAudioPath {
				return ProbeResult{Duration: 26.726168, AudioCodec: "pcm_s16le", HasAudio: true}, nil
			}
			return ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
		},
	}
	if _, err := builder.Build(context.Background(), highlight); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(first, " ")
	if !strings.Contains(joined, highlight.MasterAudioPath) || !strings.Contains(joined, "1:a:0") || !strings.Contains(joined, "-shortest") {
		t.Fatalf("separate audio was not mapped: %#v", first)
	}
	wantSyncInput := "-ss 0.959501 -i " + highlight.MasterAudioPath
	if !strings.Contains(joined, wantSyncInput) {
		t.Fatalf("audio startup latency was not corrected with %q: %#v", wantSyncInput, first)
	}
}

func TestClipBuilderRetainsDiagnosticsOnFailure(t *testing.T) {
	highlight := clipFixture(t, false)
	builder := ClipBuilder{FFmpegPath: "ffmpeg", FontPath: `C:\font.ttf`, HUDMode: model.HUDCustom, Run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("exit 1")
	}, Probe: func(context.Context, string) (ProbeResult, error) { return ProbeResult{}, nil }}
	if _, err := builder.Build(context.Background(), highlight); err == nil {
		t.Fatal("expected error")
	}
	for _, suffix := range []string{"-event.txt", "-team-a.txt", "-team-b.txt", "-score-a.txt", "-score-b.txt", "-map.txt", "-round.txt", "-player.txt", "-highlight.txt", "-horizontal.ffscript"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(highlight.Outputs.Horizontal), highlight.ID+suffix)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClipBuilderNoneModeHasNoTournamentHUD(t *testing.T) {
	highlight := clipFixture(t, false)
	var filter string
	builder := ClipBuilder{FFmpegPath: "ffmpeg", FontPath: `C:\font.ttf`, HUDMode: model.HUDNone,
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			index := slices.Index(args, "-/filter_complex")
			data, err := os.ReadFile(args[index+1])
			if err != nil {
				return nil, err
			}
			filter = string(data)
			return nil, os.WriteFile(args[len(args)-1], []byte("mp4"), 0o600)
		}, Probe: func(context.Context, string) (ProbeResult, error) {
			return ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
		}}
	if _, err := builder.Build(context.Background(), highlight); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(filter, "drawbox=") || strings.Contains(filter, "team-a.txt") {
		t.Fatalf("none mode contains custom HUD: %s", filter)
	}
}

func TestClipBuilderKeepsFullMasterWhenActionMetadataExists(t *testing.T) {
	highlight := clipFixture(t, false)
	highlight.ActionOffsets = []float64{5, 25}
	var filter string
	var args []string
	builder := ClipBuilder{FFmpegPath: "ffmpeg", FontPath: `C:\font.ttf`, HUDMode: model.HUDNone,
		Run: func(_ context.Context, _ string, values ...string) ([]byte, error) {
			args = slices.Clone(values)
			index := slices.Index(values, "-/filter_complex")
			data, err := os.ReadFile(values[index+1])
			if err != nil {
				return nil, err
			}
			filter = string(data)
			return nil, os.WriteFile(values[len(values)-1], []byte("mp4"), 0o600)
		}, Probe: func(_ context.Context, path string) (ProbeResult, error) {
			if path == highlight.MasterPath {
				return ProbeResult{Duration: 35, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
			}
			return ProbeResult{Duration: 20, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
		}}
	if _, err := builder.Build(context.Background(), highlight); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"trim=", "atempo=", "concat=", "[paced]"} {
		if strings.Contains(filter, value) {
			t.Errorf("full clip unexpectedly contains pacing filter %q: %s", value, filter)
		}
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-map 0:a:0") {
		t.Fatalf("original audio was not mapped directly: %#v", args)
	}
}

func TestClipBuilderReplacesExistingHorizontalOutput(t *testing.T) {
	highlight := clipFixture(t, false)
	if err := os.WriteFile(highlight.Outputs.Horizontal, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	builder := ClipBuilder{FFmpegPath: "ffmpeg", FontPath: `C:\font.ttf`,
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			return nil, os.WriteFile(args[len(args)-1], []byte("new"), 0o600)
		},
		Probe: func(context.Context, string) (ProbeResult, error) {
			return ProbeResult{Duration: 2, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", HasAudio: true}, nil
		},
	}
	if _, err := builder.Build(context.Background(), highlight); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(highlight.Outputs.Horizontal)
	if err != nil || string(data) != "new" {
		t.Fatalf("old output was not replaced: %q err=%v", data, err)
	}
}

func TestClipBuilderRealFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}
	font := `C:\Windows\Fonts\segoeuib.ttf`
	if _, err := os.Stat(font); err != nil {
		t.Skip("Segoe UI Bold font unavailable")
	}
	highlight := clipFixture(t, false)
	highlight.ActionOffsets = []float64{0.5, 1.5}
	command := exec.Command(ffmpeg, "-y", "-f", "lavfi", "-i", "color=c=blue:s=1920x1080:d=2:r=60", "-f", "lavfi", "-i", "sine=frequency=1000:duration=2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", highlight.MasterPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate fixture: %v: %s", err, output)
	}
	prober := Prober{Path: ffprobe}
	builder := ClipBuilder{FFmpegPath: ffmpeg, FontPath: font, Probe: prober.Probe}
	outputs, err := builder.Build(context.Background(), highlight)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := prober.Probe(context.Background(), outputs.Horizontal)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateFinal(probe, 1920, 1080); err != nil {
		t.Fatal(err)
	}
	if outputs.Vertical != "" {
		t.Fatalf("vertical output must remain disabled: %#v", outputs)
	}
}

func TestClipBuilderRealFFmpegWithExternalTheme(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}
	font := `C:\Windows\Fonts\segoeuib.ttf`
	if _, err := os.Stat(font); err != nil {
		t.Skip("Segoe UI Bold font unavailable")
	}
	highlight := clipFixture(t, false)
	highlight.HUD = model.HUDMetadata{Event: "FINAL CS2SJ"}
	themeDir := t.TempDir()
	logoPath := filepath.Join(themeDir, "team-a.png")
	if output, err := exec.Command(ffmpeg, "-y", "-f", "lavfi", "-i", "color=c=red:s=80x80", "-frames:v", "1", logoPath).CombinedOutput(); err != nil {
		t.Fatalf("generate logo fixture: %v: %s", err, output)
	}
	command := exec.Command(ffmpeg, "-y", "-f", "lavfi", "-i", "color=c=blue:s=1920x1080:d=1:r=30", "-f", "lavfi", "-i", "sine=frequency=1000:duration=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", highlight.MasterPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate fixture: %v: %s", err, output)
	}
	theme := hudtheme.Theme{Version: 1, Name: "E2E", Elements: []hudtheme.Element{
		{ID: "bar", Type: hudtheme.Box, Anchor: hudtheme.TopLeft, Width: 100, Height: 8, Visible: true, Color: "#101820", Opacity: 0.9},
		{ID: "event", Type: hudtheme.Text, Anchor: hudtheme.TopCenter, X: 25, Y: 1, Width: 50, Height: 5, ZIndex: 1, Visible: true, Binding: hudtheme.Event, FontSize: 28, Color: "#FFFFFF"},
		{ID: "team-a-logo", Type: hudtheme.Image, Anchor: hudtheme.TopLeft, X: 2, Y: 1, Width: 5, Height: 5, ZIndex: 2, Visible: true, Asset: "team-a.png"},
	}}
	prober := Prober{Path: ffprobe}
	builder := ClipBuilder{FFmpegPath: ffmpeg, FontPath: font, Theme: &theme, ThemeDir: themeDir, HUDMode: model.HUDCustom, Probe: prober.Probe}
	outputs, err := builder.Build(context.Background(), highlight)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := prober.Probe(context.Background(), outputs.Horizontal)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateFinal(probe, 1920, 1080); err != nil {
		t.Fatal(err)
	}
}

func clipFixture(t *testing.T, separateAudio bool) model.Highlight {
	t.Helper()
	root := t.TempDir()
	master := filepath.Join(root, "master.mp4")
	if err := os.WriteFile(master, []byte("master"), 0o600); err != nil {
		t.Fatal(err)
	}
	audio := ""
	if separateAudio {
		audio = filepath.Join(root, "master.wav")
		if err := os.WriteFile(audio, []byte("audio"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return model.Highlight{ID: "r01-p1-t100", Player: model.Player{Name: "Ana; quit"}, Tags: []string{"ACE", "CLUTCH"}, MasterPath: master, MasterAudioPath: audio,
		Outputs: model.OutputPaths{Horizontal: filepath.Join(root, "01-ana-ACE-16x9.mp4"), Vertical: filepath.Join(root, "01-ana-ACE-9x16.mp4")}}
}
