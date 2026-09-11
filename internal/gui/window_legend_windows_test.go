//go:build windows

package gui

import (
	"strings"
	"testing"
)

func TestHighlightLegendExplainsScoringValues(t *testing.T) {
	legend := highlightLegendText()
	for _, want := range []string{
		"1K 20, 2K 38, 3K 62, 4K 80, 5K+ 95",
		"Flash assist +18, até 36",
		"Clutch: 12 + 6 por adversário",
		"Match point +15",
		"Multi-kill 3+ +10",
	} {
		if !strings.Contains(legend, want) {
			t.Fatalf("legend does not explain %q: %q", want, legend)
		}
	}
}
