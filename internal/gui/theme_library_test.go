package gui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndListHUDThemes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "themes")
	path, err := CreateHUDTheme(root, "Final de Inverno")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(path)) != "final-de-inverno" {
		t.Fatalf("unexpected theme path %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	themes, err := ListHUDThemes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != 1 || themes[0].Path != path || themes[0].Theme.Name != "Final de Inverno" {
		t.Fatalf("unexpected themes: %#v", themes)
	}
}
