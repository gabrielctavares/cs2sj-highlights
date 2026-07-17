package feedback

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestAppendWritesPathFreeAppendOnlyJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection-decisions.jsonl")
	decision := Decision{
		DemoName: "final.dem", HighlightID: "r01-p7-t100", Perspective: "individual", Breadth: "balanced",
		PlayerSteamID: 76561198000000001, Score: 82, Confidence: model.ConfidenceHigh,
		Factors: []model.ScoreFactor{{Code: "headshot", Points: 8}}, Selected: true,
	}
	if err := Append(path, []Decision{decision}); err != nil {
		t.Fatal(err)
	}
	decision.Selected = false
	if err := Append(path, []Decision{decision}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(data, []byte("\n")) != 2 || !bytes.Contains(data, []byte(`"highlight_id":"r01-p7-t100"`)) || !bytes.Contains(data, []byte(`"code":"headshot"`)) {
		t.Fatalf("unexpected log: %s", data)
	}
	if bytes.Contains(data, []byte(`C:\\`)) || bytes.Contains(data, []byte(t.TempDir())) {
		t.Fatalf("unsafe log: %s", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("log is not private: %o", info.Mode().Perm())
	}
}

func TestAppendRejectsUnsafeOrIncompleteDecisions(t *testing.T) {
	tests := []Decision{
		{DemoName: `C:\\demos\\match.dem`, HighlightID: "clip", Perspective: "editorial", Breadth: "balanced"},
		{DemoName: "match.dem", HighlightID: "", Perspective: "editorial", Breadth: "balanced"},
		{DemoName: "match.dem", HighlightID: `C:\\private\\clip`, Perspective: "editorial", Breadth: "balanced"},
		{DemoName: "match.dem", HighlightID: "clip", Perspective: "unknown", Breadth: "balanced"},
	}
	for _, decision := range tests {
		if err := Append(filepath.Join(t.TempDir(), "decisions.jsonl"), []Decision{decision}); err == nil || !strings.Contains(err.Error(), "decisão") {
			t.Fatalf("expected validation error for %#v, got %v", decision, err)
		}
	}
}
