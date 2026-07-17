package media

import (
	"math"
	"slices"
)

type PacingSegment struct {
	Start float64
	End   float64
	Speed float64
}

// PlanPacing preserves action clusters, speeds up moderate quiet gaps and removes
// only the middle of long gaps. Offsets are seconds from the captured master.
func PlanPacing(duration float64, actionOffsets []float64) []PacingSegment {
	if duration <= 0 {
		return nil
	}
	actions := normalizedActions(duration, actionOffsets)
	if len(actions) == 0 {
		return []PacingSegment{{Start: 0, End: duration, Speed: 1}}
	}

	type block struct{ start, end float64 }
	blocks := make([]block, 0, len(actions))
	first, last := actions[0], actions[0]
	for _, action := range actions[1:] {
		if action-last <= 6 {
			last = action
			continue
		}
		blocks = append(blocks, block{start: math.Max(0, first-3), end: math.Min(duration, last+2)})
		first, last = action, action
	}
	blocks = append(blocks, block{start: math.Max(0, first-3), end: math.Min(duration, last+2)})

	merged := blocks[:0]
	for _, current := range blocks {
		if len(merged) > 0 && current.start <= merged[len(merged)-1].end {
			merged[len(merged)-1].end = math.Max(merged[len(merged)-1].end, current.end)
			continue
		}
		merged = append(merged, current)
	}

	segments := make([]PacingSegment, 0, len(merged)*3)
	cursor := 0.0
	for _, current := range merged {
		segments = appendQuietGap(segments, cursor, current.start)
		segments = appendPacingSegment(segments, current.start, current.end, 1)
		cursor = current.end
	}
	return appendQuietGap(segments, cursor, duration)
}

func normalizedActions(duration float64, offsets []float64) []float64 {
	actions := make([]float64, 0, len(offsets))
	for _, offset := range offsets {
		if math.IsNaN(offset) || math.IsInf(offset, 0) || offset < 0 || offset > duration {
			continue
		}
		actions = append(actions, offset)
	}
	slices.Sort(actions)
	return slices.Compact(actions)
}

func appendQuietGap(segments []PacingSegment, start, end float64) []PacingSegment {
	duration := end - start
	if duration <= 0 {
		return segments
	}
	if duration <= 6 {
		return appendPacingSegment(segments, start, end, 1)
	}
	if duration <= 12 {
		return appendPacingSegment(segments, start, end, 2)
	}
	segments = appendPacingSegment(segments, start, start+2, 2)
	return appendPacingSegment(segments, end-2, end, 2)
}

func appendPacingSegment(segments []PacingSegment, start, end, speed float64) []PacingSegment {
	if end-start <= 0.001 {
		return segments
	}
	if len(segments) > 0 {
		last := &segments[len(segments)-1]
		if last.Speed == speed && math.Abs(last.End-start) <= 0.001 {
			last.End = end
			return segments
		}
	}
	return append(segments, PacingSegment{Start: start, End: end, Speed: speed})
}
