package media

import (
	"reflect"
	"testing"
)

func TestPlanPacingKeepsNearbyKillsAtNormalSpeed(t *testing.T) {
	got := PlanPacing(30, []float64{10, 15})
	if !containsNormalSegment(got, 7, 17) {
		t.Fatalf("nearby kills were not preserved at 1x: %#v", got)
	}
}

func TestPlanPacingAcceleratesCompleteLongInactiveGap(t *testing.T) {
	got := PlanPacing(35, []float64{5, 25})
	want := []PacingSegment{
		{Start: 0, End: 7, Speed: 1},
		{Start: 7, End: 22, Speed: 2},
		{Start: 22, End: 27, Speed: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("segments = %#v, want %#v", got, want)
	}
}

func TestPlanPacingEndsTwoSecondsAfterLastAction(t *testing.T) {
	want := []PacingSegment{{Start: 0, End: 10, Speed: 1}}
	if got := PlanPacing(15, []float64{8}); !reflect.DeepEqual(got, want) {
		t.Fatalf("segments = %#v, want %#v", got, want)
	}
}

func TestPlanPacingKeepsClipWithoutActionMetadata(t *testing.T) {
	want := []PacingSegment{{Start: 0, End: 18, Speed: 1}}
	if got := PlanPacing(18, nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("segments = %#v, want %#v", got, want)
	}
}

func containsNormalSegment(segments []PacingSegment, start, end float64) bool {
	for _, segment := range segments {
		if segment.Speed == 1 && segment.Start <= start && segment.End >= end {
			return true
		}
	}
	return false
}
