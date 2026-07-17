# HUD externa 16:9

Com a opção **Usar HUD personalizada** ativa, selecione um `hud.json` na tela principal ou abra **Editor visual**. O tema é lido durante cada renderização: salvar o arquivo não exige recompilar o aplicativo.

Cada tema fica em uma pasta própria, por exemplo:

```text
themes/final-de-inverno/
  hud.json
  team-a.png
  team-b.png
```

O `hud.json` contém elementos `box`, `text` e `image`. As coordenadas e dimensões são porcentagens de um quadro 1920×1080 (16:9); `z_index` decide a sobreposição. Em textos, `binding` pode ser `event`, `team_a_name`, `team_b_name`, `score_a`, `score_b`, `map`, `round`, `player` ou `highlight`.

Imagens são arquivos relativos à pasta do tema. Para manter os ícones de cada time externos ao placar, adicione dois elementos `image` posicionados nas laterais e aponte `asset` para os respectivos PNGs. O carregamento rejeita caminhos que escapem dessa pasta e arquivos inexistentes.

Pelo terminal, use o mesmo tema com:

```text
cs2-highlights render DEMOS --output VIDEOS --hud custom --hud-theme C:\themes\final\hud.json
```

Por enquanto, temas são somente 16:9. Formatos verticais e de resumo continuam fora deste fluxo.
