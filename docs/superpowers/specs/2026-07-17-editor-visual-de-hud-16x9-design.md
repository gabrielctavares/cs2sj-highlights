# Editor Visual de HUD 16:9

## Objetivo

Permitir personalizar a HUD própria do CS2SJ Highlights sem recompilar o aplicativo. O usuário cria, edita, importa e seleciona temas externos para a renderização horizontal 1920×1080.

## Escopo

O editor será uma tela nativa do aplicativo Windows e terá:

- lista de temas e ações de criar, duplicar, importar, exportar e excluir;
- canvas de prévia 16:9 com dados de exemplo;
- seleção, arrastar, redimensionar, ordenar por camada e remover elementos;
- painel de propriedades para âncora, posição, tamanho, cor, opacidade, fonte, texto e vínculo de dados;
- biblioteca de elementos `box`, `text` e `image`;
- campos dinâmicos: evento, time A, time B, logo do time A, logo do time B, placar A, placar B, mapa, round, jogador e highlight;
- desfazer/refazer, salvar e validação de tema;
- seleção de um tema para a execução seguinte.

O tema padrão reproduz a HUD customizada atual. Os ícones dos times ficam nas extremidades externas do placar, deixando nomes e placar no centro.

## Fora de escopo

- HUD vertical 9:16 ou outros formatos;
- resumo, ritmo inteligente ou novos outputs de vídeo;
- editor web embutido;
- alteração da HUD nativa do CS2;
- geração automática de logos ou fontes.

## Arquivos de tema

Cada tema é autocontido:

```text
themes/
  campeonato-2026/
    hud.json
    logo-evento.png
    time-a.png
    time-b.png
    fonte.ttf
```

`hud.json` usa versão explícita e contém um canvas fixo de 1920×1080. Cada elemento tem `id`, `type`, `anchor`, `x`, `y`, `width`, `height`, `zIndex`, `visible` e propriedades específicas.

As âncoras permitidas são `top_left`, `top_center`, `top_right`, `center`, `bottom_left`, `bottom_center` e `bottom_right`. Posição e tamanho são percentuais do canvas, para a composição permanecer legível se a prévia do editor for redimensionada.

Exemplo reduzido:

```json
{
  "version": 1,
  "canvas": { "width": 1920, "height": 1080 },
  "elements": [
    {
      "id": "team-a-logo",
      "type": "image",
      "anchor": "top_left",
      "x": 8,
      "y": 4,
      "width": 5,
      "height": 9,
      "zIndex": 20,
      "binding": "team_a_logo"
    },
    {
      "id": "team-a-name",
      "type": "text",
      "anchor": "top_left",
      "x": 14,
      "y": 6,
      "width": 20,
      "height": 5,
      "zIndex": 21,
      "binding": "team_a_name",
      "font": "fonte.ttf",
      "fontSize": 31,
      "color": "#FFFFFF"
    }
  ]
}
```

Arquivos referenciados devem estar dentro da pasta do tema. Caminhos absolutos e qualquer caminho que escape dela são inválidos.

## Componentes

- `internal/hudtheme`: modelo, leitura, validação, gravação atômica e compilação do tema para as instruções FFmpeg.
- `internal/gui`: seleção de tema e editor visual nativo baseado em canvas 16:9, painel de propriedades e histórico de edição.
- `internal/media`: recebe uma composição já validada; não decide layout nem acessa arquivos de tema diretamente.
- `internal/cli`: aceita opcionalmente um caminho de tema para renderização de diagnóstico.

O pipeline mantém a responsabilidade de fornecer os dados de cada highlight. O compilador do tema resolve bindings para dados do highlight e gera filtros `drawbox`, `drawtext` e `overlay` no FFmpeg.

## Fluxo

1. A pessoa abre o Editor de HUD e seleciona ou cria um tema.
2. O editor carrega o JSON e os assets, renderiza a prévia com dados de exemplo e mantém alterações em memória.
3. Ao salvar, valida tudo e publica `hud.json` por arquivo temporário seguido de rename.
4. A tela principal salva apenas a referência ao tema selecionado.
5. Antes de processar vídeos, o aplicativo recarrega e valida o tema; em falha, bloqueia a renderização com o elemento e o motivo exibidos.
6. O `ClipBuilder` usa a composição compilada para produzir a HUD customizada 16:9.

## Erros e integridade

- tema inválido não sobrescreve a última versão válida;
- assets ausentes, elementos fora do canvas, IDs duplicados, bindings desconhecidos, cores/fontes inválidas e caminhos externos são erros de validação;
- o tema padrão interno é usado somente quando nenhum tema externo estiver selecionado;
- erro de leitura do tema selecionado interrompe a renderização antes de abrir HLAE ou CS2;
- importação copia os assets para uma nova pasta de tema e rejeita conflitos de nome sem confirmação explícita.

## Testes

- modelo e serialização de tema;
- validação de âncoras, limites, IDs, caminhos e assets;
- resolução dos bindings para metadados do highlight;
- compilação dos elementos em filtros FFmpeg;
- gravação atômica e preservação do último JSON válido;
- ações de editor: mover, redimensionar, camadas, desfazer/refazer e seleção;
- regressão: sem tema externo, a HUD atual permanece visualmente e funcionalmente equivalente;
- smoke manual com tema contendo logo do evento e logos externos dos dois times.

## Critérios de aceite

- uma pessoa consegue criar um tema de campeonato sem editar Go ou gerar build;
- consegue mover e redimensionar todos os elementos da HUD 16:9 pela tela do aplicativo;
- consegue associar logos distintos a time A e time B;
- o vídeo renderizado preserva os dados corretos do lance e a composição escolhida;
- tema inválido não inicia captura nem perde a versão válida anterior;
- instalações existentes continuam podendo usar a HUD padrão.
