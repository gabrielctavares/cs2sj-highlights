# Instalador e Releases Automáticas Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Gerar, testar e publicar automaticamente um ZIP portátil, um instalador offline por usuário e seus checksums para cada GitHub Release do CS2SJ Demo.

**Architecture:** O stage `dist/stage/CS2SJ-Demo` permanece como fonte única do runtime distribuído. Funções PowerShell testáveis validam versões, dependências, stage e hashes; scripts finos orquestram download, packaging, Inno Setup e teste de ciclo de vida; um workflow Windows publica somente depois que todas as verificações passam.

**Tech Stack:** PowerShell 7, Go 1.24.x, Python 3.9+ com Pillow 11.3.0, Inno Setup 7.1.0, GitHub Actions, GitHub CLI, HLAE 2.191.0 e FFmpeg 9.0.1 essentials x64.

**Spec:** `docs/superpowers/specs/2026-08-24-instalador-e-releases-automaticas-design.md`

## Global Constraints

- A plataforma suportada é Windows 10 ou mais recente, somente x64.
- A instalação é sempre por usuário, em `%LocalAppData%\Programs\CS2SJ-Demo`, sem elevação.
- O instalador é offline; o PC do usuário final não baixa dependências.
- As tags aceitas têm exatamente o formato `vMAJOR.MINOR.PATCH`; o parâmetro local usa `MAJOR.MINOR.PATCH`.
- Os artefatos de `v1.2.3` são exatamente `CS2SJ-Demo-v1.2.3-windows-x64.zip`, `CS2SJ-Demo-v1.2.3-Setup.exe` e `SHA256SUMS.txt`.
- O executável principal nunca é publicado isoladamente.
- O `AppId` é `CS2SJ-Demo` e não pode mudar entre versões.
- HLAE, FFmpeg e Inno Setup só podem ser extraídos ou executados após validação SHA-256.
- HLAE fica fixado em `2.191.0`, FFmpeg em `9.0.1` essentials x64 e Inno Setup em `7.1.0` x64 até atualização explícita do manifesto.
- Configurações em `%LocalAppData%\CS2SJ-Demo`, temas e vídeos do usuário nunca são removidos pelo desinstalador.
- Assinatura Authenticode, autoatualização, MSI, MSIX e instalação para todos os usuários permanecem fora do escopo.

## File Map

- Create `scripts/release/ReleaseTools.psm1`: funções reutilizáveis de validação, download verificado, stage, nomes e hashes.
- Create `scripts/test-release-tools.ps1`: harness determinístico sem dependência de Pester.
- Create `installer/dependencies.json`: versões, URLs e SHA-256 fixados.
- Modify `scripts/package.ps1`: aceitar FFmpeg e FFprobe externos e normalizar ambos no stage.
- Create `scripts/test-package.ps1`: integração do packaging com runtime mínimo falso.
- Create `requirements-build.txt`: dependência Python fixada para gerar o ícone Windows.
- Create `installer/CS2SJ-Demo.iss`: definição declarativa do instalador por usuário.
- Create `scripts/test-inno-contract.ps1`: contrato estático das diretivas críticas do `.iss`.
- Create `scripts/build-installer.ps1`: orquestrador local para package, ZIP, setup e checksums.
- Create `scripts/get-release-dependencies.ps1`: baixar, verificar e preparar runtime e compilador no CI.
- Create `scripts/test-installer.ps1`: instalação, upgrade e desinstalação silenciosos.
- Create `.github/workflows/release.yml`: pipeline acionado por release publicada.
- Modify `.github/workflows/verify.yml`: preparar a dependência Python antes da verificação que compila o pacote.
- Modify `scripts/verify.ps1`: incluir testes PowerShell puros.
- Modify `third_party/NOTICE.md`: registrar FFmpeg 9.0.1 essentials x64.
- Modify `README.md`: instalação, build local e processo de release.

---

### Task 1: Release utilities and pinned dependency manifest

**Files:**
- Create: `scripts/release/ReleaseTools.psm1`
- Create: `scripts/test-release-tools.ps1`
- Create: `installer/dependencies.json`
- Modify: `scripts/verify.ps1`

**Interfaces:**
- Produces: `Assert-StableVersion -Version <string> -> string`.
- Produces: `Get-ReleaseArtifactNames -Version <string> -> PSCustomObject{Tag,Zip,Setup,Checksums}`.
- Produces: `Read-ReleaseDependencies -Path <string> -> object[]`.
- Produces: `Get-VerifiedDownload -Dependency <object> -DestinationDirectory <string> [-DownloadFile <scriptblock>] -> string`.
- Produces: `Assert-ReleaseStage -Path <string> -> void`.
- Produces: `Write-ReleaseChecksums -ArtifactPaths <string[]> -OutputPath <string> -> void`.

