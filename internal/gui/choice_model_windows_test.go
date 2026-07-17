//go:build windows

package gui

import "testing"

func TestChoiceTableModelIsReadOnlyAndExposesScoreAndExplanation(t *testing.T) {
	model := newChoiceTableModel()
	model.SetChoices([]ClipChoice{{DemoName: "a", Map: "de_nuke", Player: "Ana", Type: "3K", Round: 2, Score: 87, Explanation: "três eliminações rápidas"}})
	if model.RowCount() != 1 || model.Value(0, 2) != "Ana" || model.Value(0, 5) != 87 || model.Value(0, 6) != "três eliminações rápidas" {
		t.Fatalf("unexpected model contents")
	}
	choice, ok := model.Choice(0)
	if !ok || choice.Player != "Ana" {
		t.Fatalf("unexpected choice: %#v %t", choice, ok)
	}
	if _, ok := model.Choice(-1); ok {
		t.Fatal("negative row must not resolve")
	}
}
