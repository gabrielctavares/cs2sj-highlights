$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $unformatted = & gofmt -l .\cmd .\internal
    if ($LASTEXITCODE -ne 0 -or $unformatted) {
        throw "Arquivos Go sem formatação:`n$unformatted"
    }

    & (Join-Path $PSScriptRoot 'test-release-tools.ps1')
    if ($LASTEXITCODE -ne 0) { throw 'Testes das ferramentas de release falharam.' }

    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'go mod verify falhou.' }

    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet falhou.' }

    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'go test falhou.' }

    & go test -count=1 -tags windows ./internal/gui
    if ($LASTEXITCODE -ne 0) { throw 'Testes Windows da GUI falharam.' }

    New-Item -ItemType Directory -Force -Path 'bin' | Out-Null
    & go build -trimpath -o 'bin\verify-cs2-highlights-cli.exe' .\cmd\cs2-highlights
    if ($LASTEXITCODE -ne 0) { throw 'Build da CLI falhou.' }

    & go build -trimpath -o 'bin\verify-CS2SJ-Demo.exe' .\cmd\cs2sj-demo
    if ($LASTEXITCODE -ne 0) { throw 'Build da GUI falhou.' }
}
finally {
    Pop-Location
}
