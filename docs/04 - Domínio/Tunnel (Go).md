---
tags: [dominio, tunnel, rede, cloudflare]
aliases: [Tunnel Go, Cloudflare Tunnel]
fase: 8
status: pronto
origem: "[[Tunnel (feature)]]"
---

# Tunnel (Go)

> Pai: [[Config (Go)]] · Origem no original: [[Tunnel]] · [[Tunnel (feature)]] · Fase: 8

## Objetivo

Expor o daemon local na internet pública via Cloudflare Tunnel, usando o binário `cloudflared` do sistema.

## Comportamento do original

Duas razões concretas ([[Tunnel]]): webhooks do Telegram precisam de URL pública, e acesso remoto ao workspace.

Ciclo de vida: valida que `hostname` **e** `token` existem, faz spawn do `cloudflared`, aguarda readiness, só então persiste `enabled: true`, notifica em [[Activity (Go)]] e devolve a URL. O `stop` limpa o estado **sem apagar** hostname e token, para permitir religar sem reconfigurar.

E o risco, registrado com clareza ([[Tunnel]]):

> Ligar o tunnel publica na internet **todas** as rotas do servidor — incluindo `/api/docs` (playground sem auth) e `/api/*` completo. Se `security.enabled` estiver `false` (o padrão observado), a API fica acessível sem autenticação para qualquer pessoa que descubra o hostname.

Sem tools MCP — expor a máquina à internet é decisão humana.

## Design em Go

```go
// internal/domain/tunnel/entity.go

type Status string

const (
	Stopped Status = "stopped"
	Starting Status = "starting"
	Running  Status = "running"
	Failed   Status = "failed"
)

type State struct {
	Status    Status     `json:"status"`
	URL       string     `json:"url,omitempty"`
	PID       int        `json:"pid,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	Error     string     `json:"error,omitempty"`
}
```

```go
type Service interface {
	// Start refuses to expose an unauthenticated API. See ADR-0009.
	Start(ctx context.Context) (State, error)
	Stop(ctx context.Context) error
	Status(ctx context.Context) (State, error)
}
```

### O guard de exposição

```go
// internal/domain/tunnel/service.go

// Start publishes the local daemon on the public internet. Before doing that
// it verifies the two things that make the difference between "remote access"
// and "an open door":
//
//  1. authentication is enabled and an API token exists;
//  2. the docs playground is either disabled or behind that authentication.
//
// The original checks neither, which is how the reverse engineering found an
// unauthenticated API reachable through a tunnel.
func (s *service) Start(ctx context.Context) (State, error) {
	cfg, err := s.config.Raw(ctx)
	if err != nil {
		return State{}, err
	}
	if !cfg.Security.Enabled || cfg.Security.APIToken == "" {
		return State{}, apperr.New("TUNNEL_INSECURE_EXPOSURE").
			Causer("tunnel.Service.Start").
			Msg("refusing to expose an API without authentication").
			CTA(apperr.CallToAction{
				Label:   "enable authentication and issue an API token first",
				Command: "aos auth token issue --name tunnel",
			})
	}
	if cfg.Tunnel.Hostname == "" || cfg.Tunnel.Token == "" {
		return State{}, errConfigIncomplete()
	}
	return s.spawn(ctx, cfg.Tunnel)
}
```

### Supervisão do processo

```go
// spawn launches cloudflared, waits for readiness with a bounded timeout, and
// keeps a supervisor goroutine that restarts it with exponential backoff when
// it dies unexpectedly. The original spawns and forgets.
func (s *service) spawn(ctx context.Context, cfg config.Tunnel) (State, error)
```

`cloudflared` não é embutido — precisa estar no `PATH`, como no original. Ausente, o erro carrega instrução de instalação.

## Decisões e divergências

> [!decision] Recusa expor API não autenticada
> A mudança mais importante. Corrige a combinação de riscos #1, #2 e #9 da análise de segurança.

> [!decision] Supervisão com backoff
> O `cloudflared` cai; sem supervisão, o canal morre em silêncio e os webhooks do Telegram param sem aviso. Adição.

> [!decision] Continua sem tools MCP
> Herdado. Fronteira de privilégio deliberada, junto com `auth` e `events`.

> [!decision] Token do Cloudflare redigido em toda saída
> Vem de `config.Tunnel.Token`, marcado `secret:"true"` ([[ADR-0010 Segredos com permissão restrita]]).

## Testes

- `Start` com `security.enabled: false` é recusado com CTA
- `Start` sem hostname ou token é recusado, distinguindo os dois casos
- `cloudflared` ausente do PATH produz erro com instrução de instalação
- Readiness com timeout: processo que não sobe é limpo, estado volta a `stopped`
- Supervisor reinicia após queda, com backoff
- `Stop` preserva hostname e token no config
- Token nunca aparece em log nem em resposta de API

## Critério de pronto

- [x] Tunnel subindo e devolvendo URL pública — `TestStartSucceedsAndReportsURL`
- [x] Guard de exposição impedindo API aberta — `TestStartRefusesWhenAPIIsNotAuthenticated`, `TestStartRefusesWhenAPITokenMissingEvenIfEnabled`, `TestStartRefusesWhenHostnameOrTokenMissing_DistinctFromInsecureExposure`
- [x] Supervisão com reinício automático — `TestSupervisorRestartsAfterUnexpectedDeath`
- [x] Segredos redigidos em toda superfície — token marcado `secret:"true"` ([[ADR-0010 Segredos com permissão restrita]]); `TestStopTerminatesTheRunningProcessAndPreservesConfig`

Pendência conhecida, fora deste critério: `frontend/src/features/tunnel/` tem só 1 arquivo (interfaces, sem painel). O backend está completo e os três comandos (`tunnel_start`/`tunnel_stop`/`tunnel_status`) já estão em `command-map.ts`; falta só a superfície visual.
