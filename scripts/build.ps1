$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $version = & go version
    if ($LASTEXITCODE -ne 0) {
        throw 'Não foi possível executar go version.'
    }
    if ($version -notmatch '^go version go1\.24(?:\.|\s)') {
        throw "Go 1.24.x é obrigatório; encontrado: $version"
    }

    New-Item -ItemType Directory -Force -Path 'bin' | Out-Null

    if (-not (Test-Path -LiteralPath 'assets\cs2sj-logo.ico' -PathType Leaf)) {
        throw 'Ícone do aplicativo não encontrado em assets\cs2sj-logo.ico.'
    }

    & go run 'github.com/akavel/rsrc@v0.10.2' -arch amd64 -ico 'assets\cs2sj-logo.ico' -manifest 'cmd\cs2sj-demo\cs2sj-demo.exe.manifest' -o 'cmd\cs2sj-demo\rsrc_windows_amd64.syso'
    if ($LASTEXITCODE -ne 0) {
        throw 'Não foi possível incorporar o ícone e o manifesto ao executável.'
    }

    & go test ./...
    if ($LASTEXITCODE -ne 0) {
        throw 'Os testes Go falharam.'
    }

    & go build -trimpath -ldflags '-s -w -H windowsgui' -o 'bin\CS2SJ-Demo.exe' '.\cmd\cs2sj-demo'
    if ($LASTEXITCODE -ne 0) {
        throw 'O build da interface gráfica falhou.'
    }

    & go build -trimpath -ldflags '-s -w' -o 'bin\cs2-highlights-cli.exe' '.\cmd\cs2-highlights'
    if ($LASTEXITCODE -ne 0) {
        throw 'O build da interface de linha de comando falhou.'
    }

    Copy-Item -LiteralPath 'cmd\cs2sj-demo\cs2sj-demo.exe.manifest' -Destination 'bin\CS2SJ-Demo.exe.manifest' -Force

    Write-Host "Interface criada em $root\bin\CS2SJ-Demo.exe"
    Write-Host "CLI criado em $root\bin\cs2-highlights-cli.exe"
}
finally {
    Pop-Location
}
