---
tags: [transporte, websocket, realtime]
aliases: [Realtime, WebSocket Go]
fase: 4
status: especificado
origem: "[[Realtime]]"
---

# Realtime WebSocket

> Pai: [[AOS]] · Origem no original: [[Realtime]] · Fase: 4

## Objetivo

Canal de *server-push* para a UI: eventos de workspace, streaming de resposta do agente e pedidos de aprovação de tool.

## Comportamento do original

Unidirecional por desenho ([[Realtime]]). O handler de `message` é **intencionalmente vazio** — o cliente nunca envia nada pelo socket; toda mutação vai por HTTP. Isso simplifica a segurança: não há superfície de comando no socket.

Um canal por workspace: `workspace::{id}`.

E o defeito #5:

```ts
const workspaceId = cookies.match(/x-workspace-id=([^;]+)/)?.[1] ?? "default";
server.upgrade(req, { data: { workspaceId } });
```

> O `workspaceId` vem direto do cookie, **sem checar se o cliente tem acesso**. Forjar o cookie dá acesso ao stream de eventos de qualquer workspace conhecido.

## Design em Go

```go
// internal/transport/realtime/upgrade.go

// Upgrade authorizes BEFORE accepting the socket. The original trusts the
// cookie; we verify that the authenticated user actually has access to the
// workspace it names. Fixes defect #5.
func Upgrade(ws workspace.Service, auth auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := identity.From(r.Context())
		if id.WorkspaceID == "" {
			http.Error(w, "workspace required", http.StatusBadRequest)
			return
		}
		if err := ws.AuthorizeAccess(r.Context(), id.WorkspaceID, id.UserID); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: allowedOrigins,
		})
		if err != nil {
			return
		}
		hub.Subscribe(r.Context(), c, channelFor(id.WorkspaceID))
	}
}
```

### O hub

```go
// internal/transport/realtime/hub.go

// Hub fans out events to subscribers of a workspace channel. Delivery is
// bounded: each subscriber has a buffered channel and a write deadline. A slow
// client is dropped, never allowed to block the publisher — the mutation that
// produced the event has already completed and must not wait on a browser.
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[*conn]struct{}
}

func (h *Hub) Publish(ctx context.Context, channel string, e Event)
```

### Tipos de evento

| Tipo | Origem | Conteúdo |
|---|---|---|
| `activity` | [[Activity (Go)]] | Novo registro de inbox |
| `chat.delta` | [[Agent Loop]] | Chunk de resposta em streaming |
| `chat.done` | [[Agent Loop]] | Fim do turno, com uso de tokens |
| `task.changed` | [[Task (Go)]] | Transição de status |
| `approval.request` | [[ADR-0007 Canal real de aprovação de tool]] | Pedido de aprovação de tool |
| `collection.changed` | [[Collections Engine]] | Watcher detectou mudança |

### Ainda unidirecional — com uma exceção

O cliente continua sem enviar comandos pelo socket. A **única** mensagem de cliente aceita é `pong`, para keepalive. Aprovações de tool voltam por HTTP (`POST /api/approvals/{id}`), não pelo socket — mantendo a propriedade de que o socket não é superfície de comando.

## Decisões e divergências

> [!decision] Autorização no upgrade
> Corrige o defeito #5.

> [!decision] `OriginPatterns` explícito
> `coder/websocket` verifica origem por padrão; declaramos a lista em vez de desabilitar. Sem isso, qualquer página aberta no navegador do usuário abre um socket para o daemon local.

> [!decision] Aprovação volta por HTTP
> Preserva o socket como canal unidirecional. Um socket que aceita decisões é um socket que precisa de autorização por mensagem.

> [!decision] Cliente lento é descartado
> O original entrega em sequência. Ver [[Padrões de Projeto Aplicados]], Observer.

## Testes

- Upgrade sem autorização ao workspace é rejeitado com 403
- Cookie forjado para outro workspace não recebe eventos
- Origem não listada é rejeitada
- Cliente lento é descartado sem bloquear o publicador (teste com escrita lenta)
- Reconexão recebe eventos a partir do momento da reconexão, sem replay
- Streaming de chat entrega chunks em ordem
- `approval.request` chega ao desktop e a decisão volta por HTTP
- Nenhuma mensagem de cliente além de `pong` é processada

## Critério de pronto

- [ ] Autorização no upgrade verificada por teste
- [ ] Seis tipos de evento entregues
- [ ] Cliente lento não afeta o publicador
- [ ] Socket permanece sem superfície de comando
