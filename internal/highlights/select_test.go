package highlights

import (
	"reflect"
	"slices"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestActionOffsetsKeepFiveSecondSpacing(t *testing.T) {
	kills := []model.Kill{{Tick: 640}, {Tick: 960}}
	got := actionOffsets(kills, 0, 1280, 64)
	want := []float64{10, 15}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDefaultRulesKeepSevenSecondsAfterLastKill(t *testing.T) {
	player := model.Player{SteamID: 1, Name: "Ana", Team: model.TeamT}
	round := model.Round{Number: 1, LiveTick: 1, EndTick: 3000, Winner: model.TeamCT, Players: []model.Player{player}, Kills: []model.Kill{
		{Tick: 1000, Killer: player}, {Tick: 1064, Killer: player}, {Tick: 1128, Killer: player},
	}}
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 1 {
		t.Fatalf("highlights = %#v", got)
	}
	wantEnd := 1128 + 7*64
	if got[0].EndTick != wantEnd {
		t.Fatalf("end tick = %d, want %d", got[0].EndTick, wantEnd)
	}
}

func TestSelectAceAndClipWindow(t *testing.T) {
	round := roundWithKills(3, 1000, 3000, player("ana", model.TeamT, 1), []int{1300, 1400, 1500, 1600, 1700})
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 1 || !slices.Contains(got[0].Tags, "ACE") || got[0].Priority != 100 {
		t.Fatalf("unexpected ace: %#v", got)
	}
	if got[0].StartTick != 1000 || got[0].EndTick != 2148 {
		t.Fatalf("unexpected window: %d-%d", got[0].StartTick, got[0].EndTick)
	}
}

func TestSelectWonClutchKeepsRoundEnding(t *testing.T) {
	round := clutchRound(7, 2000, 4200, player("bia", model.TeamCT, 2), 3)
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 1 || !slices.Contains(got[0].Tags, "CLUTCH") {
		t.Fatalf("unexpected clutch: %#v", got)
	}
	if got[0].Priority != 88 || got[0].EndTick != 4328 {
		t.Fatalf("unexpected clutch priority/window: %#v", got[0])
	}
}

func TestSelectGrenadeMultiRequiresFiveSecondWindow(t *testing.T) {
	closeKills := grenadeRound(10, []int{1000, 1250})
	farKills := grenadeRound(11, []int{1000, 1400})
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{closeKills, farKills}}, DefaultRules())
	if len(got) != 1 || got[0].Round != 10 || !slices.Contains(got[0].Tags, "GRENADE_MULTI") {
		t.Fatalf("unexpected grenade selection: %#v", got)
	}
}

func TestSelectThreeK(t *testing.T) {
	round := roundWithKills(1, 500, 2000, player("tres", model.TeamT, 1), []int{800, 900, 1000})
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 1 || got[0].Priority != 70 || PrimaryTag(got[0].Tags) != "3K" {
		t.Fatalf("unexpected 3K: %#v", got)
	}
}

func TestSelectFourK(t *testing.T) {
	round := roundWithKills(1, 500, 2000, player("quatro", model.TeamCT, 1), []int{800, 900, 1000, 1100})
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 1 || got[0].Priority != 80 || PrimaryTag(got[0].Tags) != "4K" {
		t.Fatalf("unexpected 4K: %#v", got)
	}
	if slices.Contains(got[0].Tags, "3K") {
		t.Fatalf("4K must not include redundant 3K tag: %#v", got[0].Tags)
	}
}

func TestSelectAceDoesNotIncludeLowerMultiKillTags(t *testing.T) {
	round := roundWithKills(1, 500, 2200, player("ace", model.TeamCT, 1), []int{800, 900, 1000, 1100, 1200})
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 1 || !slices.Equal(got[0].Tags, []string{"ACE"}) {
		t.Fatalf("ACE must not include lower multi-kill tags: %#v", got)
	}
}

func TestSelectDoesNotPadMatchEndWithTwoK(t *testing.T) {
	round := roundWithKills(12, 500, 2000, player("final", model.TeamT, 1), []int{800, 900})
	round.MatchEnd = true
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 0 {
		t.Fatalf("match-end 2K must not be used as filler: %#v", got)
	}
}

func TestSelectLosingClutchExcluded(t *testing.T) {
	round := clutchRound(7, 2000, 4200, player("bia", model.TeamCT, 2), 3)
	round.Winner = model.TeamT
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	for _, highlight := range got {
		if highlight.Player.SteamID == 1002 && slices.Contains(highlight.Tags, "CLUTCH") {
			t.Fatalf("losing protagonist must not receive CLUTCH: %#v", got)
		}
	}
}

func TestSelectLimitsToTenAndRestoresChronology(t *testing.T) {
	rounds := make([]model.Round, 0, 11)
	for i := 1; i <= 11; i++ {
		rounds = append(rounds, roundWithKills(i, i*1000, i*1000+800, player("p", model.TeamT, i), []int{i*1000 + 100, i*1000 + 200, i*1000 + 300}))
	}
	got := Select(model.Timeline{TickRate: 64, Rounds: rounds}, DefaultRules())
	if len(got) != 10 {
		t.Fatalf("got %d highlights", len(got))
	}
	for i, highlight := range got {
		if highlight.Round != i+1 {
			t.Fatalf("unexpected rounds: %#v", got)
		}
	}
}

func TestSelectMergesOverlappingSamePlayerRound(t *testing.T) {
	round := grenadeRound(4, []int{900, 1000, 1100})
	got := Select(model.Timeline{TickRate: 64, Rounds: []model.Round{round}}, DefaultRules())
	if len(got) != 1 || !slices.Contains(got[0].Tags, "3K") || !slices.Contains(got[0].Tags, "GRENADE_MULTI") {
		t.Fatalf("unexpected merged tags: %#v", got)
	}
}

func TestSafeName(t *testing.T) {
	tests := map[string]string{
		"João da Silva!": "joao-da-silva",
		"  ANA__BIA  ":   "ana-bia",
		"---":            "jogador",
	}
	for input, want := range tests {
		if got := SafeName(input); got != want {
			t.Errorf("SafeName(%q) = %q, want %q", input, got, want)
		}
	}
}
