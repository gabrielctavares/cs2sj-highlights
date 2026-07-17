//go:build windows

package gui

import "testing"

func TestChoiceTableModelChecksRowsAndSelectsAll(t *testing.T) {
	changes := 0
	model := newChoiceTableModel(func() { changes++ })
	model.SetChoices([]ClipChoice{
		{DemoName: "a", Map: "de_nuke", Player: "Ana", Type: "3K", Round: 2, Selected: true},
		{DemoName: "b", Map: "de_mirage", Player: "Bia", Type: "ACE", Round: 7, Selected: true},
	})
	if model.RowCount() != 2 || model.Value(0, 2) != "Ana" || !model.Checked(0) {
		t.Fatalf("unexpected model contents")
	}
	if err := model.SetChecked(0, false); err != nil {
		t.Fatal(err)
	}
	if model.Checked(0) || changes == 0 {
		t.Fatalf("checkbox change was not stored")
	}
	model.SetAll(false)
	if model.Checked(0) || model.Checked(1) || SelectedCount(model.Choices()) != 0 {
		t.Fatalf("set all did not clear selection")
	}
}
