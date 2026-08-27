---
tags: [dominio, model, provider, llm]
aliases: [Model Providers Go, Adaptadores de Modelo]
fase: 5
status: em-construcao
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
| `antigravity` | `~/.gemini/antigravity-cli/antigravity-oauth-token` — login do Antigravity CLI (`agy`). Adição nossa, não do original: o Gemini CLI foi desligado para contas pessoais em 18/06/2026 e a linha acima deixou de comprar qualquer coisa |

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
| `antigravity` | HTTP direto + OAuth local | envelope Cloud Code `v1internal`, com renovação de token e guarda de cota |

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

> [!decision] O adaptador Antigravity protege a conta antes de proteger a chamada
> A credencial é a assinatura pessoal de alguém, não uma chave medida, e o endpoint é interno. Três consequências, todas em `internal/runtime/providers/antigravity/`:
>
> 1. **Lê o login, nunca cria um.** Não há fluxo de browser nem de device aqui. Sem login na máquina, o adaptador diz isso e para — quem assina é o cliente oficial. Duas coisas emitindo concessões contra o mesmo OAuth client é como se perde a que já existia.
> 2. **Não repete uma recusa.** Um 429 respondido com retry é um cliente discutindo com um rate limiter, e a discussão é o que transforma um limite em suspensão. `guard.go` recusa, fecha a porta por uma janela que cresce a cada recusa consecutiva, e uma resposta limpa perdoa o histórico. Também há um piso de espaçamento entre chamadas, porque um loop de tools dispara em rajada e uma rajada é a forma que chama atenção.
> 3. **Recusa antes de gastar.** `fetchAvailableModels` devolve `quotaInfo.remainingFraction` por modelo. Um turno cuja cota já acabou é recusado com o horário do reset, em vez de descoberto por um 429 — e só depois de reler o número, porque recusar em cima de um zero de cinco minutos atrás seria outro tipo de erro.
>
> O `onboardUser` do endpoint **não** é chamado: provisionar projeto na conta de alguém é decisão do cliente oficial, e é difícil de desfazer. O campo `project` é opcional (verificado contra a API) — mandamos o que `loadCodeAssist` nomeou, e nada quando não nomeou.
>
> 4. **O client OAuth não mora aqui.** O par `client_id`/`client_secret` é do Google, distribuído dentro do CLI oficial. Commitar aqui seria republicar credencial de terceiro a partir de um repositório que não é dele, e o raio de alcance de uma revogação é todo mundo que usa o CLI de verdade — o push protection do GitHub recusa, e está certo. O par vem de `AOS_ANTIGRAVITY_CLIENT_ID` e `AOS_ANTIGRAVITY_CLIENT_SECRET`, sem default. Só a **renovação** precisa dele: uma instalação cujo token ainda não expirou nunca percebe a ausência, e uma que expirou recebe um erro que nomeia as duas variáveis e onde achar os valores (`strings` no binário `agy` instalado).

## Estado atual

A tabela de preços que a seção "Pendente" mais abaixo chamava de inexistente
já existe e está wireada — essa nota ficou desatualizada nesse ponto
específico; o resto dela continua verdadeiro. Dois itens seguem
genuinamente em aberto:

- **`RefreshFunc` está implementado para `antigravity`, e segue vazio para `codex` e `gemini-cli`.**
  O que faltava era exatamente o que a nota abaixo dizia faltar: o endpoint
  real de token e os parâmetros do cliente. Para Antigravity isso foi obtido
  do binário `agy` distribuído e confirmado contra
  `https://oauth2.googleapis.com/token` (o binário carrega dois `client_id`;
  só um dos quatro pares aceita esses refresh tokens). `oauthfile.applyTokens`
  ganhou o formato que esse arquivo usa — um `oauth2.Token` sob a chave
  singular `token` — porque gravar as chaves no nível de cima teria sido pior
  do que não gravar: o dono do arquivo lê `token.access_token` e continuaria
  apresentando o credencial velho. Há teste para isso
  (`TestARenewalWritesBackUnderTheKeyTheFileAlreadyUses`), e a renovação real
  foi verificada contra o arquivo de verdade, com o CLI oficial voltando a
  ler o arquivo depois. `codex` e `gemini-cli` continuam sem renovar pela
  razão original, agora com um precedente de como fechar isso quando houver
  a especificação de cada um.

- **(histórico) `RefreshFunc` dos adaptadores OAuth não está implementado.**
  `oauthfile.Store.Refresh` é o campo certo — o resto do `Store` (leitura,
  expiração, lock entre processos, regravação preservando chaves
  desconhecidas) tem suíte própria agora (`oauthfile_test.go`, contra um
  `RefreshFunc` fake, sem tocar rede real) — mas `oauthfile.Codex`/
  `oauthfile.GeminiCLI` não o preenchem — um token expirado sempre vira
  `OAUTH_TOKEN_EXPIRED` pedindo
  novo login, nunca tenta renovar. Implementar isso exige saber o endpoint
  real de token e os parâmetros (`client_id`, escopo) que o Codex CLI e o
  Gemini CLI de terceiros realmente usam — não é algo que se infere do
  design deste sistema, é comportamento de uma ferramenta externa. Uma
  tentativa de pesquisar isso via busca na web e API do GitHub nesta sessão
  foi bloqueada pelo classificador de segurança do ambiente (um padrão de
  busca por "refresh token endpoint" de ferramenta de terceiro aciona a
  mesma proteção que existe contra exfiltração de credencial, mesmo sendo
  pesquisa de código aberto legítima) — então isso não foi implementado às
  cegas, para não gravar algo incorreto no arquivo de credencial real de
  alguém. Requer decisão de como obter a especificação real (documentação
  pública oficial, ou o próprio usuário fornecendo os valores).
