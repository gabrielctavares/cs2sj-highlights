param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [string]$HLAEDir,

    [Parameter(Mandatory = $true)]
    [string]$FFmpegPath,

    [Parameter(Mandatory = $true)]
    [string]$FFprobePath,

    [string]$InnoCompilerPath
)

$ErrorActionPreference = 'Stop'

Import-Module (Join-Path $PSScriptRoot 'release\ReleaseTools.psm1') -Force

$root = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$version = Assert-StableVersion $Version
$names = Get-ReleaseArtifactNames $version
$hlae = (Resolve-Path -LiteralPath $HLAEDir).Path
$ffmpeg = (Resolve-Path -LiteralPath $FFmpegPath).Path
$ffprobe = (Resolve-Path -LiteralPath $FFprobePath).Path
$iscc = Resolve-InnoCompiler $InnoCompilerPath
$dist = Join-Path $root 'dist'
$stage = Join-Path $dist 'stage\CS2SJ-Demo'
$zip = Join-Path $dist $names.Zip
$setup = Join-Path $dist $names.Setup
$checksums = Join-Path $dist $names.Checksums
$iss = Join-Path $root 'installer\CS2SJ-Demo.iss'

if (-not (Test-Path -LiteralPath $iss -PathType Leaf)) {
    throw "Definição do instalador ausente: $iss"
}

& (Join-Path $PSScriptRoot 'package.ps1') `
    -HLAEDir $hlae `
    -FFmpegPath $ffmpeg `
    -FFprobePath $ffprobe
Assert-ReleaseStage $stage

New-Item -ItemType Directory -Force -Path $dist | Out-Null
Compress-Archive -LiteralPath $stage -DestinationPath $zip -CompressionLevel Optimal -Force
if (-not (Test-Path -LiteralPath $zip -PathType Leaf)) {
    throw "ZIP portátil não foi criado: $zip"
}

$compilerOutput = & $iscc "/DAppVersion=$version" "/DStageDir=$stage" "/DOutputDir=$dist" $iss 2>&1
if ($LASTEXITCODE -ne 0) {
    throw "ISCC.exe falhou com código $LASTEXITCODE.`n$($compilerOutput -join [Environment]::NewLine)"
}
if (-not (Test-Path -LiteralPath $setup -PathType Leaf)) {
    throw "Instalador não foi criado: $setup"
}

Write-ReleaseChecksums @($zip, $setup) $checksums

Write-Host "ZIP portátil: $zip"
Write-Host "Instalador offline: $setup"
Write-Host "Checksums: $checksums"
