---
tags: [adr, decisao, hooks, aprovacao]
aliases: [ADR-0007, Tool Approval, ask]
fase: 5
status: especificado
origem: "[[Eventos e Hooks]]"
---

# ADR-0007 — Canal real de aprovação de tool

> Pai: [[AOS]] · Origem no original: [[Eventos e Hooks]] · Detalhe técnico: [[Agent Loop]] · Fase: 5

## Contexto

O original implementa o contrato de hooks do Claude Code, incluindo `permissionDecision: "allow" | "deny" | "ask"` em `PreToolUse`. Mas:

```ts
if (preToolHook.permissionDecision === "ask")
  return { type: "denied", reason: "Interactive tool approval is not enabled in the Fractal agent runtime." };
```

**`ask` degrada silenciosamente para `deny`.** Um hook que pede confirmação humana bloqueia a chamada.

O efeito prático é perverso: um autor de hook, querendo cautela ("pergunte antes de fazer push"), obtém um sistema que nunca faz push e nunca explica direito por quê. O comportamento seguro e o comportamento útil ficam mutuamente exclusivos — o autor é empurrado a escolher `allow` para que a ferramenta funcione.

Isso conflita diretamente com a matriz de autonomia do [[System Prompt (BASE)]], que exige confirmação humana para ação irreversível ou externa. O prompt manda perguntar; o runtime não sabe perguntar.

## Decisão

Implementar um **canal real de aprovação**, com um port no domínio e três adaptadores por superfície.

```go
// internal/domain/event/port.go
type Approver interface {
	// RequestApproval blocks until a human decides, the context is cancelled,
	// or the deadline elapses. Implementations MUST NOT return Approved on timeout.
	RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResult, error)
}

type ApprovalRequest struct {
	SessionID string
	AgentID   string
	ToolName  string
	Input     json.RawMessage
	Reason    string        // texto do hook
	Risk      RiskLevel     // low | medium | high — deriva das annotations da tool
	Deadline  time.Duration
}

type ApprovalResult struct {
	Approved     bool
	Reason       string
	UpdatedInput json.RawMessage // o humano pode corrigir o payload antes de aprovar
	Remember     RememberScope   // none | session | always — persiste em política
}
```

| Superfície | Adaptador | Comportamento |
|---|---|---|
| Desktop | `wailsvc` + [[Realtime WebSocket]] | Modal com nome da tool, payload formatado, motivo do hook e os botões Aprovar / Aprovar sempre / Negar |
| CLI interativo (TTY) | `clix/approve` | Prompt no terminal com o mesmo conteúdo |
| Headless (rotina, task autônoma, MCP sem UI) | `NoopApprover` | Nega, **com motivo explícito e distinto**: `AOS_TOOL_APPROVAL_UNAVAILABLE` |

**Regra de timeout:** o default é 120 s; ao expirar, o resultado é **negação**, nunca aprovação. Fail-closed.

**`Remember: always`** grava a decisão na política de sandbox do agente ([[ADR-0006 Allowlist no sandbox]]) — um humano aprovando `gh pr create` uma vez pode promovê-lo à allowlist, e o registro fica em arquivo versionado, não em memória de sessão.

**Ressalva do prompt-mestre, preservada:** *"A user approving an action once does NOT authorize it in all contexts."* Por isso `Remember: session` é o default sugerido na UI, e `always` exige um segundo clique.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Manter `ask` → `deny`** | Reproduz o defeito #8 e torna a matriz de autonomia do prompt inexequível. |
| **`ask` → `allow`** | Inverte o problema e é perigoso: transforma um pedido de cautela em permissão. |
| **Aprovação só no desktop** | Deixa o CLI — a superfície principal para desenvolvedores — sem o recurso. |
| **Fila de aprovações assíncrona** (a chamada retorna "pendente", o humano decide depois, a tool executa em background) | Elegante para rotinas, mas quebra o modelo mental do loop: o agente precisa do resultado para continuar raciocinando. **Adiado** para modo rotina, onde faria sentido de fato. Registrado como evolução em [[Routine (Go)]]. |

## Consequências

**Positivas**
- Ações irreversíveis podem ser genuinamente confirmadas, como o prompt-mestre exige.
- Hooks de skills de terceiros ganham um mecanismo honesto: `ask` significa perguntar.
- Um agente bloqueado por allowlist pode escalar para aprovação em vez de falhar — os dois mecanismos compõem.

**Negativas**
- **Um bloqueio de loop com humano no caminho.** Uma sessão headless com hook `ask` para 120 s antes de negar. Mitigação: em contexto sabidamente headless (`RunMode == Routine || RunMode == Task`), o `NoopApprover` nega **imediatamente**, sem esperar o deadline.
- **Superfície de UI nova** no desktop e no CLI, com estados (pendente, expirado, negado) que precisam ser testados.
- **Risco de fadiga de aprovação.** Aprovar tudo por reflexo anula o mecanismo. Mitigação: `Risk` deriva das annotations da tool (`destructiveHint`, `openWorldHint`), e só `high` interrompe por padrão — `low` é auto-aprovado com registro em [[Activity (Go)]].

## Status

**Aceito.** Corrige o defeito #8 e é uma das divergências deliberadas do original.
