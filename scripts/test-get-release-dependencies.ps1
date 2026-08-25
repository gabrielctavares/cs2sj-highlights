$ErrorActionPreference = 'Stop'

$destination = Join-Path ([System.IO.Path]::GetTempPath()) ("cs2sj dependencies " + [guid]::NewGuid())
$pathsOutput = Join-Path ([System.IO.Path]::GetTempPath()) ("cs2sj-dependency-paths-" + [guid]::NewGuid() + '.json')

try {
    & (Join-Path $PSScriptRoot 'get-release-dependencies.ps1') `
        -DestinationRoot $destination `
        -PathsOutputPath $pathsOutput
    if (-not (Test-Path -LiteralPath $pathsOutput -PathType Leaf)) {
        throw "Mapa de caminhos não criado: $pathsOutput"
    }

    $paths = Get-Content -Raw -LiteralPath $pathsOutput | ConvertFrom-Json
    foreach ($property in @('hlaeDir', 'ffmpegPath', 'ffprobePath', 'innoCompilerPath')) {
        if ($null -eq $paths.PSObject.Properties[$property]) {
            throw "Mapa de caminhos sem '$property'."
        }
    }
    if (-not (Test-Path -LiteralPath (Join-Path $paths.hlaeDir 'hlae.exe') -PathType Leaf)) {
        throw 'Runtime normalizado sem hlae.exe na raiz.'
    }
    foreach ($file in @($paths.ffmpegPath, $paths.ffprobePath, $paths.innoCompilerPath)) {
        if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
            throw "Dependência preparada ausente: $file"
        }
    }
    $license = Join-Path $paths.hlaeDir 'LICENSES\FFMPEG-LICENSE.txt'
    if (-not (Test-Path -LiteralPath $license -PathType Leaf)) {
        throw "Licença do FFmpeg ausente: $license"
    }
}
finally {
    $systemTemp = [System.IO.Path]::GetTempPath()
    $resolvedDestination = [System.IO.Path]::GetFullPath($destination)
    if ($resolvedDestination.StartsWith($systemTemp, [System.StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedDestination)) {
        Remove-Item -LiteralPath $resolvedDestination -Recurse -Force
    }
    $resolvedPathsOutput = [System.IO.Path]::GetFullPath($pathsOutput)
    if ($resolvedPathsOutput.StartsWith($systemTemp, [System.StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedPathsOutput -PathType Leaf)) {
        Remove-Item -LiteralPath $resolvedPathsOutput -Force
    }
}

Write-Host 'Release dependencies: OK'
