//go:build windows

package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type themeEditorWindow struct {
	path     string
	state    EditorState
	dialog   *walk.Dialog
	canvas   *walk.CustomWidget
	layers   *walk.ListBox
	name     *walk.Label
	text     *walk.LineEdit
	binding  *walk.LineEdit
	fontSize *walk.LineEdit
	anchor   *walk.LineEdit
	opacity  *walk.LineEdit
	asset    *walk.LineEdit
	color    *walk.LineEdit
	x        *walk.LineEdit
	y        *walk.LineEdit
	width    *walk.LineEdit
	height   *walk.LineEdit
	visible  *walk.CheckBox
	dragging bool
	resizing bool
	lastX    float64
	lastY    float64
	updating bool
}

func (window *applicationWindow) openHUDThemeEditor() {
	path := window.hudThemePath()
	if path == "" {
		var err error
		path, err = DefaultHUDThemePath(filepath.Join(filepath.Dir(window.configPath), "themes"))
		if err != nil {
			window.showError("Não foi possível criar o tema", err)
			return
		}
		window.hudThemeEdit.SetText(path)
		window.customHUD.SetChecked(true)
	}
	if err := RunThemeEditor(window.mainWindow, path); err != nil {
		window.showError("Não foi possível abrir o editor", err)
	}
}

func RunThemeEditor(owner walk.Form, path string) error {
	theme, err := hudtheme.Load(path)
	if err != nil {
		return err
	}
	ApplyGuidedLayout(&theme)
	editor := &themeEditorWindow{path: path, state: NewEditorState(theme)}
	_, err = (Dialog{AssignTo: &editor.dialog, Title: "Editor visual de HUD — 16:9", MinSize: Size{Width: 1180, Height: 720}, Layout: HBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 10}, Children: []Widget{
		Composite{MinSize: Size{Width: 200}, Layout: VBox{Spacing: 6}, Children: []Widget{
			Label{Text: "Cores dos times"},
			PushButton{Text: "Azul × Laranja", OnClicked: func() { editor.applyPalette(TeamBlue, TeamOrange) }},
			PushButton{Text: "Vermelho × Verde", OnClicked: func() { editor.applyPalette(TeamRed, TeamGreen) }},
			PushButton{Text: "Roxo × Azul", OnClicked: func() { editor.applyPalette(TeamPurple, TeamBlue) }},
			Label{Text: "Camadas"}, ListBox{AssignTo: &editor.layers, Model: editor.layerNames(), StretchFactor: 1, OnCurrentIndexChanged: editor.selectLayer},
			PushButton{Text: "+ Caixa", OnClicked: func() { editor.add(hudtheme.Box) }}, PushButton{Text: "+ Texto", OnClicked: func() { editor.add(hudtheme.Text) }}, PushButton{Text: "+ Imagem", OnClicked: editor.addImage}, PushButton{Text: "Subir camada", OnClicked: func() { editor.moveLayer(1) }}, PushButton{Text: "Descer camada", OnClicked: func() { editor.moveLayer(-1) }}, PushButton{Text: "Remover", OnClicked: editor.remove},
		}},
		Composite{Layout: VBox{Spacing: 6}, StretchFactor: 1, Children: []Widget{
			Label{Text: "Prévia 16:9 — clique, arraste ou use a alça inferior direita para redimensionar"},
			CustomWidget{AssignTo: &editor.canvas, MinSize: Size{Width: 640, Height: 360}, Paint: editor.paint, PaintMode: PaintBuffered, InvalidatesOnResize: true, StretchFactor: 1, OnMouseDown: editor.mouseDown, OnMouseMove: editor.mouseMove, OnMouseUp: editor.mouseUp},
			Composite{Layout: HBox{Spacing: 8}, Children: []Widget{HSpacer{}, PushButton{Text: "Salvar tema", OnClicked: editor.save}, PushButton{Text: "Fechar", OnClicked: func() { editor.dialog.Cancel() }}}},
		}},
		Composite{MinSize: Size{Width: 280}, Layout: Grid{Columns: 1, Spacing: 3}, Children: []Widget{
			Label{Text: "Propriedades", ColumnSpan: 2}, Label{AssignTo: &editor.name, Text: "Selecione uma camada", ColumnSpan: 2},
			Label{Text: "Texto"}, LineEdit{AssignTo: &editor.text, OnTextChanged: editor.applyText},
			Label{Text: "Vínculo (ex.: nome do time, placar)"}, LineEdit{AssignTo: &editor.binding, OnTextChanged: editor.applyBinding},
			Label{Text: "Tamanho da fonte"}, LineEdit{AssignTo: &editor.fontSize, OnTextChanged: func() {
				editor.applyNumber(editor.fontSize, func(e *hudtheme.Element, v float64) { e.FontSize = int(v) })
			}},
			Label{Text: "Âncora"}, LineEdit{AssignTo: &editor.anchor, OnTextChanged: editor.applyAnchor},
			Label{Text: "Opacidade (0 a 1)"}, LineEdit{AssignTo: &editor.opacity, OnTextChanged: func() { editor.applyNumber(editor.opacity, func(e *hudtheme.Element, v float64) { e.Opacity = v }) }},
			Label{Text: "Arquivo de imagem"}, LineEdit{AssignTo: &editor.asset, OnTextChanged: editor.applyAsset},
			Label{Text: "Cor"}, LineEdit{AssignTo: &editor.color, OnTextChanged: editor.applyColor},
			Label{Text: "X (%)"}, LineEdit{AssignTo: &editor.x, OnTextChanged: func() { editor.applyNumber(editor.x, func(e *hudtheme.Element, v float64) { e.X = v }) }},
			Label{Text: "Y (%)"}, LineEdit{AssignTo: &editor.y, OnTextChanged: func() { editor.applyNumber(editor.y, func(e *hudtheme.Element, v float64) { e.Y = v }) }},
			Label{Text: "Largura"}, LineEdit{AssignTo: &editor.width, OnTextChanged: func() { editor.applyNumber(editor.width, func(e *hudtheme.Element, v float64) { e.Width = v }) }},
			Label{Text: "Altura"}, LineEdit{AssignTo: &editor.height, OnTextChanged: func() { editor.applyNumber(editor.height, func(e *hudtheme.Element, v float64) { e.Height = v }) }},
			CheckBox{AssignTo: &editor.visible, Text: "Visível", ColumnSpan: 2, OnCheckedChanged: func() { editor.applyVisible() }},
		}},
	}}).Run(owner)
	return err
}

