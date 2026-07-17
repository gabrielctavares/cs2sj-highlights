package highlights

import (
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestEvaluateSpecialSingleKillReachesBroad(t *testing.T) {
	killer := model.Player{SteamID: 7, Name: "Ana", Team: model.TeamT}
	victim := model.Player{SteamID: 8, Name: "Bia", Team: model.TeamCT}
	candidate := model.Highlight{
		Player:  killer,
		Actions: []model.Kill{{Killer: killer, Victim: victim, Weapon: "AK-47", IsHeadshot: true, Distance: 1700, DistanceKnown: true}},
		Context: model.CandidateContext{KillCount: 1}, Tags: []string{"HEADSHOT", "LONG_RANGE"},
	}
	got := Evaluate(candidate, DefaultRules())
	if got.Individual.Score != 38 || got.Individual.Confidence == model.ConfidenceLow {
		t.Fatalf("unexpected individual evaluation: %#v", got.Individual)
	}
	if !strings.Contains(got.Individual.Explanation, "longa distância") {
		t.Fatalf("unexpected explanation: %q", got.Individual.Explanation)
	}
}

func TestEvaluateThreeKReachesBalanced(t *testing.T) {
	player := model.Player{SteamID: 7, Team: model.TeamT}
	actions := make([]model.Kill, 3)
	for index := range actions {
		actions[index] = model.Kill{Killer: player, Victim: model.Player{SteamID: uint64(index + 20), Team: model.TeamCT}, Weapon: "AK-47"}
	}
	candidate := model.Highlight{Player: player, Actions: actions, Context: model.CandidateContext{KillCount: 3, WonRound: true}, Tags: []string{"3K"}}
	got := Evaluate(candidate, DefaultRules())
	if got.Individual.Score != 62 || got.Editorial.Score != 63 {
		t.Fatalf("unexpected 3K evaluation: %#v", got)
	}
}

func TestEvaluateTwoFlashAssistsReachBroad(t *testing.T) {
	candidate := model.Highlight{Context: model.CandidateContext{AssistCount: 2, FlashAssistCount: 2}, Tags: []string{"ASSIST", "FLASH_ASSIST"}}
	got := Evaluate(candidate, DefaultRules())
	if got.Individual.Score != 44 || got.Editorial.Score != 58 {
		t.Fatalf("unexpected flash assist score: %#v", got)
	}
}
