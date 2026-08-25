Set-StrictMode -Version Latest

function Assert-StableVersion {
    param([AllowEmptyString()][string]$Version)

    if ([string]::IsNullOrWhiteSpace($Version) -or $Version -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
        throw "Versão inválida '$Version'. Use MAJOR.MINOR.PATCH sem prefixo ou sufixo."
    }
    return $Version
}

function Get-ReleaseArtifactNames {
    param([Parameter(Mandatory = $true)][string]$Version)

    $valid = Assert-StableVersion $Version
    return [pscustomobject]@{
        Tag       = "v$valid"
        Zip       = "CS2SJ-Demo-v$valid-windows-x64.zip"
        Setup     = "CS2SJ-Demo-v$valid-Setup.exe"
        Checksums = 'SHA256SUMS.txt'
    }
}

function Read-ReleaseDependencies {
    param([Parameter(Mandatory = $true)][string]$Path)

    $resolved = (Resolve-Path -LiteralPath $Path).Path
    try {
        $manifest = Get-Content -Raw -LiteralPath $resolved | ConvertFrom-Json
    }
    catch {
        throw "Manifesto de dependências inválido em '$resolved': $($_.Exception.Message)"
    }
    if ($null -eq $manifest.PSObject.Properties['schemaVersion'] -or $manifest.schemaVersion -ne 1) {
        throw 'O manifesto de dependências deve usar schemaVersion 1.'
    }
    if ($null -eq $manifest.PSObject.Properties['dependencies']) {
        throw 'O manifesto não contém dependencies.'
    }

    $dependencies = @($manifest.dependencies)
    foreach ($dependency in $dependencies) {
        foreach ($field in @('name', 'version', 'url', 'sha256')) {
            if ($null -eq $dependency.PSObject.Properties[$field] -or [string]::IsNullOrWhiteSpace([string]$dependency.$field)) {
                throw "Dependência sem campo obrigatório '$field'."
            }
        }
        $uri = $null
        if (-not [System.Uri]::TryCreate([string]$dependency.url, [System.UriKind]::Absolute, [ref]$uri) -or $uri.Scheme -ne 'https') {
            throw "A dependência '$($dependency.name)' deve usar uma URL HTTPS."
        }
        if ([string]$dependency.sha256 -notmatch '^[0-9a-fA-F]{64}$') {
            throw "A dependência '$($dependency.name)' deve informar um SHA-256 com 64 dígitos hexadecimais."
        }
        $dependency.sha256 = ([string]$dependency.sha256).ToLowerInvariant()
    }

    $expectedNames = @('ffmpeg', 'hlae', 'inno-setup')
    $actualNames = @($dependencies | ForEach-Object { [string]$_.name } | Sort-Object -Unique)
    if ($dependencies.Count -ne $expectedNames.Count -or @(Compare-Object -ReferenceObject $expectedNames -DifferenceObject $actualNames).Count -ne 0) {
        throw "O manifesto deve conter exatamente: $($expectedNames -join ', ')."
    }
    return $dependencies
}

function Get-VerifiedDownload {
    param(
        [Parameter(Mandatory = $true)]$Dependency,
        [Parameter(Mandatory = $true)][string]$DestinationDirectory,
        [scriptblock]$DownloadFile
    )

    $directory = [System.IO.Path]::GetFullPath($DestinationDirectory)
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
    $uri = [System.Uri]([string]$Dependency.url)
    $fileName = [System.IO.Path]::GetFileName($uri.AbsolutePath)
    if ([string]::IsNullOrWhiteSpace($fileName)) {
        throw "A URL de '$($Dependency.name)' não contém nome de arquivo."
    }
    $destination = Join-Path $directory $fileName
    if ($null -eq $DownloadFile) {
        $DownloadFile = { param($Uri, $OutFile) Invoke-WebRequest -Uri $Uri -OutFile $OutFile }
    }
    & $DownloadFile ([string]$Dependency.url) $destination
    if (-not (Test-Path -LiteralPath $destination -PathType Leaf)) {
        throw "O download de '$($Dependency.name)' não criou '$destination'."
    }

    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $destination).Hash.ToLowerInvariant()
    $expected = ([string]$Dependency.sha256).ToLowerInvariant()
    if ($actual -ne $expected) {
        Remove-Item -LiteralPath $destination -Force
        throw "SHA-256 inválido para '$($Dependency.name)': esperado $expected, obtido $actual."
    }
    return [System.IO.Path]::GetFullPath($destination)
}

