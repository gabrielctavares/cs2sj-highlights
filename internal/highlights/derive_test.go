package highlights

import (
	"slices"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestEnrichAddsDirectAndDerivedTags(t *testing.T) {
	protagonist := player("ana", model.TeamT, 1)
	protagonist.TeamName = "Time A"
	victim := player("bia", model.TeamCT, 2)
	victim.TeamName = "Time B"
	round := model.Round{Number: 1, LiveTick: 100, EndTick: 1000, Winner: model.TeamT, ScoreA: 15, ScoreB: 12, ScoreKnown: true, MatchPoint: true, Overtime: 1, Kills: []model.Kill{{
		Tick: 300, Killer: protagonist, Victim: victim, Weapon: "AK-47", IsHeadshot: true,
		PenetratedObjects: 1, ThroughSmoke: true, AttackerBlind: true, Distance: 1700, DistanceKnown: true, KillerHealth: 12,
	}}}
	candidates, _ := Generate(model.Timeline{TeamA: "Time A", TeamB: "Time B", TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	got := Enrich(round, candidates[0], 64, DefaultRules())
	for _, tag := range []string{"HEADSHOT", "WALLBANG", "SMOKE_KILL", "BLIND_KILL", "LONG_RANGE", "ENTRY", "LOW_HP", "MATCH_POINT", "OVERTIME"} {
		if !slices.Contains(got.Tags, tag) {
			t.Fatalf("missing %s in %#v", tag, got.Tags)
		}
	}
	if !got.Context.OpeningKill || got.Context.LowestKillerHealth != 12 {
		t.Fatalf("derived context missing: %#v", got.Context)
	}
}

func TestEnrichDetectsTradeAndFastSequence(t *testing.T) {
	protagonist := player("ana", model.TeamT, 1)
	teammate := player("cris", model.TeamT, 2)
	opponent := player("bia", model.TeamCT, 3)
	other := player("dani", model.TeamCT, 4)
	round := model.Round{Number: 2, LiveTick: 100, EndTick: 1200, Winner: model.TeamT, Kills: []model.Kill{
		{Tick: 200, Killer: opponent, Victim: teammate, Weapon: "AK-47"},
		{Tick: 300, Killer: protagonist, Victim: opponent, Weapon: "AK-47"},
		{Tick: 400, Killer: protagonist, Victim: other, Weapon: "AK-47"},
	}}
	candidates, _ := Generate(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	for _, candidate := range candidates {
		if candidate.Player.SteamID != protagonist.SteamID {
			continue
		}
		got := Enrich(round, candidate, 64, DefaultRules())
		if got.Context.TradeKills != 1 || !got.Context.FastSequence || !slices.Contains(got.Tags, "TRADE") || !slices.Contains(got.Tags, "FAST_MULTI") {
			t.Fatalf("trade/sequence missing: %#v", got)
		}
		return
	}
	t.Fatal("protagonist candidate missing")
}

func TestEnrichKeepsFlashAssistSeparateFromKillMerit(t *testing.T) {
	killer := player("ana", model.TeamT, 1)
	assister := player("cris", model.TeamT, 2)
	victim := player("bia", model.TeamCT, 3)
	round := model.Round{Number: 3, LiveTick: 100, EndTick: 800, Kills: []model.Kill{{
		Tick: 300, Killer: killer, Victim: victim, Assister: assister, Weapon: "AK-47", IsHeadshot: true, AssistedFlash: true,
	}}}
	candidates, _ := Generate(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	for _, candidate := range candidates {
		if candidate.Player.SteamID == assister.SteamID {
			got := Enrich(round, candidate, 64, DefaultRules())
			if !slices.Contains(got.Tags, "ASSIST") || !slices.Contains(got.Tags, "FLASH_ASSIST") || slices.Contains(got.Tags, "HEADSHOT") {
				t.Fatalf("assist inherited killer merit: %#v", got.Tags)
			}
			return
		}
	}
	t.Fatal("assist candidate missing")
}

func TestEnrichOmitsOpponentMatchPointAndLostMatchEnd(t *testing.T) {
	protagonist := player("RFL", model.TeamCT, 1)
	protagonist.TeamName = "ONU"
	victim := player("mmT", model.TeamT, 2)
	victim.TeamName = "Tedesco"
	round := model.Round{
		Number: 13, LiveTick: 100, EndTick: 1000, Winner: model.TeamT,
		ScoreA: 12, ScoreB: 0, ScoreKnown: true, MatchPoint: true, MatchEnd: true,
		Kills: []model.Kill{{Tick: 300, Killer: protagonist, Victim: victim, Weapon: "P2000", IsHeadshot: true}},
	}
	catalog, _ := BuildCatalog(model.Timeline{TeamA: "Tedesco", TeamB: "ONU", TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(catalog) != 1 {
		t.Fatalf("catalog = %#v", catalog)
	}
	got := catalog[0]
	if !slices.Contains(got.Tags, "HEADSHOT") || slices.Contains(got.Tags, "MATCH_POINT") || slices.Contains(got.Tags, "MATCH_END") {
		t.Fatalf("irrelevant match context in tags: %#v", got.Tags)
	}
}

func TestEnrichKeepsWinningCloseoutContext(t *testing.T) {
	protagonist := player("mdk", model.TeamT, 1)
	protagonist.TeamName = "Tedesco"
	victim := player("RFL", model.TeamCT, 2)
	victim.TeamName = "ONU"
	round := model.Round{
		Number: 13, LiveTick: 100, EndTick: 1000, Winner: model.TeamT,
		ScoreA: 12, ScoreB: 0, ScoreKnown: true, MatchPoint: true, MatchEnd: true,
		Kills: []model.Kill{{Tick: 300, Killer: protagonist, Victim: victim, Weapon: "Glock-18", IsHeadshot: true}},
	}
	catalog, _ := BuildCatalog(model.Timeline{TeamA: "Tedesco", TeamB: "ONU", TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(catalog) != 1 || !slices.Contains(catalog[0].Tags, "MATCH_POINT") || !slices.Contains(catalog[0].Tags, "MATCH_END") {
		t.Fatalf("winning closeout context missing: %#v", catalog)
	}
}
