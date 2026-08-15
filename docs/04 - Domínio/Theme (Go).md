---
tags: [dominio, theme, ui]
aliases: [Theme Go, Temas]
fase: 7
status: especificado
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

- [ ] 38 temas embutidos portados e validados
- [ ] Lista de tokens gerada do frontend
- [ ] Contraste verificado por teste
- [ ] Sincronização com a aparência nativa funcionando