func (editor *themeEditorWindow) layerNames() []string {
	names := make([]string, len(editor.state.Theme.Elements))
	for index, element := range editor.state.Theme.Elements {
		names[index] = element.ID + " · " + string(element.Type)
	}
	return names
}

func (editor *themeEditorWindow) selectLayer() {
	if editor.updating || editor.layers.CurrentIndex() < 0 {
		return
	}
	editor.state.SelectedID = editor.state.Theme.Elements[editor.layers.CurrentIndex()].ID
	editor.refresh()
}

func (editor *themeEditorWindow) add(kind hudtheme.ElementType) {
	if editor.state.AddElement(kind) == nil {
		editor.refresh()
	}
}
func (editor *themeEditorWindow) applyPalette(a, b TeamPalette) {
	ApplyPalette(&editor.state.Theme, a, b)
	editor.refresh()
}
func (editor *themeEditorWindow) moveLayer(direction int) {
	if editor.state.MoveLayer(direction) == nil {
		editor.refresh()
	}
}
func (editor *themeEditorWindow) remove() {
	if err := editor.state.DeleteSelected(); err != nil {
		walk.MsgBox(editor.dialog, "HUD", err.Error(), walk.MsgBoxOK|walk.MsgBoxIconInformation)
		return
	}
	editor.refresh()
}

