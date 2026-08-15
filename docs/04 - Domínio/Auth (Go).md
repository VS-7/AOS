---
tags: [dominio, auth, seguranca, usuarios]
aliases: [Auth Go, Autenticação]
fase: 4
status: em-construcao
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

> [!decision] Onboarding fecha assim que existe uma conta
> Um endpoint sem autenticação que cria um administrador só é seguro enquanto não há administrador. Um daemon exposto depois estaria a uma requisição de um segundo.

> [!decision] Login com conta inexistente e com senha errada custam o mesmo
> Os dois devolvem o mesmo erro e fazem o mesmo trabalho de hash, contra uma senha-isca real. Distinguir os dois transforma o endpoint numa forma de enumerar contas.

> [!decision] Token hasheado com SHA-256, não com argon2id
> Um token é 256 bits de entropia que este sistema gerou; não é adivinhável, então o hash lento que protege senha escolhida por humano não compra nada e poria uma alocação de 64 MiB no caminho de toda requisição autenticada.

> [!decision] O hash de senha carrega os próprios parâmetros
> Formato PHC. Subir o custo depois não tranca ninguém fora dos hashes escritos antes da mudança.

> [!decision] Token revogado permanece no registro
> Quando foi criado, quando foi usado pela última vez e quando parou de funcionar são os três fatos de que alguém investigando um vazamento precisa.

## Critério de pronto

- [x] Onboarding gerando primeiro usuário e token — `TestOnboardingCreatesTheAdministratorAndOneToken`
- [x] Tokens hasheados, com expiração e rotação — `TestTheDiskNeverHoldsTheToken`, `TestRotationKeepsTheOldTokenWorkingDuringTheGrace`
- [x] Política de senha aplicada — `TestAPasswordOfElevenIsRejected`, `TestABreachedPasswordIsRejectedWhateverItsLength`
- [x] Auth fora do alcance do agente — não existe grupo de comando de auth, logo não existe tool

## Saída dos testes — Fase 4

```
$ go test -race ./internal/domain/auth/ ./internal/adapters/fsauth/
ok  	github.com/OWNER/aos/internal/domain/auth
ok  	github.com/OWNER/aos/internal/adapters/fsauth
```

| Caso da nota | Teste |
|---|---|
| argon2id com os parâmetros especificados | `TestTheHashCarriesItsParameters` |
| Senha de 11 rejeitada; senha vazada rejeitada | `TestAPasswordOfElevenIsRejected`, `TestABreachedPasswordIsRejectedWhateverItsLength` |
| Token mostrado uma vez, disco guarda hash | `TestTheDiskNeverHoldsTheToken` |
| Token expirado rejeitado; rotação com graça | `TestAnExpiredTokenIsRefused`, `TestRotationKeepsTheOldTokenWorkingDuringTheGrace` |
| Último `super` não pode ser removido nem rebaixado | `TestTheLastAdministratorCannotBeRemovedOrDemoted` |
| Nenhuma tool MCP de auth | Não há `Register`; o registry não tem grupo `auth` |
| Permissão `0600` no `users.json` | `TestThePermissionIsRestrictive` |

**Não verificado:**

- **Teste estatístico de timing** do `Authenticate`. O caminho de comparação é `subtle.ConstantTimeCompare` e o login faz o hash da isca mesmo sem conta, mas medir isso de forma não-flaky exige mais cuidado do que um `t.Run`, e um teste de timing instável é pior que nenhum.
- **A lista de vazamentos é uma lista inicial**, não o top-100k de um corpus real: são as credenciais que aparecem primeiro em toda wordlist, mais as que as pessoas escolhem quando a política pede doze caracteres. Trocar o arquivo é passo de build, não mudança de código.
- **Comandos de auth** (`aos auth login`, `token issue`) — o serviço existe e nada o expõe ainda. Deliberado nesta fase: a superfície de auth precisa decidir o que é local e o que é remoto, e isso pertence à mesma leva que a árvore completa da CLI.
