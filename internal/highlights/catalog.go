package highlights

import (
	"slices"
	"sort"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

type Breadth string

const (
	BreadthRestricted Breadth = "restricted"
	BreadthBalanced   Breadth = "balanced"
	BreadthBroad      Breadth = "broad"
)

func (breadth Breadth) Threshold() int {
	switch breadth {
	case BreadthRestricted:
		return 85
	case BreadthBroad:
		return 35
	default:
		return 60
	}
}

func BuildCatalog(timeline model.Timeline, rules Rules) ([]model.Highlight, []model.CandidateDiscard) {
	candidates, discards := Generate(timeline, rules)
	rounds := make(map[int]model.Round, len(timeline.Rounds))
	for _, round := range timeline.Rounds {
		rounds[round.Number] = round
	}
	for index := range candidates {
		round, ok := rounds[candidates[index].Round]
		if !ok {
			discards = append(discards, model.CandidateDiscard{Round: candidates[index].Round, Player: candidates[index].Player, Code: "round_not_found"})
			continue
		}
		candidates[index] = Enrich(round, candidates[index], timeline.TickRate, rules)
		if candidates[index].Context.ClutchOpponents > 0 && candidates[index].Context.WonRound {
			candidates[index].EndTick = round.EndTick + int(rules.RoundEndTailSeconds*timeline.TickRate)
			candidates[index].ActionOffsets = actionOffsets(candidates[index].Actions, candidates[index].StartTick, candidates[index].EndTick, timeline.TickRate)
		}
		candidates[index] = Evaluate(candidates[index], rules)
	}
	return candidates, discards
}

func PlayerView(candidates []model.Highlight, breadth Breadth, steamID uint64) []model.Highlight {
	if steamID == 0 {
		return nil
	}
	result := make([]model.Highlight, 0)
	for _, candidate := range candidates {
		if candidate.Player.SteamID == steamID && candidate.Individual.Score >= breadth.Threshold() {
			result = append(result, candidate)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Individual.Score != result[j].Individual.Score {
			return result[i].Individual.Score > result[j].Individual.Score
		}
		return candidateLess(result[i], result[j])
	})
	return result
}

func EditorialView(candidates []model.Highlight, breadth Breadth) []model.Highlight {
	filtered := make([]model.Highlight, 0)
	for _, candidate := range candidates {
		if candidate.Editorial.Score >= breadth.Threshold() {
			filtered = append(filtered, candidate)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Editorial.Score != filtered[j].Editorial.Score {
			return filtered[i].Editorial.Score > filtered[j].Editorial.Score
		}
		return candidateLess(filtered[i], filtered[j])
	})

	playerCounts := make(map[string]int)
	tagCounts := make(map[string]int)
	teamCounts := make(map[model.Team]int)
	result := make([]model.Highlight, 0, len(filtered))
	for start := 0; start < len(filtered); {
		end := start + 1
		bandTop := filtered[start].Editorial.Score
		for end < len(filtered) && bandTop-filtered[end].Editorial.Score <= 8 {
			end++
		}
		band := slices.Clone(filtered[start:end])
		for len(band) > 0 {
			best := 0
			for index := 1; index < len(band); index++ {
				if editorialChoiceLess(band[index], band[best], playerCounts, tagCounts, teamCounts) {
					best = index
				}
			}
			chosen := band[best]
			result = append(result, chosen)
			playerCounts[playerIdentity(chosen.Player)]++
			tagCounts[PrimaryTag(chosen.Tags)]++
			if chosen.Player.Team != "" {
				teamCounts[chosen.Player.Team]++
			}
			band = append(band[:best], band[best+1:]...)
		}
		start = end
	}
	return result
}

func editorialChoiceLess(a, b model.Highlight, players map[string]int, tags map[string]int, teams map[model.Team]int) bool {
	penalty := func(candidate model.Highlight) int {
		value := players[playerIdentity(candidate.Player)]*8 + tags[PrimaryTag(candidate.Tags)]*4
		if candidate.Player.Team != "" {
			value += teams[candidate.Player.Team] * 3
		}
		return value
	}
	aPenalty, bPenalty := penalty(a), penalty(b)
	if aPenalty != bPenalty {
		return aPenalty < bPenalty
	}
	if a.Editorial.Score != b.Editorial.Score {
		return a.Editorial.Score > b.Editorial.Score
	}
	return candidateLess(a, b)
}

func candidateLess(a, b model.Highlight) bool {
	if a.Round != b.Round {
		return a.Round < b.Round
	}
	if a.StartTick != b.StartTick {
		return a.StartTick < b.StartTick
	}
	return a.ID < b.ID
}
