# CS2SJ Highlights Architecture Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Versionar o CS2SJ Highlights no GitHub e corrigir os riscos arquiteturais identificados sem alterar o comportamento já validado em produção.

**Architecture:** Preservar os pacotes de domínio e infraestrutura existentes, dividir o orquestrador em arquivos focados dentro do mesmo pacote e introduzir uma camada `application` compartilhada pela GUI e pela CLI. A captura passa a controlar apenas o processo de CS2 iniciado pelo aplicativo, o manifesto preserva o tick rate real e eventos tipados substituem o uso de texto de log como protocolo de interface.

**Tech Stack:** Go 1.24.x, Windows 10+, `lxn/walk`, HLAE/AfxHookSource2, FFmpeg/FFprobe, PowerShell, Git e GitHub Actions.

## Global Constraints

- O produto e o repositório usam o nome **CS2SJ Highlights** e o módulo `github.com/gabrielctavares/cs2sj-highlights`.
- Preservar os formatos atuais de saída horizontal, masters, manifestos e configuração do usuário.
- Vertical, resumo e ritmo inteligente continuam desabilitados por padrão.
- Não substituir `lxn/walk`, HLAE, FFmpeg ou o parser de demos neste trabalho.
- Não executar capturas em paralelo; existe somente uma sessão HLAE/CS2 por vez.
- Todo commit deve ser curto e intencional, sem `Co-authored-by`, Claude, GPT ou atribuição a IA.
- Binários, pacotes portáteis, demos, vídeos e ferramentas de terceiros não entram no Git.
- Cada alteração funcional segue teste falhando, implementação mínima e teste passando.

---

## Estrutura de arquivos resultante

```text
.github/workflows/verify.yml                valida Windows, Go, testes e build
docs/architecture.md                        registra limites e fluxo do sistema
internal/application/options.go             contratos compartilhados por GUI e CLI
internal/application/progress.go            eventos tipados de progresso
internal/application/service.go             composição dos casos de uso
internal/pipeline/pipeline.go                interfaces, configuração e processamento em lote
internal/pipeline/analysis.go                análise nova e planejamento inicial
internal/pipeline/migration.go               migrações de manifestos existentes
internal/pipeline/render_workflow.go         captura, retries e pós-produção
internal/pipeline/paths.go                   caminhos e validação de outputs
internal/pipeline/events.go                  eventos internos do workflow
internal/model/versions.go                   versões persistidas e de artefatos
internal/render/process_windows.go           descoberta e encerramento por PID
scripts/verify.ps1                           verificação local reproduzível
```

## Task 1: Criar a linha de base Git e adotar a identidade canônica

**Files:**
- Modify: `.gitignore`
- Modify: `go.mod:1`
- Modify: todos os imports Go que começam com `cs2-highlights/`
- Modify: `README.md:1`
- Test: todos os pacotes Go

**Interfaces:**
- Consumes: árvore local atual, ainda sem `.git`.
- Produces: branch `main`, remoto `origin`, módulo Go canônico e primeiro commit reproduzível.

- [ ] **Step 1: Auditar os arquivos que podem entrar no histórico**

Run:

```powershell
Get-ChildItem -Recurse -File |
  Sort-Object Length -Descending |
  Select-Object -First 30 FullName, Length

rg -n -i "api[_-]?key|secret|password|passwd|token|private[_-]?key|BEGIN .* PRIVATE" `
  -g '!go.sum' -g '!*.exe' -g '!*.dll' -g '!*.zip'
