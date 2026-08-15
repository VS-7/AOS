---
tags: [transporte, http, chi, api]
aliases: [HTTP, API HTTP Go, chi]
fase: 4
status: especificado
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

## Critério de pronto

- [ ] Daemon servindo `/api` gerado do registry
- [ ] Middlewares de identidade, log, recuperação e auth
- [ ] Playground protegido
- [ ] Nenhuma credencial aceita por query string
