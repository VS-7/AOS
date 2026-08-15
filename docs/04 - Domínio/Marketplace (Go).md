---
tags: [dominio, marketplace, registry]
aliases: [Marketplace Go, Registry de Skills]
fase: 8
status: especificado
origem: "[[Marketplace]]"
---

# Marketplace (Go)

> Pai: [[Skill (Go)]] · Origem no original: [[Marketplace]] · Fase: 8

## Objetivo

Registry remoto de [[Skill (Go)]]s instaláveis: descoberta e instalação a partir de repositórios.

## Comportamento do original

Feature presente na 0.1.401, com evidências indiretas ([[Marketplace]]): campos `version` e `source` (`<owner>/<repo>`) preenchidos automaticamente na instalação, consulta aceitando `owner` e `source`, comandos `skills discovery` e `skills install`, e assets de UI dedicados.

O modelo de distribuição são **repositórios GitHub** — o mesmo padrão do auto-update do app.

A engenharia reversa registra explicitamente que o alcance real do registry não foi determinável do código local: depende de um serviço remoto não analisado ([[Metodologia da Engenharia Reversa]]).

## Design em Go

```go
// internal/domain/marketplace/entity.go

type Listing struct {
	Source      string   `json:"source"`      // "<owner>/<repo>"
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Tags        []string `json:"tags,omitempty"`
	Stars       int      `json:"stars,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`

	// Permissions is surfaced at discovery time, not only at install time, so
	// the agent and the user can filter by what a skill demands before fetching.
	Permissions skill.Permissions `json:"permissions"`
}
```

```go
// internal/domain/marketplace/port.go

// Registry is the outbound port. Two implementations are planned: a Git-based
// one (no central service required) and an HTTP one for a hosted index.
type Registry interface {
	Search(ctx context.Context, q SearchQuery) ([]Listing, error)
	Fetch(ctx context.Context, source, ref string) (Package, error)
}
```

```go
type Service interface {
	// Discovery searches the configured registries.
	Discovery(ctx context.Context, in DiscoveryInput) ([]Listing, error)

	// Install delegates to skill.Installer, which enforces manifest
	// verification and human consent.
	Install(ctx context.Context, in InstallInput) (*skill.Skill, error)
}
```

### Registries configuráveis

```jsonc
// ~/.aos/config.json
"marketplace": {
  "registries": [
    { "id": "official", "type": "git",  "url": "https://github.com/OWNER/registry" },
    { "id": "internal", "type": "http", "url": "https://skills.empresa.com" }
  ],
  "requireSignature": false
}
```

## Decisões e divergências

> [!decision] Registry baseado em Git como implementação primária
> Não exige serviço central: um repositório com um índice é suficiente. Reduz a dependência de infraestrutura própria na Fase 8 e permite registries privados corporativos de imediato.

> [!decision] Permissões visíveis na descoberta
> Ver [[ADR-0015 Skills com permissões declaradas]]. Saber o que uma skill pede **antes** de baixar é melhor que saber depois.

> [!decision] Sem publicação nesta fase
> `discovery` e `install` entram na Fase 8. Publicar (`skills publish`) exige política de moderação e assinatura, e fica para depois do nome definitivo do produto — o registry carrega a marca.

> [!decision] Escopo honesto
> Como a engenharia reversa não determinou o comportamento do registry remoto do original, não fingimos compatibilidade. Nosso registry é nosso.

## Testes

- `Search` contra um registry Git de teste, com filtros por tag e por owner
- `Fetch` de um pacote com manifesto válido e de um com manifesto inconsistente
- Listagem exibe permissões antes do download
- Registry indisponível degrada com erro claro e CTA, sem travar o CLI
- Contrato de `Registry` roda contra as duas implementações

## Critério de pronto

- [ ] `discovery` e `install` funcionando sobre registry Git
- [ ] Permissões visíveis na descoberta
- [ ] Registries múltiplos e configuráveis