func (editor *themeEditorWindow) addImage() {
	dialog := new(walk.FileDialog)
	dialog.Title = "Selecione o ícone"
	dialog.Filter = "Imagens|*.png;*.jpg;*.jpeg"
	accepted, err := dialog.ShowOpen(editor.dialog)
	if err != nil || !accepted {
		return
	}
	if err = editor.state.AddElement(hudtheme.Image); err != nil {
		return
	}
	base := filepath.Base(dialog.FilePath)
	target := filepath.Join(filepath.Dir(editor.path), base)
	if strings.EqualFold(filepath.Clean(dialog.FilePath), filepath.Clean(target)) {
		editor.state.Selected().Asset = base
		editor.refresh()
		return
	}
	data, err := os.ReadFile(dialog.FilePath)
	if err != nil {
		editor.remove()
		return
	}
	if err = os.WriteFile(target, data, 0o600); err != nil {
		editor.remove()
		return
	}
	editor.state.Selected().Asset = base
	editor.refresh()
}

func (editor *themeEditorWindow) refresh() {
	editor.updating = true
	defer func() { editor.updating = false }()
	_ = editor.layers.SetModel(editor.layerNames())
	selected := editor.state.Selected()
	if selected == nil {
		editor.name.SetText("Selecione uma camada")
		editor.canvas.Invalidate()
		return
	}
	editor.name.SetText(selected.ID + " · " + string(selected.Type))
	editor.text.SetText(selected.Text)
	editor.binding.SetText(string(selected.Binding))
	editor.fontSize.SetText(strconv.Itoa(selected.FontSize))
	editor.anchor.SetText(string(selected.Anchor))
	editor.opacity.SetText(fmt.Sprintf("%.2f", selected.Opacity))
	editor.asset.SetText(selected.Asset)
	editor.color.SetText(selected.Color)
	editor.x.SetText(fmt.Sprintf("%.1f", selected.X))
	editor.y.SetText(fmt.Sprintf("%.1f", selected.Y))
	editor.width.SetText(fmt.Sprintf("%.1f", selected.Width))
	editor.height.SetText(fmt.Sprintf("%.1f", selected.Height))
	editor.visible.SetChecked(selected.Visible)
	for index, element := range editor.state.Theme.Elements {
		if element.ID == selected.ID {
			editor.layers.SetCurrentIndex(index)
			break
		}
	}
	editor.canvas.Invalidate()
}

func (editor *themeEditorWindow) applyText() {
	if !editor.updating {
		_ = editor.state.SetSelectedElement(func(e *hudtheme.Element) { e.Text = editor.text.Text() })
		editor.canvas.Invalidate()
	}
}
func (editor *themeEditorWindow) applyColor() {
	if !editor.updating {
		_ = editor.state.SetSelectedElement(func(e *hudtheme.Element) { e.Color = editor.color.Text() })
		editor.canvas.Invalidate()
	}
}
func (editor *themeEditorWindow) applyBinding() {
	if !editor.updating {
		_ = editor.state.SetSelectedElement(func(e *hudtheme.Element) { e.Binding = hudtheme.Binding(editor.binding.Text()) })
		editor.canvas.Invalidate()
	}
}
func (editor *themeEditorWindow) applyAnchor() {
	if !editor.updating {
		_ = editor.state.SetSelectedElement(func(e *hudtheme.Element) { e.Anchor = hudtheme.Anchor(editor.anchor.Text()) })
		editor.canvas.Invalidate()
	}
}
func (editor *themeEditorWindow) applyAsset() {
	if !editor.updating {
		_ = editor.state.SetSelectedElement(func(e *hudtheme.Element) { e.Asset = filepath.Base(editor.asset.Text()) })
		editor.canvas.Invalidate()
	}
}
func (editor *themeEditorWindow) applyVisible() {
	if !editor.updating {
		_ = editor.state.SetSelectedElement(func(e *hudtheme.Element) { e.Visible = editor.visible.Checked() })
		editor.canvas.Invalidate()
	}
}
func (editor *themeEditorWindow) applyNumber(field *walk.LineEdit, set func(*hudtheme.Element, float64)) {
	if editor.updating {
		return
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(field.Text(), ",", "."), 64)
	if err == nil {
		_ = editor.state.SetSelectedElement(func(e *hudtheme.Element) { set(e, value) })
		editor.canvas.Invalidate()
	}
}

