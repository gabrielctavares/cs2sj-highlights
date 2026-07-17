package gui

import (
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestHUDModeFromChecks(t *testing.T) {
	tests := []struct {
		game, custom bool
		want         model.HUDMode
	}{
		{false, false, model.HUDNone},
		{true, false, model.HUDGame},
		{false, true, model.HUDCustom},
	}
	for _, test := range tests {
		if got := HUDModeFromChecks(test.game, test.custom); got != test.want {
			t.Fatalf("HUDModeFromChecks(%v, %v) = %q, want %q", test.game, test.custom, got, test.want)
		}
	}
}
