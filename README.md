# CS2SJ Highlights

Aplicativo local para Windows que lê demos de Counter-Strike 2, seleciona highlights automaticamente e entrega clipes MP4 em 16:9.

## Uso pelo aplicativo

1. Abra a pasta portátil `dist\stage\CS2SJ-Demo` ou extraia todo o conteúdo do ZIP criado a partir dela.
2. Abra `CS2SJ-Demo.exe`.
3. Selecione:
   - o arquivo `cs2.exe`;
   - a pasta contendo as demos `.dem`;
   - a pasta onde os vídeos serão salvos.
4. Clique em **Analisar demos**.
5. Confira a tabela com demo, mapa, jogador, tipo do highlight e round.
6. Desmarque os clipes que não deseja gerar e clique em **Processar selecionados**.

Todos os clipes vêm marcados por padrão. Os botões **Marcar todos** e **Desmarcar todos** permitem ajustar a lista rapidamente. Demos sem nenhum clipe marcado são ignoradas na captura.

Não existe quantidade mínima de clipes por partida. A seleção automática aceita apenas ACE, clutch vencido, 4K, 3K ou multi-kill de granada; uma demo sem jogadas que atendam a esses critérios aparece sem clipes e não é renderizada.

Os caminhos ficam salvos em `%LocalAppData%\CS2SJ-Demo\config.json`. A análise e a captura rodam em segundo plano, a janela mostra o andamento e permite abrir a pasta dos vídeos ao terminar.

O HLAE 2.191.0, AfxHookSource2, FFmpeg e FFprobe acompanham a pasta portátil. Se o HLAE não for encontrado, copie ou extraia novamente a pasta completa. O botão **Baixar HLAE** abre somente a página oficial: https://github.com/advancedfx/advancedfx/releases/latest

## Segurança

O HLAE inicia o CS2 com `-insecure`. Use essa sessão somente para assistir demos e produzir vídeos. Não entre em matchmaking nem em servidores protegidos por VAC enquanto o HLAE estiver ativo.

Feche o CS2 antes de processar. A ferramenta exige Windows 10 ou mais recente e pelo menos 20 GiB livres na unidade de saída.

## Saída

```text
videos-campeonato\
  final\
    manifest.json
    render.log
    clips\
      01-jogador-ACE-16x9.mp4
    masters\
      clean\highlight-id\video.mp4
      game\highlight-id\video.mp4
```

As versões verticais e os vídeos de resumo estão temporariamente desabilitados para reduzir o tempo e a carga de pós-produção. O código desses recursos foi preservado para reativação futura.

## Modos de HUD

As duas opções da tela são exclusivas:

- nenhuma marcada: captura limpa, sem HUD do jogo e sem HUD personalizada;
- **Mostrar HUD do jogo**: mantém apenas a HUD nativa do CS2;
- **Usar HUD personalizada**: usa a captura limpa e adiciona a HUD CS2SJ com campeonato, times, placar no início do round, mapa, round, jogador, tipo do highlight e logo local.

O placar é extraído da demo e preserva a identidade dos times mesmo após a troca de lados. Quando ele não puder ser determinado com segurança, a HUD mostra um traço em vez de inventar um valor. A interface de replay, FPS, telemetria e informações de build ficam ocultas em todos os modos.

Antes da captura, o aplicativo protege os arquivos pessoais `cs2_user_convars*.vcfg` e os restaura ao terminar, inclusive após erro ou cancelamento. `autoexec.cfg`, binds e arquivos do campeonato não são alterados. Se a proteção não puder ser preparada, o CS2 não é aberto.

Os modos sem HUD e HUD personalizada compartilham masters `clean`; a HUD nativa usa masters `game`. Uma nova execução reaproveita masters e vídeos já validados do modo correto. Cancelar a janela não apaga trabalho concluído.

## Ritmo dos clipes

O ritmo inteligente está temporariamente desabilitado. Cada vídeo final preserva o master completo em 1×, sem cortes, saltos ou aceleração, para mostrar o lance inteiro. Os metadados de ação continuam salvos para uma futura reativação desse recurso.

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

## CLI opcional

```powershell
.\bin\cs2-highlights-cli.exe analyze C:\demos --output C:\videos

.\bin\cs2-highlights-cli.exe render C:\demos --output C:\videos `
  --hlae C:\HLAE\hlae.exe `
  --cs2 "C:\Program Files (x86)\Steam\steamapps\common\Counter-Strike Global Offensive\game\bin\win64\cs2.exe" `
  --hook-dll C:\HLAE\x64\AfxHookSource2.dll
```

Referências: [HLAE](https://github.com/advancedfx/advancedfx), [comandos Source 2](https://github.com/advancedfx/advancedfx/wiki/Source2%3ACommands) e [`mirv_streams`](https://github.com/advancedfx/advancedfx/wiki/Source2%3Amirv_streams).
