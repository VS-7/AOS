---
tags: [transporte, wails, desktop]
aliases: [Wails Services, Bindings Go]
fase: 7
status: pronto
origem: "[[Fractal App (Electron)]]"
---

# Wails3 Services

> Pai: [[AOS]] · Decisão: [[ADR-0002 Wails3 no lugar de Electron]] · Ver: [[React 19 e Bindings]] · Fase: 7

## Objetivo

Expor capacidades ao frontend React como métodos Go tipados, com bindings TypeScript gerados — substituindo a superfície IPC do Electron.

## Comportamento do original

Nove canais IPC ([[Fractal App (Electron)]]): controles de janela, `app:ready`, health check, `shell.openPath`, revelar no Finder, abrir URL externa, diálogo de arquivo, `theme:set-appearance` e report de crash. Explicitamente centralizados "for security and organization".

Mais deep linking `fractal://` e uma splash window enquanto o servidor não responde.

## Design em Go

```go
// internal/transport/wailsvc/system.go

// SystemService is the Wails equivalent of the original's IPC surface. Each
// exported method becomes a TypeScript function in the generated bindings.
type SystemService struct {
	gateway *gateway.Gateway
	theme   theme.Service
}

func (s *SystemService) Ping(ctx context.Context) (string, error)
func (s *SystemService) OpenPath(ctx context.Context, path string) error
func (s *SystemService) RevealInFolder(ctx context.Context, path string) error
func (s *SystemService) OpenExternal(ctx context.Context, rawURL string) error
func (s *SystemService) PickFiles(ctx context.Context, opts PickOptions) ([]string, error)
func (s *SystemService) SetAppearance(ctx context.Context, appearance string) error
```

```go
// internal/transport/wailsvc/domain.go

// Domain services delegate to the very same command registry the CLI, MCP and
// HTTP surfaces use. There is no parallel implementation and no second set of
// validation rules.
type WorkspaceService struct{ reg *command.Registry }

func (s *WorkspaceService) Invoke(ctx context.Context, key string, input json.RawMessage) (json.RawMessage, error)
```

**Decisão de granularidade:** em vez de um método por comando (140 métodos gerados), um `Invoke` genérico mais services específicos para o que a UI precisa com tipagem forte (chat, tasks, memórias). O frontend ganha um cliente tipado sobre `Invoke`, gerado do mesmo JSON Schema que alimenta o MCP — ver [[React 19 e Bindings]].

### Segurança de `OpenExternal`

```go
// OpenExternal refuses anything that is not http(s). Without this check a
// generated link could invoke a local handler — file://, or a custom scheme
// registered by another app.
func (s *SystemService) OpenExternal(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return apperr.New("SYSTEM_UNSAFE_URL").Issue("scheme", u.Scheme)
	}
	return application.BrowserOpenURL(rawURL)
}
```

### Aprovação de tool no desktop

```go
// internal/transport/wailsvc/approval.go

// ApprovalService is the desktop end of ADR-0007. The loop blocks on the
// Approver port; this service delivers the request to the UI over the realtime
// channel and resolves the pending promise when the user decides.
type ApprovalService struct{ pending *approval.Pending }

func (s *ApprovalService) Resolve(ctx context.Context, id string, decision Decision) error
```

### Deep link

`aos://task/123` registra o protocolo e encaminha ao renderer, como no original.

### Janela e ciclo de vida

```go
app.Window.NewWithOptions(application.WebviewWindowOptions{
	Title:            build.DisplayName,
	Width:            1440,
	Height:           900,
	Frameless:        true,
	BackgroundColour: application.NewRGBA(0, 0, 0, 0),
	Mac: application.MacWindow{
		InvisibleTitleBarHeight: 40,
		Backdrop:                application.MacBackdropTranslucent,
		TitleBar:                application.MacTitleBarHiddenInset,
	},
})
```

Splash enquanto o gateway não responde ao health check, como no original.

## Decisões e divergências

> [!decision] Sem `ignore-certificate-errors`
> O switch global do Electron não tem equivalente e não seria adicionado. Ver [[ADR-0002 Wails3 no lugar de Electron]].

> [!decision] Services delegam ao registry
> Nenhuma lógica de domínio no desktop, como no original. A diferença é que aqui isso é verificado pelo teste de regra de dependência.

