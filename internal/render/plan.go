package render

import (
	"slices"
	"sort"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

type RenderPass struct {
	Index int
	Clips []model.Highlight
}

func PartitionPasses(clips []model.Highlight) []RenderPass {
	sorted := slices.Clone(clips)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].StartTick != sorted[j].StartTick {
			return sorted[i].StartTick < sorted[j].StartTick
		}
		if sorted[i].EndTick != sorted[j].EndTick {
			return sorted[i].EndTick < sorted[j].EndTick
		}
		return sorted[i].ID < sorted[j].ID
	})
	passes := make([]RenderPass, 0)
	for _, clip := range sorted {
		placed := false
		for i := range passes {
			last := passes[i].Clips[len(passes[i].Clips)-1]
			if last.EndTick < clip.StartTick {
				passes[i].Clips = append(passes[i].Clips, clip)
				placed = true
				break
			}
		}
		if !placed {
			passes = append(passes, RenderPass{Index: len(passes) + 1, Clips: []model.Highlight{clip}})
		}
	}
	return passes
}
