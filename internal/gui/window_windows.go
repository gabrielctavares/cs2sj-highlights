//go:build windows

package gui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/cli"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/preflight"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

const vacWarning = "O CS2 será aberto com -insecure. Não entre em servidores protegidos por VAC durante a captura."

type applicationWindow struct {
	rootContext     context.Context
	executablePath  string
	configPath      string
	controller      *Controller
	cancelWork      context.CancelFunc
	analyzing       bool
	vacWarningSeen  bool
	choiceModel     *choiceTableModel
	updatingHUD     bool
	applicationIcon *walk.Icon

	mainWindow   *walk.MainWindow
	cs2Edit      *walk.LineEdit
	inputEdit    *walk.LineEdit
	outputEdit   *walk.LineEdit
	statusEdit   *walk.TextEdit
	choiceTable  *walk.TableView
	cs2Browse    *walk.PushButton
	inputBrowse  *walk.PushButton
	outputBrowse *walk.PushButton
	analyze      *walk.PushButton
	selectAll    *walk.PushButton
	selectNone   *walk.PushButton
	selection    *walk.Label
	process      *walk.PushButton
	openOutput   *walk.PushButton
	gameHUD      *walk.CheckBox
	customHUD    *walk.CheckBox
}

func Run(ctx context.Context, executablePath, localAppData string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(localAppData) == "" {
		var err error
		localAppData, err = os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("localizar pasta de configuração: %w", err)
		}
	}

	configPath := ConfigPath(localAppData)
	config, configErr := LoadConfig(configPath)
	if configErr != nil {
		config = Config{SchemaVersion: ConfigSchemaVersion}
	}
	if !regularFile(config.CS2Path) {
		config.CS2Path = detectCS2Path()
	}

	window := &applicationWindow{
		rootContext:    ctx,
		executablePath: executablePath,
		configPath:     configPath,
		controller:     NewController(cli.RenderBatch),
	}
	if err := window.create(config); err != nil {
		return err
	}
	if configErr != nil {
		window.appendStatus("Aviso: a configuração salva não pôde ser lida e foi ignorada: " + configErr.Error())
	}
	if _, err := ResolveBundle(executablePath); err != nil {
		if err.Error() == MissingHLAEMessage {
			if dialogErr := showMissingHLAEDialog(window.mainWindow); dialogErr != nil {
				window.appendStatus("Erro ao exibir aviso do HLAE: " + dialogErr.Error())
			}
		} else {
			window.appendStatus("Erro ao verificar HLAE: " + err.Error())
		}
	}
	go func() {
		<-ctx.Done()
		if !window.mainWindow.IsDisposed() {
			window.mainWindow.Synchronize(func() {
				if window.cancelWork != nil {
					window.cancelWork()
				}
				_ = window.mainWindow.Close()
			})
		}
	}()
	window.mainWindow.Run()
	if window.applicationIcon != nil {
		window.applicationIcon.Dispose()
	}
	return nil
}

type applicationIconLoader func(string, int, int) (*walk.Icon, error)

func loadApplicationIcon(executablePath string, loader applicationIconLoader) (*walk.Icon, error) {
	return loader(executablePath, 0, 32)
}

