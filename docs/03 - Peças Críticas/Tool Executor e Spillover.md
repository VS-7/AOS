---
tags: [critico, tools, spillover, contexto]
aliases: [Tool Executor, Spillover, Truncagem]
fase: 5
status: especificado
origem: "[[Tool Executor]]"
---

# Tool Executor e Spillover ★

> Pai: [[AOS]] · Origem no original: [[Tool Executor]] · Fase: 5

## Objetivo

Resolver o gargalo prático mais importante do sistema: **saídas de ferramenta queimam a janela de contexto**. Um `Grep` num repositório grande devolve 500 KB; injetar isso consome a janela inteira numa chamada.

## Comportamento do original

`FractalAgentToolExecutorService` (830 linhas) embrulha **todas** as tools e aplica spillover com ponteiro ([[Tool Executor]]):

```
Saída da tool (500 KB)
   ├─→ 12.000 primeiros chars  → contexto do modelo
   └─→ arquivo completo         → ~/.fractal/tmp/outputs/{toolCallId}.txt
                                  + instrução de como ler o resto
```

Constantes verificadas no fonte:

| Constante | Valor |
|---|---|
| `_maxToolOutputChars` | 12.000 |
| `_spillThresholdChars` | 12.000 |
| `_outputTtlMs` | 24 h |
| `_maxBase64DataLength` | 1.024 |
| `_safeJsonStringifyLimit` | 2.000.000 |

Detalhes de implementação que herdamos:

- **Truncagem preserva par surrogate UTF-16** — não corta no meio de um caractere.
- **Spillover é best-effort:** falha de I/O devolve a saída truncada, nunca quebra a chamada.
- **Rotação no boot:** arquivos com mais de 24 h são removidos.
- **Payloads base64 acima de 1 KB são truncados** — evita que uma imagem inline devore o contexto.
- **Conteúdo multimodal legítimo passa intacto** (`file-data` do SDK, data URLs de imagem).
- **O sandbox permite leitura, nunca escrita**, do diretório de spillover.

A instrução devolvida ao modelo, literal:

> The full output of the previous tool was truncated to fit the agent context. The complete output was saved to `{path}`. Use the `Read` tool with `offset` and `limit` parameters to inspect the relevant slice (e.g. `Read({ file_path: "...", offset: 1, limit: 200 })`), or move the file to a more convenient location with `Bash` if you need to keep it for later.

## Design em Go

### Constantes

```go
// internal/runtime/toolexec/limits.go
package toolexec

const (
	MaxToolOutputChars = 12_000
	SpillThreshold     = MaxToolOutputChars
	OutputTTL          = 24 * time.Hour
	MaxBase64Len       = 1_024
	SafeJSONLimit      = 2_000_000
)
```

### O decorador

Toda tool é embrulhada. É o padrão Decorator de [[Padrões de Projeto Aplicados]]:

```go
// internal/runtime/toolexec/decorator.go

type Tool interface {
	Name() string
	Spec() ToolSpec
	Invoke(ctx context.Context, in json.RawMessage) (any, error)
}

// Wrap composes the cross-cutting concerns in a fixed order:
//   metrics → hooks → truncation+spillover → the bare tool
// Approval happens earlier, in the loop, because a denied call must never
// reach the tool at all.
func Wrap(t Tool, opts ...Option) Tool
```

### Truncagem e spillover

```go
// internal/runtime/toolexec/spill.go

type Output struct {
	Data      any        `json:"data"`
	Output    string     `json:"output,omitempty"`    // spillover path
	Truncated *Truncated `json:"truncated,omitempty"`
}

type Truncated struct {
	Original    int    `json:"original"`
	Visible     int    `json:"visible"`
	Output      string `json:"output"`
	Instruction string `json:"instruction"`
}

func (d *spilling) process(ctx context.Context, callID string, v any) (Output, error) {
	s, repr := serialize(v) // strings as-is; structs via JSON with cycle guard
	if len(s) <= MaxToolOutputChars {
		return Output{Data: repr}, nil // small output keeps its structure
	}

	visible := truncateRunes(s, MaxToolOutputChars)

	path, err := d.persist(callID, s)
	if err != nil {
		// Best-effort: on a read-only filesystem the model still gets something
		// bounded rather than an error.
		return Output{Data: visible}, nil
	}
	return Output{
		Data:   visible,
		Output: path,
		Truncated: &Truncated{
			Original: len(s), Visible: len(visible), Output: path,
			Instruction: spilloverInstruction(path),
		},
	}, nil
}

// truncateRunes cuts on a rune boundary. Go strings are UTF-8, so the original's
// UTF-16 surrogate handling becomes rune-boundary handling — same intent,
// simpler mechanics.
func truncateRunes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}
```

**Saída pequena preserva a estrutura.** Só quando trunca é que vira string. Isso importa: uma lista de 5 tasks chega ao modelo como array, não como JSON serializado dentro de uma string.

### Escrita e rotação

