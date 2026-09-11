package highlights

import (
	"fmt"
	"slices"
	"sort"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func Enrich(round model.Round, candidate model.Highlight, tickRate float64, rules Rules) model.Highlight {
	context := candidate.Context
	context.WonRound = candidate.Player.Team == round.Winner
	context.MatchPoint = context.MatchPoint && context.WonRound
	context.MatchEnd = context.MatchEnd && context.WonRound
	context.Overtime = round.Overtime

	protagonistKills := make([]model.Kill, 0, len(candidate.Actions))
	for _, kill := range candidate.Actions {
		if samePlayer(kill.Killer, candidate.Player) {
			protagonistKills = append(protagonistKills, kill)
		}
	}
	context.KillCount = len(protagonistKills)
	context.LowestKillerHealth = lowestKnownHealth(protagonistKills)
	context.FastSequence = hasFastSequence(protagonistKills, int(rules.FastSequenceSeconds*tickRate))
	context.OpeningKill = len(round.Kills) > 0 && containsKill(protagonistKills, round.Kills[0])
	context.TradeKills = countTrades(round.Kills, protagonistKills, candidate.Player, int(rules.TradeWindowSeconds*tickRate))
	if len(protagonistKills) > 0 {
		clutch := findClutchOpportunities(round.Players, round.Kills)[playerIdentity(candidate.Player)]
		if clutch.opponents > 0 && clutch.tick <= protagonistKills[len(protagonistKills)-1].Tick && context.WonRound {
			context.ClutchOpponents = clutch.opponents
		}
	}

	tags := make([]string, 0, 16)
	switch {
	case context.KillCount >= 5:
		tags = append(tags, "ACE")
	case context.KillCount == 4:
		tags = append(tags, "4K")
	case context.KillCount == 3:
		tags = append(tags, "3K")
	}
	if context.AssistCount > 0 {
		tags = append(tags, "ASSIST")
	}
	if context.FlashAssistCount > 0 {
		tags = append(tags, "FLASH_ASSIST")
	}
	if grenadeMulti(protagonistKills, int(rules.GrenadeWindow*tickRate)) {
		tags = append(tags, "GRENADE_MULTI")
	}
	for _, direct := range []struct {
		tag string
		has func(model.Kill) bool
	}{
		{"HEADSHOT", func(k model.Kill) bool { return k.IsHeadshot }},
		{"WALLBANG", func(k model.Kill) bool { return k.PenetratedObjects > 0 }},
		{"NO_SCOPE", func(k model.Kill) bool { return k.NoScope }},
		{"SMOKE_KILL", func(k model.Kill) bool { return k.ThroughSmoke }},
		{"BLIND_KILL", func(k model.Kill) bool { return k.AttackerBlind }},
		{"LONG_RANGE", func(k model.Kill) bool { return k.DistanceKnown && k.Distance >= rules.LongRangeUnits }},
	} {
		if slices.ContainsFunc(protagonistKills, direct.has) {
			tags = append(tags, direct.tag)
		}
	}
	if context.OpeningKill {
		tags = append(tags, "ENTRY")
	}
	if context.TradeKills > 0 {
		tags = append(tags, "TRADE")
	}
	if context.FastSequence && context.KillCount >= 2 {
		tags = append(tags, "FAST_MULTI")
	}
	if context.LowestKillerHealth > 0 && context.LowestKillerHealth <= rules.LowHPThreshold {
		tags = append(tags, "LOW_HP")
	}
	if context.ClutchOpponents > 0 {
		tags = append(tags, fmt.Sprintf("CLUTCH_1V%d", context.ClutchOpponents))
	}
	if context.MatchPoint {
		tags = append(tags, "MATCH_POINT")
	}
	if context.Overtime > 0 {
		tags = append(tags, "OVERTIME")
	}
	if context.MatchEnd {
		tags = append(tags, "MATCH_END")
	}
	candidate.Context = context
	candidate.Tags = tags
	return candidate
}

func samePlayer(a, b model.Player) bool {
	if a.SteamID != 0 && b.SteamID != 0 {
		return a.SteamID == b.SteamID
	}
	return a.Slot != 0 && a.Slot == b.Slot
}

func containsKill(kills []model.Kill, target model.Kill) bool {
	return slices.ContainsFunc(kills, func(kill model.Kill) bool {
		return kill.Tick == target.Tick && samePlayer(kill.Killer, target.Killer) && samePlayer(kill.Victim, target.Victim)
	})
}

func lowestKnownHealth(kills []model.Kill) int {
	lowest := 0
	for _, kill := range kills {
		if kill.KillerHealth > 0 && (lowest == 0 || kill.KillerHealth < lowest) {
			lowest = kill.KillerHealth
		}
	}
	return lowest
}

func hasFastSequence(kills []model.Kill, window int) bool {
	for index := 1; index < len(kills); index++ {
		if kills[index].Tick-kills[index-1].Tick <= window {
			return true
		}
	}
	return false
}

func countTrades(roundKills, protagonistKills []model.Kill, protagonist model.Player, window int) int {
	ordered := slices.Clone(roundKills)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Tick < ordered[j].Tick })
	count := 0
	for _, current := range protagonistKills {
		for index := len(ordered) - 1; index >= 0; index-- {
			previous := ordered[index]
			if previous.Tick >= current.Tick {
				continue
			}
			if current.Tick-previous.Tick > window {
				break
			}
			if samePlayer(current.Victim, previous.Killer) && previous.Victim.Team == protagonist.Team {
				count++
				break
			}
		}
	}
	return count
}
