---
tags: [dominio, theme, ui]
aliases: [Theme Go, Temas]
fase: 7
status: pronto
origem: "[[Theme]]"
---

# Theme (Go)

> Pai: [[Design System]] · Origem no original: [[Theme]] · Ver: [[Temas]] · Fase: 7

## Objetivo

Sistema de temas da UI: presets embutidos mais presets do usuário, com sincronização de aparência nativa.

## Comportamento do original

**38 temas embutidos**, um arquivo por tema ([[Theme]]). A curadoria mira desenvolvedores: Catppuccin (três variantes), Rose Pine, Kanagawa, Everforest, Gruvbox, Tokyo Night, Dracula, Nord, Vesper, além de homenagens (`cursor`, `vercel`) e o tema próprio do produto.

O canal IPC `theme:set-appearance` sincroniza o tema do renderer com a **aparência nativa e a vibrancy** da janela — trocar de tema na UI muda o material da janela do sistema.

## Design em Go

```go
// internal/domain/theme/entity.go

type Theme struct {
	ID string `yaml:"-" json:"id" collection:"path"`

	Name    string `yaml:"name"    json:"name"`
	Author  string `yaml:"author,omitempty" json:"author,omitempty"`
	Builtin bool   `yaml:"-"       json:"builtin"`

	// Appearance drives the native window material, not just CSS.
	Appearance Appearance `yaml:"appearance" json:"appearance"` // light | dark | auto

	Tokens map[string]string `yaml:"tokens" json:"tokens"` // design token → colour
}
```

Os temas embutidos são embarcados com `//go:embed themes/*.yaml` — sem I/O, sem possibilidade de faltar em runtime.

```go
type Service interface {
	List(ctx context.Context) ([]Theme, error)
	Get(ctx context.Context, id string) (*Theme, error)

	// Install adds a user preset, validating that every required token exists.
	Install(ctx context.Context, in InstallInput) (*Theme, error)
	Delete(ctx context.Context, id string) error
}
```

### Validação de tokens

```go
// requiredTokens is the contract between a theme and the design system.
// A theme missing a token would render an invisible element somewhere; the
// list is generated from the CSS variables the frontend actually consumes,
// so it cannot drift.
//go:embed tokens.txt
var requiredTokensRaw string

func Validate(t Theme) error
```

## Decisões e divergências

> [!decision] Lista de tokens gerada do frontend
> Como em [[View (Go)]] com o catálogo de componentes: o contrato é gerado no build, não mantido à mão. Um token novo no CSS quebra o build de temas até que todos os temas o declarem.

> [!decision] Contraste verificado
> Adição. Um teste calcula a razão de contraste dos pares texto/fundo de cada tema embutido e falha abaixo de WCAG AA. Um tema bonito e ilegível é um bug de acessibilidade.

> [!decision] Sincronização nativa via Wails
> O IPC do Electron vira uma chamada de service Wails que ajusta a aparência da janela. Ver [[Temas]] para o lado da plataforma.

## Testes

- Os 38 temas embutidos carregam e validam
- Tema sem token obrigatório é rejeitado no install
- Contraste WCAG AA em todos os embutidos
- `appearance: auto` segue a preferência do SO
- Preset do usuário sobrevive a atualização do app

## Critério de pronto

- [x] 38 temas embutidos portados e validados
- [x] Lista de tokens gerada do frontend
- [x] Contraste verificado por teste
- [x] Sincronização com a aparência nativa funcionando

## Saída dos testes — Fase 7

`go test ./internal/domain/theme/` — **90,0% de cobertura**, 17 testes.
`go test ./internal/core/oklch/` — **100%**, 8 testes.

Os 38 temas foram extraídos mecanicamente de `_extracted/.../data/themes/*.theme.ts`
e convertidos para YAML embutido. `TestTheThirtyEightBuiltinThemesLoadAndValidate`
afirma os 38 e que **cada um carrega as duas aparências**, que é o que torna
`auto` possível.

| O que a nota pede | Teste |
|---|---|
| Os 38 embutidos carregam e validam | `TestTheThirtyEightBuiltinThemesLoadAndValidate` |
| Tema sem token obrigatório é rejeitado | `TestARenderedThemeDefinesEveryTokenTheInterfaceReads`, `TestAPresetIsValidatedBeforeItIsStored` |
| Contraste WCAG AA em todos | `TestEveryBuiltinThemeReachesWCAGAAOnItsOwnText` + `TestTheAccentIsLegibleAsLargeText` |
| `appearance: auto` segue o SO | `watchSystemAppearance` em `lib/theme.ts`; verificado por leitura, não por teste |

**Divergência: o modelo é o do original, não o do esboço da nota.** A nota
descreve `Tokens map[string]string` com uma `Appearance`; o original tem
`{dark, light}` por tema, cada um com surface, ink, accent, contrast e cores
semânticas, e **deriva** os tokens. Um mapa único não consegue expressar `auto`,
que a própria nota exige. A entidade ficou `Variants map[Appearance]Palette` e a
derivação é porte fiel de `theme-provider.tsx`, mistura por mistura, em OKLCH.

**Divergência: os nomes dos tokens são os do original.** `Temas.md` esboça
`--bg`, `--fg`, `--bg-subtle`; o original usa o vocabulário shadcn
(`--background`, `--muted-foreground`, `--sidebar-accent`). Renomear quebraria o
porte fiel dos 121 componentes que a seção 5.4 do PROMPT exige, e a própria nota
delega a lista ao que o frontend consome. `tokens.txt` é gerado de
`frontend/src/styles/tokens.css` por `task gen-tokens`: 46 tokens, e a
regeneração não produz diff.

**Divergência de nome: o tema da casa.** O 38º arquivo é o tema próprio do
produto original. A paleta foi mantida; o identificador e o nome passaram a
`aos` (ADR-0000). A autoria boilerplate — 38 arquivos dizendo `author: Fractal`
e `description: "X theme from Fractal"` — foi removida: são paletas de
comunidade, e carregar o nome do outro produto seria errado nos dois sentidos.

**Adição além da nota: contraste do accent.** Além do corpo de texto, o accent é
medido contra a superfície pelo piso de texto grande — ele carrega títulos,
links e o botão primário. Os 38 passam nas duas aparências.

**Não verificado:** a sincronização com a aparência nativa da janela. O caminho
existe inteiro — `applyTheme` chama `SystemService.SetAppearance`, que valida e
chama `window.SetBackgroundColour` — e `TestSetAppearanceRefusesWhatTheWindowCannotBe`
prova o que chega à plataforma. O que **não** foi observado é o material da
janela mudando no macOS: exige abrir o app e olhar.
