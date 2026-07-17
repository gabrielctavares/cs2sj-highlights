$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $unformatted = & gofmt -l .\cmd .\internal
    if ($LASTEXITCODE -ne 0 -or $unformatted) {
        throw "Arquivos Go sem formatação:`n$unformatted"
    }

    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'go mod verify falhou.' }

    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet falhou.' }

    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'go test falhou.' }

    New-Item -ItemType Directory -Force -Path 'bin' | Out-Null
    & go build -trimpath -o 'bin\verify-cs2-highlights-cli.exe' .\cmd\cs2-highlights
    if ($LASTEXITCODE -ne 0) { throw 'Build da CLI falhou.' }
}
finally {
    Pop-Location
}
