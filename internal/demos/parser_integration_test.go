package demos

import (
	"context"
	"os"
	"testing"
)

func TestDemoParserRealDemo(t *testing.T) {
	path := os.Getenv("CS2_TEST_DEMO")
	if path == "" {
		t.Skip("set CS2_TEST_DEMO to a readable CS2 .dem file")
	}
	timeline, err := (DemoParser{}).Parse(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if timeline.TickRate <= 0 || len(timeline.Rounds) == 0 {
		t.Fatalf("incomplete timeline: %#v", timeline)
	}
	if !timeline.Rounds[len(timeline.Rounds)-1].MatchEnd {
		t.Fatal("last completed round must be marked as match end")
	}
	if timeline.TeamA == "" || timeline.TeamB == "" {
		t.Fatalf("demo team names were not extracted: %#v", timeline)
	}
	for _, round := range timeline.Rounds {
		for _, kill := range round.Kills {
			if kill.Killer.TeamName == "" {
				t.Fatalf("killer %q has no stable team name", kill.Killer.Name)
			}
		}
	}
	t.Logf("demo teams: %q vs %q", timeline.TeamA, timeline.TeamB)
}
