---
tags: [transporte, wails, desktop]
aliases: [Wails Services, Bindings Go]
fase: 7
status: especificado
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

- [ ] Services expostos e bindings gerados
- [ ] Superfície de sistema equivalente à do original, sem os defeitos
- [ ] Aprovação de tool funcionando no desktop
- [ ] Deep link e janela sem frame com material nativo
