---
tags: [entrega, build, cross-compile]
aliases: [Build, Cross-Compile, Taskfile]
fase: 9
status: pronto
origem: "[[Versões e Artefatos]]"
---

# Build e Cross-Compile

**Entregue.** `task build:verify-reproducible` prova que dois builds do mesmo
commit são byte-idênticos (corrigiu `DATE` para honrar `SOURCE_DATE_EPOCH`,
que antes ignorava silenciosamente). `task build:check-size` aplica os alvos
desta nota (30/45/55 MB, tolerância de 20%) — hoje `aos` 9 MB, `aosd` 32 MB,
`aos-desktop` 64 MB (só 2 MB de folga antes do teto, vale observar).
`task build:check-no-abspath` prova que `-trimpath` realmente removeu os
caminhos deste desenvolvedor. As seis combinações já compilavam
(`task build:all`, entregue na Fase 8); esta rodada verificou as três
propriedades que a seção "Critério de pronto" pedia e que nada checava
antes. Distribuição (`install.sh`, Homebrew, `go install`, Scoop/MSI) segue
não implementada — depende de onde os releases vão morar, uma decisão de
infraestrutura, não de código.

> Pai: [[AOS]] · Origem no original: [[Versões e Artefatos]] · Fase: 9

## Objetivo

Produzir os três binários para seis combinações de plataforma, de uma máquina, sem source maps e sem símbolos desnecessários.

## Comportamento do original

Bun `--compile` produz binários de 80 MB (CLI) e 170 MB (servidor), com o runtime embutido, distribuídos via npm com cinco `optionalDependencies` binárias e um shim Node de resolução ([[Versões e Artefatos]], [[Fractal CLI]]).

E o defeito #1: os source maps são publicados junto, somando 104 MB e contendo o TypeScript original completo.

## Design

### Alvos

| GOOS | GOARCH | Binários |
|---|---|---|
| darwin | arm64, amd64 | `aos`, `aosd`, `aos-desktop` |
| linux | arm64, amd64 | `aos`, `aosd`, `aos-desktop` |
| windows | amd64 | `aos.exe`, `aosd.exe`, `aos-desktop.exe` |

`CGO_ENABLED=0` em todos ([[ADR-0001 Go como linguagem]], [[ADR-0008 SQLite puro Go para filas]]).

### Taskfile

```yaml
# Taskfile.yml
version: '3'

vars:
  VERSION: {sh: git describe --tags --always --dirty}
  COMMIT:  {sh: git rev-parse --short HEAD}
  DATE:    {sh: date -u +%Y-%m-%dT%H:%M:%SZ}
  LDFLAGS: >-
    -s -w
    -X github.com/OWNER/aos/internal/core/build.Version={{.VERSION}}
    -X github.com/OWNER/aos/internal/core/build.Commit={{.COMMIT}}
    -X github.com/OWNER/aos/internal/core/build.Date={{.DATE}}

tasks:
  frontend:
    dir: frontend
    cmds: [npm ci, npm run build]

  gen:
    desc: Regenerate everything derived from source
    cmds:
      - task: gen-schema      # registry Go → frontend/src/lib/schema.ts
      - task: gen-components  # spec.ts → internal/domain/view/components.gen.go
      - task: gen-skill       # registry Go → pkg/skill/
      - wails3 generate bindings

  build:
    deps: [frontend]
    cmds:
      - CGO_ENABLED=0 go build -trimpath -ldflags="{{.LDFLAGS}}" -o dist/aos  ./cmd/aos
      - CGO_ENABLED=0 go build -trimpath -ldflags="{{.LDFLAGS}}" -o dist/aosd ./cmd/aosd

  build:all:
    deps: [frontend]
    cmds:
      - for: {var: TARGETS}
        cmd: |
          GOOS={{splitList "/" .ITEM | first}} GOARCH={{splitList "/" .ITEM | last}} \
          CGO_ENABLED=0 go build -trimpath -ldflags="{{.LDFLAGS}}" \
            -o dist/{{.ITEM}}/ ./cmd/aos ./cmd/aosd
```

`-trimpath` remove caminhos absolutos do binário; `-s -w` remove a tabela de símbolos e as informações de debug. Juntos, são a resposta ao defeito #1.

### Reprodutibilidade

```bash
# Two builds from the same commit produce byte-identical binaries.
# -trimpath removes machine-specific paths; SOURCE_DATE_EPOCH fixes the
# embedded date. This is what makes a release verifiable by a third party.
SOURCE_DATE_EPOCH=$(git log -1 --format=%ct) task build:all
```

Um teste de CI compila duas vezes e compara os hashes.

### Frontend embutido

```go
//go:embed all:frontend/dist
var assets embed.FS
```

O mesmo bundle vai para `aosd` (servido em `/*`) e para `aos-desktop` (WebView). Construído uma vez.

### Tamanho alvo

| Binário | Alvo | Referência do original |
|---|---|---|
| `aos` | < 30 MB | 79,5 MB |
| `aosd` | < 45 MB | 169,6 MB |
| `aos-desktop` | < 55 MB | 578 MB (bundle Electron completo) |

Um teste de CI falha se um binário ultrapassar o alvo em mais de 20% — regressão de tamanho é regressão.

### Distribuição

| Canal | Forma |
|---|---|
| Direto | `curl -sSL <url>/install.sh \| sh` — baixa o binário da plataforma |
| Homebrew | Formula com `bottle` por plataforma |
| Go | `go install github.com/OWNER/aos/cmd/aos@latest` |
| Windows | Scoop ou instalador MSI ([[Empacotamento Wails3]]) |

**Sem npm.** O shim Node do original existe para distribuir binários por um gerenciador de pacotes JavaScript; sem esse acoplamento, o shim não é necessário.

## Decisões e divergências

> [!decision] Sem source maps, com símbolos removidos
> Corrige o defeito #1. `-s -w -trimpath`.

> [!decision] Builds reprodutíveis
> Um usuário pode verificar que o binário corresponde ao commit. Para uma ferramenta que executa comandos na máquina do usuário, isso importa.

> [!decision] Sem distribuição via npm
> O launcher Node e as cinco `optionalDependencies` existem por causa do npm. Um binário Go não precisa disso.

> [!decision] Teste de tamanho
> Sem portão, o binário cresce sem que ninguém note até virar problema.

## Testes

- `task build:all` produz os binários para as seis combinações
- Cada binário roda `version` na plataforma alvo (CI com matriz)
- Build reprodutível: dois builds do mesmo commit têm o mesmo hash
- Nenhum símbolo de debug no binário de release
- Nenhum caminho absoluto embutido (`strings | grep $HOME` vazio)
- Tamanho dentro do alvo
- `task gen` seguido de `git diff --exit-code` limpo

## Critério de pronto

- [ ] Seis alvos compilando de uma máquina
- [ ] Builds reprodutíveis verificados
- [ ] Binários dentro do tamanho alvo
- [ ] Sem source maps, sem símbolos, sem caminhos absolutos
