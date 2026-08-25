# Instalador e releases automáticas do CS2SJ Demo

## Objetivo

Distribuir o CS2SJ Demo como um aplicativo Windows convencional e também como pacote portátil. Cada GitHub Release publicada deve receber automaticamente um instalador offline, um ZIP portátil completo e um arquivo de checksums, sem depender de arquivos presentes no computador do mantenedor.

## Escopo

- Gerar um único instalador offline para Windows x64 com Inno Setup.
- Instalar somente para o usuário atual, sem solicitar privilégios administrativos.
- Reutilizar o stage portátil produzido pelo projeto como fonte única dos arquivos distribuídos.
- Produzir localmente o ZIP portátil e o instalador a partir do mesmo comando de build.
- Produzir e anexar os artefatos automaticamente quando uma GitHub Release for publicada.
- Fixar e validar por SHA-256 todas as dependências baixadas durante o build de release.
- Documentar o build local e o processo de publicação.

Não fazem parte desta entrega: atualização automática dentro do aplicativo, assinatura Authenticode, publicação na Microsoft Store, MSI, instalação para todos os usuários e download de dependências no PC do usuário final.

## Artefatos públicos

Para uma release com tag `v1.2.3`, o workflow publicará exatamente:

```text
CS2SJ-Demo-v1.2.3-windows-x64.zip
CS2SJ-Demo-v1.2.3-Setup.exe
SHA256SUMS.txt
```

O executável principal não será publicado isoladamente porque depende de caminhos relativos para HLAE, AfxHookSource2, FFmpeg, FFprobe, assets e licenças. Ele estará na raiz do ZIP portátil e dentro do instalador.

O `SHA256SUMS.txt` conterá os hashes SHA-256 dos dois artefatos binários, um por linha, no formato `<hash>  <nome-do-arquivo>`.

## Arquitetura do build

O stage `dist/stage/CS2SJ-Demo` continua sendo a representação canônica de uma distribuição executável. O fluxo será:

```text
dependências validadas + código-fonte
    -> scripts/package.ps1
    -> dist/stage/CS2SJ-Demo
    -> ZIP portátil
    -> installer/CS2SJ-Demo.iss
    -> Setup.exe offline
    -> SHA256SUMS.txt
```

`scripts/package.ps1` continuará responsável por compilar os binários, copiar o runtime, os assets e as licenças e validar o stage. Ele receberá somente os ajustes necessários para nomes de artefatos versionados e para ser chamado pelo orquestrador.

Um novo `scripts/build-installer.ps1` será o ponto de entrada local. Seus parâmetros públicos serão:

```powershell
.\scripts\build-installer.ps1 `
  -Version "1.2.3" `
  -HLAEDir "C:\caminho\hlae-2.191.0" `
  -FFmpegPath "C:\caminho\ffmpeg.exe" `
  -FFprobePath "C:\caminho\ffprobe.exe" `
  [-InnoCompilerPath "C:\caminho\ISCC.exe"]
```

O parâmetro `Version` aceitará somente SemVer estável com três componentes numéricos (`MAJOR.MINOR.PATCH`). O script localizará `ISCC.exe` em caminhos conhecidos quando `InnoCompilerPath` não for informado. Ele criará o ZIP, compilará o `.iss`, validará os nomes de saída e escreverá os checksums. `scripts/package.ps1` passará a aceitar `FFmpegPath` e `FFprobePath` como fontes explícitas quando esses executáveis não estiverem dentro da distribuição HLAE e os copiará para o mesmo diretório relativo esperado pelo preflight.

## Dependências reproduzíveis

Um arquivo versionado `installer/dependencies.json` será a fonte de verdade para downloads do CI. Cada entrada conterá nome, versão, URL HTTPS e SHA-256. A configuração inicial será:

- HLAE `2.191.0`, arquivo oficial `hlae_2_191_0.zip`, SHA-256 `78efa377a2bac9522c3771a79c2503fec57e106432fc11d32244fe25b7c5b6cc`.
- FFmpeg `9.0.1` essentials x64, arquivo `ffmpeg-9.0.1-essentials_build.zip`, SHA-256 `fec81ae03971d9dd4be3ebe02e263bd2ec1d789483f931bdba5f5715e65da2e9`. Esse pacote contém `ffmpeg.exe` e `ffprobe.exe`.
- Inno Setup `7.1.0` x64, arquivo oficial `innosetup-7.1.0-x64.exe`, SHA-256 `0362a383ed217d4c4239b5933866dd96d3eb2102737da92f80f6057a4b40df2f`.

Um novo `scripts/get-release-dependencies.ps1` baixará os arquivos para uma pasta de trabalho informada pelo chamador, calculará o SHA-256 antes de extrair ou executar qualquer arquivo e falhará em caso de divergência. O script localizará por nome, sem depender do nome da pasta raiz criada pelos ZIPs, os binários `hlae.exe`, `AfxHookSource2.dll`, `ffmpeg.exe` e `ffprobe.exe`.

O Inno Setup será preparado no CI em modo portátil dentro da pasta temporária do runner. O compilador não será instalado globalmente e não será incluído nos artefatos.

## Comportamento do instalador

O arquivo `installer/CS2SJ-Demo.iss` terá um `AppId` fixo e imutável. O mesmo identificador permitirá que versões posteriores atualizem a instalação existente e mantenham uma única entrada de desinstalação.

O instalador terá estas propriedades:

- `PrivilegesRequired=lowest` e nenhuma opção para alternar para instalação administrativa.
- Destino padrão `%LocalAppData%\Programs\CS2SJ-Demo`.
- Aplicativo exibido como `CS2SJ Demo` e versão originada do parâmetro do build.
- Arquitetura permitida: Windows x64.
- Requisito mínimo: Windows 10 x64.
- Compressão sólida LZMA2 para um único arquivo offline.
- Interface e mensagens em português do Brasil.
- Ícone do setup, do atalho e da entrada de desinstalação baseado em `assets/cs2sj-logo.ico`.
- Atalho sempre criado no menu Iniciar do usuário atual.
- Atalho da área de trabalho oferecido como tarefa opcional.
- Opção para abrir o CS2SJ Demo ao concluir uma instalação interativa.
- Entrada em “Aplicativos instalados” com nome, versão, publicador, ícone e desinstalador.
- Encerramento e reinício controlado do aplicativo quando arquivos em uso precisarem ser atualizados.

Todos os arquivos do stage serão instalados preservando a estrutura relativa. Isso mantém o comportamento atual de `internal/gui.ResolveBundle`, que encontra `tools\hlae` e `assets` a partir do diretório de `CS2SJ-Demo.exe`.

O desinstalador removerá somente arquivos e atalhos que pertencem à instalação. `%LocalAppData%\CS2SJ-Demo\config.json`, temas do usuário e vídeos gerados ficarão preservados tanto em atualizações quanto na desinstalação.

## GitHub Actions

O workflow `.github/workflows/release.yml` será acionado por:

```yaml
on:
  release:
    types: [published]
