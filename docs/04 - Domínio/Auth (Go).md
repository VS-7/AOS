---
tags: [dominio, auth, seguranca, usuarios]
aliases: [Auth Go, Autenticação]
fase: 4
status: especificado
origem: "[[Auth]]"
---

# Auth (Go)

> Pai: [[Config (Go)]] · Origem no original: [[Auth]] · [[users.json]] · Fase: 4

## Objetivo

Contas, sessões e tokens de API. É a fronteira que o agente **não** atravessa.

## Comportamento do original

Persistido em `~/.fractal/users.json` ([[users.json]]). O que está certo:

- **argon2id** com `m=65536, t=2, p=1` — parâmetros adequados, mantidos
- **Sem tools MCP para auth** — o agente não gerencia identidade
- **Papéis em duas dimensões:** papel de instância (`super`/`member`) e papel de workspace (`members[].role`), comparados juntos pelos guards

O que corrigimos: senha mínima de 6 caracteres (defeito #9), token em texto plano com permissão `0644` e sem rotação nem expiração (defeitos #4 e #10).

## Design em Go

```go
// internal/domain/auth/entity.go

type Role string

const (
	Super  Role = "super"
	Member Role = "member"
)

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`

	// PasswordHash is argon2id, never plain text. Same parameters as the
	// original, which got this right: m=65536, t=2, p=1.
	PasswordHash string `json:"password" secret:"true"`
	Role         Role   `json:"role"`

	Tokens []Token `json:"tokens"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Token is stored hashed. The plain value is shown once, at creation.
type Token struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Hash      string     `json:"hash" secret:"true"`
	Prefix    string     `json:"prefix"` // "aos_a1b2" — for identification in the UI
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	LastUsed  *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}
```

```go
type Service interface {
	Onboarding(ctx context.Context, in OnboardingInput) (*User, string, error) // returns the first token, once
	Login(ctx context.Context, in LoginInput) (Session, error)
	ChangePassword(ctx context.Context, in ChangePasswordInput) error

	IssueToken(ctx context.Context, in IssueTokenInput) (Token, string, error)
	RevokeToken(ctx context.Context, id string) error
	RotateToken(ctx context.Context, id string) (Token, string, error)

	// Authenticate resolves a bearer token to a user, in constant time.
	Authenticate(ctx context.Context, bearer string) (*User, error)
}
```

### Política de senha

```go
// internal/domain/auth/password.go

const minPasswordLen = 12 // the original allowed 6, for an account that runs shell commands

//go:embed breached-top100k.txt
var breachedRaw string

// ValidatePassword enforces length and rejects known-breached passwords from
// an embedded list — no network call, so it works offline and leaks nothing.
func ValidatePassword(p string) error
```

### Comparação em tempo constante

```go
// Authenticate must not leak token validity through timing. It hashes the
// presented token and compares with subtle.ConstantTimeCompare, and it always
// performs the hash even when no user matches.
func (s *service) Authenticate(ctx context.Context, bearer string) (*User, error)
```

## Decisões e divergências

> [!decision] Tokens hasheados em repouso, mostrados uma vez
> O original guarda o token em claro em `users.json` **e** em `~/.mcp.json`. Aqui, o disco guarda o hash; o valor aparece uma vez na criação. Corrige o defeito #10 parcialmente — o `~/.mcp.json` continua precisando do valor, mas agora com permissão `0600` e possibilidade de rotação.

> [!decision] Expiração e rotação
> Nenhuma das duas existe no original. `aos auth token rotate` emite novo e revoga o antigo com período de graça configurável.

> [!decision] Senha mínima de 12 + lista de vazamentos embutida
> Corrige o defeito #9 sem chamada de rede.

> [!decision] Continua fora do MCP
> Herdado. A fronteira de privilégio é deliberada: o agente opera o domínio, não a identidade ([[Ferramentas MCP]]).

> [!decision] Sem waitlist
> O original tem `SKIP_WAITLIST_CHECK` e `FRACTAL_AUTH_WAITLIST_NOT_APPROVED`, indicando controle de acesso por lista de espera acoplado ao produto. Não portamos: é decisão de negócio do original, não capacidade técnica.

## Testes

- argon2id com os parâmetros especificados; hash verificável
- Senha com 11 caracteres rejeitada; senha vazada rejeitada
- Token: valor mostrado uma vez, disco guarda hash
- Token expirado rejeitado; rotação mantém acesso durante a graça
- `Authenticate` em tempo constante (teste estatístico de timing)
- Guard cruza papel de instância com papel de workspace
- Último `super` não pode ser removido nem rebaixado
- Nenhuma tool MCP de auth registrada (teste sobre o registry)

## Critério de pronto

- [ ] Onboarding gerando primeiro usuário e token
- [ ] Tokens hasheados, com expiração e rotação
- [ ] Política de senha aplicada
- [ ] Auth fora do alcance do agente, verificado por teste
