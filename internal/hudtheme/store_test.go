package hudtheme

import (
	"path/filepath"
	"testing"
)

func TestSaveKeepsPreviousThemeWhenNewThemeIsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hud.json")
	valid := Theme{Version: 1, Name: "ok", Elements: []Element{{ID: "box", Type: Box, Anchor: TopLeft, Width: 10, Height: 10}}}
	if err := Save(path, valid); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.Elements[0].Width = 0
	if err := Save(path, invalid); err == nil {
		t.Fatal("expected save error")
	}
	got, err := Load(path)
	if err != nil || got.Name != "ok" {
		t.Fatalf("got %#v err=%v", got, err)
	}
}