- [ ] **Step 1: Write a failing PowerShell test harness**

Create `scripts/test-release-tools.ps1` with small `Assert-Equal` and `Assert-Throws` helpers. Import the not-yet-existing module and cover version syntax, exact names, invalid manifests, hash mismatch, incomplete stage and checksum ordering:

```powershell
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Import-Module (Join-Path $PSScriptRoot 'release\ReleaseTools.psm1') -Force

function Assert-Equal($Expected, $Actual, [string]$Message) {
    if ($Expected -ne $Actual) { throw "$Message; esperado=$Expected obtido=$Actual" }
}
function Assert-Throws([scriptblock]$Action, [string]$Pattern) {
    try { & $Action; throw "Era esperada falha contendo: $Pattern" }
    catch { if ($_.Exception.Message -notmatch $Pattern) { throw } }
}

Assert-Equal '1.2.3' (Assert-StableVersion '1.2.3') 'versão válida'
@('', 'v1.2.3', '1.2', '1.2.3-beta') | ForEach-Object {
    $candidate = $_
    Assert-Throws { Assert-StableVersion $candidate } 'MAJOR\.MINOR\.PATCH'
}
$names = Get-ReleaseArtifactNames '1.2.3'
Assert-Equal 'v1.2.3' $names.Tag 'tag'
Assert-Equal 'CS2SJ-Demo-v1.2.3-windows-x64.zip' $names.Zip 'zip'
Assert-Equal 'CS2SJ-Demo-v1.2.3-Setup.exe' $names.Setup 'setup'

$temp = Join-Path ([System.IO.Path]::GetTempPath()) ("cs2sj-release-tools-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $badManifest = Join-Path $temp 'bad.json'
    '{"schemaVersion":1,"dependencies":[{"name":"x","version":"1","url":"http://example.test/x","sha256":"abc"}]}' | Set-Content -LiteralPath $badManifest -Encoding utf8NoBOM
    Assert-Throws { Read-ReleaseDependencies $badManifest } 'HTTPS|SHA-256'

    $source = Join-Path $temp 'source.bin'
    'conteudo' | Set-Content -LiteralPath $source -Encoding utf8NoBOM
    $dependency = [pscustomobject]@{ name='fixture'; version='1'; url='https://example.test/source.bin'; sha256=('0' * 64) }
    Assert-Throws {
        Get-VerifiedDownload $dependency $temp { param($Uri, $OutFile) Copy-Item -LiteralPath $source -Destination $OutFile }
    } 'SHA-256'

    $stage = Join-Path $temp 'stage'
    New-Item -ItemType Directory -Path $stage | Out-Null
    Assert-Throws { Assert-ReleaseStage $stage } 'CS2SJ-Demo.exe'

    $a = Join-Path $temp 'a.zip'; $b = Join-Path $temp 'b.exe'
    'a' | Set-Content -LiteralPath $a -Encoding utf8NoBOM
    'b' | Set-Content -LiteralPath $b -Encoding utf8NoBOM
    $sums = Join-Path $temp 'SHA256SUMS.txt'
    Write-ReleaseChecksums @($a, $b) $sums
    $lines = Get-Content -LiteralPath $sums
    Assert-Equal 2 $lines.Count 'quantidade de hashes'
    if ($lines[0] -notmatch '^[0-9a-f]{64}  a\.zip$' -or $lines[1] -notmatch '^[0-9a-f]{64}  b\.exe$') { throw "checksums inválidos: $lines" }
}
finally {
    if ($temp.StartsWith([System.IO.Path]::GetTempPath(), [System.StringComparison]::OrdinalIgnoreCase)) {
        Remove-Item -LiteralPath $temp -Recurse -Force
    }
}
Write-Host 'ReleaseTools: OK'
```

