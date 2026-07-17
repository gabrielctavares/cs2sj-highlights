package gui

import (
	"path/filepath"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/pipeline"
)

func TestChoicesFromResultsAndSelectedHighlights(t *testing.T) {
	root := t.TempDir()
	demoA := filepath.Join(root, "a.dem")
	demoB := filepath.Join(root, "b.dem")
	results := []pipeline.Result{
		{DemoPath: demoA, Manifest: model.Manifest{Map: "de_nuke", Highlights: []model.Highlight{
			{ID: "a1", Round: 3, Player: model.Player{Name: "Ana"}, Tags: []string{"CLUTCH", "3K"}},
		}}},
		{DemoPath: demoB, Manifest: model.Manifest{Map: "de_mirage", Highlights: []model.Highlight{
			{ID: "b1", Round: 7, Player: model.Player{Name: "Bia"}, Tags: []string{"ACE"}},
		}}},
	}

	choices, err := ChoicesFromResults(results)
	if err != nil {
		t.Fatal(err)
	}
	if len(choices) != 2 || !choices[0].Selected || choices[0].DemoName != "a" || choices[0].Player != "Ana" || choices[0].Type != "CLUTCH + 3K" {
		t.Fatalf("unexpected choices: %#v", choices)
	}
	choices[0].Selected = false
	selected := SelectedHighlights(choices)
	if len(selected) != 1 || len(selected[demoB]) != 1 || selected[demoB][0] != "b1" {
		t.Fatalf("unexpected selection: %#v", selected)
	}
}

func TestChoicesFromResultsReportsDemoFailure(t *testing.T) {
	_, err := ChoicesFromResults([]pipeline.Result{{DemoPath: "bad.dem", Err: filepath.ErrBadPattern}})
	if err == nil {
		t.Fatal("expected preview error")
	}
}
