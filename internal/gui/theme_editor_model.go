package gui

import (
	"fmt"
	"sort"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
)

type EditorState struct {
	Theme      hudtheme.Theme
	SelectedID string
}

func NewEditorState(theme hudtheme.Theme) EditorState { return EditorState{Theme: theme} }

func (state EditorState) Selected() *hudtheme.Element {
	for index := range state.Theme.Elements {
		if state.Theme.Elements[index].ID == state.SelectedID {
			return &state.Theme.Elements[index]
		}
	}
	return nil
}

func (state *EditorState) SelectAt(x, y float64) {
	elements := append([]hudtheme.Element(nil), state.Theme.Elements...)
	sort.SliceStable(elements, func(i, j int) bool { return elements[i].ZIndex > elements[j].ZIndex })
	state.SelectedID = ""
	for _, element := range elements {
		if element.Visible && x >= element.X && x <= element.X+element.Width && y >= element.Y && y <= element.Y+element.Height {
			state.SelectedID = element.ID
			return
		}
	}
}

func (state *EditorState) MoveSelected(dx, dy float64) error {
	return state.SetSelectedElement(func(element *hudtheme.Element) {
		element.X = clamp(element.X+dx, 0, 100-element.Width)
		element.Y = clamp(element.Y+dy, 0, 100-element.Height)
	})
}

func (state *EditorState) ResizeSelected(dw, dh float64) error {
	return state.SetSelectedElement(func(element *hudtheme.Element) {
		element.Width = clamp(element.Width+dw, 1, 100-element.X)
		element.Height = clamp(element.Height+dh, 1, 100-element.Y)
	})
}

func (state *EditorState) SetSelectedElement(update func(*hudtheme.Element)) error {
	if update == nil {
		return fmt.Errorf("editor update is nil")
	}
	for index := range state.Theme.Elements {
		if state.Theme.Elements[index].ID != state.SelectedID {
			continue
		}
		candidate := state.Theme.Elements[index]
		update(&candidate)
		candidate.X = clamp(candidate.X, 0, 100-candidate.Width)
		candidate.Y = clamp(candidate.Y, 0, 100-candidate.Height)
		if candidate.Width <= 0 || candidate.Height <= 0 || candidate.X+candidate.Width > 100 || candidate.Y+candidate.Height > 100 {
			return fmt.Errorf("element %q has invalid bounds", candidate.ID)
		}
		state.Theme.Elements[index] = candidate
		return nil
	}
	return fmt.Errorf("no selected HUD element")
}

func (state *EditorState) AddElement(kind hudtheme.ElementType) error {
	if kind != hudtheme.Box && kind != hudtheme.Text && kind != hudtheme.Image {
		return fmt.Errorf("unsupported HUD element type %q", kind)
	}
	maxZ := 0
	for _, element := range state.Theme.Elements {
		maxZ = max(maxZ, element.ZIndex)
	}
	state.Theme.Elements = append(state.Theme.Elements, hudtheme.Element{ID: fmt.Sprintf("element-%d", len(state.Theme.Elements)+1), Type: kind, Anchor: hudtheme.TopLeft, X: 40, Y: 40, Width: 20, Height: 10, ZIndex: maxZ + 1, Visible: true, Color: "#FFFFFF", FontSize: 24})
	state.SelectedID = state.Theme.Elements[len(state.Theme.Elements)-1].ID
	return nil
}

func (state *EditorState) DeleteSelected() error {
	if len(state.Theme.Elements) <= 1 {
		return fmt.Errorf("a HUD needs at least one element")
	}
	for index, element := range state.Theme.Elements {
		if element.ID == state.SelectedID {
			state.Theme.Elements = append(state.Theme.Elements[:index], state.Theme.Elements[index+1:]...)
			state.SelectedID = ""
			return nil
		}
	}
	return fmt.Errorf("no selected HUD element")
}

func (state *EditorState) MoveLayer(direction int) error {
	for index := range state.Theme.Elements {
		if state.Theme.Elements[index].ID == state.SelectedID {
			target := index + direction
			if target < 0 || target >= len(state.Theme.Elements) {
				return nil
			}
			state.Theme.Elements[index].ZIndex, state.Theme.Elements[target].ZIndex = state.Theme.Elements[target].ZIndex, state.Theme.Elements[index].ZIndex
			return nil
		}
	}
	return fmt.Errorf("no selected HUD element")
}

func clamp(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
