package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFormRequiresCS2DemoAndWritableOutput(t *testing.T) {
	root := t.TempDir()
	cs2 := touchFile(t, filepath.Join(root, "cs2.exe"))
	demos := filepath.Join(root, "demos")
	if err := os.Mkdir(demos, 0o755); err != nil {
		t.Fatal(err)
	}
	validDemos := filepath.Join(root, "valid-demos")
	if err := os.Mkdir(validDemos, 0o755); err != nil {
		t.Fatal(err)
	}
	touchFile(t, filepath.Join(validDemos, "match.dem"))
	outputFile := touchFile(t, filepath.Join(root, "output-file"))
	tests := []struct {
		name   string
		values FormValues
		want   string
	}{
		{"missing cs2", FormValues{InputDir: demos, OutputDir: filepath.Join(root, "out")}, "CS2"},
		{"cs2 is directory", FormValues{CS2Path: root, InputDir: demos, OutputDir: filepath.Join(root, "out")}, "CS2"},
		{"missing demo folder", FormValues{CS2Path: cs2, InputDir: filepath.Join(root, "missing"), OutputDir: filepath.Join(root, "out")}, "demos"},
		{"no demos", FormValues{CS2Path: cs2, InputDir: demos, OutputDir: filepath.Join(root, "out")}, ".dem"},
		{"missing output", FormValues{CS2Path: cs2, InputDir: validDemos}, "saída"},
		{"output file", FormValues{CS2Path: cs2, InputDir: validDemos, OutputDir: outputFile}, "saída"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateForm(test.values)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
				t.Fatalf("got %v, want text %q", err, test.want)
			}
		})
	}
}

func TestValidateFormAcceptsCaseInsensitiveDemoExtension(t *testing.T) {
	root := t.TempDir()
	cs2 := touchFile(t, filepath.Join(root, "CS2.EXE"))
	demos := filepath.Join(root, "demos")
	if err := os.Mkdir(demos, 0o755); err != nil {
		t.Fatal(err)
	}
	touchFile(t, filepath.Join(demos, "FINAL.DEM"))
	output := filepath.Join(root, "out")
	if err := ValidateForm(FormValues{CS2Path: cs2, InputDir: demos, OutputDir: output}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(output); err != nil || !info.IsDir() {
		t.Fatalf("output was not created: %v", err)
	}
}
