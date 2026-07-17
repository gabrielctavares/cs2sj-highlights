package highlights

import "github.com/gabrielctavares/cs2sj-highlights/internal/model"

func player(name string, team model.Team, slot int) model.Player {
	return model.Player{SteamID: uint64(1000 + slot), UserID: slot, Slot: slot, Name: name, Team: team}
}

func roundWithKills(number, live, end int, protagonist model.Player, ticks []int) model.Round {
	opponentTeam := model.TeamCT
	if protagonist.Team == model.TeamCT {
		opponentTeam = model.TeamT
	}
	players := []model.Player{protagonist}
	for i := 1; i <= 4; i++ {
		players = append(players, player("mate", protagonist.Team, 10+i))
	}
	for i := 1; i <= 5; i++ {
		players = append(players, player("opponent", opponentTeam, 20+i))
	}
	kills := make([]model.Kill, 0, len(ticks))
	for i, tick := range ticks {
		kills = append(kills, model.Kill{Tick: tick, Killer: protagonist, Victim: players[5+i]})
	}
	return model.Round{Number: number, StartTick: live - 200, LiveTick: live, EndTick: end, Winner: protagonist.Team, Players: players, Kills: kills}
}

func grenadeRound(number int, ticks []int) model.Round {
	round := roundWithKills(number, 500, 2000, player("gabi", model.TeamT, number), ticks)
	for i := range round.Kills {
		round.Kills[i].Weapon = "hegrenade"
		round.Kills[i].IsGrenadeKill = true
	}
	return round
}

func clutchRound(number, live, end int, protagonist model.Player, opponents int) model.Round {
	ct2 := player("ct2", model.TeamCT, 11)
	ct3 := player("ct3", model.TeamCT, 12)
	ct4 := player("ct4", model.TeamCT, 13)
	ct5 := player("ct5", model.TeamCT, 14)
	t1 := player("t1", model.TeamT, 21)
	t2 := player("t2", model.TeamT, 22)
	t3 := player("t3", model.TeamT, 23)
	t4 := player("t4", model.TeamT, 24)
	t5 := player("t5", model.TeamT, 25)
	players := []model.Player{protagonist, ct2, ct3, ct4, ct5, t1, t2, t3, t4, t5}
	kills := []model.Kill{
		{Tick: 2200, Killer: protagonist, Victim: t1},
		{Tick: 2300, Killer: ct2, Victim: t2},
		{Tick: 2400, Killer: t3, Victim: ct2},
		{Tick: 2500, Killer: t4, Victim: ct3},
		{Tick: 2600, Killer: t5, Victim: ct4},
		{Tick: 2700, Killer: t3, Victim: ct5},
		{Tick: 3000, Killer: protagonist, Victim: t3},
		{Tick: 3200, Killer: protagonist, Victim: t4},
		{Tick: 3400, Killer: protagonist, Victim: t5},
	}
	_ = opponents
	return model.Round{Number: number, StartTick: live - 200, LiveTick: live, EndTick: end, Winner: model.TeamCT, Players: players, Kills: kills}
}
