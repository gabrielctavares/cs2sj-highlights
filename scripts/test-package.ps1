$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$fixture = Join-Path ([System.IO.Path]::GetTempPath()) ("cs2sj-package-" + [guid]::NewGuid())
$hlae = Join-Path $fixture 'hlae'
$ffmpeg = Join-Path $fixture 'media\ffmpeg.exe'
$ffprobe = Join-Path $fixture 'media\ffprobe.exe'

function Write-FixtureFile {
    param([string]$Path, [string]$Content)
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
    [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
}

try {
    Write-FixtureFile (Join-Path $hlae 'hlae.exe') 'hlae-fixture'
    Write-FixtureFile (Join-Path $hlae 'x64\AfxHookSource2.dll') 'hook-fixture'
    Write-FixtureFile (Join-Path $hlae 'LICENSES\HLAE.txt') 'license-fixture'
    Write-FixtureFile $ffmpeg 'ffmpeg-external'
    Write-FixtureFile $ffprobe 'ffprobe-external'

    & (Join-Path $PSScriptRoot 'package.ps1') `
        -HLAEDir $hlae `
        -FFmpegPath $ffmpeg `
        -FFprobePath $ffprobe
    if ($LASTEXITCODE -ne 0) {
        throw "package.ps1 falhou com código $LASTEXITCODE."
    }

    $stage = Join-Path $root 'dist\stage\CS2SJ-Demo'
    $stagedFFmpeg = Join-Path $stage 'tools\hlae\ffmpeg\bin\ffmpeg.exe'
    $stagedFFprobe = Join-Path $stage 'tools\hlae\ffmpeg\bin\ffprobe.exe'
    foreach ($pair in @(@($ffmpeg, $stagedFFmpeg), @($ffprobe, $stagedFFprobe))) {
        if (-not (Test-Path -LiteralPath $pair[1] -PathType Leaf)) {
            throw "Arquivo de mídia ausente no stage: $($pair[1])"
        }
        $sourceHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $pair[0]).Hash
        $stageHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $pair[1]).Hash
        if ($sourceHash -ne $stageHash) {
            throw "O stage não preservou o conteúdo de $($pair[0])."
        }
    }
}
finally {
    $resolvedFixture = [System.IO.Path]::GetFullPath($fixture)
    if ($resolvedFixture.StartsWith([System.IO.Path]::GetTempPath(), [System.StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedFixture)) {
        Remove-Item -LiteralPath $resolvedFixture -Recurse -Force
    }
}

Write-Host 'Package external media: OK'
