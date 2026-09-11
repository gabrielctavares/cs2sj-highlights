# CS2SJ Highlights

Aplicativo local para Windows que lê demos de Counter-Strike 2, seleciona highlights automaticamente e entrega clipes MP4 em 16:9.

## Uso pelo aplicativo

1. Baixe `CS2SJ-Demo-vX.Y.Z-Setup.exe` na página de [Releases](https://github.com/gabrielctavares/cs2sj-highlights/releases).
2. Execute o instalador; ele funciona offline e não solicita senha de administrador.
3. Abra **CS2SJ Demo** pelo menu Iniciar.
4. Selecione:
   - o arquivo `cs2.exe`;
   - a pasta contendo as demos `.dem`;
   - a pasta onde os vídeos serão salvos.
   - o nome do campeonato, preenchido inicialmente com o nome da pasta das demos e editável antes da análise ou renderização.
5. Clique em **Analisar demos**.
6. Em **Melhores da partida**, confira os lances de maior valor para a narrativa geral; em **Por jogador**, escolha um SteamID e veja suas melhores jogadas individuais. A coluna **Por que apareceu** e a legenda da tela explicam os fatores e a nota editorial de cada candidato.
7. Escolha a abrangência **Restrita**, **Equilibrada** ou **Ampla**, adicione os lances desejados e confira a aba **Seleção final**.
8. Clique em **Processar seleção final**.

Como alternativa portátil, baixe `CS2SJ-Demo-vX.Y.Z-windows-x64.zip`, extraia todo o conteúdo para uma pasta e abra `CS2SJ-Demo.exe`. Não execute o programa de dentro do ZIP, pois HLAE, FFmpeg e os demais arquivos precisam permanecer juntos.

O instalador desta primeira versão ainda não possui assinatura Authenticode. Por isso, o Windows SmartScreen pode exibir um aviso de reputação antes da execução.

Somente itens explicitamente adicionados à seleção final são renderizados. Um mesmo candidato pode aparecer nas duas perspectivas com notas diferentes, mas entra uma única vez na seleção e gera uma única captura. O jogador escolhido fica salvo por SteamID e é pré-selecionado quando estiver presente em uma análise futura.

Não existe limite fixo de clipes. A análise considera multi-kills, clutch, assists, flash assists, headshots, wallbangs, no-scopes, kills pela smoke, longa distância, entry, trade, sequência rápida, pouca vida, match point, overtime e fim da partida. **Restrita**, **Equilibrada** e **Ampla** aplicam limiares decrescentes de 85, 60 e 35; uma demo pode produzir zero ou muitos resultados.

Os caminhos ficam salvos em `%LocalAppData%\CS2SJ-Demo\config.json`. A análise e a captura rodam em segundo plano, a janela mostra o andamento e permite abrir a pasta dos vídeos ao terminar.

O HLAE 2.191.0, AfxHookSource2, FFmpeg e FFprobe acompanham a pasta portátil. Se o HLAE não for encontrado, copie ou extraia novamente a pasta completa. O botão **Baixar HLAE** abre somente a página oficial: https://github.com/advancedfx/advancedfx/releases/latest

## Atualizar e desinstalar

Para atualizar, baixe e execute o instalador da versão nova. Ele reconhece a instalação existente do usuário e a substitui sem criar uma segunda entrada em **Aplicativos instalados**.

A desinstalação pode ser feita pelo menu Iniciar ou pelas configurações do Windows. Ela remove o programa, mas preserva configurações, temas e outros dados em `%LocalAppData%\CS2SJ-Demo`.

## Segurança

O HLAE inicia o CS2 com `-insecure`. Use essa sessão somente para assistir demos e produzir vídeos. Não entre em matchmaking nem em servidores protegidos por VAC enquanto o HLAE estiver ativo.

Feche o CS2 antes de processar. A ferramenta exige Windows 10 ou mais recente e pelo menos 20 GiB livres na unidade de saída.

## Saída

```text
videos-campeonato\
  final\
    manifest.json
    render.log
    selection-decisions.jsonl
    clips\
      01-jogador-ACE-16x9.mp4
    masters\
      clean\highlight-id\video.mp4
      game\highlight-id\video.mp4
```

As versões verticais e os vídeos de resumo estão temporariamente desabilitados para reduzir o tempo e a carga de pós-produção. O código desses recursos foi preservado para reativação futura.

`selection-decisions.jsonl` registra localmente os candidatos visíveis, perspectiva, abrangência, fatores objetivos, nota e decisão final para ajudar na calibração do beta. O aplicativo não envia esse arquivo nem qualquer telemetria; ele só é compartilhado se você o copiar manualmente.

## Modos de HUD

As duas opções da tela são exclusivas:

- nenhuma marcada: captura limpa, sem HUD do jogo e sem HUD personalizada;
- **Mostrar HUD do jogo**: mantém apenas a HUD nativa do CS2;
- **Usar HUD personalizada**: usa a captura limpa e adiciona a HUD CS2SJ com campeonato, times, placar no início do round, mapa, round, jogador, tipo do highlight e logo local.

O nome do campeonato pode ser alterado na tela principal. Quando ainda não existe um tema configurado, o aplicativo cria e seleciona automaticamente uma base pronta. O editor visual mantém os campos avançados, oferece o layout guiado e permite escolher separadamente as cores dos times A e B.

O placar é extraído da demo e preserva a identidade dos times mesmo após a troca de lados. Quando ele não puder ser determinado com segurança, a HUD mostra um traço em vez de inventar um valor. A interface de replay, FPS, telemetria e informações de build ficam ocultas em todos os modos.

Antes da captura, o aplicativo protege os arquivos pessoais `cs2_user_convars*.vcfg` e os restaura ao terminar, inclusive após erro ou cancelamento. `autoexec.cfg`, binds e arquivos do campeonato não são alterados. Se a proteção não puder ser preparada, o CS2 não é aberto.

Os modos sem HUD e HUD personalizada compartilham masters `clean`; a HUD nativa usa masters `game`. Uma nova execução reaproveita masters e vídeos já validados do modo correto. Cancelar a janela não apaga trabalho concluído.

## Ritmo dos clipes

O ritmo inteligente é aplicado depois que vídeo e áudio já foram reunidos no mesmo master. A edição mantém 3 segundos antes e 2 segundos depois de cada evento em 1×; lacunas ociosas maiores que 6 segundos entre eventos são reproduzidas integralmente em 2×. O vídeo termina após os 2 segundos de desfecho do último evento, sem preservar uma cauda que sugira outra jogada. Vídeo e áudio passam juntos pela mesma transformação de tempo.

Quando o HLAE entrega vídeo e WAV separados, a pós-produção cria primeiro um `*-av.mkv` junto à captura: vídeo copiado sem recompressão e áudio FLAC a 48 kHz. A HUD e o MP4 final usam esse arquivo único. Os originais são preservados e o master unificado é reconstruído em cada nova tentativa. A compensação de início do WAV da versão anterior foi mantida por compatibilidade; durações iguais não comprovam sincronização perceptiva.

Limitação conhecida, reproduzida com marcadores sintéticos: se um WAV tiver 500 ms extras no **fim**, a compensação herdada interpreta essa diferença como sobra no início e adianta o áudio em 500 ms, tanto no MKV quanto no MP4. O teste `TestUnifiedMasterTimingFFmpegIntegration` documenta esse comportamento, não uma aprovação de sincronismo nesse cenário. A validação com uma captura HLAE real ainda é necessária.

## Compilar

Requer Go 1.24.x:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Isso cria:

- `bin\CS2SJ-Demo.exe`, a interface gráfica;
- `bin\cs2-highlights-cli.exe`, a interface de linha de comando para diagnóstico.

Para montar a pasta portátil com uma distribuição HLAE 2.191.0 já extraída:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package.ps1 `
  -HLAEDir "C:\caminho\para\hlae-2.191.0" `
  -FFprobePath "C:\caminho\para\ffmpeg\bin\ffprobe.exe"
```

O parâmetro `-FFprobePath` é necessário quando a distribuição do HLAE contém `ffmpeg.exe`, mas não contém o utilitário `ffprobe.exe` do mesmo projeto.

Por padrão, o resultado fica em `dist\stage\CS2SJ-Demo`, pronto para uso ou para você compactar quando necessário. Para o script também gerar `dist\CS2SJ-Demo-windows-x64.zip`, acrescente `-CreateZip` ao comando.

## Gerar instalador localmente

Requer Go 1.24.x e acesso à internet somente durante a preparação. As versões e os hashes de HLAE, FFmpeg e Inno Setup ficam fixados em `installer\dependencies.json`.

```powershell
$depsRoot = Join-Path $env:TEMP 'cs2sj-release-dependencies'
$paths = Join-Path $env:TEMP 'cs2sj-release-dependency-paths.json'
.\scripts\get-release-dependencies.ps1 `
  -DestinationRoot $depsRoot `
  -PathsOutputPath $paths

$deps = Get-Content -Raw -LiteralPath $paths | ConvertFrom-Json
.\scripts\build-installer.ps1 `
  -Version '1.0.0' `
  -HLAEDir $deps.hlaeDir `
  -FFmpegPath $deps.ffmpegPath `
  -FFprobePath $deps.ffprobePath `
  -InnoCompilerPath $deps.innoCompilerPath
```

O comando cria em `dist` o ZIP portátil, o instalador offline e `SHA256SUMS.txt`.

## Publicar uma release

Crie e publique uma GitHub Release com tag exatamente no formato `vMAJOR.MINOR.PATCH`, por exemplo `v1.0.0`. O workflow Windows valida o código, baixa somente dependências com SHA-256 conferido, gera e testa o instalador e anexa automaticamente:

- `CS2SJ-Demo-vMAJOR.MINOR.PATCH-windows-x64.zip`;
- `CS2SJ-Demo-vMAJOR.MINOR.PATCH-Setup.exe`;
- `SHA256SUMS.txt`.

Se qualquer verificação falhar, nenhum arquivo é enviado para a release.

## CLI opcional

Sem uma seleção explícita fornecida pela GUI, o comando `render` usa a visão editorial com abrangência **Equilibrada** como padrão.

```powershell
.\bin\cs2-highlights-cli.exe analyze C:\demos --output C:\videos

.\bin\cs2-highlights-cli.exe analyze C:\demos --output C:\videos --event "Final CS2 São José"

.\bin\cs2-highlights-cli.exe render C:\demos --output C:\videos `
	--event "Final CS2 São José" `
  --hlae C:\HLAE\hlae.exe `
  --cs2 "C:\Program Files (x86)\Steam\steamapps\common\Counter-Strike Global Offensive\game\bin\win64\cs2.exe" `
  --hook-dll C:\HLAE\x64\AfxHookSource2.dll
```

Referências: [HLAE](https://github.com/advancedfx/advancedfx), [comandos Source 2](https://github.com/advancedfx/advancedfx/wiki/Source2%3ACommands) e [`mirv_streams`](https://github.com/advancedfx/advancedfx/wiki/Source2%3Amirv_streams).

## Desenvolvimento

- [Arquitetura](docs/architecture.md)
- Verificação local: `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1`
