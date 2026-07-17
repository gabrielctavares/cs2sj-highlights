package highlights

import (
	"fmt"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestBreadthThresholdsAndNoCap(t *testing.T) {
	items := make([]model.Highlight, 30)
	for index := range items {
		items[index] = model.Highlight{ID: fmt.Sprint(index), Editorial: model.Evaluation{Score: 60}}
	}
	if got := EditorialView(items, BreadthBalanced); len(got) != 30 {
		t.Fatalf("balanced view capped at %d", len(got))
	}
	if got := EditorialView(items, BreadthRestricted); len(got) != 0 {
		t.Fatalf("restricted view accepted %d balanced items", len(got))
	}
	items[0].Editorial.Score = 35
	if got := EditorialView(items[:1], BreadthBroad); len(got) != 1 {
		t.Fatalf("broad threshold rejected score 35")
	}
}

func TestPlayerViewFiltersSteamIDAndUsesIndividualScore(t *testing.T) {
	items := []model.Highlight{
		{ID: "a", Player: model.Player{SteamID: 7}, Individual: model.Evaluation{Score: 70}},
		{ID: "b", Player: model.Player{SteamID: 8}, Individual: model.Evaluation{Score: 90}},
		{ID: "c", Player: model.Player{SteamID: 7}, Individual: model.Evaluation{Score: 80}},
	}
	got := PlayerView(items, BreadthBalanced, 7)
	if len(got) != 2 || got[0].ID != "c" || got[1].ID != "a" {
		t.Fatalf("unexpected player view: %#v", got)
	}
}

func TestEditorialDiversityReordersOnlyClosePlays(t *testing.T) {
	items := []model.Highlight{
		{ID: "top", Player: model.Player{SteamID: 1}, Tags: []string{"3K"}, Editorial: model.Evaluation{Score: 95}},
		{ID: "same", Player: model.Player{SteamID: 1}, Tags: []string{"3K"}, Editorial: model.Evaluation{Score: 90}},
		{ID: "diverse", Player: model.Player{SteamID: 2}, Tags: []string{"HEADSHOT"}, Editorial: model.Evaluation{Score: 88}},
		{ID: "lower-band", Player: model.Player{SteamID: 3}, Tags: []string{"HEADSHOT"}, Editorial: model.Evaluation{Score: 70}},
	}
	got := EditorialView(items, BreadthBalanced)
	if got[0].ID != "top" || got[1].ID != "diverse" || got[2].ID != "same" || got[3].ID != "lower-band" {
		t.Fatalf("unexpected diversity order: %#v", got)
	}
}

func TestBuildCatalogEvaluatesGeneratedCandidates(t *testing.T) {
	protagonist := player("ana", model.TeamT, 1)
	round := roundWithKills(1, 100, 1000, protagonist, []int{200, 300, 400})
	got, discards := BuildCatalog(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(discards) != 0 || len(got) != 1 || got[0].Individual.Score < 62 || got[0].Editorial.Score < 60 {
		t.Fatalf("catalog=%#v discards=%#v", got, discards)
	}
}
