---
tags: [dominio, gateway, processo, supervisao]
aliases: [Gateway Go, Supervisor]
fase: 4
status: especificado
origem: "[[Gateway]]"
---

# Gateway (Go)

> Pai: [[Visão Geral Go]] · Origem no original: [[Gateway]] · [[Gateway (feature)]] · Fase: 4

## Objetivo

Supervisionar o processo do daemon: iniciar, parar, reiniciar e reportar estado. É o **único** grupo de comando que roda localmente, sem passar pelo daemon — não faz sentido iniciar um processo remoto por HTTP.

## Comportamento do original

Estado em `~/.fractal/runtime/gateway/{pid, meta.json, gateway.log}` ([[Gateway]]).

Máquina de três estados, cruzando `meta.json` com a liveness real do processo:

| Estado | Condição |
|---|---|
| `stopped` | Não existe `meta.json` |
| `stale` | Existe, mas o PID não está vivo |
| `running` | Existe e o PID está vivo |

O estado `stale` é o que dá robustez a crashes: em vez de "acha que está rodando", detecta o órfão e limpa antes de subir de novo.

Resolução do binário em três estratégias em cascata: caminho explícito por env, binário irmão do executável atual, e modo desenvolvimento.

E o defeito #18:

> O gateway não usa lock de arquivo — apenas o par `pid` + liveness. Duas invocações simultâneas de `start()` podem, em teoria, ambas passar pela checagem e disparar dois `spawn`.

## Design em Go

```go
// internal/gateway/state.go

type Status string

const (
	Stopped Status = "stopped"
	Stale   Status = "stale"
	Running Status = "running"
)

type Meta struct {
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	StartedAt time.Time `json:"startedAt"`
	Version   string    `json:"version"`
	Command   string    `json:"command"`
	Args      []string  `json:"args"`
}

// readState crosses the persisted meta with actual process liveness. A stale
// entry is a crashed daemon, not a running one.
func readState(dir string) (Status, *Meta, error)
```

### Lock, primeiro

```go
// internal/gateway/gateway.go

// Start is idempotent and safe under concurrency. The lock is taken BEFORE the
// state is read, which is what closes the race the original leaves open: two
// simultaneous starts can otherwise both observe "stopped" and both spawn.
func (g *Gateway) Start(ctx context.Context) (Meta, error) {
	unlock, err := g.lock(ctx) // flock on gateway.lock, with timeout
	if err != nil {
		return Meta{}, err
	}
	defer unlock()

	st, meta, err := readState(g.dir)
	if err != nil {
		return Meta{}, err
	}
	switch st {
	case Running:
		return *meta, nil // idempotent
	case Stale:
		if err := g.cleanup(); err != nil {
			return Meta{}, err
		}
	}

	cmd, err := g.resolveCommand()
	if err != nil {
		return Meta{}, err
	}
	proc, err := g.spawnDetached(cmd)
	if err != nil {
		return Meta{}, err
	}
	if err := g.writeMeta(proc); err != nil {
		_ = proc.Kill()
		return Meta{}, err
	}
	return g.waitHealthy(ctx, proc)
}
```

### Resolução do binário

```go
// resolveCommand mirrors the original's cascade:
//   1. an explicit path from the environment (used by the desktop app)
//   2. a sibling binary next to the current executable
//   3. development mode: `go run ./cmd/aosd`
func (g *Gateway) resolveCommand() (Command, error)
```

### Espera por saúde, não por vida

```go
// waitHealthy polls /api/health instead of merely checking that the PID is
// alive. A process that started and is failing to bind the port is not
// "running" in any useful sense — the original waits 1.5s for liveness only.
func (g *Gateway) waitHealthy(ctx context.Context, p *os.Process) (Meta, error)
```

### Parada

`SIGTERM` → espera o `shutdownTimeout` → `SIGKILL` → limpa arquivos de runtime. Em Windows, `taskkill` equivalente.

## Decisões e divergências

> [!decision] Lockfile antes da leitura de estado
> Corrige o defeito #18.

> [!decision] Health check em vez de liveness
> O original espera 1,5 s pelo processo ficar vivo. Um daemon que subiu e não conseguiu abrir a porta passaria nesse teste. Esperamos pelo endpoint de saúde, com timeout maior e mensagem clara ao estourar.

> [!decision] Uma implementação de supervisão, usada por CLI e desktop
> No original, o Electron tem sua própria lógica de spawn e supervisão do servidor, separada do gateway. Duas implementações, dois comportamentos. Aqui, `aos-desktop` chama `gateway.EnsureRunning(ctx)` ([[ADR-0002 Wails3 no lugar de Electron]]).

> [!decision] `gateway_restart` continua disponível ao agente
> O original expõe e a engenharia reversa nota o efeito: o agente pode reiniciar o processo que está atendendo a própria chamada — a chamada não retorna. Mantemos, porque é útil após atualização de config, mas o comando avisa no `Doc` que a resposta não chegará, e o restart acontece **depois** de a resposta ser enviada.

## Testes

- Três estados detectados corretamente, inclusive `stale` com PID reciclado por outro processo (verifica `startedAt` além do PID)
- Dois `Start` concorrentes: um sobe, o outro devolve o mesmo meta; um único processo existe
- `Start` idempotente quando já rodando
- Daemon que sobe e falha ao bindar a porta é detectado pelo health check e limpo
- `Stop` com processo que ignora `SIGTERM` é escalado para `SIGKILL`
- Resolução de binário nas três estratégias
- Crash do daemon deixa estado `stale`; próximo `Start` limpa e sobe
- `restart` responde antes de derrubar o processo

## Critério de pronto

- [ ] `aos gateway start|stop|restart|status` operando o daemon
- [ ] Lockfile impedindo corrida
- [ ] Health check antes de declarar sucesso
- [ ] Desktop e CLI usando a mesma supervisão
