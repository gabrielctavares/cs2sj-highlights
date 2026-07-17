param([string]$FFmpegPath)

$ErrorActionPreference = 'Stop'
$root = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
if ([string]::IsNullOrWhiteSpace($FFmpegPath)) {
    $candidate = Get-ChildItem -LiteralPath (Join-Path $root 'dist\stage\CS2SJ-Demo\tools') -Filter ffmpeg.exe -File -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $candidate) { throw 'ffmpeg.exe não encontrado no pacote portátil; informe -FFmpegPath.' }
    $FFmpegPath = $candidate.FullName
}
$font = 'C:\Windows\Fonts\segoeuib.ttf'
$logo = Join-Path $root 'assets\cs2sj-logo.jpg'
$work = Join-Path $root 'work\hud-preview'
New-Item -ItemType Directory -Force -Path $work | Out-Null
$filterPath = Join-Path $work 'preview.ffscript'
$previewPath = Join-Path $work 'preview.png'
$fontFilter = $font.Replace('\', '/').Replace(':', '\:')

$filter = "[0:v]drawbox=x=430:y=12:w=1060:h=30:color=0x0d1219@0.94:t=fill," +
    "drawbox=x=350:y=42:w=1220:h=78:color=0x111923@0.94:t=fill," +
    "drawbox=x=350:y=42:w=440:h=78:color=0x0b4f71@0.96:t=fill," +
    "drawbox=x=1130:y=42:w=440:h=78:color=0xc66a14@0.96:t=fill," +
    "drawbox=x=790:y=42:w=340:h=78:color=0x090d12@0.98:t=fill," +
    "drawbox=x=710:y=970:w=500:h=74:color=0x0d1219@0.92:t=fill," +
    "drawtext=fontfile='$fontFilter':text='1º CAMP MONTADO CS2 SJ':fontcolor=white:fontsize=18:x=(w-text_w)/2:y=17," +
    "drawtext=fontfile='$fontFilter':text='ONU':fontcolor=white:fontsize=31:x=380:y=66," +
    "drawtext=fontfile='$fontFilter':text='G3NERATION Z':fontcolor=white:fontsize=31:x=1540-text_w:y=66," +
    "drawtext=fontfile='$fontFilter':text='8':fontcolor=white:fontsize=36:x=750-text_w:y=60," +
    "drawtext=fontfile='$fontFilter':text='4':fontcolor=white:fontsize=36:x=1145:y=60," +
    "drawtext=fontfile='$fontFilter':text='ANCIENT':fontcolor=white:fontsize=22:x=960-text_w/2:y=54," +
    "drawtext=fontfile='$fontFilter':text='ROUND 13':fontcolor=white:fontsize=16:x=960-text_w/2:y=88," +
    "drawtext=fontfile='$fontFilter':text='DUZINVL':fontcolor=white:fontsize=26:x=(w-text_w)/2:y=976[base];" +
    "[1:v]scale=60:60[logo];[base][logo]overlay=x=805:y=51:format=auto[v]"
$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($filterPath, $filter, $utf8)

& $FFmpegPath -y -f lavfi -i 'color=c=0x48515c:s=1920x1080:d=1' -i $logo '-/filter_complex' $filterPath -map '[v]' -frames:v 1 $previewPath
if ($LASTEXITCODE -ne 0) { throw "Falha ao gerar preview da HUD (código $LASTEXITCODE)." }
Write-Host "Preview criado em $previewPath"