```

Expected: os maiores arquivos pertencem a `dist/` ou `bin/`; nenhuma credencial real é encontrada.

- [ ] **Step 2: Completar o `.gitignore`**

Substituir o conteúdo por:

```gitignore
.superpowers/
bin/
dist/
work/
outputs/
coverage.out
*.partial.mp4
*.dem
*.mp4
*.wav
cmd/cs2sj-demo/rsrc_windows_*.syso
assets/cs2sj-logo.ico
```

- [ ] **Step 3: Renomear o módulo e os imports**

Em `go.mod`, usar:

```go
module github.com/gabrielctavares/cs2sj-highlights
```

Em todos os arquivos Go, substituir imports como:

```go
"cs2-highlights/internal/model"
```

por:

```go
"github.com/gabrielctavares/cs2sj-highlights/internal/model"
```

Aplicar o mesmo prefixo a `cli`, `demos`, `gui`, `highlights`, `manifest`, `media`, `pipeline`, `preflight`, `render` e `subprocess`. Executar `gofmt -w cmd internal` após a troca.

- [ ] **Step 4: Atualizar o nome visível no README**

O início deve ficar:

```markdown
# CS2SJ Highlights

Aplicativo local para Windows que lê demos de Counter-Strike 2, seleciona highlights automaticamente e entrega clipes MP4 em 16:9.
```

Não renomear executáveis, pasta de configuração ou diretórios HLAE nesta tarefa; isso evita quebrar instalações existentes.

- [ ] **Step 5: Verificar o módulo renomeado**

Run:

```powershell
go mod tidy
go mod verify
go test ./...
go vet ./...
```

Expected: módulos verificados, 152 ou mais testes aprovados e `go vet` sem diagnósticos.

- [ ] **Step 6: Inicializar e conferir o histórico local**

Run:

```powershell
git init -b main
git remote add origin https://github.com/gabrielctavares/cs2sj-highlights.git
git add .gitignore go.mod go.sum README.md assets cmd internal scripts third_party docs
git status --short
git diff --cached --stat
git diff --cached --check
```

Expected: nenhum arquivo abaixo de `bin/`, `dist/`, `work/` ou `outputs/`; nenhum `.exe`, `.dll`, `.zip`, `.dem`, `.mp4` ou `.wav` staged.

- [ ] **Step 7: Criar o commit inicial**

```powershell
git commit -m "chore: establish CS2SJ Highlights repository"
```

Expected: commit raiz na branch `main`, sem trailer de coautoria.

---

## Task 2: Encerrar somente o CS2 iniciado pela captura

**Files:**
- Create: `internal/render/process_windows.go`
- Create: `internal/render/process_windows_test.go`
- Modify: `internal/render/runner_windows.go:22-227`
- Modify: `internal/cli/app.go`

**Interfaces:**
- Produces: `ProcessTracker` com `Snapshot()`, `FindNewCS2()` e `KillPID()`; `Runner` nunca usa `/IM cs2.exe`.
- Consumes: `CommandRunner` existente para executar `taskkill /PID` de forma testável.

- [ ] **Step 1: Escrever testes falhando para propriedade do processo**

Adicionar testes que descrevam o contrato:

```go
func TestNewProcessPIDSelectsOnlyPIDAbsentFromBaseline(t *testing.T) {
	baseline := map[uint32]struct{}{100: {}, 200: {}}
	current := []processEntry{{PID: 100, Name: "cs2.exe"}, {PID: 300, Name: "CS2.EXE"}, {PID: 400, Name: "steam.exe"}}

	pid, ok := newProcessPID(baseline, current, "cs2.exe")
	if !ok || pid != 300 {
		t.Fatalf("got pid=%d ok=%v", pid, ok)
	}
}

func TestStopTrackedCS2UsesPIDNotImageName(t *testing.T) {
	var args []string
	runner := Runner{Run: func(_ context.Context, name string, values ...string) ([]byte, error) {
		args = append([]string{name}, values...)
		return nil, nil
	}}
	if err := runner.stopTrackedCS2(context.Background(), 4312); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if joined != "taskkill /PID 4312 /T /F" || strings.Contains(joined, "/IM") {
		t.Fatalf("unsafe command: %s", joined)
	}
}
```

- [ ] **Step 2: Executar os testes e confirmar a falha**

Run: `go test ./internal/render -run 'Test(NewProcessPID|StopTrackedCS2)' -v`

Expected: FAIL porque `processEntry`, `newProcessPID` e `stopTrackedCS2` ainda não existem.

- [ ] **Step 3: Implementar enumeração de processos Windows**

Criar os contratos:

```go
type processEntry struct {
	PID  uint32
	Name string
}

