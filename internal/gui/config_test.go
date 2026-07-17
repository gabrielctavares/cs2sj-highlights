package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestConfigPathUsesLocalAppData(t *testing.T) {
	want := filepath.Join(`C:\Users\Ana\AppData\Local`, "CS2SJ-Demo", "config.json")
	if got := ConfigPath(`C:\Users\Ana\AppData\Local`); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoadConfigAbsentReturnsDefaults(t *testing.T) {
	got, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != ConfigSchemaVersion {
		t.Fatalf("got %#v", got)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	want := Config{SchemaVersion: ConfigSchemaVersion, CS2Path: `C:\cs2.exe`, InputDir: `C:\demos`, OutputDir: `D:\videos`, HUDMode: model.HUDCustom, HUDThemePath: `D:\themes\final\hud.json`, FavoriteSteamID: 76561198000000001}
	if err := SaveConfig(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	want.OutputDir = `E:\videos`
	if err := SaveConfig(path, want); err != nil {
		t.Fatal(err)
	}
	got, err = LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("replacement got %#v, want %#v", got, want)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains: %v", err)
	}
}

func TestSaveConfigForcesCurrentSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := SaveConfig(path, Config{SchemaVersion: 99, CS2Path: `C:\cs2.exe`}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != ConfigSchemaVersion {
		t.Fatalf("got schema %d", got.SchemaVersion)
	}
}

func TestLoadConfigRejectsCorruptAndUnsupportedData(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		data string
	}{
		{"corrupt", `{`},
		{"unsupported", `{"schema_version":99}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, test.name+".json")
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadConfig(path); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadConfigMigratesSchemaOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"input_dir":"C:\\demos","hud_mode":"none"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != ConfigSchemaVersion || got.InputDir != `C:\demos` || got.FavoriteSteamID != 0 {
		t.Fatalf("unexpected migrated config: %#v", got)
	}
}