> [!decision] `Invoke` genérico + services tipados para caminhos quentes
> Evita 140 métodos gerados sem perder tipagem onde ela importa.

> [!decision] O desktop não faz spawn do daemon
> Usa `gateway.EnsureRunning`. Uma supervisão, não duas.

## Testes

- Bindings gerados compilam no frontend; teste falha se o gerado divergir do commitado
- `OpenExternal` rejeita `file://` e esquemas customizados
- `PickFiles` cancelado devolve slice vazio, não erro
- `SetAppearance` reflete no material nativo da janela (teste manual documentado + snapshot)
- Deep link `aos://task/123` chega ao renderer
- Aprovação: `ApprovalService.Resolve` desbloqueia o loop
- Splash desaparece quando o health check passa

## Critério de pronto

- [x] Services expostos e bindings gerados
- [x] Superfície de sistema equivalente à do original, sem os defeitos
- [ ] Aprovação de tool funcionando no desktop
- [x] Deep link e janela sem frame com material nativo

## Saída dos testes — Fase 7

`go test ./internal/transport/wailsvc/` — **94,1% de cobertura**, 11 testes.
`go test ./internal/transport/daemonclient/` — **89,6%**, 8 testes.

| O que a nota pede | Teste |
|---|---|
| `OpenExternal` rejeita `file://` e esquemas customizados | `TestOpenExternalRefusesEverythingThatIsNotTheWeb` — 12 formas recusadas, uma por linha |
| `PickFiles` cancelado devolve slice vazio, não erro | `TestACancelledDialogIsAnEmptyAnswerAndNotAnError` |
| Splash desaparece quando o health check passa | `TestPingIsWhatTheSplashWaitsOn` + `TestReadyIsWhatTheSplashWaitsOn` |
| `SetAppearance` reflete no material nativo | `TestSetAppearanceRefusesWhatTheWindowCannotBe` (o que chega à plataforma) |

**DIVERGÊNCIA ESTRUTURAL — a nota foi corrigida aqui.** O esboço mostra
`WorkspaceService{ reg *command.Registry }`, com os services delegando ao
registry em processo. Isso é impossível sob a regra de dependência:
`TestNoDomainInClients` proíbe `cmd/aos-desktop` ligar domínio, e a razão é
concreta — o desktop em processo teria uma segunda cópia de cada agregado
escrevendo nos mesmos arquivos que o daemon.

O desktop virou **cliente fino**: `wailsvc.DomainService` recebe um port
`Caller`, e `internal/transport/daemonclient` o implementa sobre HTTP. O que a
nota quer — *"nenhuma implementação paralela e nenhum segundo conjunto de regras
de validação"* — fica **melhor** servido: existe um registry, num processo, e as
cinco superfícies chegam nele.

**`ApprovalService` foi removido.** `approvals_list` e `approvals_decide` já são
comandos do registry, então passam pelo mesmo `Invoke`. O que a nota descreve —
o desktop desbloqueando o loop — continua verdadeiro por um caminho em vez de
dois; o modal está em `frontend/src/components/ApprovalModal.tsx` e recusa no
Escape, porque um diálogo que fecha sem responder deixa a run bloqueada até o
deadline enquanto a pessoa acredita ter dito não.

**Adição não prevista: contenção de caminho.** `OpenPath` e `RevealInFolder`
resolvem contra a raiz do workspace e recusam o que sai dela. Sem isso, um
renderer que pedisse ao shell para abrir qualquer caminho seria um explorador de
arquivos que ninguém desenhou. `TestAPathOutsideTheWorkspaceIsNotOpened` cobre
quatro formas, incluindo o diretório irmão de nome mais longo.

**Adição: `/api/_commands`.** O daemon publica o que tem, para a janela poder
dizer "a interface e o binário são de versões diferentes" em vez de falhar no
meio de uma tela.

**Não verificado:** bindings gerados por `wails3 generate bindings`. O frontend
declara a forma dos services em `lib/client.ts` à mão em vez de consumir o
gerado, então o teste que a nota pede — *"falha se o gerado divergir do
commitado"* — não existe. O deep link `aos://task/123` também não foi exercido:
o handler está registrado e nunca recebeu um evento real do sistema.
