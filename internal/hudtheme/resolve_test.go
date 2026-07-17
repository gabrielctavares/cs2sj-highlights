package hudtheme

import (
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestValuesForUsesHighlightData(t *testing.T) {
	values := ValuesFor(model.Highlight{
		Round: 18, Player: model.Player{Name: "Gabi"}, Tags: []string{"ACE"},
		HUD: model.HUDMetadata{Event: "Final", TeamA: "ONU", TeamB: "Tedesco", ScoreA: 3, ScoreB: 2, ScoreKnown: true, Map: "NUKE"},
	})
	if values[TeamAName] != "ONU" || values[ScoreB] != "2" || values[Highlight] != "ACE" || values[Round] != "ROUND 18" {
		t.Fatalf("unexpected values: %#v", values)
	}
}
