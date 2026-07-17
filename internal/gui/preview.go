package gui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/pipeline"
)

type ClipChoice struct {
	DemoPath string
	DemoName string
	Map      string
	ID       string
	Player   string
	Type     string
	Round    int
	Selected bool
}

func ChoicesFromResults(results []pipeline.Result) ([]ClipChoice, error) {
	choices := make([]ClipChoice, 0)
	for _, result := range results {
		if result.Err != nil {
			return nil, fmt.Errorf("analisar %s: %w", filepath.Base(result.DemoPath), result.Err)
		}
		demoName := strings.TrimSuffix(filepath.Base(result.DemoPath), filepath.Ext(result.DemoPath))
		for _, highlight := range result.Manifest.Highlights {
			choices = append(choices, ClipChoice{
				DemoPath: result.DemoPath,
				DemoName: demoName,
				Map:      result.Manifest.Map,
				ID:       highlight.ID,
				Player:   highlight.Player.Name,
				Type:     strings.Join(highlight.Tags, " + "),
				Round:    highlight.Round,
				Selected: true,
			})
		}
	}
	return choices, nil
}

func SelectedHighlights(choices []ClipChoice) map[string][]string {
	selected := make(map[string][]string)
	for _, choice := range choices {
		if choice.Selected {
			selected[choice.DemoPath] = append(selected[choice.DemoPath], choice.ID)
		}
	}
	return selected
}

func SelectedCount(choices []ClipChoice) int {
	count := 0
	for _, choice := range choices {
		if choice.Selected {
			count++
		}
	}
	return count
}
