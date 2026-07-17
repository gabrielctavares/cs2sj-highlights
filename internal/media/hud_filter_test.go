package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
)

func TestBuildHUDFilterOrdersBoxesByZIndex(t *testing.T) {
	theme := hudtheme.Theme{Version: 1, Name: "test", Elements: []hudtheme.Element{
		{ID: "front", Type: hudtheme.Box, Anchor: hudtheme.TopLeft, Width: 10, Height: 10, ZIndex: 2, Color: "#FFFFFF", Opacity: 1},
		{ID: "back", Type: hudtheme.Box, Anchor: hudtheme.TopLeft, Width: 10, Height: 10, ZIndex: 1, Color: "#000000", Opacity: 1},
	}}
	plan, err := BuildHUDFilter(theme, t.TempDir(), nil, "C:\\font.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(plan.Filter, "0x000000") > strings.Index(plan.Filter, "0xFFFFFF") {
		t.Fatalf("layers are reversed: %s", plan.Filter)
	}
}

func TestBuildHUDFilterRendersBoundTextAndThemeImages(t *testing.T) {
	themeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(themeDir, "team-a.png"), []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	theme := hudtheme.Theme{Version: 1, Name: "test", Elements: []hudtheme.Element{
		{ID: "event", Type: hudtheme.Text, Anchor: hudtheme.TopCenter, Width: 40, Height: 5, ZIndex: 1, Visible: true, Binding: hudtheme.Event, FontSize: 24, Color: "#FFFFFF"},
		{ID: "team-a-logo", Type: hudtheme.Image, Anchor: hudtheme.TopLeft, Width: 5, Height: 8, ZIndex: 2, Visible: true, Asset: "team-a.png"},
	}}
	plan, err := BuildHUDFilter(theme, themeDir, hudtheme.Values{hudtheme.Event: "Final SJ"}, "C:\\font.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Filter, "drawtext=") || !strings.Contains(plan.Filter, "text='Final SJ'") {
		t.Fatalf("expected bound text in filter: %s", plan.Filter)
	}
	if len(plan.Inputs) != 1 || plan.Inputs[0] != filepath.Join(themeDir, "team-a.png") {
		t.Fatalf("unexpected image inputs: %#v", plan.Inputs)
	}
	if !strings.Contains(plan.Filter, "overlay=") {
		t.Fatalf("expected image overlay in filter: %s", plan.Filter)
	}
}