- [ ] **Step 2: Run the harness and verify it fails because the module is absent**

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\test-release-tools.ps1`

Expected: FAIL at `Import-Module` because `scripts/release/ReleaseTools.psm1` does not exist.

- [ ] **Step 3: Add the exact pinned manifest**

Create `installer/dependencies.json`:

```json
{
  "schemaVersion": 1,
  "dependencies": [
    {
      "name": "hlae",
      "version": "2.191.0",
      "url": "https://github.com/advancedfx/advancedfx/releases/download/v2.191.0/hlae_2_191_0.zip",
      "sha256": "78efa377a2bac9522c3771a79c2503fec57e106432fc11d32244fe25b7c5b6cc"
    },
    {
      "name": "ffmpeg",
      "version": "9.0.1",
      "url": "https://www.gyan.dev/ffmpeg/builds/packages/ffmpeg-9.0.1-essentials_build.zip",
      "sha256": "fec81ae03971d9dd4be3ebe02e263bd2ec1d789483f931bdba5f5715e65da2e9"
    },
    {
      "name": "inno-setup",
      "version": "7.1.0",
      "url": "https://github.com/jrsoftware/issrc/releases/download/is-7_1_0/innosetup-7.1.0-x64.exe",
      "sha256": "0362a383ed217d4c4239b5933866dd96d3eb2102737da92f80f6057a4b40df2f"
    }
  ]
}
```

- [ ] **Step 4: Implement the reusable module**

Create `scripts/release/ReleaseTools.psm1`. Use `Resolve-Path` for inputs, require `schemaVersion = 1`, require exactly the dependency names above, reject non-HTTPS URLs and require lowercase or uppercase 64-digit hexadecimal hashes. `Get-VerifiedDownload` must download to `<DestinationDirectory>\<URL filename>`, compare `Get-FileHash -Algorithm SHA256`, delete only that failed leaf file, and return its absolute path. `Assert-ReleaseStage` must require these relative files plus recursive hook/media matches:

```powershell
$required = @(
    'CS2SJ-Demo.exe',
    'CS2SJ-Demo.exe.manifest',
    'cs2-highlights-cli.exe',
    'tools\hlae\hlae.exe',
    'assets\cs2sj-logo.jpg',
    'assets\cs2sj-logo.ico',
    'licenses\NOTICE.md',
    'licenses\WALK-LICENSE.txt'
)
```

Use these exact artifact-name properties:

```powershell
function Get-ReleaseArtifactNames([string]$Version) {
    $valid = Assert-StableVersion $Version
    [pscustomobject]@{
        Tag       = "v$valid"
        Zip       = "CS2SJ-Demo-v$valid-windows-x64.zip"
        Setup     = "CS2SJ-Demo-v$valid-Setup.exe"
        Checksums = 'SHA256SUMS.txt'
    }
}
```

`Write-ReleaseChecksums` must sort by filename using ordinal-ignore-case order and write UTF-8 without BOM with lowercase hashes. Export only the six public functions listed in **Interfaces**.

- [ ] **Step 5: Run the unit harness**

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\test-release-tools.ps1`

Expected: `ReleaseTools: OK` and exit code 0.

- [ ] **Step 6: Add the harness to normal verification**

In `scripts/verify.ps1`, run the harness before `go mod verify`:

```powershell
& (Join-Path $PSScriptRoot 'test-release-tools.ps1')
if ($LASTEXITCODE -ne 0) { throw 'Testes das ferramentas de release falharam.' }
```

- [ ] **Step 7: Run repository verification**

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1`

Expected: `ReleaseTools: OK`, all Go checks pass, and the CLI verification binary is built.

- [ ] **Step 8: Commit the utility boundary**

```powershell
git add scripts/release/ReleaseTools.psm1 scripts/test-release-tools.ps1 installer/dependencies.json scripts/verify.ps1
git commit -m "build: add verified release utilities"
```

### Task 2: Package external FFmpeg and FFprobe deterministically

**Files:**
- Modify: `scripts/package.ps1`
- Create: `scripts/test-package.ps1`
- Create: `requirements-build.txt`
- Modify: `scripts/verify.ps1`
- Modify: `.github/workflows/verify.yml`

**Interfaces:**
- Consumes: `Assert-ReleaseStage -Path <string>` from Task 1.
- Produces: `scripts/package.ps1 -HLAEDir <dir> [-FFmpegPath <file>] [-FFprobePath <file>] [-CreateZip] [-ZipPath <file>]`.

- [ ] **Step 1: Write a failing packaging integration test**

Create `scripts/test-package.ps1`. Build a unique temp fixture containing empty `hlae.exe`, `x64\AfxHookSource2.dll`, external `ffmpeg.exe`, external `ffprobe.exe` and `LICENSES\HLAE.txt`; invoke `package.ps1` with both external media paths; then assert both are at `dist\stage\CS2SJ-Demo\tools\hlae\ffmpeg\bin`. The `finally` block may remove only the validated fixture under the system temp directory, never `dist`.

Core invocation and assertions:

```powershell
& (Join-Path $PSScriptRoot 'package.ps1') `
    -HLAEDir $hlae `
    -FFmpegPath (Join-Path $fixture 'media\ffmpeg.exe') `
    -FFprobePath (Join-Path $fixture 'media\ffprobe.exe')
if ($LASTEXITCODE -ne 0) { throw 'package.ps1 falhou com mídia externa.' }

$stage = Join-Path $root 'dist\stage\CS2SJ-Demo'
foreach ($name in @('ffmpeg.exe', 'ffprobe.exe')) {
    $path = Join-Path $stage "tools\hlae\ffmpeg\bin\$name"
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Ausente: $path" }
}
```

