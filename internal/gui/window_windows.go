//go:build windows

package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/cli"
	"github.com/gabrielctavares/cs2sj-highlights/internal/highlights"
	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
	"github.com/gabrielctavares/cs2sj-highlights/internal/preflight"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
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
	preview         *PreviewState
	editorialModel  *choiceTableModel
	playerModel     *choiceTableModel
	finalModel      *choiceTableModel
	players         []PlayerOption
	favoriteSteamID uint64
	updatingHUD     bool
	applicationIcon *walk.Icon

	mainWindow     *walk.MainWindow
	cs2Edit        *walk.LineEdit
	inputEdit      *walk.LineEdit
	outputEdit     *walk.LineEdit
	hudThemeEdit   *walk.LineEdit
	hudEventEdit   *walk.LineEdit
	statusEdit     *walk.TextEdit
	editorialTable *walk.TableView
	playerTable    *walk.TableView
	finalTable     *walk.TableView
	breadthCombo   *walk.ComboBox
	playerCombo    *walk.ComboBox
	cs2Browse      *walk.PushButton
	inputBrowse    *walk.PushButton
	outputBrowse   *walk.PushButton
	analyze        *walk.PushButton
	addEditorial   *walk.PushButton
	addPlayer      *walk.PushButton
	removeFinal    *walk.PushButton
	selection      *walk.Label
	process        *walk.PushButton
	openOutput     *walk.PushButton
	gameHUD        *walk.CheckBox
	customHUD      *walk.CheckBox
	hudThemeBrowse *walk.PushButton
	hudEditor      *walk.PushButton
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

func clipTableColumns() []TableViewColumn {
	return []TableViewColumn{
		{Title: "Demo", Width: 150},
		{Title: "Mapa", Width: 95},
		{Title: "Jogador", Width: 120},
		{Title: "Tipo", Width: 180},
		{Title: "Round", Width: 55, Alignment: AlignFar},
		{Title: "Nota", Width: 55, Alignment: AlignFar},
		{Title: "Por que apareceu", Width: 330},
	}
}

func highlightLegendText() string {
	return "Nota geral: de 0 a 100 pontos. 70% vêm da nota técnica. O restante vem do contexto do round.\n\n" +
		"Nota técnica: kills: 1K 20, 2K 38, 3K 62, 4K 80, 5K+ 95.\n" +
		"Assistência +4, até 8. Flash assist +18, até 36. Headshot +8, até 16.\n" +
		"Wallbang +15. No-scope +18. Smoke +18. Cego +12. Longa distância, sequência rápida e pouco HP: +10 cada.\n" +
		"Clutch: 12 + 6 por adversário.\n\n" +
		"Contexto: round vencido +10. Entry +8. Trade +5, até 10. Clutch: 15 + 5 por adversário.\n" +
		"Match point +15. Overtime +10. Fim da partida +8. Multi-kill 3+ +10.\n\n" +
		"A aba de detalhes mostra os pontos aplicados em cada lance."
}

