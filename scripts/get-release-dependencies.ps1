param(
    [Parameter(Mandatory = $true)]
    [string]$DestinationRoot,

    [Parameter(Mandatory = $true)]
    [string]$PathsOutputPath
)

$ErrorActionPreference = 'Stop'

Import-Module (Join-Path $PSScriptRoot 'release\ReleaseTools.psm1') -Force

function Find-UniqueFile {
    param([string]$Root, [string]$Name)

    $matches = @(Get-ChildItem -LiteralPath $Root -Filter $Name -File -Recurse)
    if ($matches.Count -ne 1) {
        throw "Esperado exatamente um '$Name' em '$Root'; encontrados: $($matches.Count)."
    }
    return $matches[0].FullName
}

$root = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$destination = [System.IO.Path]::GetFullPath($DestinationRoot)
$pathsOutput = [System.IO.Path]::GetFullPath($PathsOutputPath)
if (Test-Path -LiteralPath $destination) {
    throw "A pasta de dependências já existe; informe uma pasta nova: $destination"
}

$downloads = Join-Path $destination 'downloads'
$hlaeExtracted = Join-Path $destination 'hlae-extracted'
$runtimeHLAE = Join-Path $destination 'runtime\hlae'
$ffmpegExtracted = Join-Path $destination 'ffmpeg-extracted'
$innoRoot = Join-Path $destination 'inno'
foreach ($directory in @($downloads, $hlaeExtracted, $runtimeHLAE, $ffmpegExtracted, $innoRoot)) {
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
}

$dependencies = @(Read-ReleaseDependencies (Join-Path $root 'installer\dependencies.json'))
$downloaded = @{}
foreach ($dependency in $dependencies) {
    $downloaded[[string]$dependency.name] = Get-VerifiedDownload $dependency $downloads
}

Expand-Archive -LiteralPath $downloaded['hlae'] -DestinationPath $hlaeExtracted
$hlaeExecutable = Find-UniqueFile $hlaeExtracted 'hlae.exe'
$hlaeSource = Split-Path -Parent $hlaeExecutable
Copy-Item -Path (Join-Path $hlaeSource '*') -Destination $runtimeHLAE -Recurse -Force
if (-not (Test-Path -LiteralPath (Join-Path $runtimeHLAE 'hlae.exe') -PathType Leaf)) {
    throw 'O HLAE extraído não pôde ser normalizado com hlae.exe na raiz.'
}
$hook = Get-ChildItem -LiteralPath $runtimeHLAE -Filter 'AfxHookSource2.dll' -File -Recurse | Select-Object -First 1
if ($null -eq $hook) {
    throw 'AfxHookSource2.dll não foi encontrado no HLAE validado.'
}

Expand-Archive -LiteralPath $downloaded['ffmpeg'] -DestinationPath $ffmpegExtracted
$ffmpegSource = Find-UniqueFile $ffmpegExtracted 'ffmpeg.exe'
$ffprobeSource = Find-UniqueFile $ffmpegExtracted 'ffprobe.exe'
$mediaDir = Join-Path $runtimeHLAE 'ffmpeg\bin'
New-Item -ItemType Directory -Path $mediaDir -Force | Out-Null
$ffmpegPath = Join-Path $mediaDir 'ffmpeg.exe'
$ffprobePath = Join-Path $mediaDir 'ffprobe.exe'
Copy-Item -LiteralPath $ffmpegSource -Destination $ffmpegPath -Force
Copy-Item -LiteralPath $ffprobeSource -Destination $ffprobePath -Force

$ffmpegLicenses = @(Get-ChildItem -LiteralPath $ffmpegExtracted -File -Recurse | Where-Object { $_.Name -match '^LICENSE(?:\..+)?$' } | Sort-Object { $_.FullName.Length })
if ($ffmpegLicenses.Count -eq 0) {
    throw 'A licença do FFmpeg não foi encontrada no arquivo validado.'
}
$licensesDir = Join-Path $runtimeHLAE 'LICENSES'
New-Item -ItemType Directory -Path $licensesDir -Force | Out-Null
Copy-Item -LiteralPath $ffmpegLicenses[0].FullName -Destination (Join-Path $licensesDir 'FFMPEG-LICENSE.txt') -Force

$innoArguments = @(
    '/PORTABLE=1',
    '/CURRENTUSER',
    '/VERYSILENT',
    '/SUPPRESSMSGBOXES',
    '/NORESTART',
    "/DIR=`"$innoRoot`""
)
$innoProcess = Start-Process -FilePath $downloaded['inno-setup'] -ArgumentList $innoArguments -Wait -PassThru -WindowStyle Hidden
if ($innoProcess.ExitCode -ne 0) {
    throw "Preparação portátil do Inno Setup falhou com código $($innoProcess.ExitCode)."
}
$innoCompilerPath = Find-UniqueFile $innoRoot 'ISCC.exe'

$pathsParent = Split-Path -Parent $pathsOutput
if (-not (Test-Path -LiteralPath $pathsParent -PathType Container)) {
    New-Item -ItemType Directory -Path $pathsParent -Force | Out-Null
}
$pathMap = [pscustomobject]@{
    hlaeDir         = [System.IO.Path]::GetFullPath($runtimeHLAE)
    ffmpegPath      = [System.IO.Path]::GetFullPath($ffmpegPath)
    ffprobePath     = [System.IO.Path]::GetFullPath($ffprobePath)
    innoCompilerPath = [System.IO.Path]::GetFullPath($innoCompilerPath)
}
$json = $pathMap | ConvertTo-Json
[System.IO.File]::WriteAllText($pathsOutput, $json, [System.Text.UTF8Encoding]::new($false))

Write-Host "Dependências verificadas e preparadas em $destination"
Write-Host "Mapa de caminhos criado em $pathsOutput"
