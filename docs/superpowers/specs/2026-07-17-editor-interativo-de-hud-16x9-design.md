# Editor interativo de HUD 16:9

## Objetivo

Substituir o editor atual, que expõe JSON e uma prévia estática, por uma tela nativa utilizável para personalizar um tema de HUD sem recompilar o aplicativo.

## Interface

- Um canvas central 16:9 mostra uma prévia com dados de exemplo e todos os elementos do tema.
- Clique seleciona um elemento. Arrastar move o elemento; a alça no canto inferior direito redimensiona-o. As posições e medidas são persistidas como porcentagens do canvas.
- Uma lista de camadas permite selecionar, adicionar, remover e alterar a ordem dos elementos.
- O painel de propriedades do elemento selecionado permite editar tipo, texto ou vínculo de dados, cor, opacidade, fonte, tamanho de fonte, arquivo de imagem e visibilidade.
- Botões criam `box`, `text` e `image`. A seleção de imagem copia o arquivo para a pasta do tema e grava um caminho relativo no `hud.json`.
- Salvar valida o tema e atualiza o mesmo `hud.json`; a tela principal continua usando esse arquivo no próximo processamento.

## Limites

- Apenas canvas e renderização 16:9 nesta etapa.
- O JSON permanece como formato externo, mas não é mostrado nem precisa ser editado pelo usuário.
- A prévia mostra logos como áreas identificadas quando não houver imagem configurada; o renderizador mantém a validação de assets no processamento.

## Tratamento de erros e testes

- Campos inválidos mostram uma mensagem no editor e não alteram o arquivo salvo.
- Arquivos escolhidos são copiados apenas após a validação do tema.
- Testes unitários cobrem movimentação, redimensionamento, seleção, ordenação e atualização de propriedades. Um smoke test do executável confere a abertura da tela e a gravação de um tema.