func (window *applicationWindow) create(config Config) error {
	window.editorialModel = newChoiceTableModel()
	window.playerModel = newChoiceTableModel()
	window.finalModel = newChoiceTableModel()
	window.favoriteSteamID = config.FavoriteSteamID
	if strings.TrimSpace(config.HUDThemePath) == "" {
		path, err := DefaultHUDThemePath(filepath.Join(filepath.Dir(window.configPath), "themes"))
		if err != nil {
			return fmt.Errorf("criar tema padrão da HUD: %w", err)
		}
		config.HUDThemePath = path
	}
	eventName := strings.TrimSpace(config.EventName)
	if eventName == "" {
		eventName = defaultEventNameFromInput(config.InputDir)
	}
	definition := MainWindow{
		AssignTo: &window.mainWindow,
		Title:    "CS2SJ Demo - Highlights",
		MinSize:  Size{Width: 760, Height: 600},
		Size:     Size{Width: 1000, Height: 720},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 10, Right: 12, Bottom: 10}, Spacing: 7},
		OnSizeChanged: func() {
			ensureMainWindowChrome(window.mainWindow)
		},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 3, Spacing: 6},
				Children: []Widget{
					Label{Text: "CS2 (cs2.exe)"},
					LineEdit{AssignTo: &window.cs2Edit, Text: config.CS2Path, StretchFactor: 1},
					PushButton{AssignTo: &window.cs2Browse, Text: "Procurar", OnClicked: window.browseCS2},
				},
			},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					Composite{StretchFactor: 1, Layout: Grid{Columns: 3, Spacing: 6}, Children: []Widget{
						Label{Text: "Pasta das demos"},
						LineEdit{AssignTo: &window.inputEdit, Text: config.InputDir, StretchFactor: 1, OnTextChanged: window.invalidatePreview},
						PushButton{AssignTo: &window.inputBrowse, Text: "Procurar", OnClicked: func() { window.browseFolder(window.inputEdit, "Selecione a pasta das demos") }},
					}},
					Composite{StretchFactor: 1, Layout: Grid{Columns: 3, Spacing: 6}, Children: []Widget{
						Label{Text: "Pasta de saída"},
						LineEdit{AssignTo: &window.outputEdit, Text: config.OutputDir, StretchFactor: 1, OnTextChanged: func() { window.invalidatePreview(); window.updateOpenOutput() }},
						PushButton{AssignTo: &window.outputBrowse, Text: "Procurar", OnClicked: func() { window.browseFolder(window.outputEdit, "Selecione a pasta de saída") }},
					}},
				},
			},
			Composite{
				Layout: HBox{Spacing: 6},
				Children: []Widget{
					Label{Text: "HUD do vídeo:"},
					CheckBox{AssignTo: &window.gameHUD, Text: "Mostrar HUD do jogo", Checked: config.HUDMode == model.HUDGame, OnCheckedChanged: window.gameHUDChanged},
					CheckBox{AssignTo: &window.customHUD, Text: "Usar HUD personalizada", Checked: config.HUDMode == model.HUDCustom, OnCheckedChanged: window.customHUDChanged},
					Label{Text: "Tema:"},
					LineEdit{AssignTo: &window.hudThemeEdit, Text: config.HUDThemePath, StretchFactor: 1},
					PushButton{AssignTo: &window.hudThemeBrowse, Text: "Selecionar", OnClicked: window.browseHUDTheme},
					PushButton{AssignTo: &window.hudEditor, Text: "Editor visual", OnClicked: window.openHUDThemeEditor},
				},
			},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					PushButton{AssignTo: &window.analyze, Text: "Analisar demos", MinSize: Size{Width: 140, Height: 34}, OnClicked: window.startAnalysis},
					Label{Text: "Nome do campeonato:"},
					LineEdit{AssignTo: &window.hudEventEdit, Text: eventName, MinSize: Size{Width: 230}, OnTextChanged: window.invalidatePreview},
					Label{Text: "Abrangência:"},
					ComboBox{AssignTo: &window.breadthCombo, Model: []string{"Restrita", "Equilibrada", "Ampla"}, CurrentIndex: 1, Enabled: false, OnCurrentIndexChanged: window.refreshCatalogViews},
					Label{AssignTo: &window.selection, Text: "Nenhum clipe analisado"},
					HSpacer{},
				},
			},
			TabWidget{
				StretchFactor: 1,
				Pages: []TabPage{
					{Title: "Melhores da partida", Layout: VBox{Spacing: 8}, Children: []Widget{
						Label{Text: "Visão da organização: lances de maior valor para a narrativa da partida."},
						TableView{AssignTo: &window.editorialTable, Model: window.editorialModel, AlternatingRowBG: true, ColumnsSizable: true, MinSize: Size{Width: 700, Height: 220}, StretchFactor: 1, Columns: clipTableColumns(), OnCurrentIndexChanged: window.updateSelectionStatus},
						Composite{Layout: HBox{Spacing: 8}, Children: []Widget{PushButton{AssignTo: &window.addEditorial, Text: "Adicionar à seleção", Enabled: false, OnClicked: window.addEditorialChoice}, HSpacer{}}},
					}},
					{Title: "Por jogador", Layout: VBox{Spacing: 8}, Children: []Widget{
						Composite{Layout: HBox{Spacing: 8}, Children: []Widget{Label{Text: "Jogador:"}, ComboBox{AssignTo: &window.playerCombo, Enabled: false, OnCurrentIndexChanged: window.playerChanged}, HSpacer{}}},
						TableView{AssignTo: &window.playerTable, Model: window.playerModel, AlternatingRowBG: true, ColumnsSizable: true, MinSize: Size{Width: 700, Height: 220}, StretchFactor: 1, Columns: clipTableColumns(), OnCurrentIndexChanged: window.updateSelectionStatus},
						Composite{Layout: HBox{Spacing: 8}, Children: []Widget{PushButton{AssignTo: &window.addPlayer, Text: "Adicionar à seleção", Enabled: false, OnClicked: window.addPlayerChoice}, HSpacer{}}},
					}},
					{Title: "Seleção final", Layout: VBox{Spacing: 8}, Children: []Widget{
						Label{Text: "Somente estes clipes serão processados. Itens repetidos entre as duas visões aparecem uma vez."},
						TableView{AssignTo: &window.finalTable, Model: window.finalModel, AlternatingRowBG: true, ColumnsSizable: true, MinSize: Size{Width: 700, Height: 220}, StretchFactor: 1, Columns: clipTableColumns(), OnCurrentIndexChanged: window.updateSelectionStatus},
						Composite{Layout: HBox{Spacing: 8}, Children: []Widget{PushButton{AssignTo: &window.removeFinal, Text: "Remover da seleção", Enabled: false, OnClicked: window.removeFinalChoice}, HSpacer{}}},
					}},
				},
			},
			Label{Text: highlightLegendText()},
			Label{Text: vacWarning},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					PushButton{AssignTo: &window.process, Text: "Processar seleção final", Enabled: false, MinSize: Size{Width: 190, Height: 36}, OnClicked: window.startProcessing},
					PushButton{AssignTo: &window.openOutput, Text: "Abrir pasta dos vídeos", MinSize: Size{Width: 165, Height: 36}, OnClicked: window.openOutputFolder},
					HSpacer{},
				},
			},
			Label{Text: "Status"},
			TextEdit{AssignTo: &window.statusEdit, ReadOnly: true, VScroll: true, MinSize: Size{Width: 700, Height: 100}},
		},
	}
	if err := definition.Create(); err != nil {
		return fmt.Errorf("criar janela principal: %w", err)
	}
	ensureMainWindowChrome(window.mainWindow)
	if icon, iconErr := loadApplicationIcon(window.executablePath, walk.NewIconExtractedFromFileWithSize); iconErr == nil && icon != nil {
		if setErr := window.mainWindow.SetIcon(icon); setErr == nil {
			window.applicationIcon = icon
		} else {
			icon.Dispose()
		}
	}
	window.mainWindow.Closing().Attach(window.onClosing)
	window.updateSelectionStatus()
	window.updateOpenOutput()
	return nil
}