```go
// persist writes the full output under ~/.aos/tmp/outputs/{callID}.txt.
// The file name is derived from the tool call id so retrieval is deterministic
// from the model's point of view.
func (d *spilling) persist(callID, content string) (string, error)

// Rotate removes spillover files older than OutputTTL. Runs at boot and hourly.
func Rotate(ctx context.Context, dir string, ttl time.Duration) (removed int, err error)
```

O `callID` é sanitizado antes de virar nome de arquivo — um id malformado não escapa do diretório.

### Dados binários

```go
// internal/runtime/toolexec/multimodal.go

// passthrough detects payloads that must reach the model untouched so the
// provider renders them as image/audio/video: SDK file parts and data: URLs.
// Everything else that looks like base64 above MaxBase64Len is truncated,
// so an inline image never eats the context window by accident.
func passthrough(v any) (any, bool)
```

### Tools nativas

Registradas fora do sistema de comandos, com os **nomes idênticos aos do Claude Code** ([[ADR-0016 Compatibilidade de nomes com o original]]):

| Toolset | Tools |
|---|---|
| `fs` | `Read`, `View`, `Write`, `Edit`, `Glob`, `Grep`, `Bash`, `Listen`, `Imagine` |
| `web` | `WebSearch`, `WebFetch` |
| `jobs` | `JobList`, `JobOutput`, `JobStop`, `JobWait` |
| `agents` | `CreateAgent`, `GetAgent`, `ListAgents`, `UpdateAgent`, `DeleteAgent` |

Todas passam pelo [[Sandbox (Go)]]. Cada uma recebe **apenas a interface de que precisa**:

```go
// internal/runtime/toolexec/tools/fs.go

// Read only needs to read. It cannot write or execute because it does not
// hold those interfaces — the restriction is in the type, not in a check.
func NewRead(fr sandbox.FileReader) Tool
func NewWrite(fw sandbox.FileWriter) Tool
func NewBash(cr sandbox.CommandRunner) Tool
func NewGlob(g sandbox.Globber) Tool
```

**`WebFetch`** usa `go-shiori/go-readability` + conversão para Markdown, e o resultado entra no prompt como `trust="unverified"` ([[Prompt Assembly]]) — conteúdo externo nunca é autoridade.

**`jobs.toolset`** permite disparar um comando longo e continuar raciocinando, colhendo o resultado depois. É o que torna build e teste viáveis dentro de um turno.

## Decisões e divergências

> [!decision] Saída pequena preserva estrutura
> O original serializa para truncar e devolve string. Nós só serializamos quando a truncagem acontece. Menos ruído para o modelo em 90% das chamadas.

> [!decision] Fronteira de rune, não de surrogate
> Go usa UTF-8; a preocupação do original com pares surrogate UTF-16 vira preocupação com fronteira de rune. Mesma intenção, mecânica mais simples.

> [!decision] Rotação periódica, não só no boot
> O original rotaciona na inicialização do executor. Um daemon que roda por semanas acumularia. Rotacionamos no boot **e** de hora em hora.

> [!decision] Interfaces segregadas por tool
> Divergência de ISP em relação ao original, que passa o sandbox inteiro a todas as tools. Ver [[SOLID no Go]].

> [!decision] Browser toolset continua desabilitado
> O original tem um `AgentBrowserService` de 488 linhas implementado e **não exposto** (`// export * from "./browser.toolset";`). Seguimos a mesma decisão: automação de navegador não entra na Fase 5. Registrado como escopo futuro.

## Testes

- **Truncagem:** saída de 500 KB → 12.000 chars visíveis + arquivo com o conteúdo íntegro; `Truncated.Original` bate com o tamanho real.
- **Fronteira de rune:** saída com emoji e caracteres CJK exatamente no limite não produz string inválida (`utf8.ValidString`).
- **Estrutura preservada:** saída de 100 bytes chega como `map`/`slice`, não como string.
- **Spillover best-effort:** diretório read-only → saída truncada devolvida, sem erro.
- **Rotação:** arquivo com 25 h é removido; com 23 h, permanece.
- **Sanitização de nome:** `callID` com `../` não escapa do diretório de saídas.
- **Base64:** data URL de 2 KB é truncada; `file-data` do SDK passa intacto.
- **Ciclo:** estrutura com referência circular não trava a serialização (guarda de ciclo + `SafeJSONLimit`).
- **Golden da instrução:** o texto devolvido ao modelo é comparado com `testdata/toolexec/instruction.golden`.
- **Sandbox:** `Write` no diretório de spillover é negado; `Read` é permitido.

## Critério de pronto

- [ ] Spillover funcionando com TTL e rotação periódica
- [ ] Truncagem segura em UTF-8 verificada
- [ ] Toda tool registrada passa por `Wrap`
- [ ] Tools nativas com os nomes do Claude Code implementadas
- [ ] Cada tool recebe apenas a interface de sandbox de que precisa
- [ ] Golden da instrução de spillover estável