- [ ] **Step 2: Run the test and verify the new parameter is rejected**

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\test-package.ps1`

Expected: FAIL because `package.ps1` has no `FFmpegPath` parameter.

- [ ] **Step 3: Extend packaging with explicit media fallbacks**

Add optional `FFmpegPath` and `ZipPath` parameters. Resolve bundled media first; when absent, require the matching explicit path and validate both leaf existence and exact filename case-insensitively. After copying HLAE, always normalize selected binaries to:

```powershell
$mediaDir = Join-Path $stage 'tools\hlae\ffmpeg\bin'
New-Item -ItemType Directory -Force -Path $mediaDir | Out-Null
Copy-Item -LiteralPath $ffmpeg.FullName -Destination (Join-Path $mediaDir 'ffmpeg.exe') -Force
Copy-Item -LiteralPath $ffprobe.FullName -Destination (Join-Path $mediaDir 'ffprobe.exe') -Force
```

When `CreateZip` is present, use the absolute `ZipPath` if supplied; otherwise retain `dist\CS2SJ-Demo-windows-x64.zip`. Reject a supplied ZIP destination whose extension is not `.zip`. Import Task 1's module and replace the duplicated final stage checks with `Assert-ReleaseStage $stage`.

- [ ] **Step 4: Pin the icon-generation dependency in local and CI builds**

Create `requirements-build.txt`:

```text
Pillow==11.3.0
```

In `.github/workflows/verify.yml`, add `actions/setup-python@v5` with `python-version: '3.13'`, followed by `python -m pip install --requirement requirements-build.txt`, before the Verify step. Do not install unpinned Python packages.

- [ ] **Step 5: Run packaging and repository checks**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test-package.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1
```

Expected: packaging fixture succeeds and all verification checks pass.

- [ ] **Step 6: Add packaging integration to verification and commit**

Add after the pure release tests in `scripts/verify.ps1`:

```powershell
& (Join-Path $PSScriptRoot 'test-package.ps1')
if ($LASTEXITCODE -ne 0) { throw 'Teste do pacote portátil falhou.' }
```

Then commit:

```powershell
git add scripts/package.ps1 scripts/test-package.ps1 requirements-build.txt scripts/verify.ps1 .github/workflows/verify.yml
git commit -m "build: package external media tools"
```

### Task 3: Inno Setup definition and contract tests

**Files:**
- Create: `installer/CS2SJ-Demo.iss`
- Create: `scripts/test-inno-contract.ps1`
- Modify: `scripts/verify.ps1`

**Interfaces:**
- Consumes: preprocessor defines `/DAppVersion=`, `/DStageDir=` and `/DOutputDir=`.
- Produces: `CS2SJ-Demo-v<AppVersion>-Setup.exe`.
- Produces: uninstall registry identity `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\CS2SJ-Demo_is1`.

- [ ] **Step 1: Write a failing static contract test**

Create `scripts/test-inno-contract.ps1` to read `installer/CS2SJ-Demo.iss` as raw text and require each exact security/distribution directive:

```powershell
$requiredPatterns = @(
    '(?m)^AppId=CS2SJ-Demo$',
    '(?m)^PrivilegesRequired=lowest$',
    '(?m)^DefaultDirName=\{localappdata\}\\Programs\\CS2SJ-Demo$',
    '(?m)^ArchitecturesAllowed=x64compatible$',
    '(?m)^MinVersion=10\.0$',
    '(?m)^Compression=lzma2/ultra64$',
    '(?m)^SolidCompression=yes$',
    '(?m)^OutputBaseFilename=CS2SJ-Demo-v\{#AppVersion\}-Setup$',
    '(?m)^Name: "brazilianportuguese";',
    '(?m)^Name: "desktopicon";.*unchecked',
    '(?m)^Filename: "\{app\}\\CS2SJ-Demo\.exe";.*postinstall.*skipifsilent'
)
```

Also reject any `[UninstallDelete]` entry containing `{localappdata}\CS2SJ-Demo`.

- [ ] **Step 2: Run the test and verify the `.iss` is absent**

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\test-inno-contract.ps1`

Expected: FAIL because `installer/CS2SJ-Demo.iss` does not exist.

- [ ] **Step 3: Create the installer definition**

Create `installer/CS2SJ-Demo.iss` with mandatory preprocessor guards and these sections:

```ini
#ifndef AppVersion
  #error AppVersion is required
#endif
#ifndef StageDir
  #error StageDir is required
#endif
#ifndef OutputDir
  #error OutputDir is required
#endif

