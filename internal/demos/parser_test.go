package demos

import (
	"reflect"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
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
