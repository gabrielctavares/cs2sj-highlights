package highlights

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

type Rules struct {
	MaxHighlights       int
	PreRollSeconds      float64
	PostRollSeconds     float64
	RoundEndTailSeconds float64
	GrenadeWindow       float64
	CandidateGapSeconds float64
	FastSequenceSeconds float64
	TradeWindowSeconds  float64
	LongRangeUnits      float64
	LowHPThreshold      int
}

func DefaultRules() Rules {
	return Rules{
		MaxHighlights: 10, PreRollSeconds: 8, PostRollSeconds: 7, RoundEndTailSeconds: 2,
		GrenadeWindow: 5, CandidateGapSeconds: 12, FastSequenceSeconds: 5,
		TradeWindowSeconds: 5, LongRangeUnits: 1500, LowHPThreshold: 20,
	}
}

type clutchOpportunity struct {
	tick      int
	opponents int
}

type playerRound struct {
	player model.Player
	kills  []model.Kill
}

func Select(timeline model.Timeline, rules Rules) []model.Highlight {
	if timeline.TickRate <= 0 || rules.MaxHighlights <= 0 {
		return nil
	}
	var candidates []model.Highlight
	for _, round := range timeline.Rounds {
		kills := slices.Clone(round.Kills)
		sort.SliceStable(kills, func(i, j int) bool { return kills[i].Tick < kills[j].Tick })
		clutches := findClutchOpportunities(round.Players, kills)
		groups := groupKills(kills)
		for _, group := range groups {
			if candidate, ok := buildCandidate(round, group, clutches[playerIdentity(group.player)], timeline.TickRate, rules); ok {
				candidates = append(candidates, candidate)
			}
		}
	}

	candidates = mergeCandidates(candidates)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		if candidates[i].Round != candidates[j].Round {
			return candidates[i].Round < candidates[j].Round
		}
		if candidates[i].StartTick != candidates[j].StartTick {
			return candidates[i].StartTick < candidates[j].StartTick
		}
		return candidates[i].ID < candidates[j].ID
	})
	if len(candidates) > rules.MaxHighlights {
		candidates = candidates[:rules.MaxHighlights]
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].StartTick != candidates[j].StartTick {
			return candidates[i].StartTick < candidates[j].StartTick
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates
}

func groupKills(kills []model.Kill) []playerRound {
	byPlayer := make(map[string]int)
	groups := make([]playerRound, 0)
	for _, kill := range kills {
		key := playerIdentity(kill.Killer)
		index, ok := byPlayer[key]
		if !ok {
			index = len(groups)
			byPlayer[key] = index
			groups = append(groups, playerRound{player: kill.Killer})
		}
		groups[index].kills = append(groups[index].kills, kill)
	}
	return groups
}

func findClutchOpportunities(players []model.Player, kills []model.Kill) map[string]clutchOpportunity {
	alive := map[model.Team]map[string]model.Player{
		model.TeamT:  {},
		model.TeamCT: {},
	}
	for _, player := range players {
		if player.Team == model.TeamT || player.Team == model.TeamCT {
			alive[player.Team][playerIdentity(player)] = player
		}
	}
	result := make(map[string]clutchOpportunity)
	for _, kill := range kills {
		victims, ok := alive[kill.Victim.Team]
		if !ok {
			continue
		}
		key := playerIdentity(kill.Victim)
		if _, valid := victims[key]; !valid {
			continue
		}
		delete(victims, key)
		for _, team := range []model.Team{model.TeamT, model.TeamCT} {
			if len(alive[team]) != 1 || len(alive[opposite(team)]) == 0 {
				continue
			}
			for survivorKey := range alive[team] {
				if _, recorded := result[survivorKey]; !recorded {
					result[survivorKey] = clutchOpportunity{tick: kill.Tick, opponents: len(alive[opposite(team)])}
				}
			}
		}
	}
	return result
}

func buildCandidate(round model.Round, group playerRound, clutch clutchOpportunity, tickRate float64, rules Rules) (model.Highlight, bool) {
	if len(group.kills) == 0 {
		return model.Highlight{}, false
	}
	tags := make([]string, 0, 6)
	priority := 0
	if len(group.kills) >= 5 {
		tags = append(tags, "ACE")
		priority = max(priority, 100)
	} else if len(group.kills) >= 4 {
		tags = append(tags, "4K")
		priority = max(priority, 80)
	} else if len(group.kills) >= 3 {
		tags = append(tags, "3K")
		priority = max(priority, 70)
	}
	if clutch.opponents > 0 && group.player.Team == round.Winner {
		tags = append(tags, "CLUTCH")
		priority = max(priority, 85+clutch.opponents)
	}
	if grenadeMulti(group.kills, int(rules.GrenadeWindow*tickRate)) {
		tags = append(tags, "GRENADE_MULTI")
		priority = max(priority, 60)
	}
	if priority == 0 {
		return model.Highlight{}, false
	}
	if round.MatchEnd {
		priority += 3
	}

	firstAction := group.kills[0].Tick
	lastAction := group.kills[len(group.kills)-1].Tick
	startFloor := round.LiveTick
	if startFloor <= 0 {
		startFloor = round.StartTick
	}
	start := max(startFloor, firstAction-int(rules.PreRollSeconds*tickRate))
	roundTail := round.EndTick + int(rules.RoundEndTailSeconds*tickRate)
	end := min(lastAction+int(rules.PostRollSeconds*tickRate), roundTail)
	if clutch.opponents > 0 && group.player.Team == round.Winner || round.MatchEnd {
		end = roundTail
	}
	if end <= start {
		return model.Highlight{}, false
	}
	return model.Highlight{
		ID:            fmt.Sprintf("r%02d-p%d-t%d", round.Number, group.player.SteamID, start),
		Round:         round.Number,
		Player:        group.player,
		Tags:          tags,
		StartTick:     start,
		EndTick:       end,
		Priority:      priority,
		Status:        model.ClipPending,
		ActionOffsets: actionOffsets(round.Kills, start, end, tickRate),
	}, true
}

