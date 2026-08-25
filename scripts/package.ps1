param(
    [Parameter(Mandatory = $true)]
    [string]$HLAEDir,

    [string]$FFmpegPath,

    [string]$FFprobePath,

    [switch]$CreateZip
)

$ErrorActionPreference = 'Stop'

Import-Module (Join-Path $PSScriptRoot 'release\ReleaseTools.psm1') -Force

$root = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$hlae = (Resolve-Path -LiteralPath $HLAEDir).Path
$dist = [System.IO.Path]::GetFullPath((Join-Path $root 'dist'))
$stageParent = [System.IO.Path]::GetFullPath((Join-Path $dist 'stage'))
$stage = [System.IO.Path]::GetFullPath((Join-Path $stageParent 'CS2SJ-Demo'))
$zip = [System.IO.Path]::GetFullPath((Join-Path $dist 'CS2SJ-Demo-windows-x64.zip'))

if (-not $stage.StartsWith($stageParent + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Pasta temporária fora do destino permitido: $stage"
}

$hlaeExe = Join-Path $hlae 'hlae.exe'
if (-not (Test-Path -LiteralPath $hlaeExe -PathType Leaf)) {
    throw "hlae.exe não encontrado em $hlae"
}
$hook = Get-ChildItem -LiteralPath $hlae -Filter 'AfxHookSource2.dll' -File -Recurse | Select-Object -First 1
if ($null -eq $hook) {
    throw "AfxHookSource2.dll não encontrado em $hlae"
}
$ffmpeg = Get-ChildItem -LiteralPath $hlae -Filter 'ffmpeg.exe' -File -Recurse | Select-Object -First 1
$ffprobe = Get-ChildItem -LiteralPath $hlae -Filter 'ffprobe.exe' -File -Recurse | Select-Object -First 1
if ($null -eq $ffmpeg) {
    if ([string]::IsNullOrWhiteSpace($FFmpegPath)) {
        throw 'ffmpeg.exe não acompanha esta distribuição do HLAE; informe -FFmpegPath com o binário oficial correspondente.'
    }
    $resolvedFFmpeg = (Resolve-Path -LiteralPath $FFmpegPath).Path
    if (-not (Test-Path -LiteralPath $resolvedFFmpeg -PathType Leaf) -or -not [string]::Equals([System.IO.Path]::GetFileName($resolvedFFmpeg), 'ffmpeg.exe', [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "FFmpeg inválido: $resolvedFFmpeg"
    }
    $ffmpeg = Get-Item -LiteralPath $resolvedFFmpeg
}
if ($null -eq $ffprobe) {
    if ([string]::IsNullOrWhiteSpace($FFprobePath)) {
        throw 'ffprobe.exe não acompanha esta distribuição do HLAE; informe -FFprobePath com o binário oficial correspondente.'
    }
    $resolvedFFprobe = (Resolve-Path -LiteralPath $FFprobePath).Path
    if (-not (Test-Path -LiteralPath $resolvedFFprobe -PathType Leaf) -or -not [string]::Equals([System.IO.Path]::GetFileName($resolvedFFprobe), 'ffprobe.exe', [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "FFprobe inválido: $resolvedFFprobe"
    }
    $ffprobe = Get-Item -LiteralPath $resolvedFFprobe
}

& (Join-Path $PSScriptRoot 'build.ps1')
if ($LASTEXITCODE -ne 0) {
    throw 'O build falhou antes do empacotamento.'
}

New-Item -ItemType Directory -Force -Path $stageParent | Out-Null
if (Test-Path -LiteralPath $stage) {
    Remove-Item -LiteralPath $stage -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $stage | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $stage 'tools\hlae') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $stage 'licenses') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $stage 'assets') | Out-Null

Copy-Item -LiteralPath (Join-Path $root 'bin\CS2SJ-Demo.exe') -Destination $stage -Force
Copy-Item -LiteralPath (Join-Path $root 'bin\CS2SJ-Demo.exe.manifest') -Destination $stage -Force
Copy-Item -LiteralPath (Join-Path $root 'bin\cs2-highlights-cli.exe') -Destination $stage -Force
Copy-Item -LiteralPath (Join-Path $root 'assets\cs2sj-logo.jpg') -Destination (Join-Path $stage 'assets\cs2sj-logo.jpg') -Force
Copy-Item -LiteralPath (Join-Path $root 'assets\cs2sj-logo.ico') -Destination (Join-Path $stage 'assets\cs2sj-logo.ico') -Force
Copy-Item -Path (Join-Path $hlae '*') -Destination (Join-Path $stage 'tools\hlae') -Recurse -Force
$mediaDir = Join-Path $stage 'tools\hlae\ffmpeg\bin'
New-Item -ItemType Directory -Force -Path $mediaDir | Out-Null
Copy-Item -LiteralPath $ffmpeg.FullName -Destination (Join-Path $mediaDir 'ffmpeg.exe') -Force
Copy-Item -LiteralPath $ffprobe.FullName -Destination (Join-Path $mediaDir 'ffprobe.exe') -Force
Copy-Item -LiteralPath (Join-Path $root 'third_party\walk\LICENSE') -Destination (Join-Path $stage 'licenses\WALK-LICENSE.txt') -Force
Copy-Item -LiteralPath (Join-Path $root 'third_party\NOTICE.md') -Destination (Join-Path $stage 'licenses\NOTICE.md') -Force
if (Test-Path -LiteralPath (Join-Path $hlae 'LICENSES')) {
    Copy-Item -LiteralPath (Join-Path $hlae 'LICENSES') -Destination (Join-Path $stage 'licenses\HLAE-LICENSES') -Recurse -Force
}

Assert-ReleaseStage $stage

Write-Host "Pasta portátil preparada em $stage"
if ($CreateZip) {
    New-Item -ItemType Directory -Force -Path $dist | Out-Null
    Compress-Archive -LiteralPath $stage -DestinationPath $zip -CompressionLevel Optimal -Force
    Write-Host "ZIP criado em $zip"
}