func ensureMainWindowChrome(mainWindow *walk.MainWindow) {
	if mainWindow == nil || mainWindow.IsDisposed() {
		return
	}
	wanted := int32(win.WS_CAPTION | win.WS_SYSMENU | win.WS_MINIMIZEBOX | win.WS_MAXIMIZEBOX | win.WS_THICKFRAME)
	style := win.GetWindowLong(mainWindow.Handle(), win.GWL_STYLE)
	if style&wanted == wanted {
		return
	}
	win.SetWindowLong(mainWindow.Handle(), win.GWL_STYLE, style|wanted)
	win.SetWindowPos(mainWindow.Handle(), 0, 0, 0, 0, 0, win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOOWNERZORDER|win.SWP_NOSIZE|win.SWP_NOZORDER)
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
	eventName := window.hudEventName()
	if err := ValidateForm(values); err != nil {
		window.showError("Verifique os caminhos", err)
		return
	}
	if err := SaveConfig(window.configPath, Config{
		CS2Path: values.CS2Path, InputDir: values.InputDir, OutputDir: values.OutputDir, HUDMode: window.hudMode(), HUDThemePath: window.hudThemePath(), EventName: window.hudEventName(), FavoriteSteamID: window.favoriteSteamID,
	}); err != nil {
		window.showError("Não foi possível salvar os caminhos", err)
		return
	}

	analysisContext, cancel := context.WithCancel(window.rootContext)
	window.cancelWork = cancel
	window.analyzing = true
	window.clearPreview()
	window.setRunning(true)
	window.appendStatus("Analisando demos e procurando highlights...")

	go func() {
		results, err := cli.AnalyzeBatch(analysisContext, cli.Options{
			Command: "analyze", InputDir: values.InputDir, OutputDir: values.OutputDir, EventName: eventName,
		}, nil)
		var preview *PreviewState
		if err == nil {
			preview, err = NewPreviewState(results)
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
			window.preview = preview
			if window.hudEventName() == "" {
				window.hudEventEdit.SetText(window.defaultEventName())
			}
			window.loadPlayers()
			window.refreshCatalogViews()
			window.appendStatus(fmt.Sprintf("Análise concluída: %d candidato(s) encontrado(s). Adicione à seleção final apenas o que deseja gerar.", len(preview.candidates)))
		})
	}()
}