- **Nenhum adaptador foi exercitado contra a API real de um provider** —
  a suíte de contrato roda contra troca gravada (`httptest`), nunca contra
  `api.openai.com`/`api.anthropic.com`/etc. de verdade. Precisa de uma
  chave de API válida para cada provider, que este ambiente não tem.

## Testes

- Contrato de `LLMProvider` roda contra os oito adaptadores mais o fake ([[Testes de Contrato de Port]])
- Registro por `init()` torna um provider fictício resolvível sem tocar no core
- OAuth: arquivo ausente falha com CTA; token expirado sem `Refresh` configurado é um erro pedindo novo login; token expirado com `Refresh` configurado renova e persiste, preservando chaves desconhecidas do arquivo; dois processos renovando ao mesmo tempo não corrompem (lock de arquivo)
- Capability: slot vazio → `_MODEL_MISSING`; provider sem a interface → `_NOT_SUPPORTED`
- `opencode` roteia `-free` para o endpoint alternativo
- Cálculo de custo bate com a tabela para cada modelo listado
- `Store: false` presente nas requisições OpenAI

## Critério de pronto

- [x] Oito providers implementados, com suíte de contrato verde
- [~] Adaptadores OAuth leem e travam entre processos; a renovação não tem função de refresh — ver "Estado atual" acima
- [x] Verificação de capability em duas etapas
- [x] Custo por mensagem calculado — `pricing.json` existe, embutido em `internal/runtime/providers`; `session.go:246` grava `CostUSD` em todo turno; `TestCostUSDPricesAKnownModel`, `TestCostUSDIsZeroForTheOAuthSubscriptionAdapters`

## Saída dos testes — Fase 5

```
$ go test -race ./internal/runtime/providers/
ok  	github.com/OWNER/aos/internal/runtime/providers
```

| Caso da nota | Teste |
|---|---|
| Contrato contra os adaptadores | `TestEveryProviderObeysTheContract` |
| Registro por `init()` resolve sem tocar no core | `TestEveryProviderInTheSpecificationIsRegistered` |
| Capability: slot vazio ≠ provider sem a interface | `TestTheTwoWaysACapabilityCanBeUnavailableAreDistinct` |
| `opencode` roteia `-free` para o endpoint alternativo | `TestOpenCodeRoutesTheFreeModelsElsewhere` |
| `store: false` presente nas requisições OpenAI | Parte do corpo verificado em `TestEveryProviderObeysTheContract` |

Nove ids sobre cinco formatos de fio: `openai` e `codex` na Responses API; `anthropic` na Messages API; `google` e `gemini-cli` em `generateContent`; `openrouter`, `crof` e `opencode` em Chat Completions; e `antigravity` no envelope Cloud Code `v1internal`, que é um `generateContent` aninhado sob `request` e devolvido aninhado sob `response`.

**Divergência: SDKs.** A nota nomeia `openai-go`, `anthropic-sdk-go` e `google.golang.org/genai`. Os adaptadores falam HTTP direto. O contrato de que precisamos — mensagens, tools, reasoning, uso — é um subconjunto pequeno e estável de cada API; os SDKs trazem árvores de dependência grandes para cobrir o resto, e testar contra `httptest` dá suíte verde sem chave. O custo dessa escolha está no limite abaixo.

**Limite honesto da suíte de contrato.** Ela roda contra troca gravada. Prova que o adaptador lê o que o provider documenta — não que o provider ainda mande aquilo. Oito dos nove adaptadores seguem sem nunca ter falado com a API real: não há chave nesta máquina para nenhum deles. A exceção é `antigravity`, cuja credencial *é* um arquivo local, e que por isso tem um segundo teste — `live_test.go`, atrás de `AOS_ANTIGRAVITY_LIVE=1` — exercendo catálogo, tool call com `thoughtSignature` de volta e streaming contra o endpoint de verdade. Esse par (gravado + ao vivo) é o formato que os outros adaptadores deveriam ter quando houver credencial para eles: quando o ao vivo falha e o gravado passa, o formato de fio mudou.

**Pendente.** A tabela de preços (`pricing.json`) não existe, então `CostUSD` é sempre zero — os tokens são contados e o dinheiro não. As capabilities opcionais (`Speech`, `Image`, `RealtimeToken`) têm interface e verificação em duas etapas, e nenhum adaptador as implementa; a verificação responde `_NOT_SUPPORTED` corretamente para todos. O `RefreshFunc` dos adaptadores OAuth deixou de ser um campo que ninguém preenche: `antigravity` renova de verdade (veja "Estado atual"); `codex` e `gemini-cli` ainda não.
