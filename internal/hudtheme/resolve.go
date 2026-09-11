package hudtheme

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/highlights"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func ValuesFor(highlight model.Highlight) Values {
	event := fallback(highlight.HUD.Event, "CS2 SJ")
	teamA := fallback(highlight.HUD.TeamA, "TIME A")
	teamB := fallback(highlight.HUD.TeamB, "TIME B")
	mapName := fallback(highlight.HUD.Map, "MAPA")
	tag := highlights.PrimaryTagLabel(highlight.Tags)
	scoreA, scoreB := "–", "–"
	if highlight.HUD.ScoreKnown {
		scoreA = strconv.Itoa(highlight.HUD.ScoreA)
		scoreB = strconv.Itoa(highlight.HUD.ScoreB)
	}
	return Values{
		Event: event, TeamAName: teamA, TeamBName: teamB, ScoreA: scoreA, ScoreB: scoreB,
		Map: mapName, Round: fmt.Sprintf("ROUND %d", highlight.Round), Player: fallback(highlight.Player.Name, "PLAYER"), Highlight: tag,
	}
}

func fallback(value, fallbackValue string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallbackValue
}
