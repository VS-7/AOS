---
tags: [dominio, artifact, publicacao]
aliases: [Artifact Go, Aplicação Hospedada]
fase: 8
status: especificado
origem: "[[Artifact]]"
---

# Artifact (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Artifact]] · Ver: [[Artifacts e Estáticos]] · Fase: 8

## Objetivo

Aplicações web estáticas registradas no workspace e servidas pelo próprio daemon — dashboards, relatórios, landing pages. É como o agente **entrega algo publicável** sem infraestrutura de deploy.

## Comportamento do original

Servido em `/v/{workspace}/artifacts/{id}/*`, com três níveis de visibilidade: `private`, `workspace`, `by_password` ([[Artifact]]).

E um defeito grave, registrado na engenharia reversa:

> O `FractalArtifactTransport` recebe um `processSecret` gerado com `randomUUID()` **no boot do servidor**. Como o segredo não é persistido, senhas de artifacts com visibilidade `by_password` **mudam a cada restart**. Links compartilhados param de funcionar após um reinício.

Defeito #19.

## Design em Go

```go
// internal/domain/artifact/entity.go

type Visibility string

const (
	Private   Visibility = "private"
	Workspace Visibility = "workspace"
	ByPassword Visibility = "by_password"
)

type Artifact struct {
	ID string `yaml:"-" json:"id" collection:"path"`

	Name        string     `yaml:"name"        json:"name"`
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	Entrypoint  string     `yaml:"entrypoint"  json:"entrypoint"` // HTML served as root
	Visibility  Visibility `yaml:"visibility"  json:"visibility"`
	Skill       string     `yaml:"skill,omitempty" json:"skill,omitempty"`

	// PasswordHash replaces the original's process-derived password. It is
	// persisted, so a shared link survives a restart. Fixes defect #19.
	PasswordHash string `yaml:"passwordHash,omitempty" json:"-"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"`
}
```

```go
type Service interface {
	List(ctx context.Context, q Query) ([]Artifact, error)
	Get(ctx context.Context, id string) (*Artifact, error)

	// Create scaffolds the entrypoint when one is not provided, matching the
	// original's behaviour.
	Create(ctx context.Context, in CreateInput) (*Artifact, error)
	Update(ctx context.Context, in UpdateInput) (*Artifact, error)
	Delete(ctx context.Context, id string) error

	// SetPassword hashes with argon2id and persists. Returns the URL to share.
	SetPassword(ctx context.Context, id, password string) (string, error)

	// Authorize decides whether a request may read this artifact.
	Authorize(ctx context.Context, a *Artifact, req AccessRequest) error
}
```

### Servir com segurança

Ver [[Artifacts e Estáticos]] para o transporte. Três garantias:

```go
// Serving rules, enforced in the transport:
//  1. Files are resolved inside the artifact directory only; ".." never escapes.
//  2. A strict Content-Security-Policy is applied, so a generated page cannot
//     call out to arbitrary hosts or exfiltrate workspace data.
//  3. Content-Type comes from the extension, never from the file content, and
//     unknown types are served as application/octet-stream with
//     Content-Disposition: attachment.
```

## Decisões e divergências

> [!decision] Senha persistida com argon2id
> Corrige o defeito #19. Um link compartilhado sobrevive a restart, que é o mínimo que "compartilhar um link" significa.

> [!decision] CSP estrita por padrão
> Adição. Um artifact é HTML gerado por um LLM servido no mesmo host da API. Sem CSP, um script na página poderia chamar `/api/*` com o cookie de sessão do usuário. A política default bloqueia rede externa e `unsafe-inline`; relaxá-la é opt-in por artifact, registrado no frontmatter.

> [!decision] Artifacts servidos em caminho isolado
> Já é assim no original (`/v/{ws}/artifacts/{id}/`). Reforçamos com `Cross-Origin-Resource-Policy` e sem cookies de sessão nessa rota — o conteúdo não é confiável, e o navegador precisa saber disso.

## Testes

- Round-trip completo
- `create` sem entrypoint faz scaffold de um HTML mínimo
- Senha persistida: reiniciar o daemon e validar a mesma senha
- Path traversal em `/v/{ws}/artifacts/{id}/../../etc/passwd` é negado
- Visibilidade `private` nega acesso não autenticado; `workspace` exige membro; `by_password` valida hash
- CSP presente na resposta; sem cookie de sessão nessa rota
- Extensão desconhecida vira `attachment`

## Critério de pronto

- [ ] CRUD com scaffold de entrypoint
- [ ] Senha persistida sobrevivendo a restart
- [ ] Três visibilidades aplicadas
- [ ] CSP e isolamento de origem verificados
