---
tags: [dominio, config, global]
aliases: [Config Go, Configuração]
fase: 0
status: especificado
origem: "[[Config]]"
---

# Config (Go)

> Pai: [[AOS]] · Origem no original: [[Config]] · [[config.json]] · Decisão: [[ADR-0010 Segredos com permissão restrita]] · Fase: 0

## Objetivo

Configuração global da instalação em `~/.aos/config.json`, com redação de segredos na saída para agentes.

## Comportamento do original

Serviço singleton **sem estado**: relê o disco a cada chamada, o que permite editar o arquivo à mão com o servidor rodando ([[Config]]).

Oito seções: `user`, `agents` (providers e seis slots de modelo), `region`, `general`, `notifications`, `security`, `telemetry`, `tunnel`.

E o defeito #7, registrado com clareza:

> As tools `config_get` e `config_update` estão expostas via MCP. Um agente externo pode ler `security.secret`, `security.apiToken`, chaves de provider e o token do tunnel — e alterá-los. Não foi encontrado filtro de campos sensíveis na resposta.

## Design em Go

```go
// internal/domain/config/entity.go

type Config struct {
	User          User          `json:"user"`
	Agents        Agents        `json:"agents"`
	Region        Region        `json:"region"`
	General       General       `json:"general"`
	Notifications Notifications `json:"notifications"`
	Security      Security      `json:"security"`
	Telemetry     Telemetry     `json:"telemetry"`
	Tunnel        Tunnel        `json:"tunnel"`
	Marketplace   Marketplace   `json:"marketplace"`
	MCP           MCP           `json:"mcp"`
}

type Security struct {
	Enabled  bool   `json:"enabled"`                                  // default TRUE — ADR-0009
	Password string `json:"password,omitempty" secret:"true"`
	Secret   string `json:"secret"             secret:"true"`
	APIToken string `json:"apiToken"           secret:"true"`
}

type Provider struct {
	ID  string `json:"id"`
	Key string `json:"key" secret:"true"`
}

type Agents struct {
	Providers []Provider          `json:"providers"`
	Models    map[string]ModelRef `json:"models"` // default, subconscious, realtime, voice, image, video
}
```

A tag `secret:"true"` é o mecanismo, não uma convenção: a redação é feita por reflexão sobre ela.

```go
// internal/domain/config/redact.go

// Redact returns a deep copy where every field tagged secret:"true" is replaced
// by a fingerprint: "sk-…a1b2" — enough to identify which key is configured,
// useless to whoever reads it.
func Redact(c Config) Config

// agentWritable lists the only paths an agent may change. Anything else fails
// with AOS_CONFIG_FIELD_FORBIDDEN and a CTA pointing the human at the UI.
var agentWritable = []string{
	"region.language", "region.city", "region.country", "region.timezone",
	"general.preventSleep",
	"notifications.enabled",
}
```

```go
type Service interface {
	// Get always redacts for agents. Humans on a TTY may pass Reveal, which
	// requires an interactive confirmation.
	Get(ctx context.Context, in GetInput) (Config, error)
	Update(ctx context.Context, in UpdateInput) (Config, error)

	// Raw is internal-only: it never crosses a transport boundary.
	Raw(ctx context.Context) (Config, error)
}
```

### Leitura a cada chamada, com invalidação

```go
// load rereads the file when its modtime changed, so hand edits are picked up
// without a restart — the original's behaviour — but without paying a disk read
// on every single call.
func (s *service) load() (Config, error)
```

## Decisões e divergências

> [!decision] Redação por tag, sempre, para agentes
> Corrige o defeito #7. Não há caminho pelo qual `config_get` devolva um segredo a um agente.

> [!decision] Allowlist de campos escreíveis por agente
> Um agente ajusta timezone; não toca em `security` nem em chaves de provider.

> [!decision] `security.enabled` default `true`
> Divergência de default em relação ao original. Ver [[ADR-0009 Bind em loopback por padrão]].

> [!decision] Telemetria opt-in, não opt-out
> O original tem `telemetry.enabled: true` e `sentryEnabled: true` por padrão, com um `installationId` identificando a instalação. Invertemos: um produto local-first que promete privacidade não deve enviar nada sem consentimento explícito. O onboarding pergunta uma vez.

## Testes

- Redação: nenhum campo `secret:"true"` alcançável por `Get` sem `Reveal`
- Reflexão de redação cobre campos aninhados e slices (`providers[].key`)
- `Update` de campo fora da allowlist por agente falha com CTA
- `Update` do mesmo campo por humano autenticado passa
- Edição manual do arquivo é vista sem restart
- Migração de config antiga preenche defaults novos sem perder valores
- Escrita usa `0600` e escrita atômica

## Critério de pronto

- [ ] Redação por tag verificada por teste sobre a struct inteira
- [ ] Allowlist de escrita por agente aplicada
- [ ] Defaults seguros
- [ ] Telemetria opt-in
