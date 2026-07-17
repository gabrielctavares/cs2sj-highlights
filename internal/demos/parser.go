package demos

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	msg "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

type Parser interface {
	Parse(context.Context, string) (model.Timeline, error)
}

type DemoParser struct{}

func (DemoParser) Parse(ctx context.Context, path string) (timeline model.Timeline, err error) {
	file, err := os.Open(path)
	if err != nil {
		return model.Timeline{}, fmt.Errorf("open demo %q: %w", path, err)
	}

	p := demoinfocs.NewParser(file)
	defer func() {
		if closeErr := p.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close parser for %q: %w", path, closeErr)
		}
	}()

	timeline = model.Timeline{DemoPath: path, Map: "unknown"}
	teamNames := make([]string, 0, 2)
	captureTeamNames := func() {
		state := p.GameState()
		if state == nil {
			return
		}
		if team := state.TeamTerrorists(); team != nil {
			teamNames = appendTeamName(teamNames, team.ClanName())
		}
		if team := state.TeamCounterTerrorists(); team != nil {
			teamNames = appendTeamName(teamNames, team.ClanName())
		}
		if len(teamNames) > 0 {
			timeline.TeamA = teamNames[0]
		}
		if len(teamNames) > 1 {
			timeline.TeamB = teamNames[1]
		}
	}
	var current *model.Round
	currentTick := func() int {
		if state := p.GameState(); state != nil && state.IngameTick() > 0 {
			return state.IngameTick()
		}
		return p.CurrentFrame()
	}
	finalizePreviousRound := func(frame int) {
		if current == nil {
			return
		}
		if current.LiveTick <= 0 || len(current.Players) == 0 {
			current = nil
			return
		}
		if current.EndTick <= 0 {
			current.EndTick = frame
		}
		if current.EndTick <= current.LiveTick {
			current = nil
			return
		}
		timeline.Rounds = append(timeline.Rounds, *current)
		current = nil
	}

	p.RegisterNetMessageHandler(func(e *msg.CSVCMsg_ServerInfo) {
		if e != nil && e.GetMapName() != "" {
			timeline.Map = e.GetMapName()
		}
	})
	p.RegisterEventHandler(func(events.RoundStart) {
		finalizePreviousRound(currentTick())
		current = &model.Round{Number: len(timeline.Rounds) + 1, StartTick: currentTick()}
	})
	p.RegisterEventHandler(func(events.RoundFreezetimeEnd) {
		if current == nil || p.GameState().IsWarmupPeriod() {
			return
		}
		captureTeamNames()
		var tName, ctName string
		var tScore, ctScore int
		if team := p.GameState().TeamTerrorists(); team != nil {
			tName, tScore = team.ClanName(), team.Score()
		}
		if team := p.GameState().TeamCounterTerrorists(); team != nil {
			ctName, ctScore = team.ClanName(), team.Score()
		}
		current.ScoreA, current.ScoreB, current.ScoreKnown = logicalScore(timeline.TeamA, timeline.TeamB, tName, tScore, ctName, ctScore)
		current.LiveTick = currentTick()
		current.Players = snapshotPlaying(p.GameState().Participants().Playing())
	})
	p.RegisterEventHandler(func(e events.Kill) {
		if current == nil || p.GameState().IsWarmupPeriod() || e.Killer == nil || e.Victim == nil || e.Killer == e.Victim {
			return
		}
		weapon := "unknown"
		grenade := false
		if e.Weapon != nil {
			weapon = e.Weapon.String()
			grenade = e.Weapon.Type == common.EqHE || e.Weapon.Type == common.EqMolotov || e.Weapon.Type == common.EqIncendiary
		}
		current.Kills = append(current.Kills, model.Kill{
			Tick: currentTick(), Killer: snapshotPlayer(e.Killer), Victim: snapshotPlayer(e.Victim),
			Weapon: weapon, IsGrenadeKill: grenade,
		})
	})
	p.RegisterEventHandler(func(e events.RoundEnd) {
		if current == nil || p.GameState().IsWarmupPeriod() {
			return
		}
		captureTeamNames()
		current.EndTick = currentTick()
		current.Winner = mapTeam(e.Winner)
		if team := p.GameState().TeamTerrorists(); team != nil {
			current.ScoreT = team.Score()
		}
		if team := p.GameState().TeamCounterTerrorists(); team != nil {
			current.ScoreCT = team.Score()
		}
		if current.Winner == model.TeamT {
			current.ScoreT++
		}
		if current.Winner == model.TeamCT {
			current.ScoreCT++
		}
	})
	p.RegisterEventHandler(func(events.RoundEndOfficial) {
		finalizePreviousRound(currentTick())
	})

	parseDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			p.Cancel()
		case <-parseDone:
		}
	}()
	parseErr := p.ParseToEnd()
	close(parseDone)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return model.Timeline{}, fmt.Errorf("parse demo %q: %w", path, ctxErr)
	}
	if parseErr != nil {
		return model.Timeline{}, fmt.Errorf("parse demo %q: %w", path, parseErr)
	}

	finalizePreviousRound(currentTick())
	timeline.TickRate = p.TickRate()
	if len(timeline.Rounds) > 0 {
		timeline.Rounds[len(timeline.Rounds)-1].MatchEnd = true
	}
	return timeline, nil
}

func mapTeam(team common.Team) model.Team {
	switch team {
	case common.TeamTerrorists:
		return model.TeamT
	case common.TeamCounterTerrorists:
		return model.TeamCT
	default:
		return ""
	}
}

func snapshotPlayer(player *common.Player) model.Player {
	if player == nil {
		return model.Player{}
	}
	result := model.Player{
		SteamID: player.SteamID64,
		UserID:  player.UserID,
		Slot:    player.EntityID,
		Name:    player.Name,
		Team:    mapTeam(player.Team),
	}
	if player.TeamState != nil {
		result.TeamName = strings.TrimSpace(player.TeamState.ClanName())
	}
	return result
}

func snapshotPlaying(players []*common.Player) []model.Player {
	result := make([]model.Player, 0, len(players))
	for _, player := range players {
		if player != nil && (player.Team == common.TeamTerrorists || player.Team == common.TeamCounterTerrorists) {
			result = append(result, snapshotPlayer(player))
		}
	}
	return result
}

func appendTeamName(names []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || len(names) >= 2 {
		return names
	}
	for _, existing := range names {
		if strings.EqualFold(existing, value) {
			return names
		}
	}
	return append(names, value)
}

func logicalScore(teamA, teamB, tName string, tScore int, ctName string, ctScore int) (scoreA, scoreB int, known bool) {
	teamA, teamB = strings.TrimSpace(teamA), strings.TrimSpace(teamB)
	tName, ctName = strings.TrimSpace(tName), strings.TrimSpace(ctName)
	if teamA == "" || teamB == "" || tName == "" || ctName == "" {
		return 0, 0, false
	}
	switch {
	case strings.EqualFold(teamA, tName) && strings.EqualFold(teamB, ctName):
		return tScore, ctScore, true
	case strings.EqualFold(teamA, ctName) && strings.EqualFold(teamB, tName):
		return ctScore, tScore, true
	default:
		return 0, 0, false
	}
}