func (window *applicationWindow) startProcessing() {
	values := window.formValues()
	if err := ValidateForm(values); err != nil {
		window.showError("Verifique os caminhos", err)
		return
	}
	if window.preview == nil {
		window.showError("Selecione os clipes", fmt.Errorf("analise as demos antes de processar"))
		return
	}
	choices := window.preview.FinalChoices()
	selected := window.preview.SelectedHighlights()
	decisions := FeedbackDecisions(window.editorialModel.Choices(), window.playerModel.Choices(), choices, window.currentBreadth())
	if len(choices) == 0 {
		window.showError("Selecione os clipes", fmt.Errorf("adicione pelo menos um clipe à seleção final"))
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
		CS2Path: values.CS2Path, InputDir: values.InputDir, OutputDir: values.OutputDir, HUDMode: window.hudMode(), HUDThemePath: window.hudThemePath(), EventName: window.hudEventName(), FavoriteSteamID: window.favoriteSteamID,
	}); err != nil {
		window.showError("Não foi possível salvar os caminhos", err)
		return
	}

	renderContext, cancel := context.WithCancel(window.rootContext)
	window.cancelWork = cancel
	window.setRunning(true)
	window.appendStatus(fmt.Sprintf("Iniciando processamento de %d clipe(s) na seleção final...", len(choices)))
	err = window.controller.Start(renderContext, StartRequest{Values: values, Bundle: bundle, SelectedHighlights: selected, HUDMode: window.hudMode(), HUDThemePath: window.hudThemePath(), EventName: window.hudEventName(), Decisions: decisions}, window.report)
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
	for _, control := range []walk.Widget{window.cs2Edit, window.inputEdit, window.outputEdit, window.hudEventEdit, window.cs2Browse, window.inputBrowse, window.outputBrowse, window.analyze, window.editorialTable, window.playerTable, window.finalTable, window.breadthCombo, window.playerCombo, window.gameHUD, window.customHUD} {
		control.SetEnabled(!running)
	}
	if running {
		window.process.SetEnabled(false)
		window.addEditorial.SetEnabled(false)
		window.addPlayer.SetEnabled(false)
		window.removeFinal.SetEnabled(false)
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

func (window *applicationWindow) hudThemePath() string {
	if window.hudThemeEdit == nil {
		return ""
	}
	return strings.TrimSpace(window.hudThemeEdit.Text())
}

func (window *applicationWindow) hudEventName() string {
	if window.hudEventEdit == nil {
		return ""
	}
	return strings.TrimSpace(window.hudEventEdit.Text())
}

func defaultEventNameFromInput(inputDir string) string {
	clean := filepath.Clean(strings.TrimSpace(inputDir))
	if clean == "" || clean == "." {
		return ""
	}
	return filepath.Base(clean)
}

func (window *applicationWindow) defaultEventName() string {
	if window.preview != nil && len(window.preview.candidates) > 0 {
		return filepath.Base(filepath.Dir(window.preview.candidates[0].choice.DemoPath))
	}
	return defaultEventNameFromInput(window.inputEdit.Text())
}

func (window *applicationWindow) browseHUDTheme() {
	dialog := new(walk.FileDialog)
	dialog.Title = "Selecione o hud.json"
	dialog.Filter = "Tema de HUD (hud.json)|hud.json|JSON (*.json)|*.json"
	dialog.FilePath = window.hudThemePath()
	accepted, err := dialog.ShowOpen(window.mainWindow)
	if err != nil {
		window.showError("NÃ£o foi possÃ­vel selecionar o tema", err)
		return
	}
	if accepted {
		window.hudThemeEdit.SetText(dialog.FilePath)
		window.customHUD.SetChecked(true)
	}
}

// openHUDThemeEditor keeps the editable theme in its own folder. The JSON is
// deliberately shown in the native screen as an escape hatch for every theme
// property while the 16:9 editor evolves.
func (window *applicationWindow) openLegacyHUDThemeEditor() {
	path := window.hudThemePath()
	if path == "" {
		var err error
		path, err = CreateHUDTheme(filepath.Join(filepath.Dir(window.configPath), "themes"), "Meu HUD")
		if err != nil {
			window.showError("NÃ£o foi possÃ­vel criar o tema", err)
			return
		}
		window.hudThemeEdit.SetText(path)
		window.customHUD.SetChecked(true)
	}
	theme, err := hudtheme.Load(path)
	if err != nil {
		window.showError("NÃ£o foi possÃ­vel abrir o tema", err)
		return
	}
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		window.showError("NÃ£o foi possÃ­vel preparar o tema", err)
		return
	}
	var dialog *walk.Dialog
	var editor *walk.TextEdit
	var save *walk.PushButton
	if _, err = (Dialog{AssignTo: &dialog, Title: "Editor de HUD 16:9", MinSize: Size{Width: 760, Height: 620}, Layout: VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8}, Children: []Widget{
		Label{Text: "Tema externo: " + path},
		Label{Text: "Use elementos box, text e image em porcentagens do quadro 16:9. Assets ficam na mesma pasta do hud.json."},
		CustomWidget{MinSize: Size{Width: 640, Height: 300}, Paint: paintHUDThemePreview(theme), PaintMode: PaintBuffered},
		TextEdit{AssignTo: &editor, Text: string(data), VScroll: true, HScroll: true, StretchFactor: 1},
		Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
			HSpacer{},
			PushButton{AssignTo: &save, Text: "Salvar tema", OnClicked: func() {
				var updated hudtheme.Theme
				if decodeErr := json.Unmarshal([]byte(editor.Text()), &updated); decodeErr != nil {
					walk.MsgBox(dialog, "Tema invÃ¡lido", decodeErr.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
					return
				}
				if saveErr := hudtheme.Save(path, updated); saveErr != nil {
					walk.MsgBox(dialog, "Tema invÃ¡lido", saveErr.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
					return
				}
				dialog.Accept()
			}},
			PushButton{Text: "Fechar", OnClicked: func() { dialog.Cancel() }},
		}},
	}}).Run(window.mainWindow); err != nil {
		window.showError("NÃ£o foi possÃ­vel abrir o editor", err)
	}
}