func (window *applicationWindow) create(config Config) error {
	window.choiceModel = newChoiceTableModel(window.updateSelectionStatus)
	definition := MainWindow{
		AssignTo: &window.mainWindow,
		Title:    "CS2SJ Demo - Highlights",
		MinSize:  Size{Width: 900, Height: 650},
		Size:     Size{Width: 1100, Height: 780},
		Layout:   VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}, Spacing: 10},
		Children: []Widget{
			Label{Text: "Selecione os caminhos, analise as demos e marque os highlights que deseja gerar."},
			Composite{
				Layout: Grid{Columns: 3, Spacing: 8},
				Children: []Widget{
					Label{Text: "CS2 (cs2.exe)"},
					LineEdit{AssignTo: &window.cs2Edit, Text: config.CS2Path, StretchFactor: 1},
					PushButton{AssignTo: &window.cs2Browse, Text: "Procurar", OnClicked: window.browseCS2},
					Label{Text: "Pasta das demos"},
					LineEdit{AssignTo: &window.inputEdit, Text: config.InputDir, StretchFactor: 1, OnTextChanged: window.invalidatePreview},
					PushButton{AssignTo: &window.inputBrowse, Text: "Procurar", OnClicked: func() { window.browseFolder(window.inputEdit, "Selecione a pasta das demos") }},
					Label{Text: "Pasta de saída"},
					LineEdit{AssignTo: &window.outputEdit, Text: config.OutputDir, StretchFactor: 1, OnTextChanged: func() { window.invalidatePreview(); window.updateOpenOutput() }},
					PushButton{AssignTo: &window.outputBrowse, Text: "Procurar", OnClicked: func() { window.browseFolder(window.outputEdit, "Selecione a pasta de saída") }},
				},
			},
			Composite{
				Layout: HBox{Spacing: 16},
				Children: []Widget{
					Label{Text: "HUD do vídeo:"},
					CheckBox{AssignTo: &window.gameHUD, Text: "Mostrar HUD do jogo", Checked: config.HUDMode == model.HUDGame, OnCheckedChanged: window.gameHUDChanged},
					CheckBox{AssignTo: &window.customHUD, Text: "Usar HUD personalizada", Checked: config.HUDMode == model.HUDCustom, OnCheckedChanged: window.customHUDChanged},
					HSpacer{},
				},
			},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					PushButton{AssignTo: &window.analyze, Text: "Analisar demos", MinSize: Size{Width: 140, Height: 34}, OnClicked: window.startAnalysis},
					PushButton{AssignTo: &window.selectAll, Text: "Marcar todos", Enabled: false, OnClicked: func() { window.choiceModel.SetAll(true) }},
					PushButton{AssignTo: &window.selectNone, Text: "Desmarcar todos", Enabled: false, OnClicked: func() { window.choiceModel.SetAll(false) }},
					Label{AssignTo: &window.selection, Text: "Nenhum clipe analisado"},
					HSpacer{},
				},
			},
			TableView{
				AssignTo: &window.choiceTable, Model: window.choiceModel, CheckBoxes: true,
				AlternatingRowBG: true, ColumnsSizable: true, MinSize: Size{Width: 850, Height: 280}, StretchFactor: 1,
				Columns: []TableViewColumn{
					{Title: "Demo", Width: 360},
					{Title: "Mapa", Width: 105},
					{Title: "Jogador", Width: 130},
					{Title: "Tipo", Width: 250},
					{Title: "Round", Width: 70, Alignment: AlignFar},
				},
			},
			Label{Text: vacWarning},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					PushButton{AssignTo: &window.process, Text: "Processar selecionados", Enabled: false, MinSize: Size{Width: 180, Height: 36}, OnClicked: window.startProcessing},
					PushButton{AssignTo: &window.openOutput, Text: "Abrir pasta dos vídeos", MinSize: Size{Width: 165, Height: 36}, OnClicked: window.openOutputFolder},
					HSpacer{},
				},
			},
			Label{Text: "Status"},
			TextEdit{AssignTo: &window.statusEdit, ReadOnly: true, VScroll: true, MinSize: Size{Width: 850, Height: 120}},
		},
	}
	if err := definition.Create(); err != nil {
		return fmt.Errorf("criar janela principal: %w", err)
	}
	if icon, iconErr := loadApplicationIcon(window.executablePath, walk.NewIconExtractedFromFileWithSize); iconErr == nil && icon != nil {
		if setErr := window.mainWindow.SetIcon(icon); setErr == nil {
			window.applicationIcon = icon
		} else {
			icon.Dispose()
		}
	}
	window.mainWindow.Closing().Attach(window.onClosing)
	window.updateOpenOutput()
	return nil
}

func (window *applicationWindow) browseCS2() {
	dialog := new(walk.FileDialog)
	dialog.Title = "Selecione o cs2.exe"
	dialog.Filter = "Counter-Strike 2 (cs2.exe)|cs2.exe|Executáveis (*.exe)|*.exe"
	dialog.FilePath = window.cs2Edit.Text()
	accepted, err := dialog.ShowOpen(window.mainWindow)
	if err != nil {
		window.showError("Não foi possível abrir o seletor do CS2", err)
		return
	}
	if accepted {
		window.cs2Edit.SetText(dialog.FilePath)
	}
}

func (window *applicationWindow) browseFolder(target *walk.LineEdit, title string) {
	dialog := new(walk.FileDialog)
	dialog.Title = title
	dialog.FilePath = target.Text()
	accepted, err := dialog.ShowBrowseFolder(window.mainWindow)
	if err != nil {
		window.showError("Não foi possível abrir o seletor de pasta", err)
		return
	}
	if accepted {
		target.SetText(dialog.FilePath)
	}
}