```

Ele terá apenas `contents: write` e usará o `GITHUB_TOKEN` fornecido pelo GitHub; nenhum segredo adicional será necessário. O job rodará em runner Windows e executará, na ordem:

1. Validar que `github.event.release.tag_name` corresponde a `vMAJOR.MINOR.PATCH`.
2. Fazer checkout do repositório na tag da release, não na branch padrão.
3. Configurar Go `1.24.x` com cache de módulos.
4. Baixar e validar as dependências descritas em `installer/dependencies.json`.
5. Preparar o Inno Setup portátil no diretório temporário do runner.
6. Executar a verificação Go existente.
7. Executar o build versionado do ZIP e do instalador.
8. Executar o teste de instalação e desinstalação.
9. Confirmar que existem exatamente os três artefatos públicos esperados.
10. Enviar os três arquivos, em uma única chamada, para a release correspondente com `gh release upload` e `--clobber` para permitir a repetição segura de um job.

O upload ocorrerá somente depois que todos os artefatos e verificações tiverem sido concluídos. Uma falha anterior deixará a release sem novos anexos, evitando publicar apenas parte do conjunto.

## Validação

Os scripts PowerShell separarão validação pura de efeitos externos para permitir testes determinísticos. Os testes cobrirão:

- aceitação de `1.2.3` e rejeição de versão vazia, `v1.2.3`, `1.2` e versões com sufixos;
- carregamento do manifesto de dependências e rejeição de campos ausentes, URL não HTTPS e SHA-256 inválido;
- falha quando um download não corresponde ao hash fixado;
- falha quando o stage não contém executável, HLAE, hook, FFmpeg, FFprobe, assets ou avisos de licença;
- geração exata dos nomes versionados e do conteúdo de `SHA256SUMS.txt`;
- compilação real do script Inno Setup;
- instalação silenciosa em um diretório temporário por usuário;
- presença e estrutura dos arquivos instalados;
- execução de `cs2-highlights-cli.exe` sem argumentos, confirmando a mensagem de uso e o código de saída esperado, para provar que o binário instalado inicia;
- reinstalação de uma segunda versão sobre o mesmo `AppId` sem criar outra entrada de desinstalação;
- desinstalação silenciosa e remoção da pasta instalada;
- preservação de um arquivo sentinela criado na pasta de configuração do usuário.

O teste de instalação usará um nome e diretório isolados e sempre tentará executar o desinstalador no bloco de limpeza. Ele não abrirá CS2 nem HLAE.

## Tratamento de erros e segurança

- Todos os scripts usarão `$ErrorActionPreference = 'Stop'` e verificarão códigos de saída de programas externos.
- Nenhum arquivo baixado será extraído ou executado antes da validação SHA-256.
- Downloads e builds usarão diretórios temporários explícitos; os scripts verificarão os caminhos absolutos antes de qualquer remoção recursiva.
- Mensagens de erro identificarão a dependência, arquivo ou etapa que falhou.
- O workflow não executará código vindo de pull requests e só publicará a partir do commit associado à tag da release.
- As licenças do HLAE e do FFmpeg acompanharão o pacote conforme o fluxo de packaging existente e o `third_party/NOTICE.md` será atualizado com a versão de FFmpeg distribuída.
- A assinatura digital fica fora do escopo. Portanto, o Windows poderá exibir aviso de reputação/SmartScreen até que o projeto adote um certificado e acumule reputação.

## Documentação

O `README.md` será atualizado para priorizar o instalador para usuários comuns, manter o ZIP como alternativa portátil e documentar:

- instalação, atualização e desinstalação;
- preservação das configurações do usuário;
- build local com Inno Setup;
- formato obrigatório das tags de release;
- artefatos criados automaticamente e uso do arquivo de checksums;
- procedimento para atualizar versões e hashes em `installer/dependencies.json`.