[Setup]
AppId=CS2SJ-Demo
AppName=CS2SJ Demo
AppVersion={#AppVersion}
AppVerName=CS2SJ Demo {#AppVersion}
AppPublisher=CS2SJ
DefaultDirName={localappdata}\Programs\CS2SJ-Demo
DefaultGroupName=CS2SJ Demo
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir={#OutputDir}
OutputBaseFilename=CS2SJ-Demo-v{#AppVersion}-Setup
SetupIconFile={#StageDir}\assets\cs2sj-logo.ico
UninstallDisplayIcon={app}\CS2SJ-Demo.exe
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
CloseApplications=force
RestartApplications=yes
VersionInfoVersion={#AppVersion}
VersionInfoProductName=CS2SJ Demo

[Languages]
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"

[Tasks]
Name: "desktopicon"; Description: "Criar um atalho na área de trabalho"; GroupDescription: "Atalhos adicionais:"; Flags: unchecked

[Files]
Source: "{#StageDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\CS2SJ Demo"; Filename: "{app}\CS2SJ-Demo.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\CS2SJ Demo"; Filename: "{app}\CS2SJ-Demo.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\CS2SJ-Demo.exe"; Description: "Abrir o CS2SJ Demo"; WorkingDir: "{app}"; Flags: nowait postinstall skipifsilent
```

Do not add `[UninstallDelete]`; Inno's file log removes installed files while user configuration remains outside `{app}`.

- [ ] **Step 4: Run the contract test and register it in verification**

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\test-inno-contract.ps1`

Expected: exit 0 and `Inno contract: OK`.

Add the same invocation to `scripts/verify.ps1`, then run the full verification.

- [ ] **Step 5: Commit the installer contract**

```powershell
git add installer/CS2SJ-Demo.iss scripts/test-inno-contract.ps1 scripts/verify.ps1
git commit -m "build: define per-user Windows installer"
```

### Task 4: Local installer build orchestrator

**Files:**
- Create: `scripts/build-installer.ps1`
- Modify: `scripts/test-release-tools.ps1`

**Interfaces:**
- Consumes: Task 1 artifact helpers and Task 2 `package.ps1` parameters.
- Consumes: Task 3 `CS2SJ-Demo.iss` preprocessor defines.
- Produces: the three exact public artifacts under `dist`.

- [ ] **Step 1: Extend the failing tests for compiler discovery and output naming**

Add `Resolve-InnoCompiler -ExplicitPath <string> -> string` to the expected Task 1 interface. Test that a valid temp leaf named `ISCC.exe` is returned and a leaf named `compiler.exe` is rejected. Add it to `Export-ModuleMember` only after the test fails.

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\test-release-tools.ps1`

Expected: FAIL because `Resolve-InnoCompiler` is undefined.

- [ ] **Step 2: Implement compiler resolution**

Implement `Resolve-InnoCompiler` in `ReleaseTools.psm1`. When the explicit path is empty, search in this order:

```powershell
@(
    (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 7\ISCC.exe'),
    (Join-Path $env:ProgramFiles 'Inno Setup 7\ISCC.exe'),
    (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 7\ISCC.exe'),
    (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 6\ISCC.exe'),
    (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe')
)
```

Ignore candidates whose parent environment variable is empty. Return an absolute leaf named `ISCC.exe`; otherwise throw a message naming `-InnoCompilerPath`.

- [ ] **Step 3: Create `build-installer.ps1`**

Use mandatory `Version`, `HLAEDir`, `FFmpegPath`, `FFprobePath` and optional `InnoCompilerPath`. The script must:

1. validate the version and resolve all inputs before changing `dist`;
2. call `package.ps1` with an explicit versioned `ZipPath`;
3. call `Assert-ReleaseStage`;
4. invoke `ISCC.exe` with quoted `/DAppVersion`, `/DStageDir`, `/DOutputDir` and the `.iss` path;
5. check `$LASTEXITCODE` and exact setup path;
6. write `SHA256SUMS.txt` for only ZIP and setup;
7. print the three absolute artifact paths.

The external invocation must have this shape:

```powershell
& $iscc `
    "/DAppVersion=$version" `
    "/DStageDir=$stage" `
    "/DOutputDir=$dist" `
    (Join-Path $root 'installer\CS2SJ-Demo.iss')
if ($LASTEXITCODE -ne 0) { throw "ISCC.exe falhou com código $LASTEXITCODE." }
```

- [ ] **Step 4: Run all tests that do not require Inno Setup**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test-release-tools.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\test-inno-contract.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1
```

Expected: all pass. A real compilation is deferred to Task 6, where a verified compiler and runtime exist.

- [ ] **Step 5: Commit the local build entry point**

```powershell
git add scripts/release/ReleaseTools.psm1 scripts/test-release-tools.ps1 scripts/build-installer.ps1
git commit -m "build: orchestrate offline installer artifacts"
```

### Task 5: Verified release dependency acquisition

**Files:**
- Create: `scripts/get-release-dependencies.ps1`
- Modify: `scripts/test-release-tools.ps1`

**Interfaces:**
- Consumes: `installer/dependencies.json`, `Read-ReleaseDependencies`, and `Get-VerifiedDownload`.
- Produces: normalized runtime at `<DestinationRoot>\runtime\hlae`.
- Produces: compiler at `<DestinationRoot>\inno\ISCC.exe` or a recursively discovered equivalent.
- Produces: JSON path map `{hlaeDir,ffmpegPath,ffprobePath,innoCompilerPath}` at `PathsOutputPath`.

- [ ] **Step 1: Add a successful injected-download test**

In `scripts/test-release-tools.ps1`, create a source leaf, calculate its real hash, pass a downloader scriptblock that copies that file, and assert `Get-VerifiedDownload` returns an existing leaf with the same hash. Run the harness once to ensure the success path is now protected.

- [ ] **Step 2: Implement dependency acquisition without cleanup of caller-owned directories**

Create `scripts/get-release-dependencies.ps1` with mandatory `DestinationRoot` and `PathsOutputPath`. Fail if `DestinationRoot` already exists. Then:

- read and validate `installer/dependencies.json`;
- create `downloads`, `hlae-extracted`, `runtime\hlae`, `ffmpeg-extracted` and `inno` below the destination;
- call `Get-VerifiedDownload` for all three entries;
- expand the HLAE ZIP into `hlae-extracted`, locate the unique recursive `hlae.exe`, and copy the complete contents of its parent into `runtime\hlae`; this removes any archive root folder without assuming its name;
- require `runtime\hlae\hlae.exe` and a recursive `AfxHookSource2.dll`;
- expand FFmpeg, locate unique recursive `ffmpeg.exe` and `ffprobe.exe`, and copy them into `runtime\hlae\ffmpeg\bin`;
- find the FFmpeg archive's `LICENSE` leaf and copy it to `runtime\hlae\LICENSES\FFMPEG-LICENSE.txt`;
- execute the verified Inno installer with `/PORTABLE=1 /VERYSILENT /SUPPRESSMSGBOXES /NORESTART` and `/DIR=<DestinationRoot>\inno`;
- locate `ISCC.exe` recursively and validate it with `Resolve-InnoCompiler`;
- write the four absolute paths as JSON UTF-8 without BOM.

The Inno invocation must check its exit code before searching for the compiler:

```powershell
& $innoInstaller '/PORTABLE=1' '/VERYSILENT' '/SUPPRESSMSGBOXES' '/NORESTART' "/DIR=$innoRoot"
if ($LASTEXITCODE -ne 0) { throw "Preparação portátil do Inno Setup falhou com código $LASTEXITCODE." }
```

- [ ] **Step 3: Execute the real acquisition once**

Run with a fresh explicit temp child:

```powershell
$depsRoot = Join-Path $env:TEMP ("cs2sj-deps-" + [guid]::NewGuid())
$paths = Join-Path $env:TEMP ("cs2sj-paths-" + [guid]::NewGuid() + '.json')
powershell -ExecutionPolicy Bypass -File .\scripts\get-release-dependencies.ps1 -DestinationRoot $depsRoot -PathsOutputPath $paths
Get-Content -Raw -LiteralPath $paths | ConvertFrom-Json | Format-List
```

Expected: all downloads match their pinned hashes; the four reported paths exist. Remove only the printed temp paths after resolving and confirming they are children of `$env:TEMP`.

- [ ] **Step 4: Re-run pure tests and commit**

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test-release-tools.ps1
git add scripts/get-release-dependencies.ps1 scripts/test-release-tools.ps1
git commit -m "build: acquire pinned release dependencies"
```

### Task 6: Installer lifecycle integration test

**Files:**
- Create: `scripts/test-installer.ps1`

**Interfaces:**
- Consumes: current setup, `ISCC.exe`, stage, `.iss`, and current version.
- Produces: verified baseline-to-current upgrade, CLI startup, uninstall and config preservation.

- [ ] **Step 1: Create the lifecycle test with a deliberate precondition failure**

Create `scripts/test-installer.ps1` with mandatory `SetupPath`, `InnoCompilerPath`, `StagePath` and `Version`. Begin by validating all four and requiring that `SetupPath` equals the exact name returned by `Get-ReleaseArtifactNames`. Run it against a nonexistent setup path and confirm it fails before writing registry or filesystem state.

- [ ] **Step 2: Implement isolated install, upgrade and uninstall checks**

The script must:

1. create a unique install directory under `$env:TEMP` and verify the resolved path remains under `$env:TEMP`;
2. compile a baseline `0.0.0` setup from the same stage into another unique temp directory;
3. create a unique sentinel leaf under `%LocalAppData%\CS2SJ-Demo` without modifying `config.json`;
4. install the baseline silently with `/CURRENTUSER /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /DIR=<test dir>`;
5. assert all stage requirements against the installed directory;
6. assert exactly one `CS2SJ-Demo_is1` uninstall key under HKCU and `DisplayVersion = 0.0.0`;
7. install the current setup to the same directory and assert the same key now has `DisplayVersion = <Version>`;
8. execute installed `cs2-highlights-cli.exe` without arguments, require exit code 1 and output containing `Uso:`;
9. invoke `<test dir>\unins000.exe` with `/VERYSILENT /SUPPRESSMSGBOXES /NORESTART`;
10. assert the install directory and uninstall key are gone while the sentinel remains;
11. remove only the sentinel and validated temp children in `finally`.

Use `Start-Process -Wait -PassThru -WindowStyle Hidden` for setup and uninstall processes. Do not launch the GUI, HLAE or CS2.

- [ ] **Step 3: Build real artifacts with verified dependencies**

Using the paths produced in Task 5:

```powershell
$deps = Get-Content -Raw -LiteralPath $paths | ConvertFrom-Json
powershell -ExecutionPolicy Bypass -File .\scripts\build-installer.ps1 `
  -Version '1.0.0' `
  -HLAEDir $deps.hlaeDir `
  -FFmpegPath $deps.ffmpegPath `
  -FFprobePath $deps.ffprobePath `
  -InnoCompilerPath $deps.innoCompilerPath
```

Expected: the exact ZIP, setup and checksum files exist under `dist`.

- [ ] **Step 4: Run the lifecycle test**

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test-installer.ps1 `
  -SetupPath .\dist\CS2SJ-Demo-v1.0.0-Setup.exe `
  -InnoCompilerPath $deps.innoCompilerPath `
  -StagePath .\dist\stage\CS2SJ-Demo `
  -Version '1.0.0'
```

Expected: baseline install, upgrade, CLI startup, uninstall and sentinel preservation all pass; no test uninstall entry remains.

- [ ] **Step 5: Commit the lifecycle test**

```powershell
git add scripts/test-installer.ps1
git commit -m "test: verify installer lifecycle"
```

### Task 7: GitHub Release workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: `release.published`, release tag, scripts from Tasks 1-6.
- Produces: three assets attached to the matching GitHub Release.

- [ ] **Step 1: Add the release workflow with minimal permissions**

Create `.github/workflows/release.yml`:

```yaml
name: release

on:
  release:
    types: [published]

permissions:
  contents: write

jobs:
  windows-release:
    runs-on: windows-latest
    steps:
      - name: Check out release tag
        uses: actions/checkout@v4
        with:
          ref: ${{ github.event.release.tag_name }}

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
          cache: true

      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: '3.13'
          cache: pip

      - name: Install pinned build dependency
        shell: pwsh
        run: python -m pip install --requirement requirements-build.txt

      - name: Validate release tag
        id: version
        shell: pwsh
        env:
          RELEASE_TAG: ${{ github.event.release.tag_name }}
        run: |
          if ($env:RELEASE_TAG -notmatch '^v([0-9]+\.[0-9]+\.[0-9]+)$') {
            throw "Tag inválida: $env:RELEASE_TAG. Use vMAJOR.MINOR.PATCH."
          }
          "value=$($Matches[1])" >> $env:GITHUB_OUTPUT

      - name: Verify source
        shell: pwsh
        run: ./scripts/verify.ps1

      - name: Acquire pinned dependencies
        shell: pwsh
        run: |
          ./scripts/get-release-dependencies.ps1 `
            -DestinationRoot (Join-Path $env:RUNNER_TEMP 'cs2sj-dependencies') `
            -PathsOutputPath (Join-Path $env:RUNNER_TEMP 'cs2sj-dependency-paths.json')

      - name: Build offline artifacts
        shell: pwsh
        env:
          RELEASE_VERSION: ${{ steps.version.outputs.value }}
        run: |
          $deps = Get-Content -Raw (Join-Path $env:RUNNER_TEMP 'cs2sj-dependency-paths.json') | ConvertFrom-Json
          ./scripts/build-installer.ps1 `
            -Version $env:RELEASE_VERSION `
            -HLAEDir $deps.hlaeDir `
            -FFmpegPath $deps.ffmpegPath `
            -FFprobePath $deps.ffprobePath `
            -InnoCompilerPath $deps.innoCompilerPath

      - name: Test install, upgrade and uninstall
        shell: pwsh
        env:
          RELEASE_VERSION: ${{ steps.version.outputs.value }}
        run: |
          $deps = Get-Content -Raw (Join-Path $env:RUNNER_TEMP 'cs2sj-dependency-paths.json') | ConvertFrom-Json
          ./scripts/test-installer.ps1 `
            -SetupPath "./dist/CS2SJ-Demo-v$env:RELEASE_VERSION-Setup.exe" `
            -InnoCompilerPath $deps.innoCompilerPath `
            -StagePath './dist/stage/CS2SJ-Demo' `
            -Version $env:RELEASE_VERSION

      - name: Publish release assets
        shell: pwsh
        env:
          GH_TOKEN: ${{ github.token }}
          RELEASE_TAG: ${{ github.event.release.tag_name }}
          RELEASE_VERSION: ${{ steps.version.outputs.value }}
        run: |
          $assets = @(
            "dist/CS2SJ-Demo-v$env:RELEASE_VERSION-windows-x64.zip",
            "dist/CS2SJ-Demo-v$env:RELEASE_VERSION-Setup.exe",
            'dist/SHA256SUMS.txt'
          )
          foreach ($asset in $assets) {
            if (-not (Test-Path -LiteralPath $asset -PathType Leaf)) { throw "Artefato ausente: $asset" }
          }
          gh release upload $env:RELEASE_TAG @assets --repo $env:GITHUB_REPOSITORY --clobber
```

- [ ] **Step 2: Validate YAML and workflow references locally**

Run:

```powershell
rg -n "release:|types: \[published\]|contents: write|github\.event\.release\.tag_name|gh release upload" .github/workflows/release.yml
powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1
```

Expected: each required workflow contract appears once and all local checks pass.

- [ ] **Step 3: Commit release automation**

```powershell
git add .github/workflows/release.yml
git commit -m "ci: publish Windows release artifacts"
```

### Task 8: Notices, user documentation and final verification

**Files:**
- Modify: `third_party/NOTICE.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: artifact names, commands and behavior implemented in Tasks 1-7.
- Produces: user and maintainer instructions that match the shipped flow.

- [ ] **Step 1: Update third-party notices**

Change the FFmpeg line to identify `FFmpeg 9.0.1 essentials x64 build by Gyan Doshi`, retain the official FFmpeg project URL, add `https://www.gyan.dev/ffmpeg/builds/`, and state that its GPLv3 license is included as `licenses/HLAE-LICENSES/FFMPEG-LICENSE.txt` in packaged releases.

- [ ] **Step 2: Rewrite the README distribution entry points**

At the start of **Uso pelo aplicativo**, make the recommended path:

```text
1. Baixe `CS2SJ-Demo-vX.Y.Z-Setup.exe` na página de Releases.
2. Execute o instalador; ele não solicita senha de administrador.
3. Abra **CS2SJ Demo** pelo menu Iniciar.
```

Document the portable ZIP as the alternative that must be fully extracted. Add sections **Atualizar e desinstalar**, **Gerar instalador localmente**, and **Publicar uma release**. State that a new setup upgrades the existing per-user install, uninstall preserves `%LocalAppData%\CS2SJ-Demo`, tags must be `vMAJOR.MINOR.PATCH`, and the release workflow uploads the exact three artifact names.
Also state plainly that this first version is not Authenticode-signed and Windows SmartScreen may therefore show a reputation warning.

- [ ] **Step 3: Run complete verification with a fresh dependency directory**

Run, in order:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\get-release-dependencies.ps1 -DestinationRoot $depsRoot -PathsOutputPath $paths
$deps = Get-Content -Raw -LiteralPath $paths | ConvertFrom-Json
powershell -ExecutionPolicy Bypass -File .\scripts\build-installer.ps1 -Version '1.0.0' -HLAEDir $deps.hlaeDir -FFmpegPath $deps.ffmpegPath -FFprobePath $deps.ffprobePath -InnoCompilerPath $deps.innoCompilerPath
powershell -ExecutionPolicy Bypass -File .\scripts\test-installer.ps1 -SetupPath .\dist\CS2SJ-Demo-v1.0.0-Setup.exe -InnoCompilerPath $deps.innoCompilerPath -StagePath .\dist\stage\CS2SJ-Demo -Version '1.0.0'
Get-Content .\dist\SHA256SUMS.txt
```

Expected: all source checks and lifecycle tests pass; ZIP and setup exist; `SHA256SUMS.txt` has exactly two lowercase SHA-256 lines with the expected filenames.

- [ ] **Step 4: Inspect the final installer artifacts**

Confirm `Get-Item` reports nonzero sizes, `Get-AuthenticodeSignature` reports `NotSigned` as explicitly expected for this scope, and the ZIP contains the same relative layout asserted by `Assert-ReleaseStage`.

- [ ] **Step 5: Commit documentation and notices**

```powershell
git add README.md third_party/NOTICE.md
git commit -m "docs: document installer and release flow"
```

- [ ] **Step 6: Request code review before integration**

Invoke `superpowers:requesting-code-review` against the full commit range from the pre-Task-1 base through HEAD. Address findings through `superpowers:receiving-code-review`, rerun Step 3, and only then invoke `superpowers:finishing-a-development-branch`.
