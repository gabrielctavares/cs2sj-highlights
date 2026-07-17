package gui

import (
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
)

func TestEditorStateMoveSelectedClampsInsideCanvas(t *testing.T) {
	state := NewEditorState(hudtheme.Theme{Version: 1, Name: "test", Elements: []hudtheme.Element{{ID: "logo", Type: hudtheme.Image, Anchor: hudtheme.TopLeft, X: 90, Width: 10, Height: 10, Visible: true}}})
	state.SelectedID = "logo"
	if err := state.MoveSelected(20, 0); err != nil {
		t.Fatal(err)
	}
	if got := state.Theme.Elements[0].X; got != 90 {
		t.Fatalf("x = %v, want 90", got)
	}
}

func TestEditorStateSelectAtChoosesTopmostVisibleElement(t *testing.T) {
	state := NewEditorState(hudtheme.Theme{Version: 1, Name: "test", Elements: []hudtheme.Element{
		{ID: "back", Type: hudtheme.Box, Anchor: hudtheme.TopLeft, Width: 30, Height: 30, Visible: true, ZIndex: 1},
		{ID: "front", Type: hudtheme.Text, Anchor: hudtheme.TopLeft, Width: 30, Height: 30, Visible: true, ZIndex: 2},
	}})
	state.SelectAt(10, 10)
	if state.SelectedID != "front" {
		t.Fatalf("selected = %q, want front", state.SelectedID)
	}
}

func TestEditorStateSetSelectedElementUpdatesProperties(t *testing.T) {
	state := NewEditorState(hudtheme.Theme{Version: 1, Name: "test", Elements: []hudtheme.Element{{ID: "event", Type: hudtheme.Text, Anchor: hudtheme.TopLeft, Width: 20, Height: 10, Visible: true, Color: "#FFFFFF"}}})
	state.SelectedID = "event"
	if err := state.SetSelectedElement(func(element *hudtheme.Element) { element.Text = "Final"; element.Color = "#112233" }); err != nil {
		t.Fatal(err)
	}
	if got := state.Selected().Color; got != "#112233" {
		t.Fatalf("color = %q", got)
	}
}

func TestEditorStateMoveLayerChangesZOrder(t *testing.T) {
	state := NewEditorState(hudtheme.Theme{Version: 1, Name: "test", Elements: []hudtheme.Element{{ID: "a", Type: hudtheme.Box, Anchor: hudtheme.TopLeft, Width: 10, Height: 10, ZIndex: 1}, {ID: "b", Type: hudtheme.Box, Anchor: hudtheme.TopLeft, Width: 10, Height: 10, ZIndex: 2}}})
	state.SelectedID = "a"
	if err := state.MoveLayer(1); err != nil {
		t.Fatal(err)
	}
	if state.Theme.Elements[0].ZIndex != 2 || state.Theme.Elements[1].ZIndex != 1 {
		t.Fatalf("unexpected layers: %#v", state.Theme.Elements)
	}
}
