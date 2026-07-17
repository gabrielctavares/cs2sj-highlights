package media

import (
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