func (window *applicationWindow) startAnalysis() {
	values := window.formValues()
	if err := ValidateForm(values); err != nil {
		window.showError("Verifique os caminhos", err)
		return
	}
	if err := SaveConfig(window.configPath, Config{
		CS2Path: values.CS2Path, InputDir: values.InputDir, OutputDir: values.OutputDir, HUDMode: window.hudMode(),
	}); err != nil {
		window.showError("Não foi possível salvar os caminhos", err)
		return
	}

	analysisContext, cancel := context.WithCancel(window.rootContext)
	window.cancelWork = cancel
	window.analyzing = true
	window.choiceModel.SetChoices(nil)
	window.setRunning(true)
	window.appendStatus("Analisando demos e procurando highlights...")

	go func() {
		results, err := cli.AnalyzeBatch(analysisContext, cli.Options{
			Command: "analyze", InputDir: values.InputDir, OutputDir: values.OutputDir,
		}, nil)
		var choices []ClipChoice
		if err == nil {
			choices, err = ChoicesFromResults(results)
		}
		if window.mainWindow.IsDisposed() {
			return
		}
		window.mainWindow.Synchronize(func() {
			if window.mainWindow.IsDisposed() {
				return
			}
			window.analyzing = false
			window.cancelWork = nil
			window.setRunning(false)
			if analysisContext.Err() != nil {
				window.appendStatus("Análise cancelada.")
				return
			}
			if err != nil {
				window.showError("Não foi possível analisar as demos", err)
				return
			}
			window.choiceModel.SetChoices(choices)
			window.appendStatus(fmt.Sprintf("Análise concluída: %d clipe(s) encontrado(s). Desmarque o que não deseja gerar.", len(choices)))
		})
	}()
}

func (window *applicationWindow) startProcessing() {
	values := window.formValues()
	if err := ValidateForm(values); err != nil {
		window.showError("Verifique os caminhos", err)
		return
	}
	choices := window.choiceModel.Choices()
	selected := SelectedHighlights(choices)
	if SelectedCount(choices) == 0 {
		window.showError("Selecione os clipes", fmt.Errorf("marque pelo menos um clipe antes de processar"))
		return
	}
	bundle, err := ResolveBundle(window.executablePath)
	if err != nil {
		if err.Error() == MissingHLAEMessage {
			if dialogErr := showMissingHLAEDialog(window.mainWindow); dialogErr != nil {
				window.showError("HLAE não encontrado", dialogErr)
			}
			return
		}
		window.showError("Não foi possível verificar o HLAE", err)
		return
	}
	if !window.vacWarningSeen {
		walk.MsgBox(window.mainWindow, "Aviso de segurança", vacWarning, walk.MsgBoxOK|walk.MsgBoxIconWarning)
		window.vacWarningSeen = true
	}
	if err := SaveConfig(window.configPath, Config{
		CS2Path: values.CS2Path, InputDir: values.InputDir, OutputDir: values.OutputDir, HUDMode: window.hudMode(),
	}); err != nil {
		window.showError("Não foi possível salvar os caminhos", err)
		return
	}

	renderContext, cancel := context.WithCancel(window.rootContext)
	window.cancelWork = cancel
	window.setRunning(true)
	window.appendStatus(fmt.Sprintf("Iniciando processamento de %d clipe(s) selecionado(s)...", SelectedCount(choices)))
	err = window.controller.Start(renderContext, StartRequest{Values: values, Bundle: bundle, SelectedHighlights: selected, HUDMode: window.hudMode()}, window.report)
	if err != nil {
		cancel()
		window.cancelWork = nil
		window.setRunning(false)
		window.showError("Não foi possível iniciar", err)
	}
}

func (window *applicationWindow) report(event Event) {
	if window.mainWindow.IsDisposed() {
		return
	}
	window.mainWindow.Synchronize(func() {
		if window.mainWindow.IsDisposed() {
			return
		}
		window.appendStatus(event.Message)
		switch event.State {
		case StateCompleted, StatePartial, StateFailed, StateCancelled:
			window.cancelWork = nil
			window.setRunning(false)
			window.updateOpenOutput()
		}
	})
}

func (window *applicationWindow) setRunning(running bool) {
	for _, control := range []walk.Widget{window.cs2Edit, window.inputEdit, window.outputEdit, window.cs2Browse, window.inputBrowse, window.outputBrowse, window.analyze, window.choiceTable, window.gameHUD, window.customHUD} {
		control.SetEnabled(!running)
	}
	if running {
		window.process.SetEnabled(false)
		window.selectAll.SetEnabled(false)
		window.selectNone.SetEnabled(false)
		window.openOutput.SetEnabled(false)
	} else {
		window.updateSelectionStatus()
		window.updateOpenOutput()
	}
}

func HUDModeFromChecks(game, custom bool) model.HUDMode {
	if game {
		return model.HUDGame
	}
	if custom {
		return model.HUDCustom
	}
	return model.HUDNone
}

func (window *applicationWindow) hudMode() model.HUDMode {
	return HUDModeFromChecks(window.gameHUD != nil && window.gameHUD.Checked(), window.customHUD != nil && window.customHUD.Checked())
}

