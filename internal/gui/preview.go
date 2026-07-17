package gui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/feedback"
	"github.com/gabrielctavares/cs2sj-highlights/internal/highlights"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/pipeline"
)

type Perspective string

const (
	PerspectiveEditorial  Perspective = "editorial"
	PerspectiveIndividual Perspective = "individual"
)

type PlayerOption struct {
	SteamID  uint64
	Name     string
	TeamName string
}

type ClipChoice struct {
	DemoPath    string
	DemoName    string
	Map         string
	ID          string
	Player      string
	SteamID     uint64
	TeamName    string
	Type        string
	Round       int
	Score       int
	Explanation string
	Perspective Perspective
	Factors     []model.ScoreFactor
	Confidence  model.Confidence
	Selected    bool // compatibilidade temporária com a tabela legada do Windows
}

type previewCandidate struct {
	choice    ClipChoice
	highlight model.Highlight
}

type PreviewState struct {
	candidates []previewCandidate
	selected   map[string]ClipChoice
	order      []string
}

func NewPreviewState(results []pipeline.Result) (*PreviewState, error) {
	state := &PreviewState{selected: make(map[string]ClipChoice)}
	for _, result := range results {
		if result.Err != nil {
			return nil, fmt.Errorf("analisar %s: %w", filepath.Base(result.DemoPath), result.Err)
		}
		demoName := strings.TrimSuffix(filepath.Base(result.DemoPath), filepath.Ext(result.DemoPath))
		for _, highlight := range result.Manifest.Highlights {
			state.candidates = append(state.candidates, previewCandidate{
				choice: ClipChoice{
					DemoPath: result.DemoPath,
					DemoName: demoName,
					Map:      result.Manifest.Map,
					ID:       highlight.ID,
					Player:   highlight.Player.Name,
					SteamID:  highlight.Player.SteamID,
					TeamName: highlight.Player.TeamName,
					Type:     strings.Join(highlight.Tags, " + "),
					Round:    highlight.Round,
				},
				highlight: highlight,
			})
		}
	}
	return state, nil
}

func (state *PreviewState) Players() []PlayerOption {
	byID := make(map[uint64]PlayerOption)
	for _, candidate := range state.candidates {
		player := candidate.highlight.Player
		if player.SteamID == 0 {
			continue
		}
		if _, exists := byID[player.SteamID]; !exists {
			byID[player.SteamID] = PlayerOption{SteamID: player.SteamID, Name: player.Name, TeamName: player.TeamName}
		}
	}
	players := make([]PlayerOption, 0, len(byID))
	for _, player := range byID {
		players = append(players, player)
	}
	sort.SliceStable(players, func(i, j int) bool {
		left := strings.ToLower(players[i].Name)
		right := strings.ToLower(players[j].Name)
		if left != right {
			return left < right
		}
		return players[i].SteamID < players[j].SteamID
	})
	return players
}

func (state *PreviewState) PreferredPlayer(favorite uint64) uint64 {
	players := state.Players()
	for _, player := range players {
		if player.SteamID == favorite {
			return favorite
		}
	}
	if len(players) == 0 {
		return 0
	}
	return players[0].SteamID
}

func (state *PreviewState) EditorialChoices(breadth highlights.Breadth) []ClipChoice {
	return state.choices(highlights.EditorialView(state.highlights(), breadth), PerspectiveEditorial)
}

func (state *PreviewState) PlayerChoices(breadth highlights.Breadth, steamID uint64) []ClipChoice {
	return state.choices(highlights.PlayerView(state.highlights(), breadth, steamID), PerspectiveIndividual)
}

func (state *PreviewState) Add(choice ClipChoice) {
	key := choiceKey(choice.DemoPath, choice.ID)
	if _, exists := state.selected[key]; exists {
		return
	}
	state.selected[key] = choice
	state.order = append(state.order, key)
}

func (state *PreviewState) Remove(demoPath, id string) {
	key := choiceKey(demoPath, id)
	if _, exists := state.selected[key]; !exists {
		return
	}
	delete(state.selected, key)
	for index, orderedKey := range state.order {
		if orderedKey == key {
			state.order = append(state.order[:index], state.order[index+1:]...)
			break
		}
	}
}