func paintHUDThemePreview(theme hudtheme.Theme) walk.PaintFunc {
	return func(canvas *walk.Canvas, bounds walk.Rectangle) error {
		background, err := walk.NewSolidColorBrush(walk.RGB(20, 28, 38))
		if err != nil {
			return err
		}
		defer background.Dispose()
		if err := canvas.FillRectangle(background, bounds); err != nil {
			return err
		}
		for _, element := range theme.Elements {
			if !element.Visible {
				continue
			}
			red, green, blue := uint8(245), uint8(245), uint8(245)
			if element.Color != "" {
				_, _ = fmt.Sscanf(element.Color, "#%02x%02x%02x", &red, &green, &blue)
			}
			brush, brushErr := walk.NewSolidColorBrush(walk.RGB(red, green, blue))
			if brushErr != nil {
				return brushErr
			}
			rectangle := walk.Rectangle{X: bounds.X + int(element.X/100*float64(bounds.Width)), Y: bounds.Y + int(element.Y/100*float64(bounds.Height)), Width: max(1, int(element.Width/100*float64(bounds.Width))), Height: max(1, int(element.Height/100*float64(bounds.Height)))}
			if element.Type == hudtheme.Image {
				rectangle.Width = max(rectangle.Width, 20)
				rectangle.Height = max(rectangle.Height, 20)
			}
			err = canvas.FillRectangle(brush, rectangle)
			brush.Dispose()
			if err != nil {
				return err
			}
		}
		return nil
	}
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
	if window.preview == nil || window.analyzing || window.controller.IsRunning() {
		return
	}
	window.clearPreview()
}

func (window *applicationWindow) updateSelectionStatus() {
	if window.editorialModel == nil || window.selection == nil {
		return
	}
	total := window.editorialModel.RowCount() + window.playerModel.RowCount()
	selected := window.finalModel.RowCount()
	if window.preview == nil {
		window.selection.SetText("Nenhum clipe analisado")
	} else {
		window.selection.SetText(fmt.Sprintf("%d clipe(s) na seleção final · %d resultado(s) visível(is)", selected, total))
	}
	working := window.analyzing || window.controller.IsRunning()
	window.breadthCombo.SetEnabled(!working && window.preview != nil)
	window.playerCombo.SetEnabled(!working && len(window.players) > 0)
	window.addEditorial.SetEnabled(!working && window.editorialTable.CurrentIndex() >= 0)
	window.addPlayer.SetEnabled(!working && window.playerTable.CurrentIndex() >= 0)
	window.removeFinal.SetEnabled(!working && window.finalTable.CurrentIndex() >= 0)
	window.process.SetEnabled(!working && selected > 0)
}

