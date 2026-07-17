package gui

import (
	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
	"testing"
)

func TestApplyGuidedLayoutKeepsScoreboardSlotsApart(t *testing.T) {
	theme := hudtheme.DefaultTheme()
	ApplyGuidedLayout(&theme)
	for _, element := range theme.Elements {
		if element.X < 0 || element.Y < 0 || element.X+element.Width > 100 || element.Y+element.Height > 100 {
			t.Fatalf("outside canvas: %#v", element)
		}
	}
}

func TestApplyPaletteUpdatesTeamColors(t *testing.T) {
	theme := hudtheme.DefaultTheme()
	ApplyPalette(&theme, TeamBlue, TeamOrange)
	if findGuidedElement(theme, "team-a-panel").Color != "#1D4ED8" || findGuidedElement(theme, "team-b-panel").Color != "#EA580C" {
		t.Fatal("palette was not applied")
	}
}