func actionOffsets(kills []model.Kill, start, end int, tickRate float64) []float64 {
	if tickRate <= 0 || end <= start {
		return nil
	}
	result := make([]float64, 0, len(kills))
	lastTick := -1
	for _, kill := range kills {
		if kill.Tick < start || kill.Tick > end || kill.Tick == lastTick {
			continue
		}
		result = append(result, float64(kill.Tick-start)/tickRate)
		lastTick = kill.Tick
	}
	return result
}

func grenadeMulti(kills []model.Kill, windowTicks int) bool {
	grenades := make([]int, 0, len(kills))
	for _, kill := range kills {
		if kill.IsGrenadeKill {
			grenades = append(grenades, kill.Tick)
		}
	}
	for i := 1; i < len(grenades); i++ {
		if grenades[i]-grenades[i-1] <= windowTicks {
			return true
		}
	}
	return false
}

func mergeCandidates(candidates []model.Highlight) []model.Highlight {
	merged := make([]model.Highlight, 0, len(candidates))
	for _, candidate := range candidates {
		found := -1
		for i := range merged {
			if merged[i].Round == candidate.Round && playerIdentity(merged[i].Player) == playerIdentity(candidate.Player) && intervalsOverlap(merged[i], candidate) {
				found = i
				break
			}
		}
		if found < 0 {
			merged = append(merged, candidate)
			continue
		}
		target := &merged[found]
		target.StartTick = min(target.StartTick, candidate.StartTick)
		target.EndTick = max(target.EndTick, candidate.EndTick)
		target.Priority = max(target.Priority, candidate.Priority)
		target.Tags = unionTags(target.Tags, candidate.Tags)
		target.ID = fmt.Sprintf("r%02d-p%d-t%d", target.Round, target.Player.SteamID, target.StartTick)
	}
	return merged
}

func intervalsOverlap(a, b model.Highlight) bool {
	return a.StartTick <= b.EndTick && b.StartTick <= a.EndTick
}

func unionTags(a, b []string) []string {
	all := append(slices.Clone(a), b...)
	result := make([]string, 0, len(all))
	multiKill := ""
	for _, tag := range []string{"ACE", "4K", "3K"} {
		if slices.Contains(all, tag) {
			multiKill = tag
			break
		}
	}
	for _, tag := range []string{"ACE", "CLUTCH", "4K", "3K", "GRENADE_MULTI"} {
		if (tag == "ACE" || tag == "4K" || tag == "3K") && tag != multiKill {
			continue
		}
		if slices.Contains(all, tag) {
			result = append(result, tag)
		}
	}
	return result
}

func opposite(team model.Team) model.Team {
	if team == model.TeamT {
		return model.TeamCT
	}
	return model.TeamT
}

func playerIdentity(player model.Player) string {
	if player.SteamID != 0 {
		return fmt.Sprintf("steam:%d", player.SteamID)
	}
	return fmt.Sprintf("slot:%d", player.Slot)
}

func SafeName(value string) string {
	var builder strings.Builder
	separator := false
	for _, raw := range strings.ToLower(value) {
		r := foldASCII(raw)
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(r)
			separator = false
			continue
		}
		separator = true
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "jogador"
	}
	return result
}

func foldASCII(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ã', 'ä', 'å':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'ô', 'õ', 'ö':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ç':
		return 'c'
	case 'ñ':
		return 'n'
	case 'ý', 'ÿ':
		return 'y'
	}
	if r < 128 {
		return r
	}
	return 0
}

func PrimaryTag(tags []string) string {
	for _, tag := range []string{
		"ACE", "CLUTCH_1V5", "CLUTCH_1V4", "CLUTCH_1V3", "CLUTCH_1V2", "CLUTCH_1V1", "4K", "3K",
		"GRENADE_MULTI", "NO_SCOPE", "WALLBANG", "SMOKE_KILL", "HEADSHOT", "FLASH_ASSIST", "LONG_RANGE",
		"FAST_MULTI", "ENTRY", "TRADE", "LOW_HP", "ASSIST",
	} {
		if slices.Contains(tags, tag) {
			return tag
		}
	}
	return "HIGHLIGHT"
}

func PrimaryTagLabel(tags []string) string {
	tag := PrimaryTag(tags)
	labels := map[string]string{
		"ACE":           "Ace",
		"4K":            "4K",
		"3K":            "3K",
		"GRENADE_MULTI": "Multi-kill de granada",
		"NO_SCOPE":      "No-scope",
		"WALLBANG":      "Wallbang",
		"SMOKE_KILL":    "Abate pela smoke",
		"HEADSHOT":      "Headshot",
		"FLASH_ASSIST":  "Assistência de flash",
		"LONG_RANGE":    "Longa distância",
		"FAST_MULTI":    "Sequência rápida",
		"ENTRY":         "Entry",
		"TRADE":         "Trade",
		"LOW_HP":        "Pouca vida",
		"ASSIST":        "Assistência",
		"HIGHLIGHT":     "Highlight",
	}
	if label, ok := labels[tag]; ok {
		return label
	}
	if strings.HasPrefix(tag, "CLUTCH_1V") {
		return "Clutch 1v" + strings.TrimPrefix(tag, "CLUTCH_1V")
	}
	return "Highlight"
}
