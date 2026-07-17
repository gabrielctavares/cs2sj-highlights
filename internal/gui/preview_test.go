package gui

import (
	"path/filepath"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/highlights"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/pipeline"
)

func previewResults(root string) []pipeline.Result {
	demoA := filepath.Join(root, "a.dem")
	demoB := filepath.Join(root, "b.dem")
	return []pipeline.Result{
		{DemoPath: demoA, Manifest: model.Manifest{Map: "de_nuke", Highlights: []model.Highlight{
			{ID: "a1", Round: 3, Player: model.Player{SteamID: 10, Name: "Ana", TeamName: "Azul"}, Tags: []string{"CLUTCH", "3K"}, Editorial: model.Evaluation{Score: 90, Explanation: "clutch 1v3"}, Individual: model.Evaluation{Score: 82, Explanation: "três eliminações"}},
			{ID: "a2", Round: 4, Player: model.Player{SteamID: 20, Name: "Bia"}, Tags: []string{"HEADSHOT"}, Editorial: model.Evaluation{Score: 40}, Individual: model.Evaluation{Score: 65, Explanation: "headshot"}},
		}}},
		{DemoPath: demoB, Manifest: model.Manifest{Map: "de_mirage", Highlights: []model.Highlight{
			{ID: "b1", Round: 7, Player: model.Player{SteamID: 10, Name: "Ana", TeamName: "Azul"}, Tags: []string{"ACE"}, Editorial: model.Evaluation{Score: 70, Explanation: "ace"}, Individual: model.Evaluation{Score: 96, Explanation: "cinco eliminações"}},
		}}},
	}
}

func TestPreviewStateBuildsIndependentCatalogViews(t *testing.T) {
	state, err := NewPreviewState(previewResults(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	players := state.Players()
	if len(players) != 2 || players[0].SteamID != 10 || players[0].Name != "Ana" || players[0].TeamName != "Azul" {
		t.Fatalf("unexpected players: %#v", players)
	}
	editorial := state.EditorialChoices(highlights.BreadthBalanced)
	if len(editorial) != 2 || editorial[0].ID != "a1" || editorial[0].Perspective != PerspectiveEditorial || editorial[0].Score != 90 || editorial[0].Explanation != "clutch 1v3" {
		t.Fatalf("unexpected editorial view: %#v", editorial)
	}
	individual := state.PlayerChoices(highlights.BreadthBalanced, 10)
	if len(individual) != 2 || individual[0].ID != "b1" || individual[0].Perspective != PerspectiveIndividual || individual[0].Score != 96 || individual[0].SteamID != 10 {
		t.Fatalf("unexpected player view: %#v", individual)
	}
	if got := state.PlayerChoices(highlights.BreadthBalanced, 0); len(got) != 0 {
		t.Fatalf("zero SteamID returned %#v", got)
	}
}

func TestPreviewSelectionIsExplicitAndDeduplicatedAcrossPerspectives(t *testing.T) {
	state, err := NewPreviewState(previewResults(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if len(state.FinalChoices()) != 0 || len(state.SelectedHighlights()) != 0 {
		t.Fatal("selection must start empty")
	}
	editorial := state.EditorialChoices(highlights.BreadthBalanced)[0]
	individual := state.PlayerChoices(highlights.BreadthBalanced, editorial.SteamID)[1]
	if editorial.ID != individual.ID {
		t.Fatalf("test setup did not find same clip: %#v %#v", editorial, individual)
	}
	state.Add(editorial)
	state.Add(individual)
	if got := state.FinalChoices(); len(got) != 1 || got[0].Perspective != PerspectiveEditorial {
		t.Fatalf("unexpected final choices: %#v", got)
	}
	selected := state.SelectedHighlights()
	if len(selected) != 1 || len(selected[editorial.DemoPath]) != 1 || selected[editorial.DemoPath][0] != editorial.ID {
		t.Fatalf("unexpected selected IDs: %#v", selected)
	}
	state.Remove(editorial.DemoPath, editorial.ID)
	if len(state.FinalChoices()) != 0 {
		t.Fatalf("remove failed: %#v", state.FinalChoices())
	}
}

func TestPreferredPlayerUsesFavoriteThenStableFallback(t *testing.T) {
	state, err := NewPreviewState(previewResults(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if got := state.PreferredPlayer(20); got != 20 {
		t.Fatalf("favorite present: got %d", got)
	}
	if got := state.PreferredPlayer(999); got != 10 {
		t.Fatalf("favorite absent fallback: got %d", got)
	}
}

func TestPreviewStateReportsDemoFailure(t *testing.T) {
	_, err := NewPreviewState([]pipeline.Result{{DemoPath: "bad.dem", Err: filepath.ErrBadPattern}})
	if err == nil {
		t.Fatal("expected preview error")
	}
}
