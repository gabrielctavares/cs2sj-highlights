package model

import (
	"encoding/json"
	"testing"
)

func TestNewManifestStartsPending(t *testing.T) {
	timeline := Timeline{DemoPath: `C:\demos\final.dem`, Map: "de_nuke", TickRate: 64}
	highlights := []Highlight{{ID: "r03-p765-t1200", Status: ClipPending}}

	got := NewManifest(timeline, "abc123", "cfg456", highlights)

	if got.SchemaVersion != ManifestSchemaVersion || got.RulesVersion != RulesVersion || got.DemoMetadata != DemoMetadataVersion {
		t.Fatalf("unexpected versions: %#v", got)
	}
	if got.State != DemoPending || got.Highlights[0].Status != ClipPending {
		t.Fatalf("unexpected initial state: %#v", got)
	}
}

func TestNewManifestPreservesTickRate(t *testing.T) {
	got := NewManifest(Timeline{DemoPath: "match.dem", Map: "de_nuke", TickRate: 128}, "hash", "cfg", nil)
	if got.TickRate != 128 {
		t.Fatalf("tick rate = %v", got.TickRate)
	}
}

func TestNewManifestWithoutHighlightsIsComplete(t *testing.T) {
	got := NewManifest(Timeline{DemoPath: "empty.dem"}, "hash", "cfg", nil)
	if got.State != DemoNoHighlights {
		t.Fatalf("unexpected state: %s", got.State)
	}
}

func TestHUDModeValidationAndCaptureMode(t *testing.T) {
	if !HUDNone.Valid() || !HUDGame.Valid() || !HUDCustom.Valid() || HUDMode("invalid").Valid() {
		t.Fatal("unexpected HUD mode validation")
	}
	if HUDNone.CaptureMode() != "clean" || HUDCustom.CaptureMode() != "clean" || HUDGame.CaptureMode() != "game" {
		t.Fatal("unexpected capture mode mapping")
	}
}

func TestManifestPersistsDualEvaluationsAndSelection(t *testing.T) {
	highlight := Highlight{
		ID:         "r03-steam-7-t1200",
		Individual: Evaluation{Score: 82, Confidence: ConfidenceHigh, Explanation: "headshot de longa distância"},
		Editorial:  Evaluation{Score: 91, Confidence: ConfidenceHigh, Explanation: "match point"},
	}
	want := NewManifest(Timeline{DemoPath: "match.dem", TickRate: 64}, "demo", "capture", []Highlight{highlight})
	want.SelectedHighlightIDs = []string{highlight.ID}
	want.DiscardedCandidates = []CandidateDiscard{{Round: 4, Code: "empty_window"}}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.CandidateVersion != CandidateVersion || got.ScoringVersion != ScoringVersion || got.DiversityVersion != DiversityVersion {
		t.Fatalf("missing catalog versions: %#v", got)
	}
	if got.Highlights[0].Individual.Score != 82 || got.SelectedHighlightIDs[0] != highlight.ID || got.DiscardedCandidates[0].Code != "empty_window" {
		t.Fatalf("catalog fields were not retained: %#v", got)
	}
}