func (window *applicationWindow) currentBreadth() highlights.Breadth {
	if window.breadthCombo == nil {
		return highlights.BreadthBalanced
	}
	switch window.breadthCombo.CurrentIndex() {
	case 0:
		return highlights.BreadthRestricted
	case 2:
		return highlights.BreadthBroad
	default:
		return highlights.BreadthBalanced
	}
}

func (window *applicationWindow) loadPlayers() {
	window.players = window.preview.Players()
	labels := make([]string, 0, len(window.players))
	for _, player := range window.players {
		label := player.Name
		if player.TeamName != "" {
			label += " — " + player.TeamName
		}
		labels = append(labels, label)
	}
	_ = window.playerCombo.SetModel(labels)
	preferred := window.preview.PreferredPlayer(window.favoriteSteamID)
	index := -1
	for playerIndex, player := range window.players {
		if player.SteamID == preferred {
			index = playerIndex
			break
		}
	}
	window.playerCombo.SetCurrentIndex(index)
}

func (window *applicationWindow) selectedPlayerSteamID() uint64 {
	if window.playerCombo == nil {
		return 0
	}
	index := window.playerCombo.CurrentIndex()
	if index < 0 || index >= len(window.players) {
		return 0
	}
	return window.players[index].SteamID
}

func (window *applicationWindow) playerChanged() {
	steamID := window.selectedPlayerSteamID()
	if steamID != 0 {
		window.favoriteSteamID = steamID
		values := window.formValues()
		if err := SaveConfig(window.configPath, Config{
			CS2Path: values.CS2Path, InputDir: values.InputDir, OutputDir: values.OutputDir, HUDMode: window.hudMode(), HUDThemePath: window.hudThemePath(), EventName: window.hudEventName(), FavoriteSteamID: window.favoriteSteamID,
		}); err != nil {
			window.appendStatus("Aviso: não foi possível salvar o jogador favorito: " + err.Error())
		}
	}
	window.refreshCatalogViews()
}

func (window *applicationWindow) refreshCatalogViews() {
	if window.preview == nil {
		return
	}
	window.editorialModel.SetChoices(window.preview.EditorialChoices(window.currentBreadth()))
	window.playerModel.SetChoices(window.preview.PlayerChoices(window.currentBreadth(), window.selectedPlayerSteamID()))
	window.finalModel.SetChoices(window.preview.FinalChoices())
	window.updateSelectionStatus()
}

func (window *applicationWindow) addEditorialChoice() {
	window.addChoice(window.editorialModel, window.editorialTable)
}

func (window *applicationWindow) addPlayerChoice() {
	window.addChoice(window.playerModel, window.playerTable)
}

func (window *applicationWindow) addChoice(model *choiceTableModel, table *walk.TableView) {
	if window.preview == nil {
		return
	}
	choice, ok := model.Choice(table.CurrentIndex())
	if !ok {
		return
	}
	window.preview.Add(choice)
	window.finalModel.SetChoices(window.preview.FinalChoices())
	window.updateSelectionStatus()
}

func (window *applicationWindow) removeFinalChoice() {
	if window.preview == nil {
		return
	}
	choice, ok := window.finalModel.Choice(window.finalTable.CurrentIndex())
	if !ok {
		return
	}
	window.preview.Remove(choice.DemoPath, choice.ID)
	window.finalModel.SetChoices(window.preview.FinalChoices())
	window.updateSelectionStatus()
}

func (window *applicationWindow) clearPreview() {
	window.preview = nil
	window.players = nil
	window.editorialModel.SetChoices(nil)
	window.playerModel.SetChoices(nil)
	window.finalModel.SetChoices(nil)
	if window.playerCombo != nil {
		_ = window.playerCombo.SetModel([]string{})
		window.playerCombo.SetCurrentIndex(-1)
	}
	window.updateSelectionStatus()
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