func (state *PreviewState) FinalChoices() []ClipChoice {
	choices := make([]ClipChoice, 0, len(state.order))
	for _, key := range state.order {
		if choice, exists := state.selected[key]; exists {
			choices = append(choices, choice)
		}
	}
	return choices
}

func (state *PreviewState) SelectedHighlights() map[string][]string {
	return selectedHighlights(state.FinalChoices(), true)
}

func (state *PreviewState) highlights() []model.Highlight {
	items := make([]model.Highlight, 0, len(state.candidates))
	for _, candidate := range state.candidates {
		item := candidate.highlight
		item.ID = choiceKey(candidate.choice.DemoPath, candidate.choice.ID)
		items = append(items, item)
	}
	return items
}

func (state *PreviewState) choices(items []model.Highlight, perspective Perspective) []ClipChoice {
	byKey := make(map[string]ClipChoice, len(state.candidates))
	for _, candidate := range state.candidates {
		byKey[choiceKey(candidate.choice.DemoPath, candidate.choice.ID)] = candidate.choice
	}
	choices := make([]ClipChoice, 0, len(items))
	for _, item := range items {
		choice, exists := byKey[item.ID]
		if !exists {
			continue
		}
		choice.Perspective = perspective
		if perspective == PerspectiveIndividual {
			choice.Score = item.Individual.Score
			choice.Explanation = item.Individual.Explanation
			choice.Factors = append([]model.ScoreFactor(nil), item.Individual.Factors...)
			choice.Confidence = item.Individual.Confidence
		} else {
			choice.Score = item.Editorial.Score
			choice.Explanation = item.Editorial.Explanation
			choice.Factors = append([]model.ScoreFactor(nil), item.Editorial.Factors...)
			choice.Confidence = item.Editorial.Confidence
		}
		choices = append(choices, choice)
	}
	return choices
}

func FeedbackDecisions(editorial, individual, final []ClipChoice, breadth highlights.Breadth) []feedback.Decision {
	selected := make(map[string]struct{}, len(final))
	for _, choice := range final {
		selected[choiceKey(choice.DemoPath, choice.ID)] = struct{}{}
	}
	visible := make([]ClipChoice, 0, len(editorial)+len(individual))
	visible = append(visible, editorial...)
	visible = append(visible, individual...)
	decisions := make([]feedback.Decision, 0, len(visible))
	for _, choice := range visible {
		_, included := selected[choiceKey(choice.DemoPath, choice.ID)]
		decisions = append(decisions, feedback.Decision{
			DemoName:      filepath.Base(choice.DemoPath),
			HighlightID:   choice.ID,
			Perspective:   string(choice.Perspective),
			Breadth:       string(breadth),
			PlayerSteamID: choice.SteamID,
			Score:         choice.Score,
			Factors:       append([]model.ScoreFactor(nil), choice.Factors...),
			Confidence:    choice.Confidence,
			Selected:      included,
		})
	}
	return decisions
}

func choiceKey(demoPath, id string) string {
	return strings.ToLower(filepath.Clean(demoPath)) + "\x00" + id
}

// ChoicesFromResults mantém a API usada pela janela legada até a migração visual.
func ChoicesFromResults(results []pipeline.Result) ([]ClipChoice, error) {
	state, err := NewPreviewState(results)
	if err != nil {
		return nil, err
	}
	choices := make([]ClipChoice, 0, len(state.candidates))
	for _, candidate := range state.candidates {
		choice := candidate.choice
		choice.Selected = true
		choices = append(choices, choice)
	}
	return choices, nil
}

func SelectedHighlights(choices []ClipChoice) map[string][]string {
	return selectedHighlights(choices, false)
}

func selectedHighlights(choices []ClipChoice, explicit bool) map[string][]string {
	selected := make(map[string][]string)
	seen := make(map[string]struct{})
	for _, choice := range choices {
		if !explicit && !choice.Selected {
			continue
		}
		key := choiceKey(choice.DemoPath, choice.ID)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		selected[choice.DemoPath] = append(selected[choice.DemoPath], choice.ID)
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