func (editor *themeEditorWindow) mouseDown(x, y int, _ walk.MouseButton) {
	px, py := editor.percent(x, y)
	editor.state.SelectAt(px, py)
	selected := editor.state.Selected()
	if selected != nil {
		editor.resizing = px >= selected.X+selected.Width-2 && py >= selected.Y+selected.Height-2
		editor.dragging = !editor.resizing
	}
	editor.lastX, editor.lastY = px, py
	editor.refresh()
}
func (editor *themeEditorWindow) mouseMove(x, y int, _ walk.MouseButton) {
	if !editor.dragging && !editor.resizing {
		return
	}
	px, py := editor.percent(x, y)
	if editor.dragging {
		_ = editor.state.MoveSelected(px-editor.lastX, py-editor.lastY)
	} else {
		_ = editor.state.ResizeSelected(px-editor.lastX, py-editor.lastY)
	}
	editor.lastX, editor.lastY = px, py
	editor.refresh()
}
func (editor *themeEditorWindow) mouseUp(int, int, walk.MouseButton) {
	editor.dragging, editor.resizing = false, false
}
func (editor *themeEditorWindow) percent(x, y int) (float64, float64) {
	bounds := editor.canvas.Bounds()
	return float64(x) * 100 / float64(max(1, bounds.Width)), float64(y) * 100 / float64(max(1, bounds.Height))
}

func (editor *themeEditorWindow) save() {
	if err := hudtheme.Save(editor.path, editor.state.Theme); err != nil {
		walk.MsgBox(editor.dialog, "Tema inválido", err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
		return
	}
	editor.dialog.Accept()
}

func (editor *themeEditorWindow) paint(canvas *walk.Canvas, bounds walk.Rectangle) error {
	background, err := walk.NewSolidColorBrush(walk.RGB(18, 26, 36))
	if err != nil {
		return err
	}
	defer background.Dispose()
	if err = canvas.FillRectangle(background, bounds); err != nil {
		return err
	}
	font, err := walk.NewFont("Segoe UI", 10, 0)
	if err != nil {
		return err
	}
	defer font.Dispose()
	for _, element := range editor.state.Theme.Elements {
		if !element.Visible {
			continue
		}
		rect := editor.rect(element, bounds)
		color := previewColor(element.Color)
		brush, brushErr := walk.NewSolidColorBrush(color)
		if brushErr != nil {
			return brushErr
		}
		if err = canvas.FillRectangle(brush, rect); err != nil {
			brush.Dispose()
			return err
		}
		brush.Dispose()
		label := element.Text
		if element.Binding != "" {
			label = string(element.Binding)
		}
		if label == "" && element.Type == hudtheme.Image {
			label = "IMG " + element.Asset
		}
		if label != "" {
			_ = canvas.DrawText(label, font, walk.RGB(0, 0, 0), rect, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
		}
	}
	if selected := editor.state.Selected(); selected != nil {
		rect := editor.rect(*selected, bounds)
		pen, penErr := walk.NewCosmeticPen(walk.PenSolid, walk.RGB(0, 174, 239))
		if penErr != nil {
			return penErr
		}
		defer pen.Dispose()
		_ = canvas.DrawRectangle(pen, rect)
		handle, _ := walk.NewSolidColorBrush(walk.RGB(0, 174, 239))
		defer handle.Dispose()
		_ = canvas.FillRectangle(handle, walk.Rectangle{X: rect.X + rect.Width - 8, Y: rect.Y + rect.Height - 8, Width: 8, Height: 8})
	}
	return nil
}

func (editor *themeEditorWindow) rect(element hudtheme.Element, bounds walk.Rectangle) walk.Rectangle {
	return walk.Rectangle{X: int(element.X / 100 * float64(bounds.Width)), Y: int(element.Y / 100 * float64(bounds.Height)), Width: max(1, int(element.Width/100*float64(bounds.Width))), Height: max(1, int(element.Height/100*float64(bounds.Height)))}
}
func previewColor(value string) walk.Color {
	var red, green, blue uint8 = 245, 245, 245
	_, _ = fmt.Sscanf(value, "#%02x%02x%02x", &red, &green, &blue)
	return walk.RGB(red, green, blue)
}
