package model

import "testing"

func TestNewManifestStartsPending(t *testing.T) {
	timeline := Timeline{DemoPath: `C:\demos\final.dem`, Map: "de_nuke", TickRate: 64}
	highlights := []Highlight{{ID: "r03-p765-t1200", Status: ClipPending}}

	got := NewManifest(timeline, "abc123", "cfg456", highlights)

	if got.SchemaVersion != "manifest-v1" || got.RulesVersion != "rules-v2" {
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
