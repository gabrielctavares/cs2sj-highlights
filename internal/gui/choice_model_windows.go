//go:build windows

package gui

import "github.com/lxn/walk"

type choiceTableModel struct {
	walk.TableModelBase
	items     []ClipChoice
	onChanged func()
}

func newChoiceTableModel(onChanged func()) *choiceTableModel {
	return &choiceTableModel{onChanged: onChanged}
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
	default:
		return ""
	}
}

func (model *choiceTableModel) Checked(row int) bool {
	return model.items[row].Selected
}

func (model *choiceTableModel) SetChecked(row int, checked bool) error {
	model.items[row].Selected = checked
	if model.onChanged != nil {
		model.onChanged()
	}
	return nil
}

func (model *choiceTableModel) SetChoices(choices []ClipChoice) {
	model.items = append(model.items[:0], choices...)
	model.PublishRowsReset()
	if model.onChanged != nil {
		model.onChanged()
	}
}

func (model *choiceTableModel) SetAll(selected bool) {
	for index := range model.items {
		model.items[index].Selected = selected
	}
	model.PublishRowsReset()
	if model.onChanged != nil {
		model.onChanged()
	}
}

func (model *choiceTableModel) Choices() []ClipChoice {
	return append([]ClipChoice(nil), model.items...)
}
