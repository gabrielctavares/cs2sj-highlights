package gui

import (
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
)

type TeamPalette string

const (
	TeamBlue   TeamPalette = "blue"
	TeamOrange TeamPalette = "orange"
	TeamRed    TeamPalette = "red"
	TeamGreen  TeamPalette = "green"
	TeamPurple TeamPalette = "purple"
)

var teamPalettes = []TeamPalette{TeamBlue, TeamOrange, TeamRed, TeamGreen, TeamPurple}

func paletteLabels() []string {
	return []string{"Azul", "Laranja", "Vermelho", "Verde", "Roxo"}
}

func paletteAt(index int) TeamPalette {
	if index < 0 || index >= len(teamPalettes) {
		return TeamBlue
	}
	return teamPalettes[index]
}

func paletteIndex(color string, fallback TeamPalette) int {
	for index, palette := range teamPalettes {
		if strings.EqualFold(paletteColor(palette), strings.TrimSpace(color)) {
			return index
		}
	}
	for index, palette := range teamPalettes {
		if palette == fallback {
			return index
		}
	}
	return 0
}

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
func ApplyPalette(theme *hudtheme.Theme, a, b TeamPalette) {
	ensureGuidedPanels(theme)
	findGuidedElement(*theme, "team-a-panel").Color = paletteColor(a)
	findGuidedElement(*theme, "team-b-panel").Color = paletteColor(b)
}
func paletteColor(p TeamPalette) string {
	switch p {
	case TeamBlue:
		return "#1D4ED8"
	case TeamOrange:
		return "#EA580C"
	case TeamRed:
		return "#DC2626"
	case TeamGreen:
		return "#16A34A"
	case TeamPurple:
		return "#7C3AED"
	}
	return "#111827"
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
