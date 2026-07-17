package highlights

import (
	"fmt"
	"sort"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

type candidateAction struct {
	kill        model.Kill
	isKill      bool
	isAssist    bool
	flashAssist bool
}

type playerActions struct {
	player  model.Player
	actions []candidateAction
}

func Generate(timeline model.Timeline, rules Rules) ([]model.Highlight, []model.CandidateDiscard) {
	if timeline.TickRate <= 0 {
		return nil, []model.CandidateDiscard{{Code: "invalid_tick_rate"}}
	}
	var candidates []model.Highlight
	var discards []model.CandidateDiscard
	seen := make(map[string]struct{})
	for _, round := range timeline.Rounds {
		streams := make(map[string]*playerActions)
		add := func(player model.Player, action candidateAction) {
			identity, ok := candidateIdentity(player)
			if !ok {
				discards = append(discards, model.CandidateDiscard{Round: round.Number, Player: player, Code: "missing_protagonist"})
				return
			}
			stream := streams[identity]
			if stream == nil {
				stream = &playerActions{player: player}
				streams[identity] = stream
			}
			stream.actions = append(stream.actions, action)
		}
		for _, kill := range round.Kills {
			if kill.Tick <= 0 {
				discards = append(discards, model.CandidateDiscard{Round: round.Number, Player: kill.Killer, Code: "invalid_tick"})
				continue
			}
			add(kill.Killer, candidateAction{kill: kill, isKill: true})
			if assisterID, ok := candidateIdentity(kill.Assister); ok {
				killerID, _ := candidateIdentity(kill.Killer)
				if assisterID != killerID {
					add(kill.Assister, candidateAction{kill: kill, isAssist: true, flashAssist: kill.AssistedFlash})
				}
			}
		}

		identities := make([]string, 0, len(streams))
		for identity := range streams {
			identities = append(identities, identity)
		}
		sort.Strings(identities)
		for _, identity := range identities {
			stream := streams[identity]
			sort.SliceStable(stream.actions, func(i, j int) bool { return stream.actions[i].kill.Tick < stream.actions[j].kill.Tick })
			gap := int(rules.CandidateGapSeconds * timeline.TickRate)
			start := 0
			for start < len(stream.actions) {
				end := start + 1
				for end < len(stream.actions) && stream.actions[end].kill.Tick-stream.actions[end-1].kill.Tick <= gap {
					end++
				}
				candidate, discard, ok := buildGeneratedCandidate(round, stream.player, stream.actions[start:end], timeline.TickRate, rules)
				if discard.Code != "" {
					discards = append(discards, discard)
				}
				if ok {
					if _, duplicate := seen[candidate.ID]; duplicate {
						discards = append(discards, model.CandidateDiscard{Round: round.Number, Player: stream.player, Code: "duplicate_id"})
					} else {
						seen[candidate.ID] = struct{}{}
						candidates = append(candidates, candidate)
					}
				}
				start = end
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].StartTick != candidates[j].StartTick {
			return candidates[i].StartTick < candidates[j].StartTick
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates, discards
}

func buildGeneratedCandidate(round model.Round, player model.Player, actions []candidateAction, tickRate float64, rules Rules) (model.Highlight, model.CandidateDiscard, bool) {
	discard := model.CandidateDiscard{Round: round.Number, Player: player}
	if len(actions) == 0 {
		discard.Code = "empty_window"
		return model.Highlight{}, discard, false
	}
	firstTick := actions[0].kill.Tick
	lastTick := actions[len(actions)-1].kill.Tick
	startFloor := round.LiveTick
	if startFloor <= 0 {
		startFloor = round.StartTick
	}
	if firstTick < startFloor || round.EndTick > 0 && lastTick > round.EndTick {
		discard.Code = "action_outside_round"
		return model.Highlight{}, discard, false
	}
	start := max(startFloor, firstTick-int(rules.PreRollSeconds*tickRate))
	roundTail := round.EndTick + int(rules.RoundEndTailSeconds*tickRate)
	end := lastTick + int(rules.PostRollSeconds*tickRate)
	if round.EndTick > 0 {
		end = min(end, roundTail)
	}
	if round.MatchEnd {
		end = roundTail
	}
	if end <= start {
		discard.Code = "empty_window"
		return model.Highlight{}, discard, false
	}
	identity, ok := candidateIdentity(player)
	if !ok {
		discard.Code = "missing_protagonist"
		return model.Highlight{}, discard, false
	}
	kills := make([]model.Kill, 0, len(actions))
	context := model.CandidateContext{WonRound: player.Team == round.Winner, MatchPoint: round.MatchPoint, MatchEnd: round.MatchEnd, Overtime: round.Overtime}
	for _, action := range actions {
		kills = append(kills, action.kill)
		if action.isKill {
			context.KillCount++
		}
		if action.isAssist {
			context.AssistCount++
		}
		if action.flashAssist {
			context.FlashAssistCount++
		}
	}
	return model.Highlight{
		ID: fmt.Sprintf("r%02d-%s-t%d", round.Number, identity, start), Round: round.Number, Player: player,
		StartTick: start, EndTick: end, Actions: kills, Context: context, Status: model.ClipPending,
		ActionOffsets: actionOffsets(kills, start, end, tickRate),
	}, model.CandidateDiscard{}, true
}

func candidateIdentity(player model.Player) (string, bool) {
	if player.SteamID != 0 {
		return fmt.Sprintf("steam-%d", player.SteamID), true
	}
	if player.Slot != 0 {
		return fmt.Sprintf("slot-%d", player.Slot), true
	}
	return "", false
}