function Assert-ReleaseStage {
    param([Parameter(Mandatory = $true)][string]$Path)

    $stage = (Resolve-Path -LiteralPath $Path).Path
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
    foreach ($relative in $required) {
        $candidate = Join-Path $stage $relative
        if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) {
            throw "Arquivo obrigatório ausente no pacote: $relative"
        }
    }

    $toolsRoot = Join-Path $stage 'tools\hlae'
    foreach ($name in @('AfxHookSource2.dll', 'ffmpeg.exe', 'ffprobe.exe')) {
        $match = Get-ChildItem -LiteralPath $toolsRoot -Filter $name -File -Recurse | Select-Object -First 1
        if ($null -eq $match) {
            throw "Arquivo obrigatório ausente no pacote: $name"
        }
    }
}

function Write-ReleaseChecksums {
    param(
        [Parameter(Mandatory = $true)][string[]]$ArtifactPaths,
        [Parameter(Mandatory = $true)][string]$OutputPath
    )

    $resolvedArtifacts = @($ArtifactPaths | ForEach-Object { (Resolve-Path -LiteralPath $_).Path })
    $ordered = @($resolvedArtifacts | Sort-Object { [System.IO.Path]::GetFileName($_).ToLowerInvariant() })
    $lines = @($ordered | ForEach-Object {
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $_).Hash.ToLowerInvariant()
        "$hash  $([System.IO.Path]::GetFileName($_))"
    })
    $absoluteOutput = [System.IO.Path]::GetFullPath($OutputPath)
    $parent = Split-Path -Parent $absoluteOutput
    if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
    [System.IO.File]::WriteAllLines($absoluteOutput, $lines, [System.Text.UTF8Encoding]::new($false))
}

function Resolve-InnoCompiler {
    param([string]$ExplicitPath)

    if (-not [string]::IsNullOrWhiteSpace($ExplicitPath)) {
        $resolved = (Resolve-Path -LiteralPath $ExplicitPath).Path
        if (-not (Test-Path -LiteralPath $resolved -PathType Leaf) -or -not [string]::Equals([System.IO.Path]::GetFileName($resolved), 'ISCC.exe', [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Compilador Inno inválido; informe o caminho de ISCC.exe: $resolved"
        }
        return $resolved
    }

    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        $candidates += (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 7\ISCC.exe')
        $candidates += (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 6\ISCC.exe')
    }
    if (-not [string]::IsNullOrWhiteSpace($env:ProgramFiles)) {
        $candidates += (Join-Path $env:ProgramFiles 'Inno Setup 7\ISCC.exe')
    }
    $programFilesX86 = [System.Environment]::GetEnvironmentVariable('ProgramFiles(x86)')
    if (-not [string]::IsNullOrWhiteSpace($programFilesX86)) {
        $candidates += (Join-Path $programFilesX86 'Inno Setup 7\ISCC.exe')
        $candidates += (Join-Path $programFilesX86 'Inno Setup 6\ISCC.exe')
    }
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return [System.IO.Path]::GetFullPath($candidate)
        }
    }
    throw 'ISCC.exe não encontrado. Instale o Inno Setup ou informe -InnoCompilerPath.'
}

Export-ModuleMember -Function @(
    'Assert-StableVersion',
    'Get-ReleaseArtifactNames',
    'Read-ReleaseDependencies',
    'Get-VerifiedDownload',
    'Assert-ReleaseStage',
    'Write-ReleaseChecksums',
    'Resolve-InnoCompiler'
)
