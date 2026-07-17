# Arquitetura do CS2SJ Highlights

O CS2SJ Highlights é um aplicativo Windows local e sequencial. Uma execução controla uma única sessão HLAE/CS2, analisa demos e publica clipes validados por FFprobe.

## Fluxo

```text
GUI ou CLI
    -> preflight de caminhos e ferramentas
    -> parser da demo
    -> seleção de highlights
    -> manifesto persistente
    -> planejamento de passes HLAE
    -> captura de masters
    -> pós-produção FFmpeg
    -> validação e publicação do MP4
```

## Pacotes

- `internal/demos`: transforma uma demo em timeline, rounds, jogadores e kills.
- `internal/highlights`: aplica regras determinísticas e seleciona os lances.
- `internal/pipeline`: orquestra análise, retomada, captura, retries e pós-produção.
- `internal/render`: gera CFGs do HLAE, executa a captura e protege configurações pessoais do CS2.
- `internal/media`: executa FFmpeg/FFprobe, valida arquivos e contém os recursos editoriais ainda desabilitados.
- `internal/manifest`: calcula fingerprints e grava manifestos atomicamente.
- `internal/gui` e `internal/cli`: adaptadores para interface Windows e terminal.
- `internal/model`: contratos persistidos e versões de artefatos.

## Invariantes operacionais

- Capturas não rodam em paralelo.
- O CS2 sempre inicia com `-insecure` para reprodução de demos.
- Cancelamento e timeout encerram somente o PID de CS2 descoberto após o início da captura.
- Configurações `cs2_user_convars*.vcfg` são protegidas antes da captura e restauradas ao final.
- Manifestos e configurações são publicados por arquivo temporário seguido de rename.
- Masters e outputs só são reutilizados quando versão, modo de HUD e validação de mídia são compatíveis.
- Falha ao limpar um CFG gera warning, mas não invalida uma captura concluída.

## Versões e compatibilidade

As versões persistidas estão centralizadas em `internal/model/versions.go`. Alterações que invalidem seleção, metadados, master ou output devem atualizar a constante correspondente e incluir teste de migração/reuso.

## Outputs atuais

A configuração de produção gera apenas MP4 horizontal 1920×1080 em velocidade integral. Código de vertical, resumo e ritmo inteligente permanece isolado em `internal/media`, mas não participa do pipeline enquanto esses recursos estiverem desabilitados.

## Verificação

Execute `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1`. O script verifica formatação, módulos, `go vet`, testes e build da CLI. O workflow `.github/workflows/verify.yml` executa o mesmo processo no Windows.