type ProcessTracker interface {
	Snapshot() (map[uint32]struct{}, error)
	FindNewCS2(map[uint32]struct{}) (uint32, bool, error)
}

type WindowsProcessTracker struct{}
```

`Snapshot` e `FindNewCS2` devem usar `golang.org/x/sys/windows` com `CreateToolhelp32Snapshot`, `Process32First` e `Process32Next`. Fechar o snapshot com `windows.CloseHandle`; comparar nomes com `strings.EqualFold`; ignorar PID zero.

O seletor puro deve ser:

```go
func newProcessPID(baseline map[uint32]struct{}, entries []processEntry, name string) (uint32, bool) {
	for _, entry := range entries {
		if entry.PID == 0 || !strings.EqualFold(entry.Name, name) {
			continue
		}
		if _, existed := baseline[entry.PID]; !existed {
			return entry.PID, true
		}
	}
	return 0, false
}
```

- [ ] **Step 4: Integrar rastreamento ao `Runner`**

Adicionar:

```go
Tracker ProcessTracker
```

Em `RunPass`, capturar o baseline antes de iniciar HLAE. Durante o polling, chamar `FindNewCS2` até obter um PID e mantê-lo em `trackedPID`. Em timeout ou cancelamento, chamar apenas:

```go
func (runner Runner) stopTrackedCS2(ctx context.Context, pid uint32) error {
	if pid == 0 {
		return nil
	}
	_, err := runner.Run(ctx, "taskkill", "/PID", strconv.FormatUint(uint64(pid), 10), "/T", "/F")
	return err
}
```

Usar `context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)` para o `taskkill`. Remover completamente `taskkill /IM cs2.exe` e `context.Background()` desse fluxo.

- [ ] **Step 5: Adicionar regressões de cancelamento e timeout**

Nos testes existentes de `RunPass`, injetar um tracker falso que devolva PID `4312` e afirmar que cancelamento e timeout usam `/PID 4312`. Adicionar um caso sem PID descoberto e afirmar que nenhum `taskkill` amplo é executado.

- [ ] **Step 6: Rodar e confirmar**

Run:

```powershell
go test ./internal/render -v
go test ./...
go vet ./...
```

Expected: todos os testes aprovados; `rg -n 'taskkill.*(/IM|cs2\.exe)' internal` não encontra comando destrutivo por nome.

- [ ] **Step 7: Commit**

```powershell
git add internal/render
git commit -m "fix: stop only the captured CS2 process"
```

---

## Task 3: Preservar e usar o tick rate real da demo

**Files:**
- Modify: `internal/model/model.go:125-158`
- Modify: `internal/model/model_test.go`
- Modify: `internal/pipeline/pipeline.go:111-257,472-500`
- Modify: `internal/pipeline/pipeline_test.go`

**Interfaces:**
- Produces: `Manifest.TickRate float64` persistido em JSON e usado em `Capturer.RunPass`.

- [ ] **Step 1: Escrever o teste de manifesto falhando**

```go
func TestNewManifestPreservesTickRate(t *testing.T) {
	manifest := NewManifest(Timeline{DemoPath: "match.dem", Map: "de_nuke", TickRate: 128}, "hash", "fingerprint", nil)
	if manifest.TickRate != 128 {
		t.Fatalf("tick rate = %v", manifest.TickRate)
	}
}
```

- [ ] **Step 2: Escrever o teste de captura falhando**

Configurar o parser falso com `TickRate: 128`, capturar o argumento `rate` recebido por `capturerFunc` e exigir `128`, não `64`.

- [ ] **Step 3: Confirmar as falhas**

Run: `go test ./internal/model ./internal/pipeline -run TickRate -v`

Expected: FAIL porque o manifesto não contém `TickRate` e a captura recebe `64`.

- [ ] **Step 4: Implementar persistência e uso**

Adicionar ao manifesto:

```go
TickRate float64 `json:"tick_rate,omitempty"`
```

Em `NewManifest`, atribuir `TickRate: t.TickRate`. Em `captureAttempt`, validar:

```go
if manifest.TickRate <= 0 {
	return nil, fmt.Errorf("manifest has invalid tick rate %.3f", manifest.TickRate)
}
assets, err := pipeline.Capturer.RunPass(ctx, demoPath, pass, manifest.TickRate)
```

Para manifestos antigos com `tick_rate` ausente, `AnalyzeDemo` deve reutilizar a análise já necessária para metadados antigos ou executar uma única nova análise, preencher `TickRate` e salvar o manifesto antes de renderizar. A Task 5 apenas extrairá essa lógica para o migrador dedicado.

- [ ] **Step 5: Verificar**

Run:

```powershell
go test ./internal/model ./internal/pipeline -v
go test ./...
```

Expected: testes novos e existentes aprovados.

- [ ] **Step 6: Commit**

```powershell
git add internal/model internal/pipeline
git commit -m "fix: preserve demo tick rate during capture"
```

---

## Task 4: Não invalidar uma captura por falha de limpeza

**Files:**
- Modify: `internal/render/runner_windows.go:22-128`
- Modify: `internal/render/runner_windows_test.go`
- Modify: `internal/cli/app.go:176-184`

**Interfaces:**
- Produces: `Runner.Logger *slog.Logger`; falhas de remoção viram warning e os assets continuam válidos.

- [ ] **Step 1: Criar teste falhando**

Injetar uma função de remoção e um logger capturável:

```go
runner.Remove = func(string) error { return errors.New("access denied") }
assets, err := runner.RunPass(context.Background(), `C:\demos\final.dem`, pass, 64)
if err != nil || assets["clip"].VideoPath == "" {
	t.Fatalf("capture discarded after cleanup error: assets=%#v err=%v", assets, err)
}
if !strings.Contains(logOutput.String(), "cfg.cleanup_failed") {
	t.Fatalf("missing cleanup warning: %s", logOutput.String())
}
```

- [ ] **Step 2: Confirmar a falha**

Run: `go test ./internal/render -run Cleanup -v`

Expected: FAIL porque `Runner.Remove` e o warning ainda não existem.

- [ ] **Step 3: Implementar cleanup não fatal**

Adicionar ao `Runner`:

```go
Logger *slog.Logger
Remove func(string) error
```

Em `setDefaults`, usar `os.Remove` e logger descartável quando ausentes. Depois da captura:

```go
for _, path := range cfgPaths {
	if removeErr := runner.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		runner.Logger.Warn("cfg.cleanup_failed", "path", path, "error", removeErr)
	}
}
return assets, nil
```

Na composição, passar o logger já usado pela aplicação ao `render.Runner`.

- [ ] **Step 4: Verificar e commit**

Run:

```powershell
go test ./internal/render ./internal/cli -v
go test ./...
```

```powershell
git add internal/render internal/cli
git commit -m "fix: preserve captures when cfg cleanup fails"
```

---

## Task 5: Centralizar versões e migrações de manifestos

**Files:**
- Create: `internal/model/versions.go`
- Create: `internal/pipeline/migration.go`
- Create: `internal/pipeline/migration_test.go`
- Modify: `internal/model/model.go`
- Modify: `internal/manifest/store.go:92-97`
- Modify: `internal/pipeline/pipeline.go:21-257,587-640`
- Modify: testes que referenciam versões literais

**Interfaces:**
- Produces: constantes únicas de versão e `migrateManifest(context.Context, string, model.Manifest) (model.Manifest, bool, error)`.

- [ ] **Step 1: Escrever testes de migração falhando**

Cobrir manifesto sem tick rate, manifesto com `DemoMetadata` antigo e master/output obsoletos. O teste principal deve afirmar que uma única reanálise preenche tick rate, times, HUD e metadados sem descartar um master compatível.

- [ ] **Step 2: Criar constantes centrais**

```go
package model

