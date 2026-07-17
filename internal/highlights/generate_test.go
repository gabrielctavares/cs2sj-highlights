package highlights

import (
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestGenerateKeepsSingleKillAndSplitsDistantActions(t *testing.T) {
	protagonist := player("ana", model.TeamT, 1)
	round := roundWithKills(2, 500, 4000, protagonist, []int{800, 900, 2500})
	got, discards := Generate(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(discards) != 0 {
		t.Fatalf("unexpected discards: %#v", discards)
	}
	if len(got) != 2 || len(got[0].Actions) != 2 || len(got[1].Actions) != 1 {
		t.Fatalf("unexpected clusters: %#v", got)
	}
	if got[0].ID == got[1].ID || got[0].StartTick >= got[1].StartTick {
		t.Fatalf("IDs/order are not stable: %#v", got)
	}
}

func TestGenerateCreatesAssistCandidateForAssister(t *testing.T) {
	killer := player("ana", model.TeamT, 1)
	assister := player("cris", model.TeamT, 2)
	victim := player("bia", model.TeamCT, 3)
	round := model.Round{Number: 1, LiveTick: 100, EndTick: 1000, Winner: model.TeamT, Kills: []model.Kill{{
		Tick: 300, Killer: killer, Victim: victim, Assister: assister, Weapon: "AK-47", AssistedFlash: true,
	}}}
	got, discards := Generate(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(discards) != 0 || len(got) != 2 {
		t.Fatalf("got=%#v discards=%#v", got, discards)
	}
	for _, candidate := range got {
		if candidate.Player.SteamID == assister.SteamID {
			if candidate.Context.KillCount != 0 || candidate.Context.AssistCount != 1 || candidate.Context.FlashAssistCount != 1 {
				t.Fatalf("unexpected assist context: %#v", candidate.Context)
			}
			return
		}
	}
	t.Fatal("assister candidate not generated")
}

func TestGenerateUsesSlotIdentityAndDiscardsMissingIdentity(t *testing.T) {
	withSlot := model.Player{Slot: 7, Name: "bot", Team: model.TeamT}
	missing := model.Player{Name: "unknown", Team: model.TeamT}
	victim := player("victim", model.TeamCT, 9)
	round := model.Round{Number: 3, LiveTick: 100, EndTick: 1000, Kills: []model.Kill{
		{Tick: 200, Killer: withSlot, Victim: victim, Weapon: "Glock-18"},
		{Tick: 300, Killer: missing, Victim: victim, Weapon: "Glock-18"},
	}}
	got, discards := Generate(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 1 || !strings.Contains(got[0].ID, "slot-7") {
		t.Fatalf("slot identity missing: %#v", got)
	}
	if len(discards) != 1 || discards[0].Code != "missing_protagonist" {
		t.Fatalf("unexpected discards: %#v", discards)
	}
}

func TestGenerateDiscardsInvalidTicksWithoutLosingValidCandidate(t *testing.T) {
	protagonist := player("ana", model.TeamT, 1)
	victim := player("bia", model.TeamCT, 2)
	round := model.Round{Number: 4, LiveTick: 100, EndTick: 1000, Kills: []model.Kill{
		{Tick: 0, Killer: protagonist, Victim: victim},
		{Tick: 300, Killer: protagonist, Victim: victim},
	}}
	got, discards := Generate(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 1 || len(discards) != 1 || discards[0].Code != "invalid_tick" {
		t.Fatalf("got=%#v discards=%#v", got, discards)
	}
}
