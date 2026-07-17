package demos

import (
	"reflect"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

func TestMapTeam(t *testing.T) {
	tests := []struct {
		input common.Team
		want  model.Team
	}{
		{common.TeamTerrorists, model.TeamT},
		{common.TeamCounterTerrorists, model.TeamCT},
		{common.TeamSpectators, ""},
	}
	for _, test := range tests {
		if got := mapTeam(test.input); got != test.want {
			t.Errorf("mapTeam(%d) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestSnapshotPlayer(t *testing.T) {
	input := &common.Player{SteamID64: 76561198000000001, UserID: 7, EntityID: 3, Name: "Ana", Team: common.TeamCounterTerrorists}
	want := model.Player{SteamID: input.SteamID64, UserID: 7, Slot: 3, Name: "Ana", Team: model.TeamCT}
	if got := snapshotPlayer(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestAppendTeamNameKeepsTwoDistinctNames(t *testing.T) {
	names := appendTeamName(nil, " ONU ")
	names = appendTeamName(names, "onu")
	names = appendTeamName(names, "Tedesco")
	names = appendTeamName(names, "Ignored")
	want := []string{"ONU", "Tedesco"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %#v, want %#v", names, want)
	}
}

func TestLogicalScoreSurvivesSideSwap(t *testing.T) {
	a, b, ok := logicalScore("ONU", "Tedesco", "Tedesco", 7, "ONU", 5)
	if !ok || a != 5 || b != 7 {
		t.Fatalf("score = %d-%d known=%v", a, b, ok)
	}
}

func TestIsMatchPointInRegulationAndOvertime(t *testing.T) {
	tests := []struct {
		a, b, max, overtimeMax, overtime int
		want                             bool
	}{
		{12, 10, 24, 6, 0, true},
		{12, 12, 24, 6, 0, false},
		{15, 14, 24, 6, 1, true},
		{15, 15, 24, 6, 1, false},
		{7, 5, 0, 0, 0, false},
	}
	for _, test := range tests {
		if got := isMatchPoint(test.a, test.b, test.max, test.overtimeMax, test.overtime); got != test.want {
			t.Errorf("isMatchPoint(%+v) = %v", test, got)
		}
	}
}

func TestParseRuleIntRejectsMissingAndMalformedValues(t *testing.T) {
	values := map[string]string{"mp_maxrounds": " 24 ", "bad": "x"}
	if got := parseRuleInt(values, "mp_maxrounds"); got != 24 {
		t.Fatalf("max rounds = %d", got)
	}
	if parseRuleInt(values, "bad") != 0 || parseRuleInt(values, "missing") != 0 {
		t.Fatal("malformed or missing rules must be unknown")
	}
}

func TestSnapshotKillKeepsTechnicalMetadata(t *testing.T) {
	killer := &common.Player{SteamID64: 1, Name: "Ana", Team: common.TeamTerrorists}
	victim := &common.Player{SteamID64: 2, Name: "Bia", Team: common.TeamCounterTerrorists}
	assister := &common.Player{SteamID64: 3, Name: "Cris", Team: common.TeamTerrorists}
	event := events.Kill{
		Killer: killer, Victim: victim, Assister: assister, Weapon: common.NewEquipment(common.EqAK47),
		PenetratedObjects: 2, IsHeadshot: true, AssistedFlash: true, AttackerBlind: true,
		NoScope: true, ThroughSmoke: true, Distance: 1700,
	}
	got := snapshotKill(event, 1234)
	if got.Tick != 1234 || got.Killer.SteamID != 1 || got.Victim.SteamID != 2 || got.Assister.SteamID != 3 || got.Weapon != "AK-47" {
		t.Fatalf("identity metadata lost: %#v", got)
	}
	if got.PenetratedObjects != 2 || !got.IsHeadshot || !got.AssistedFlash || !got.AttackerBlind || !got.NoScope || !got.ThroughSmoke || !got.DistanceKnown || got.Distance != 1700 {
		t.Fatalf("technical metadata lost: %#v", got)
	}
}
