param(
    [Parameter(Mandatory = $true)][string]$Demo,
    [Parameter(Mandatory = $true)][string]$Output,
    [Parameter(Mandatory = $true)][string]$HLAE,
    [Parameter(Mandatory = $true)][string]$CS2,
    [Parameter(Mandatory = $true)][string]$HookDLL,
    [Parameter(Mandatory = $true)][string]$FFmpeg,
    [Parameter(Mandatory = $true)][string]$FFprobe
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

function Resolve-RequiredFile([string]$Label, [string]$Path) {
    $resolved = Resolve-Path -LiteralPath $Path -ErrorAction Stop
    if ((Get-Item -LiteralPath $resolved.Path).PSIsContainer) {
        throw "$Label deve apontar para um arquivo: $Path"
    }
    return $resolved.Path
}

$demoPath = Resolve-RequiredFile 'Demo' $Demo
$hlaePath = Resolve-RequiredFile 'HLAE' $HLAE
$cs2Path = Resolve-RequiredFile 'CS2' $CS2
$hookPath = Resolve-RequiredFile 'HookDLL' $HookDLL
$ffmpegPath = Resolve-RequiredFile 'FFmpeg' $FFmpeg
$ffprobePath = Resolve-RequiredFile 'FFprobe' $FFprobe

New-Item -ItemType Directory -Force -Path $Output | Out-Null
$outputPath = (Resolve-Path -LiteralPath $Output).Path
$binary = Resolve-RequiredFile 'cs2-highlights-cli.exe' (Join-Path $root 'bin\cs2-highlights-cli.exe')

$jobRoot = Join-Path $root (Join-Path 'work' ("smoke-" + [guid]::NewGuid().ToString('N')))
$inputPath = Join-Path $jobRoot 'demos'
New-Item -ItemType Directory -Force -Path $inputPath | Out-Null
Copy-Item -LiteralPath $demoPath -Destination (Join-Path $inputPath (Split-Path -Leaf $demoPath))

$env:CS2_TEST_DEMO = $demoPath
Push-Location $root
try {
    & go test ./internal/demos -run TestDemoParserRealDemo -v
    if ($LASTEXITCODE -ne 0) {
        throw 'O parser não aceitou a demo real.'
    }

    & $binary render $inputPath --output $outputPath --hlae $hlaePath --cs2 $cs2Path --hook-dll $hookPath --ffmpeg $ffmpegPath --ffprobe $ffprobePath
    if ($LASTEXITCODE -ne 0) {
        throw "A renderização terminou com código $LASTEXITCODE. Os logs foram preservados em $outputPath."
    }

    $horizontal = @(Get-ChildItem -LiteralPath $outputPath -Recurse -File -Filter '*-16x9.mp4' | Where-Object { $_.Name -ne 'resumo-16x9.mp4' })
    $vertical = @(Get-ChildItem -LiteralPath $outputPath -Recurse -File -Filter '*-9x16.mp4' | Where-Object { $_.Name -ne 'resumo-9x16.mp4' })
    $summaryHorizontal = @(Get-ChildItem -LiteralPath $outputPath -Recurse -File -Filter 'resumo-16x9.mp4')
    $summaryVertical = @(Get-ChildItem -LiteralPath $outputPath -Recurse -File -Filter 'resumo-9x16.mp4')
    $manifests = @(Get-ChildItem -LiteralPath $outputPath -Recurse -File -Filter 'manifest.json')

    if ($horizontal.Count -lt 1 -or $vertical.Count -ne 0 -or $summaryHorizontal.Count -ne 0 -or $summaryVertical.Count -ne 0 -or $manifests.Count -lt 1) {
        throw "Smoke incompleto. Verifique manifest.json e render.log em $outputPath."
    }

    foreach ($manifestFile in $manifests) {
        $manifest = Get-Content -LiteralPath $manifestFile.FullName -Raw | ConvertFrom-Json
        if (-not $manifest.candidate_version -or -not $manifest.scoring_version -or -not $manifest.diversity_version) {
            throw "Manifesto sem versões do catálogo: $($manifestFile.FullName)"
        }
        if ($null -eq $manifest.selected_highlight_ids -or -not ($manifest.selected_highlight_ids -is [array])) {
            throw "Manifesto sem selected_highlight_ids: $($manifestFile.FullName)"
        }
        foreach ($selectedID in $manifest.selected_highlight_ids) {
            $highlight = @($manifest.highlights | Where-Object { $_.id -eq $selectedID })
            if ($highlight.Count -ne 1 -or $null -eq $highlight[0].individual -or $null -eq $highlight[0].editorial) {
                throw "Highlight selecionado sem avaliações duplas: $selectedID"
            }
        }
    }

    Write-Host "Smoke concluído. Saída: $outputPath"
}
finally {
    Pop-Location
}
