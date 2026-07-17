package demos

import (
	"os"
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

func TestRealDemoFrameAndIngameTickMapping(t *testing.T) {
	path := os.Getenv("CS2_TEST_DEMO")
	if path == "" {
		t.Skip("CS2_TEST_DEMO not set")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	parser := demoinfocs.NewParser(file)
	defer parser.Close()
	maxDifference := 0
	lastIngameKillTick := 0
	parser.RegisterEventHandler(func(event events.Kill) {
		difference := parser.CurrentFrame() - parser.GameState().IngameTick()
		if difference < 0 {
			difference = -difference
		}
		if difference > maxDifference {
			maxDifference = difference
		}
		lastIngameKillTick = parser.GameState().IngameTick()
	})
	if err := parser.ParseToEnd(); err != nil {
		t.Fatal(err)
	}
	t.Logf("maximum frame/tick difference: %d", maxDifference)
	if maxDifference == 0 {
		t.Fatal("fixture does not reproduce frame/tick drift")
	}
	timeline, err := (DemoParser{}).Parse(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	lastParsedKillTick := 0
	for _, round := range timeline.Rounds {
		for _, kill := range round.Kills {
			if kill.Tick > lastParsedKillTick {
				lastParsedKillTick = kill.Tick
			}
		}
	}
	if lastParsedKillTick != lastIngameKillTick {
		t.Fatalf("parser stored demo frame %d instead of server tick %d", lastParsedKillTick, lastIngameKillTick)
	}
}
