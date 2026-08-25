param(
    [Parameter(Mandatory = $true)][string]$HLAEDir,
    [Parameter(Mandatory = $true)][string]$FFmpegPath,
    [Parameter(Mandatory = $true)][string]$FFprobePath,
    [Parameter(Mandatory = $true)][string]$InnoCompilerPath
)

$ErrorActionPreference = 'Stop'

$root = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$version = '1.0.0'
& (Join-Path $PSScriptRoot 'build-installer.ps1') `
    -Version $version `
    -HLAEDir $HLAEDir `
    -FFmpegPath $FFmpegPath `
    -FFprobePath $FFprobePath `
    -InnoCompilerPath $InnoCompilerPath

$dist = Join-Path $root 'dist'
$zip = Join-Path $dist 'CS2SJ-Demo-v1.0.0-windows-x64.zip'
$setup = Join-Path $dist 'CS2SJ-Demo-v1.0.0-Setup.exe'
$sums = Join-Path $dist 'SHA256SUMS.txt'
foreach ($artifact in @($zip, $setup, $sums)) {
    if (-not (Test-Path -LiteralPath $artifact -PathType Leaf) -or (Get-Item -LiteralPath $artifact).Length -eq 0) {
        throw "Artefato ausente ou vazio: $artifact"
    }
}

$checksumLines = @(Get-Content -LiteralPath $sums)
if ($checksumLines.Count -ne 2) {
    throw "SHA256SUMS.txt deve conter duas linhas; contém $($checksumLines.Count)."
}
foreach ($artifact in @($zip, $setup)) {
    $name = [System.IO.Path]::GetFileName($artifact)
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $artifact).Hash.ToLowerInvariant()
    if ($checksumLines -notcontains "$hash  $name") {
        throw "Checksum ausente ou incorreto para $name."
    }
}

$zipEntries = @(tar -tf $zip)
if ($LASTEXITCODE -ne 0 -or $zipEntries -notcontains 'CS2SJ-Demo/CS2SJ-Demo.exe' -or $zipEntries -notcontains 'CS2SJ-Demo/tools/hlae/hlae.exe') {
    throw 'O ZIP não preservou a raiz e o runtime portátil esperados.'
}

$productVersion = (Get-Item -LiteralPath $setup).VersionInfo.ProductVersion.Trim()
if ($productVersion -ne $version) {
    throw "Versão do Setup.exe incorreta: $productVersion"
}

Write-Host 'Installer artifacts: OK'
