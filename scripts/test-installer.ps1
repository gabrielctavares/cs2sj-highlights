param(
    [string]$SetupPath,

    [Parameter(Mandatory = $true)]
    [string]$InnoCompilerPath,

    [Parameter(Mandatory = $true)]
    [string]$StagePath,

    [Parameter(Mandatory = $true)]
    [string]$Version
)

$ErrorActionPreference = 'Stop'

Import-Module (Join-Path $PSScriptRoot 'release\ReleaseTools.psm1') -Force

function Start-CheckedProcess {
    param([string]$FilePath, [string[]]$Arguments, [string]$Description)

    if (-not (Test-Path -LiteralPath $FilePath -PathType Leaf)) {
        throw "$Description não encontrou o executável: $FilePath"
    }
    $process = Start-Process -FilePath $FilePath -ArgumentList $Arguments -Wait -PassThru -WindowStyle Hidden
    if ($process.ExitCode -ne 0) {
        throw "$Description falhou com código $($process.ExitCode)."
    }
}

function Compile-TestSetup {
    param([string]$Compiler, [string]$IssPath, [string]$AppVersion, [string]$Stage, [string]$Output)

    $compilerOutput = & $Compiler "/DAppVersion=$AppVersion" "/DStageDir=$Stage" "/DOutputDir=$Output" $IssPath 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "ISCC.exe falhou ao compilar a versão $AppVersion com código $LASTEXITCODE.`n$($compilerOutput | Out-String)"
    }
    $expected = Join-Path $Output "CS2SJ-Demo-v$AppVersion-Setup.exe"
    if (-not (Test-Path -LiteralPath $expected -PathType Leaf)) {
        throw "Instalador compilado ausente: $expected"
    }
    return $expected
}

$root = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$version = Assert-StableVersion $Version
$compiler = (Resolve-Path -LiteralPath $InnoCompilerPath).Path
if (-not [string]::Equals([System.IO.Path]::GetFileName($compiler), 'ISCC.exe', [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Compilador Inno inválido: $compiler"
}
$stage = (Resolve-Path -LiteralPath $StagePath).Path
Assert-ReleaseStage $stage
$iss = Join-Path $root 'installer\CS2SJ-Demo.iss'
if (-not (Test-Path -LiteralPath $iss -PathType Leaf)) {
    throw "Definição do instalador ausente: $iss"
}

$uninstallKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\CS2SJ-Demo_is1'
if (Test-Path -LiteralPath $uninstallKey) {
    throw 'Teste cancelado: existe uma instalação do CS2SJ Demo para este usuário.'
}

$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("cs2sj installer test " + [guid]::NewGuid())
$baselineOutput = Join-Path $testRoot 'baseline'
$currentOutput = Join-Path $testRoot 'current'
$installDir = Join-Path $testRoot 'installed'
New-Item -ItemType Directory -Path $baselineOutput, $currentOutput | Out-Null

$configDir = Join-Path $env:LOCALAPPDATA 'CS2SJ-Demo'
New-Item -ItemType Directory -Force -Path $configDir | Out-Null
$sentinel = Join-Path $configDir ("installer-test-sentinel-" + [guid]::NewGuid() + '.txt')
[System.IO.File]::WriteAllText($sentinel, 'preserve', [System.Text.UTF8Encoding]::new($false))

try {
    $baselineSetup = Compile-TestSetup $compiler $iss '0.0.0' $stage $baselineOutput
    if ([string]::IsNullOrWhiteSpace($SetupPath)) {
        $currentSetup = Compile-TestSetup $compiler $iss $version $stage $currentOutput
    }
    else {
        $currentSetup = (Resolve-Path -LiteralPath $SetupPath).Path
        $expectedName = (Get-ReleaseArtifactNames $version).Setup
        if ([System.IO.Path]::GetFileName($currentSetup) -ne $expectedName) {
            throw "Nome do instalador inválido: esperado $expectedName."
        }
    }

    $installArguments = @('/CURRENTUSER', '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', "/DIR=`"$installDir`"")
    Start-CheckedProcess $baselineSetup $installArguments 'Instalação baseline'
    Assert-ReleaseStage $installDir
    if (-not (Test-Path -LiteralPath $uninstallKey)) {
        throw "Entrada de desinstalação ausente: $uninstallKey"
    }
    if ((Get-ItemPropertyValue -LiteralPath $uninstallKey -Name DisplayVersion) -ne '0.0.0') {
        throw 'A versão baseline não foi registrada corretamente.'
    }

    Start-CheckedProcess $currentSetup $installArguments 'Atualização do instalador'
    $matchingKeys = @(Get-ChildItem 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall' | Where-Object { $_.PSChildName -eq 'CS2SJ-Demo_is1' })
    if ($matchingKeys.Count -ne 1) {
        throw "A atualização deixou $($matchingKeys.Count) entradas de desinstalação."
    }
    if ((Get-ItemPropertyValue -LiteralPath $uninstallKey -Name DisplayVersion) -ne $version) {
        throw "A atualização não registrou DisplayVersion=$version."
    }

    $cliStdout = Join-Path $testRoot 'cli-stdout.txt'
    $cliStderr = Join-Path $testRoot 'cli-stderr.txt'
    $cliProcess = Start-Process -FilePath (Join-Path $installDir 'cs2-highlights-cli.exe') -RedirectStandardOutput $cliStdout -RedirectStandardError $cliStderr -Wait -PassThru -WindowStyle Hidden
    $cliOutput = ((Get-Content -Raw -LiteralPath $cliStdout) + (Get-Content -Raw -LiteralPath $cliStderr))
    if ($cliProcess.ExitCode -ne 1 -or $cliOutput -notmatch 'Uso:') {
        throw "CLI instalada não iniciou como esperado; código=$($cliProcess.ExitCode) saída=$cliOutput"
    }

    $uninstaller = Join-Path $installDir 'unins000.exe'
    if (-not (Test-Path -LiteralPath $uninstaller -PathType Leaf)) {
        throw "Desinstalador ausente: $uninstaller"
    }
    Start-CheckedProcess $uninstaller @('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART') 'Desinstalação'
    if (Test-Path -LiteralPath $uninstallKey) {
        throw 'A entrada de desinstalação permaneceu após remover o aplicativo.'
    }
    if (Test-Path -LiteralPath $installDir) {
        throw "A pasta instalada permaneceu após a desinstalação: $installDir"
    }
    if (-not (Test-Path -LiteralPath $sentinel -PathType Leaf)) {
        throw 'A desinstalação removeu dados do usuário.'
    }
}
finally {
    $remainingUninstaller = Join-Path $installDir 'unins000.exe'
    if (Test-Path -LiteralPath $remainingUninstaller -PathType Leaf) {
        Start-Process -FilePath $remainingUninstaller -ArgumentList @('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART') -Wait -WindowStyle Hidden
    }
    if (Test-Path -LiteralPath $sentinel -PathType Leaf) {
        Remove-Item -LiteralPath $sentinel -Force
    }
    $resolvedTestRoot = [System.IO.Path]::GetFullPath($testRoot)
    if ($resolvedTestRoot.StartsWith([System.IO.Path]::GetTempPath(), [System.StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedTestRoot)) {
        Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
    }
}

Write-Host 'Installer lifecycle: OK'
