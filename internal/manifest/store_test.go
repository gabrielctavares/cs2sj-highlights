package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestSaveLoadAndCompatibility(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	want := model.Manifest{SchemaVersion: "manifest-v1", RulesVersion: "rules-v2", DemoSHA256: "demo", ConfigFingerprint: "cfg"}
	store := Store{}
	if err := store.Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	if !Compatible(got, "demo", "cfg") {
		t.Fatal("expected compatible manifest")
	}
	if Compatible(got, "changed", "cfg") {
		t.Fatal("changed demo hash must invalidate resume")
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file leaked: %v", err)
	}
}

func TestSHA256File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value.txt")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFingerprintStableForEqualStructs(t *testing.T) {
	type config struct {
		Rules  string
		Width  int
		Height int
	}
	a, err := Fingerprint(config{Rules: "rules-v1", Width: 1920, Height: 1080})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Fingerprint(config{Rules: "rules-v1", Width: 1920, Height: 1080})
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a == "" {
		t.Fatalf("unstable fingerprints: %q %q", a, b)
	}
}

func TestSaveReplacesExistingManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "manifest.json")
	store := Store{}
	if err := store.Save(path, model.Manifest{DemoSHA256: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(path, model.Manifest{DemoSHA256: "new"}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.DemoSHA256 != "new" {
		t.Fatalf("got %q", got.DemoSHA256)
	}
}
