---
tags: [transporte, http, chi, api]
aliases: [HTTP, API HTTP Go, chi]
fase: 4
status: pronto
origem: "[[API HTTP]]"
---

# HTTP chi

> Pai: [[AOS]] · Origem no original: [[API HTTP]] · Fase: 4

## Objetivo

Expor o registry de comandos como API HTTP, com middlewares de identidade, log e correlação — sem que o transporte contenha regra de negócio.

## Comportamento do original

26 controllers agregados num router, com `query` (GET) e `mutation` (POST/PUT/DELETE), procedures como middleware e schemas Zod validados automaticamente ([[API HTTP]]).

Duas coisas boas que herdamos: `X-Request-ID` em toda resposta, e o envelope `ResponseWithCTA` que sugere o próximo passo.

Duas que corrigimos: playground sem autenticação (defeito #3) e token aceito em query string (defeito #4).

## Design em Go

```go
// internal/transport/httpapi/server.go

func New(reg *command.Registry, ws workspace.Service, cfg Config) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(requestID)        // X-Request-ID on every response
	r.Use(recoverer)        // a panic degrades one request, never the daemon
	r.Use(accessLog)        // METHOD /route STATUS - Xms
	r.Use(corsPolicy(cfg))
	r.Use(ambientIdentity)  // workspace, agent, user into context
	r.Use(authenticate(cfg))

	r.Route("/api", func(r chi.Router) {
		command.Mount(r, reg)          // POST /api/{group}/{name}
		r.Get("/health", healthHandler)
		if cfg.DocsEnabled {
			r.With(requireAuth).Mount("/docs", docsHandler(reg))
		}
	})

	r.Mount("/mcp", mcpserver.HTTPHandler(reg))
	r.Get("/ws", realtime.Upgrade(ws))
	r.Mount("/v", artifacts.Handler(cfg))
	r.Mount("/", spaHandler(cfg))

	return &Server{r: r, cfg: cfg}
}
```

### Identidade ambiente

```go
// ambientIdentity extracts workspace, agent and token, then puts them in the
// context. Unlike the original, query string is NOT a source for the token —
// only headers and cookies. See ADR-0009.
func ambientIdentity(next http.Handler) http.Handler
```

| Campo | Header | Cookie | Query |
|---|---|---|---|
| workspace | `X-Workspace-ID` | `x-workspace-id` | `?workspace=` (permitido — não é segredo) |
| agent | `X-Agent-ID` | — | `?agent=` |
| token | `Authorization: Bearer`, `X-Auth-Token` | `sessionToken` | **nunca** |

### Envelope

```go
// writeEnvelope renders the command result. Success carries optional CTAs —
// guidance for the agent's next step, exactly like the original's _cta.
func writeEnvelope(w http.ResponseWriter, out any, err error)
```

### Log de acesso

Formato herdado: `POST - /api/memories/store 200 - 12.34ms`. Nível por faixa de status. Corpo de streaming não é consumido. Em erro, o campo estruturado é logado com o `requestID`.

### Recuperação

```go
// recoverer turns a panic into a 500 with the request id, logs the stack, and
// keeps the daemon serving. Fixes defect #16. See Concorrência e Context.
func recoverer(next http.Handler) http.Handler
```

## Decisões e divergências

> [!decision] Rotas geradas do registry, não escritas à mão
> `command.Mount` percorre o registry. Adicionar um comando adiciona a rota. Não existem 26 controllers para manter em sincronia ([[Command Layer]]).

> [!decision] Playground autenticado e condicionado ao ambiente
> Corrige o defeito #3.

> [!decision] Token nunca em query
> Corrige o defeito #4. `?workspace=` e `?agent=` continuam, porque não são credenciais.

> [!decision] Sem `development: true`
> O original tem a flag fixa no binário de produção (defeito #12). Aqui, comportamento de desenvolvimento vem de `AOS_ENV`, e o default é produção.

## Testes

- Toda rota do registry responde; nenhuma rota órfã
- `X-Request-ID` presente em toda resposta, inclusive erro
- Panic em handler → 500 com id, servidor continua
- Token em query é ignorado; em header, aceita
- `/api/docs` sem token → 401; com `AOS_ENV=production` → 404
- CORS: origem não listada é rejeitada
- Streaming não tem o corpo consumido pelo logger
- Golden do envelope de sucesso e de erro

> [!decision] `command.Mount` virou `mountCommands`, dentro do transporte
> O esboço desta nota põe o montador em `internal/core/command`. Isso faria a Command Layer conhecer o chi, e a razão de ela existir é não conhecer transporte nenhum. O laço sobre o registry mora aqui.

> [!decision] Tudo é POST, inclusive leitura
> Uma leitura aqui carrega o mesmo payload JSON que carrega em toda outra superfície, e um GET com corpo é algo que proxy tem direito de descartar.

> [!decision] `_reasoning` não é exigido em HTTP
> É obrigação de superfície de tool: existe para o modelo dizer por que está chamando, e vale no MCP e no registry do agente. HTTP é o que o terminal e o desktop falam. Travado por `TestReasoningIsNotRequiredOverHTTP`.

> [!decision] O playground continua fechado mesmo com `security.enabled: false`
> Desligar a autenticação por conveniência num daemon de loopback não é decisão de publicar a superfície inteira. `TestTheDocsRouteStaysGuardedEvenWithSecurityOff`.

## Critério de pronto

- [x] Daemon servindo `/api` gerado do registry — `TestEveryCommandOfTheRegistryHasARoute`
- [x] Middlewares de identidade, log, recuperação e auth
- [x] Playground protegido — `TestTheDocsRouteIsAuthenticatedAndOptional`
- [x] Nenhuma credencial aceita por query string — `TestATokenInTheQueryStringIsIgnored`

## Saída dos testes — Fase 4

```
$ go test -race ./internal/transport/httpapi/
ok  	github.com/OWNER/aos/internal/transport/httpapi
```

| Caso da nota | Teste |
|---|---|
| Toda rota do registry responde | `TestEveryCommandOfTheRegistryHasARoute` |
| `X-Request-ID` em toda resposta, inclusive erro | `TestTheRequestIdIsOnEveryResponse` |
| Panic em handler → 500, servidor continua | `TestAPanicDegradesOneRequest` |
| Token em query ignorado; em header, aceito | `TestATokenInTheQueryStringIsIgnored`, `TestTheTokenIsAcceptedFromHeadersAndCookie` |
| `/api/docs` sem token → 401; desabilitado → 404 | `TestTheDocsRouteIsAuthenticatedAndOptional` |
| CORS: origem não listada é rejeitada | `TestCorsAllowsOnlyTheDeclaredOrigins`, `TestNoOriginIsAllowedByDefault` |
| Streaming não tem o corpo consumido pelo logger | O logger só observa status e bytes; nunca lê o corpo |
| Alias deprecado responde no caminho antigo | `TestARenamedCommandKeepsAnsweringAtItsOldPath` |

**Não verificado:** golden do envelope. As asserções hoje são sobre forma e status; o golden entra junto com o do CLI, quando houver saída estável que o justifique.

**Resolvido:** `/mcp` sobre HTTP — `internal/transport/mcpserver.NewHTTPHandler` envolve o
mesmo `*mcp.Server` de `ServeStdio` (`mcp.NewStreamableHTTPHandler` do SDK), e
`app.Serve` liga `Config.MCP` a ele. Fica atrás do mesmo middleware de bearer
token do grupo `/api` guardado — não se autentica sozinho como `/ws`, porque
alcança exatamente o mesmo registry — e segue o mesmo `SecurityEnabled` desse
grupo (não é uma segunda porta mais estrita que `/api/docs`). Ver
`TestMCPOverHTTPRoundTripsThroughTheRunningDaemon` e
`TestMCPOverHTTPRequiresAuthenticationOnTheRunningDaemon`
(`internal/app/daemon_test.go`).

**Pendente:** `/v/*` é da **Fase 8**.
