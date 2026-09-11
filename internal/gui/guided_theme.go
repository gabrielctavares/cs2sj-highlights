package gui

import (
	"strconv"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
)

func ApplyGuidedLayout(theme *hudtheme.Theme) {
	ensureGuidedPanels(theme)
	for index := range theme.Elements {
		e := &theme.Elements[index]
		switch e.ID {
		case "team-a-logo":
			e.X, e.Y, e.Width, e.Height = 18, 4, 5, 8
		case "team-b-logo":
			e.X, e.Y, e.Width, e.Height = 77, 4, 5, 8
		case "team-a":
			e.X, e.Y, e.Width, e.Height = 24, 6, 14, 4
		case "team-b":
			e.X, e.Y, e.Width, e.Height = 62, 6, 14, 4
		case "score-a":
			e.X, e.Y, e.Width, e.Height = 42, 5, 5, 5
		case "score-b":
			e.X, e.Y, e.Width, e.Height = 53, 5, 5, 5
		case "map":
			e.X, e.Y, e.Width, e.Height = 46, 5, 8, 3
		case "round":
			e.X, e.Y, e.Width, e.Height = 46, 8, 8, 2
		}
	}
}

func ensureGuidedPanels(theme *hudtheme.Theme) {
	ensureGuidedBox(theme, "team-a-panel", 18, 4, 22, 8, "#1D4ED8")
	ensureGuidedBox(theme, "team-b-panel", 60, 4, 22, 8, "#EA580C")
	ensureGuidedBox(theme, "score-panel", 40, 4, 20, 8, "#111827")
}
func ApplyTeamColors(theme *hudtheme.Theme, teamA, teamB string) {
	ensureGuidedPanels(theme)
	findGuidedElement(*theme, "team-a-panel").Color = normalizeTeamColor(teamA, "#1D4ED8")
	findGuidedElement(*theme, "team-b-panel").Color = normalizeTeamColor(teamB, "#EA580C")
}

func teamColor(theme hudtheme.Theme, panelID, fallback string) string {
	if panel := findGuidedElement(theme, panelID); panel != nil {
		return normalizeTeamColor(panel.Color, fallback)
	}
	return fallback
}

func normalizeTeamColor(color, fallback string) string {
	color = strings.ToUpper(strings.TrimSpace(color))
	if len(color) == 7 && color[0] == '#' {
		if _, err := strconv.ParseUint(color[1:], 16, 24); err == nil {
			return color
		}
	}
	return strings.ToUpper(fallback)
}
func ensureGuidedBox(theme *hudtheme.Theme, id string, x, y, w, h float64, color string) {
	if findGuidedElement(*theme, id) != nil {
		return
	}
	theme.Elements = append(theme.Elements, hudtheme.Element{ID: id, Type: hudtheme.Box, Anchor: hudtheme.TopLeft, X: x, Y: y, Width: w, Height: h, ZIndex: 1, Visible: true, Color: color, Opacity: .96})
}
func findGuidedElement(theme hudtheme.Theme, id string) *hudtheme.Element {
	for i := range theme.Elements {
		if theme.Elements[i].ID == id {
			return &theme.Elements[i]
		}
	}
	return nil
}
