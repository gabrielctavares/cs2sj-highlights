$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Import-Module (Join-Path $PSScriptRoot 'release\ReleaseTools.psm1') -Force

function Assert-Equal {
    param($Expected, $Actual, [string]$Message)
    if ($Expected -ne $Actual) {
        throw "$Message; esperado=<$Expected> obtido=<$Actual>"
    }
}

function Assert-Throws {
    param([scriptblock]$Action, [string]$Pattern)
    try {
        & $Action
    }
    catch {
        if ($_.Exception.Message -notmatch $Pattern) {
            throw "Falha inesperada: $($_.Exception.Message); esperado padrão <$Pattern>"
        }
        return
    }
    throw "Era esperada uma falha contendo <$Pattern>."
}

function Write-TestFile {
    param([string]$Path, [byte[]]$Bytes = [byte[]](0x01))
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
    [System.IO.File]::WriteAllBytes($Path, $Bytes)
}

Assert-Equal '1.2.3' (Assert-StableVersion '1.2.3') 'versão válida'
foreach ($candidate in @('', 'v1.2.3', '1.2', '1.2.3-beta')) {
    Assert-Throws { Assert-StableVersion $candidate } 'MAJOR\.MINOR\.PATCH'
}

$names = Get-ReleaseArtifactNames '1.2.3'
Assert-Equal 'v1.2.3' $names.Tag 'tag'
Assert-Equal 'CS2SJ-Demo-v1.2.3-windows-x64.zip' $names.Zip 'nome do ZIP'
Assert-Equal 'CS2SJ-Demo-v1.2.3-Setup.exe' $names.Setup 'nome do instalador'
Assert-Equal 'SHA256SUMS.txt' $names.Checksums 'nome dos checksums'

$temp = Join-Path ([System.IO.Path]::GetTempPath()) ("cs2sj-release-tools-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $dependencies = Read-ReleaseDependencies (Join-Path $root 'installer\dependencies.json')
    Assert-Equal 3 $dependencies.Count 'quantidade de dependências'

    $badManifest = Join-Path $temp 'bad.json'
    $badManifestContent = @'
{
  "schemaVersion": 1,
  "dependencies": [
    {"name":"hlae","version":"1","url":"http://example.test/hlae.zip","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
    {"name":"ffmpeg","version":"1","url":"https://example.test/ffmpeg.zip","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
    {"name":"inno-setup","version":"1","url":"https://example.test/inno.exe","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}
  ]
}
'@
    [System.IO.File]::WriteAllText($badManifest, $badManifestContent, [System.Text.UTF8Encoding]::new($false))
    Assert-Throws { Read-ReleaseDependencies $badManifest } 'HTTPS'

    $source = Join-Path $temp 'source.bin'
    Write-TestFile $source ([byte[]](0x61))
    $realHash = 'ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb'
    $dependency = [pscustomobject]@{
        name = 'fixture'
        version = '1'
        url = 'https://example.test/source.bin'
        sha256 = $realHash
    }
    $downloadSource = $source
    $copyDownload = { param($Uri, $OutFile) Copy-Item -LiteralPath $downloadSource -Destination $OutFile }.GetNewClosure()
    $downloadDir = Join-Path $temp 'downloads-ok'
    $downloaded = Get-VerifiedDownload $dependency $downloadDir $copyDownload
    Assert-Equal $realHash ((Get-FileHash -Algorithm SHA256 -LiteralPath $downloaded).Hash.ToLowerInvariant()) 'download verificado'

    $dependency.sha256 = ('0' * 64)
    Assert-Throws { Get-VerifiedDownload $dependency (Join-Path $temp 'downloads-bad') $copyDownload } 'SHA-256'

    $compiler = Join-Path $temp 'compiler\ISCC.exe'
    Write-TestFile $compiler
    Assert-Equal ([System.IO.Path]::GetFullPath($compiler)) (Resolve-InnoCompiler $compiler) 'compilador explícito'
    $wrongCompiler = Join-Path $temp 'compiler\compiler.exe'
    Write-TestFile $wrongCompiler
    Assert-Throws { Resolve-InnoCompiler $wrongCompiler } 'ISCC.exe'

    $stage = Join-Path $temp 'stage'
    New-Item -ItemType Directory -Path $stage | Out-Null
    Assert-Throws { Assert-ReleaseStage $stage } 'CS2SJ-Demo.exe'
    foreach ($relative in @(
        'CS2SJ-Demo.exe',
        'CS2SJ-Demo.exe.manifest',
        'cs2-highlights-cli.exe',
        'tools\hlae\hlae.exe',
        'tools\hlae\x64\AfxHookSource2.dll',
        'tools\hlae\ffmpeg\bin\ffmpeg.exe',
        'tools\hlae\ffmpeg\bin\ffprobe.exe',
        'assets\cs2sj-logo.jpg',
        'assets\cs2sj-logo.ico',
        'licenses\NOTICE.md',
        'licenses\WALK-LICENSE.txt'
    )) {
        Write-TestFile (Join-Path $stage $relative)
    }
    Assert-ReleaseStage $stage

    $a = Join-Path $temp 'a.zip'
    $b = Join-Path $temp 'b.exe'
    Write-TestFile $a ([byte[]](0x61))
    Write-TestFile $b ([byte[]](0x62))
    $sums = Join-Path $temp 'SHA256SUMS.txt'
    Write-ReleaseChecksums @($b, $a) $sums
    $lines = @(Get-Content -LiteralPath $sums)
    Assert-Equal 2 $lines.Count 'quantidade de hashes'
    Assert-Equal 'ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb  a.zip' $lines[0] 'hash do ZIP'
    Assert-Equal '3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d  b.exe' $lines[1] 'hash do instalador'
}
finally {
    $resolvedTemp = [System.IO.Path]::GetFullPath($temp)
    if ($resolvedTemp.StartsWith([System.IO.Path]::GetTempPath(), [System.StringComparison]::OrdinalIgnoreCase)) {
        Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
    }
}

Write-Host 'ReleaseTools: OK'
