//go:build windows

package gui

import "github.com/lxn/walk"

type choiceTableModel struct {
	walk.TableModelBase
	items []ClipChoice
}

func newChoiceTableModel() *choiceTableModel {
	return &choiceTableModel{}
}

func (model *choiceTableModel) RowCount() int {
	return len(model.items)
}

func (model *choiceTableModel) Value(row, column int) interface{} {
	item := model.items[row]
	switch column {
	case 0:
		return item.DemoName
	case 1:
		return item.Map
	case 2:
		return item.Player
	case 3:
		return item.Type
	case 4:
		return item.Round
	case 5:
		return item.Score
	case 6:
		return item.Explanation
	default:
		return ""
	}
}

func (model *choiceTableModel) SetChoices(choices []ClipChoice) {
	model.items = append(model.items[:0], choices...)
	model.PublishRowsReset()
}

func (model *choiceTableModel) Choices() []ClipChoice {
	return append([]ClipChoice(nil), model.items...)
}

func (model *choiceTableModel) Choice(row int) (ClipChoice, bool) {
	if row < 0 || row >= len(model.items) {
		return ClipChoice{}, false
	}
	return model.items[row], true
}
