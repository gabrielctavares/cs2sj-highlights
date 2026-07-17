package render

import (
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestPartitionPassesSeparatesOverlaps(t *testing.T) {
	clips := []model.Highlight{
		{ID: "a", StartTick: 100, EndTick: 200},
		{ID: "b", StartTick: 150, EndTick: 250},
		{ID: "c", StartTick: 300, EndTick: 400},
	}
	got := PartitionPasses(clips)
	if len(got) != 2 {
		t.Fatalf("got %d passes", len(got))
	}
	if ids(got[0].Clips) != "a,c" || ids(got[1].Clips) != "b" {
		t.Fatalf("unexpected partition: %#v", got)
	}
}

func TestPartitionPassesSortsUnorderedInput(t *testing.T) {
	clips := []model.Highlight{{ID: "c", StartTick: 300, EndTick: 400}, {ID: "a", StartTick: 100, EndTick: 200}}
	got := PartitionPasses(clips)
	if len(got) != 1 || ids(got[0].Clips) != "a,c" {
		t.Fatalf("unexpected partition: %#v", got)
	}
	if clips[0].ID != "c" {
		t.Fatal("input slice was mutated")
	}
}

func TestPartitionPassesTreatsTouchingIntervalsAsOverlap(t *testing.T) {
	got := PartitionPasses([]model.Highlight{{ID: "a", StartTick: 100, EndTick: 200}, {ID: "b", StartTick: 200, EndTick: 300}})
	if len(got) != 2 {
		t.Fatalf("touching clips must use separate passes: %#v", got)
	}
}

func ids(clips []model.Highlight) string {
	values := make([]string, len(clips))
	for i, clip := range clips {
		values[i] = clip.ID
	}
	return strings.Join(values, ",")
}