const (
	ManifestSchemaVersion = "manifest-v1"
	RulesVersion          = "rules-v2"
	DemoMetadataVersion   = "demo-v6-tick-rate"
	MasterVersion         = "capture-v5-crosshair"
	OutputVersion         = "full-clip-v2-audio-sync"
)
```

Substituir todos os literais equivalentes. `manifest.Compatible` deve usar `model.ManifestSchemaVersion` e `model.RulesVersion`.

- [ ] **Step 3: Extrair migração**

Mover de `AnalyzeDemo` para `migration.go` a atualização de:

- `TickRate`, `TeamA`, `TeamB` e nomes estáveis dos jogadores;
- ticks, tags, prioridade e `ActionOffsets` recalculados;
- placar e HUD;
- modo/versão do master e versão do output;
- status `completed -> captured/pending` quando metadados mudam.

A assinatura deve ser:

```go
func (pipeline *Pipeline) migrateManifest(ctx context.Context, demoPath string, existing model.Manifest) (model.Manifest, bool, error)
```

Fazer no máximo um `Parser.Parse` por migração, mesmo quando mais de um campo estiver obsoleto.

- [ ] **Step 4: Simplificar `AnalyzeDemo`**

Depois de validar hash e fingerprint:

```go
migrated, changed, err := pipeline.migrateManifest(ctx, demoPath, existing)
if err != nil {
	return model.Manifest{}, err
}
if changed {
	if err := pipeline.Store.Save(manifestPath, migrated); err != nil {
		return model.Manifest{}, err
	}
}
return migrated, nil
```

- [ ] **Step 5: Verificar e commit**

Run:

```powershell
go test ./internal/model ./internal/manifest ./internal/pipeline -v
go test ./...
go vet ./...
```

```powershell
git add internal/model internal/manifest internal/pipeline
git commit -m "refactor: centralize manifest version migrations"
```

---

## Task 6: Dividir o pipeline sem mudar seu comportamento

**Files:**
- Create: `internal/pipeline/analysis.go`
- Create: `internal/pipeline/render_workflow.go`
- Create: `internal/pipeline/paths.go`
- Modify: `internal/pipeline/pipeline.go`
- Test: `internal/pipeline/*_test.go`

**Interfaces:**
- Consumes: API pública atual de `Pipeline`.
- Produces: os mesmos nomes e assinaturas, organizados por responsabilidade.

- [ ] **Step 1: Registrar a API pública antes da divisão**

Run: `go doc ./internal/pipeline > $env:TEMP\pipeline-before.txt`

- [ ] **Step 2: Mover análise**

Mover sem alterar corpos:

```text
DiscoverDemos
AnalyzeDemo
matchHUDMetadata
displayName
allDigits
timelinePlayerTeamNames
```

para `analysis.go`.

- [ ] **Step 3: Mover workflow de renderização**

Mover para `render_workflow.go`:

```text
RenderDemo
captureAttempt
recordCapture
allCompletedOutputsValid
includes
includedCount
```

- [ ] **Step 4: Mover caminhos e validações**

Mover para `paths.go`:

```text
masterValid
outputsValid
fileReady
highlightIndex
manifestPath
reconcileHUDMode
masterPath
```

Manter em `pipeline.go` as interfaces, structs, `ProcessDirectory`, `defaults` e `logger`.

- [ ] **Step 5: Confirmar API e comportamento idênticos**

Run:

```powershell
gofmt -w internal/pipeline
go test ./internal/pipeline -v
go test ./...
go doc ./internal/pipeline > $env:TEMP\pipeline-after.txt
Compare-Object (Get-Content $env:TEMP\pipeline-before.txt) (Get-Content $env:TEMP\pipeline-after.txt)
```

Expected: testes aprovados e nenhuma diferença de API pública.

- [ ] **Step 6: Commit**

```powershell
git add internal/pipeline
git commit -m "refactor: split pipeline responsibilities"
```

---

## Task 7: Criar a camada de aplicação e eventos tipados

**Files:**
- Create: `internal/application/options.go`
- Create: `internal/application/progress.go`
- Create: `internal/application/service.go`
- Create: `internal/application/service_test.go`
- Create: `internal/pipeline/events.go`
- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/analysis.go`
- Modify: `internal/pipeline/render_workflow.go`
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/app_test.go`
- Modify: `internal/gui/controller.go`
- Modify: `internal/gui/controller_test.go`
- Modify: `internal/gui/window_windows.go`

**Interfaces:**
- Produces: `application.Service`, `application.Options`, `application.Result`, `application.Progress`.
- Consumers: GUI e CLI; nenhuma delas usa contratos da outra.

- [ ] **Step 1: Definir contratos em testes**

Criar testes que exijam estes tipos:

```go
type ProgressKind string

const (
	ProgressAnalysisStarted  ProgressKind = "analysis_started"
	ProgressAnalysisDone     ProgressKind = "analysis_completed"
	ProgressCaptureStarted   ProgressKind = "capture_started"
	ProgressCaptureDone      ProgressKind = "capture_completed"
	ProgressClipDone         ProgressKind = "clip_completed"
	ProgressFailed           ProgressKind = "failed"
)

type Progress struct {
	Kind        ProgressKind
	DemoPath    string
	HighlightID string
	Message     string
	Completed   int
	Total       int
}

type Reporter func(Progress)
```

O teste deve verificar que uma execução com dois clipes emite eventos na ordem início da captura, conclusão por clipe e conclusão da demo, sem analisar texto de `slog`.

- [ ] **Step 2: Adicionar evento interno ao pipeline**

Criar:

```go
type EventKind string

const (
	EventAnalysisStarted EventKind = "analysis_started"
	EventAnalysisDone    EventKind = "analysis_completed"
	EventCaptureStarted  EventKind = "capture_started"
	EventCaptureDone     EventKind = "capture_completed"
	EventClipDone        EventKind = "clip_completed"
)

type Event struct {
	Kind        EventKind
	DemoPath    string
	HighlightID string
	Completed   int
	Total       int
}
```

Adicionar `Report func(Event)` ao `Pipeline`; `defaults` deve instalar uma função vazia quando ausente. Emitir eventos nos limites do workflow, mantendo `slog` apenas para diagnóstico.

- [ ] **Step 3: Mover opções e composição para `application`**

Mover `cli.Options`, `AnalyzeBatch`, `RenderBatch` e `writeRenderLogs` para a nova camada, ajustando o pacote. Definir:

```go
type Service struct {
	Analyze func(context.Context, Options, *slog.Logger, Reporter) ([]Result, error)
	Render  func(context.Context, Options, *slog.Logger, Reporter) ([]Result, error)
}

func DefaultService() Service
func Analyze(ctx context.Context, options Options, logger *slog.Logger, report Reporter) ([]Result, error)
func Render(ctx context.Context, options Options, logger *slog.Logger, report Reporter) ([]Result, error)
```

`Result` pode ser um alias explícito durante esta etapa:

```go
type Result = pipeline.Result
```

Isso impede GUI e CLI de dependerem uma da outra, sem duplicar o modelo de resultado.

- [ ] **Step 4: Adaptar a CLI**

`cli.App` passa a receber funções de `application.Service`. A CLI converte `application.Progress` em texto somente na fronteira de terminal. `ParseArgs` e `Usage` permanecem no pacote `cli`.

- [ ] **Step 5: Adaptar a GUI**

Alterar o controller para:

```go
type RenderFunc func(context.Context, application.Options, *slog.Logger, application.Reporter) ([]application.Result, error)
```

Remover `eventWriter`. Mapear `application.Progress` para mensagens em português em uma função pura `progressMessage`, coberta por teste. `window_windows.go` chama `application.Analyze` e injeta `application.Render` no controller.

- [ ] **Step 6: Confirmar limites de dependência**

Run:

```powershell
go test ./internal/application ./internal/cli ./internal/gui ./internal/pipeline -v
go test ./...
go vet ./...
rg -n 'internal/cli|internal/gui' internal/application internal/pipeline
rg -n 'internal/cli' internal/gui
```

Expected: testes aprovados; nenhuma dependência `application -> cli/gui`, `pipeline -> cli/gui` ou `gui -> cli`.

- [ ] **Step 7: Commit**

```powershell
git add internal/application internal/pipeline internal/cli internal/gui
git commit -m "refactor: add shared application layer"
```

---

## Task 8: Tornar recursos editoriais uma política explícita

**Files:**
- Create: `internal/application/output_policy.go`
- Create: `internal/application/output_policy_test.go`
- Modify: `internal/application/options.go`
- Modify: `internal/application/service.go`
- Modify: `internal/media/clips.go`
- Modify: `internal/pipeline/analysis.go`
- Modify: `internal/pipeline/render_workflow.go`
- Modify: testes de pipeline e mídia

**Interfaces:**
- Produces: `OutputPolicy{Vertical, Summary, SmartPacing bool}` com `ProductionPolicy()` retornando tudo falso.

- [ ] **Step 1: Escrever teste de compatibilidade falhando**

```go
func TestProductionPolicyPreservesCurrentOutputs(t *testing.T) {
	policy := ProductionPolicy()
	if policy.Vertical || policy.Summary || policy.SmartPacing {
		t.Fatalf("production behavior changed: %#v", policy)
	}
}
```

Adicionar teste de pipeline que assegure apenas `Horizontal` planejado e nenhum `Summary` com a política de produção.

- [ ] **Step 2: Implementar a política**

```go
type OutputPolicy struct {
	Vertical    bool
	Summary     bool
	SmartPacing bool
}

func ProductionPolicy() OutputPolicy { return OutputPolicy{} }
```

Adicionar `Policy OutputPolicy` a `application.Options` e normalizar o valor zero para `ProductionPolicy()`.

- [ ] **Step 3: Substituir comentários de código desabilitado por decisões explícitas**

O planejamento só cria caminho vertical quando `Policy.Vertical`. `ClipBuilder` recebe `SmartPacing` e escolhe entre master integral e `PlanPacing`. O workflow só chama `SummaryBuilder` quando `Policy.Summary`; deve rejeitar `Summary=true` sem `Vertical=true` enquanto o builder de resumo exigir os dois formatos.

Não expor controles novos na GUI e não ativar flags na CLI nesta tarefa.

- [ ] **Step 4: Verificar comportamento atual**

Run:

```powershell
go test ./internal/application ./internal/pipeline ./internal/media -v
go test ./...
```

Expected: política padrão gera somente clipes horizontais completos em 1×, sem resumo.

- [ ] **Step 5: Commit**

```powershell
git add internal/application internal/pipeline internal/media
git commit -m "refactor: make output features explicit"
```

---

## Task 9: Documentar a arquitetura e automatizar verificação no Windows

**Files:**
- Create: `docs/architecture.md`
- Create: `scripts/verify.ps1`
- Create: `.github/workflows/verify.yml`
- Modify: `README.md`

**Interfaces:**
- Produces: comando único de verificação local e CI equivalente.

- [ ] **Step 1: Criar `scripts/verify.ps1`**

```powershell
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $unformatted = & gofmt -l .
    if ($LASTEXITCODE -ne 0 -or $unformatted) {
        throw "Arquivos Go sem formatação:`n$unformatted"
    }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'go mod verify falhou.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet falhou.' }
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'go test falhou.' }
    New-Item -ItemType Directory -Force -Path 'bin' | Out-Null
    & go build -trimpath -o 'bin\verify-cs2-highlights-cli.exe' ./cmd/cs2-highlights
    if ($LASTEXITCODE -ne 0) { throw 'Build da CLI falhou.' }
}
finally {
    Pop-Location
}
```

- [ ] **Step 2: Criar workflow Windows**

```yaml
name: verify

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  go:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
          cache: true
      - name: Verify
        shell: pwsh
        run: ./scripts/verify.ps1
```

- [ ] **Step 3: Documentar limites e fluxo**

`docs/architecture.md` deve registrar:

- GUI e CLI como adaptadores;
- `application` como composição e casos de uso;
- `pipeline` como workflow persistente e sequencial;
- pacotes `demos`, `highlights`, `render`, `media` e `manifest`;
- ciclo `analisar -> migrar/reusar -> capturar -> validar -> publicar`;
- propriedade por PID e restauração da configuração Steam;
- política padrão de outputs;
- versões de manifesto/master/output e regra de migração.

Adicionar links para esse documento e para `scripts/verify.ps1` no README.

- [ ] **Step 4: Executar verificação fresca**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
git diff --check
git status --short
```

Expected: verificação e build com código zero; somente arquivos intencionais modificados.

- [ ] **Step 5: Commit**

```powershell
git add .github/workflows/verify.yml docs/architecture.md scripts/verify.ps1 README.md
git commit -m "ci: verify CS2SJ Highlights on Windows"
```

---

## Task 10: Auditoria final, smoke test e publicação

**Files:**
- Verify only: toda a árvore versionada
- Runtime output: pasta externa ao repositório

**Interfaces:**
- Produces: branch publicada em `gabrielctavares/cs2sj-highlights`, com histórico limpo e evidências de verificação.

- [ ] **Step 1: Confirmar escopo e ausência de artefatos**

Run:

```powershell
git status --short
git log --oneline --decorate
git ls-files | rg '\.(exe|dll|zip|dem|mp4|wav)$'
git log --format='%H%n%B' | rg -i 'co-authored-by|claude|gpt|openai|anthropic'
```

Expected: árvore limpa; nenhum binário/dado pesado versionado; nenhuma atribuição a IA.

- [ ] **Step 2: Executar verificações finais**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Expected: ambos terminam com código zero.

- [ ] **Step 3: Executar smoke com uma demo real**

Fechar previamente qualquer CS2 aberto. Executar:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke.ps1 `
  -Demo "C:\caminho\partida.dem" `
  -Output "C:\caminho\smoke-cs2sj-highlights" `
  -HLAE "C:\caminho\hlae.exe" `
  -CS2 "C:\Program Files (x86)\Steam\steamapps\common\Counter-Strike Global Offensive\game\bin\win64\cs2.exe" `
  -HookDLL "C:\caminho\AfxHookSource2.dll" `
  -FFmpeg "C:\caminho\ffmpeg.exe" `
  -FFprobe "C:\caminho\ffprobe.exe"
```

Expected: pelo menos um MP4 horizontal validado, nenhum vertical/resumo, manifesto concluído e configurações Steam restauradas.

- [ ] **Step 4: Autenticar e consultar o remoto**

Run:

```powershell
gh auth status
gh repo view gabrielctavares/cs2sj-highlights --json defaultBranchRef,url
gh api repos/gabrielctavares/cs2sj-highlights --jq '.size'
```

Se `gh` não estiver instalado, instalar com `winget install --id GitHub.cli` e executar `gh auth login` antes de continuar.

- [ ] **Step 5: Publicar conforme o estado do remoto**

Se o repositório estiver vazio:

```powershell
git push -u origin main
```

Se o remoto já tiver histórico, não forçar. Buscar e comparar:

```powershell
git fetch origin
git log --oneline --left-right main...origin/main
```

Integrar somente após revisar a divergência; nunca usar `--force` ou reescrever o histórico remoto sem autorização explícita.

- [ ] **Step 6: Confirmar publicação**

Run:

```powershell
git status -sb
git remote -v
gh repo view gabrielctavares/cs2sj-highlights --web
```

Expected: `main` acompanha `origin/main`, árvore limpa e workflow `verify` iniciado no GitHub.

---

## Critérios finais de aceite

- O cancelamento e o timeout nunca usam `taskkill /IM cs2.exe`.
- A captura usa o tick rate persistido da demo.
- Falha ao apagar CFG não provoca recaptura.
- Migrações possuem um ponto de entrada e versões centralizadas.
- `pipeline.go` deixa de concentrar análise, migração e renderização no mesmo arquivo.
- GUI não importa `internal/cli` e usa eventos tipados.
- Recursos editoriais desligados são representados por uma política explícita.
- A política padrão mantém exatamente o comportamento horizontal atual.
- Testes, vet, módulos, build e smoke passam.
- O Git não contém distribuição, demos, mídia ou credenciais.
- O histórico não contém atribuição a Claude, GPT ou outra IA.
- `main` é publicada em `gabrielctavares/cs2sj-highlights` sem force push.