func (window *applicationWindow) gameHUDChanged() {
	if window.updatingHUD || window.gameHUD == nil || window.customHUD == nil || !window.gameHUD.Checked() {
		return
	}
	window.updatingHUD = true
	defer func() { window.updatingHUD = false }()
	window.customHUD.SetChecked(false)
}

func (window *applicationWindow) customHUDChanged() {
	if window.updatingHUD || window.gameHUD == nil || window.customHUD == nil || !window.customHUD.Checked() {
		return
	}
	window.updatingHUD = true
	defer func() { window.updatingHUD = false }()
	window.gameHUD.SetChecked(false)
}

func (window *applicationWindow) formValues() FormValues {
	return FormValues{
		CS2Path:   strings.TrimSpace(window.cs2Edit.Text()),
		InputDir:  strings.TrimSpace(window.inputEdit.Text()),
		OutputDir: strings.TrimSpace(window.outputEdit.Text()),
	}
}

func (window *applicationWindow) invalidatePreview() {
	if window.choiceModel == nil || window.analyzing || window.controller.IsRunning() {
		return
	}
	if window.choiceModel.RowCount() > 0 {
		window.choiceModel.SetChoices(nil)
	}
}

func (window *applicationWindow) updateSelectionStatus() {
	if window.choiceModel == nil || window.selection == nil {
		return
	}
	total := window.choiceModel.RowCount()
	selected := SelectedCount(window.choiceModel.Choices())
	if total == 0 {
		window.selection.SetText("Nenhum clipe analisado")
	} else {
		window.selection.SetText(fmt.Sprintf("%d de %d clipe(s) marcado(s)", selected, total))
	}
	working := window.analyzing || window.controller.IsRunning()
	window.selectAll.SetEnabled(!working && total > 0)
	window.selectNone.SetEnabled(!working && total > 0)
	window.process.SetEnabled(!working && selected > 0)
}

func (window *applicationWindow) updateOpenOutput() {
	if window.openOutput == nil || window.outputEdit == nil {
		return
	}
	info, err := os.Stat(strings.TrimSpace(window.outputEdit.Text()))
	window.openOutput.SetEnabled(!window.analyzing && !window.controller.IsRunning() && err == nil && info.IsDir())
}

func (window *applicationWindow) appendStatus(message string) {
	message = strings.TrimSpace(message)
	if message == "" || window.statusEdit == nil {
		return
	}
	if window.statusEdit.TextLength() > 0 {
		window.statusEdit.AppendText("\r\n")
	}
	window.statusEdit.AppendText(message)
}

func (window *applicationWindow) showError(title string, err error) {
	window.appendStatus(title + ": " + err.Error())
	walk.MsgBox(window.mainWindow, title, err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
}

func (window *applicationWindow) openOutputFolder() {
	path := strings.TrimSpace(window.outputEdit.Text())
	if err := exec.Command("explorer.exe", path).Start(); err != nil {
		window.showError("Não foi possível abrir a pasta", err)
	}
}

func (window *applicationWindow) onClosing(cancelled *bool, _ walk.CloseReason) {
	if !window.analyzing && !window.controller.IsRunning() {
		return
	}
	answer := walk.MsgBox(window.mainWindow, "Cancelar trabalho?", "Há uma análise ou renderização em andamento. Deseja cancelar e fechar?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)
	if answer != walk.DlgCmdYes {
		*cancelled = true
		return
	}
	if window.cancelWork != nil {
		window.cancelWork()
	}
}

func showMissingHLAEDialog(owner walk.Form) error {
	var dialog *walk.Dialog
	var closeButton *walk.PushButton
	_, err := (Dialog{
		AssignTo:     &dialog,
		Title:        "HLAE não encontrado",
		MinSize:      Size{Width: 590, Height: 220},
		Size:         Size{Width: 620, Height: 240},
		CancelButton: &closeButton,
		Layout:       VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}, Spacing: 12},
		Children: []Widget{
			TextEdit{Text: MissingHLAEMessage, ReadOnly: true, MinSize: Size{Width: 550, Height: 100}},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					HSpacer{},
					PushButton{Text: "Baixar HLAE", OnClicked: func() {
						if openErr := openHLAEDownloadPage(); openErr != nil {
							walk.MsgBox(dialog, "Não foi possível abrir o navegador", openErr.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
							return
						}
						dialog.Accept()
					}},
					PushButton{AssignTo: &closeButton, Text: "Fechar", OnClicked: func() { dialog.Cancel() }},
				},
			},
		},
	}).Run(owner)
	return err
}

func openHLAEDownloadPage() error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", HLAEDownloadURL).Start()
}

func detectCS2Path() string {
	for _, candidate := range preflight.CommonCS2Paths() {
		if regularFile(candidate) {
			absolute, err := filepath.Abs(candidate)
			if err == nil {
				return absolute
			}
		}
	}
	return ""
}
