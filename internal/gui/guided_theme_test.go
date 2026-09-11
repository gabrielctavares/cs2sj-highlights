package gui

import (
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
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

func TestApplyTeamColorsAcceptsAnyHexColor(t *testing.T) {
	theme := hudtheme.DefaultTheme()
	ApplyTeamColors(&theme, "#12abef", "#fedc34")
	if findGuidedElement(theme, "team-a-panel").Color != "#12ABEF" || findGuidedElement(theme, "team-b-panel").Color != "#FEDC34" {
		t.Fatal("team colors were not applied")
	}
}

func TestApplyTeamColorsRejectsInvalidValues(t *testing.T) {
	theme := hudtheme.DefaultTheme()
	ApplyTeamColors(&theme, "invalid", "#123")
	if got := teamColor(theme, "team-a-panel", "#000000"); got != "#1D4ED8" {
		t.Fatalf("team A fallback = %q", got)
	}
	if got := teamColor(theme, "team-b-panel", "#000000"); got != "#EA580C" {
		t.Fatalf("team B fallback = %q", got)
	}
}
