---
tags: [dominio, model, provider, llm]
aliases: [Model Providers Go, Adaptadores de Modelo]
fase: 5
status: especificado
origem: "[[Model (Providers)]]"
---

# Model Providers (Go)

> Pai: [[Config (Go)]] · Origem no original: [[Model (Providers)]] · Ver: [[Agent Loop]] · Fase: 5

## Objetivo

Camada de abstração sobre provedores de LLM, com uma interface única e adaptadores por provider.

## Comportamento do original

Oito adaptadores ([[Model (Providers)]]), cada um com `id`, `name`, `init(model)` e, opcionalmente, `speech()`, `realtime()`, `image()`, `video()`.

Os dois mais interessantes são os de **OAuth local**:

| `id` | Como autentica |
|---|---|
| `codex` | `~/.codex/auth.json` — tokens OAuth do ChatGPT/Codex, renovados quando necessário |
| `gemini-cli` | `~/.gemini/oauth_creds.json` — credenciais do Gemini CLI instalado |

A implicação, registrada na engenharia reversa:

> Esses dois adaptadores permitem usar a **assinatura já paga** do ChatGPT Plus/Pro ou do Google, em vez de pagar por token de API. O sistema lê arquivos de credencial de terceiros no home do usuário. Legítimo e documentado, mas vale saber.

Era exatamente a configuração observada na máquina: `provider: codex, model: gpt-5.5`.

Verificação em duas etapas para capabilities, com códigos distintos: `_MODEL_MISSING` ("você não configurou") ≠ `_NOT_SUPPORTED` ("esse provider não faz isso").

## Design em Go

```go
// internal/runtime/providers/registry.go

type Factory func(cfg ProviderConfig) (agentloop.LLMProvider, error)

var registry = map[string]Factory{}

// Register wires a provider id to its factory. Adapters call this from init(),
// so adding a provider never touches the core (OCP).
func Register(id string, f Factory)
```

```go
// internal/runtime/agentloop/provider.go

type LLMProvider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (Stream, error)
}

// Optional capabilities. A provider implements only what it supports, and the
// two-step check distinguishes "not configured" from "not supported".
type SpeechProvider interface {
	Speech(ctx context.Context, in SpeechInput) (Audio, error)
}
type ImageProvider interface {
	Image(ctx context.Context, in ImageInput) (Image, error)
}
type RealtimeProvider interface {
	RealtimeToken(ctx context.Context) (string, error)
}
```

### Providers

| `id` | SDK | Nota |
|---|---|---|
| `openai` | `github.com/openai/openai-go` | `Store: false`, reasoning encriptado |
| `anthropic` | `github.com/anthropics/anthropic-sdk-go` | |
| `google` | `google.golang.org/genai` | thinking config |
| `openrouter` | `openai-go` com `baseURL` | |
| `crof` | `openai-go` com `baseURL` | |
| `opencode` | `openai-go` com `baseURL` | roteamento por sufixo `-free` |
| `codex` | `openai-go` + OAuth local | lê `~/.codex/auth.json` |
| `gemini-cli` | `genai` + OAuth local | lê `~/.gemini/oauth_creds.json` |

### Adaptadores OAuth

```go
// internal/runtime/providers/oauthfile/oauthfile.go

// Store reads and refreshes OAuth credentials written by another CLI on this
// machine. It never writes to the third-party file except to persist a
// refreshed token, and it fails loudly when the file is absent instead of
// silently falling back to an API key.
type Store struct {
	Path    string // ~/.codex/auth.json
	Refresh RefreshFunc
	mu      sync.Mutex
}

func (s *Store) Token(ctx context.Context) (string, error)
```

O refresh é serializado por mutex e por lock de arquivo: dois processos (`aosd` e um `aos --mcp`) renovando ao mesmo tempo corromperiam o arquivo de credencial de outra ferramenta — o que seria um dano fora do nosso sistema.

### Verificação em duas etapas

```go
// capability resolves a modality slot and reports the two failure modes with
// distinct codes, exactly as the original does.
func capability[T any](cfg Config, slot string, reg *Registry) (T, error) {
	ref := cfg.Agents.Models[slot]
	if ref.Provider == "" || ref.Model == "" {
		return zero[T](), errCapabilityModelMissing(slot) // you did not configure it
	}
	p, err := reg.Get(ref.Provider)
	if err != nil {
		return zero[T](), err
	}
	typed, ok := any(p).(T)
	if !ok {
		return zero[T](), errCapabilityNotSupported(slot, ref.Provider) // it cannot do this
	}
	return typed, nil
}
```

### Custo

```go
// Pricing is a versioned table, embedded and updatable, used to compute the
// per-message cost recorded in Chat.Run. The original tracks tokens but not
// money, which leaves the user unable to see what a session costs.
//go:embed pricing.json
var pricingRaw []byte
```

## Decisões e divergências

> [!decision] Refresh de OAuth com lock entre processos
> O original renova o token sem coordenação. Com múltiplos processos, isso pode corromper o arquivo de credencial do Codex ou do Gemini CLI — dano em ferramenta de terceiro.

> [!decision] Falha explícita quando o arquivo OAuth some
> Sem fallback silencioso para chave de API: o usuário escolheu OAuth por uma razão (custo), e trocar sozinho para cobrança por token seria uma surpresa cara.

> [!decision] Reasoning por agente, com default global
> Divergência. No original, o nível de reasoning vem **sempre** da config global, nunca do agente. Um agente de revisão crítica e um de triagem não deveriam pensar igual. Mantemos o default global e permitimos override no frontmatter do agente.

> [!decision] Tabela de preços versionada
> Adição, para dar custo por mensagem em [[Chat (Go)]].

## Testes

- Contrato de `LLMProvider` roda contra os oito adaptadores mais o fake ([[Testes de Contrato de Port]])
- Registro por `init()` torna um provider fictício resolvível sem tocar no core
- OAuth: arquivo ausente falha com CTA; token expirado é renovado uma vez; dois processos renovando concorrentes não corrompem
- Capability: slot vazio → `_MODEL_MISSING`; provider sem a interface → `_NOT_SUPPORTED`
- `opencode` roteia `-free` para o endpoint alternativo
- Cálculo de custo bate com a tabela para cada modelo listado
- `Store: false` presente nas requisições OpenAI

## Critério de pronto

- [ ] Oito providers implementados, com suíte de contrato verde
- [ ] Adaptadores OAuth lendo e renovando com segurança entre processos
- [ ] Verificação de capability em duas etapas
- [ ] Custo por mensagem calculado
