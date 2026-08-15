---
tags: [adr, decisao, seguranca, segredos]
aliases: [ADR-0010, Segredos, 0600]
fase: 0
status: especificado
origem: "[[Autenticação e Credenciais]]"
---

# ADR-0010 — Segredos com permissão restrita e redação na saída

> Pai: [[AOS]] · Origem no original: [[Autenticação e Credenciais]] · Fase: 0

## Contexto

Inventário de segredos do original, com permissões observadas na máquina ([[Autenticação e Credenciais]], [[Instalação Local Observada]]):

| Segredo | Onde | Formato | Permissão |
|---|---|---|---|
| Senha de usuário | `users.json` | argon2id `m=65536,t=2,p=1` | 0644 |
| Token de usuário | `users.json` | texto plano | 0644 |
| `FRACTAL_TOKEN` | `~/.mcp.json` | texto plano | 0644 |
| `security.secret` (128 hex) | `config.json` | texto plano | 0644 |
| `security.apiToken` | `config.json` | texto plano | 0644 |
| Chaves de provider | `config.json` | texto plano | 0644 |
| Token do Cloudflare | `config.json` | texto plano | 0644 |

**Só a senha é hasheada** — e com parâmetros adequados, o que está correto. Todo o resto está legível por qualquer processo do usuário.

Agrava-se por dois lados. Primeiro, a senha tem mínimo de 6 caracteres para uma conta `super` que controla execução de comandos. Segundo, `config_get` está exposto como tool MCP **sem filtro de campos sensíveis** — um agente externo lê `security.secret`, `apiToken`, chaves de provider e token do tunnel, e pode alterá-los com `config_update`.

## Decisão

Cinco medidas.

**1. Permissão `0600` na criação, verificada no boot.**

```go
// internal/core/config/secure.go
const secretFileMode os.FileMode = 0o600

// WriteSecret writes atomically with 0600 and verifies the resulting mode.
func WriteSecret(path string, data []byte) error

// AuditSecrets runs at boot; it repairs loose permissions and logs each repair.
func AuditSecrets(paths ...string) []Repair
```

No boot, cada arquivo de segredo é verificado. Permissão mais frouxa que `0600` é **corrigida** e registrada em log e em [[Activity (Go)]]. No Windows, o equivalente é ACL restrita ao usuário corrente; onde não for possível aplicar, o boot emite aviso persistente.

**2. Redação na saída para o agente.** O comando `config get` tem duas projeções:

```go
type ConfigView struct {
	Security SecurityView `json:"security"`
	Agents   AgentsView   `json:"agents"`
}

// Redact returns a copy where every field tagged `secret:"true"` becomes
// "***" plus a fingerprint (first 4 + last 4 chars) for identification.
func Redact(c Config) ConfigView
```

Chamado por um humano em TTY sem `--reveal`: redigido. Chamado por agente (MCP ou registry interno): **sempre redigido, sem exceção**. Revelar exige `aos config get --reveal` num terminal interativo, com confirmação. Corrige o defeito #7.

**3. `config update` com allowlist de campos escreíveis por agente.** Um agente pode alterar `region.timezone` e `general.preventSleep`; **não pode** alterar nada sob `security`, `agents.providers[].key` ou `tunnel.token`. Tentar produz `AOS_CONFIG_FIELD_FORBIDDEN` com CTA orientando o humano a fazer pela UI.

**4. Senha mínima de 12 caracteres**, com verificação contra a lista de senhas mais vazadas (embutida, sem chamada de rede). Corrige o defeito #9. Hash: **argon2id com os mesmos parâmetros do original** (`m=65536, t=2, p=1`), que estavam corretos — via `golang.org/x/crypto/argon2`.

**5. Token de API com prefixo, expiração opcional e rotação.** Formato `aos_` + 64 hex, com `expiresAt` opcional e `aos auth token rotate`. O original não tem rotação nem expiração.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Keychain do SO** (Keychain, DPAPI, Secret Service) | Proteção real em repouso. **Adiado, não descartado**: três implementações, degradação necessária em Linux headless, e conflita com o modelo de arquivo editável à mão. Registrado em [[Segurança e Hardening]] como evolução para as chaves de provider. |
| **Criptografar `config.json` com senha mestre** | Exige desbloqueio a cada boot do daemon, o que quebra start automático — o caso de uso central do [[Gateway (Go)]]. |
| **Só documentar `chmod 600`** | É o que o original efetivamente faz (a recomendação está na documentação). Não funciona: default inseguro com documentação segura continua inseguro. |

## Consequências

**Positivas**
- Corrige os defeitos #7, #9 e #10 de uma vez.
- Um agente comprometido ou um MCP de terceiro não extrai credenciais via `config_get`.
- A auditoria de boot repara instalações antigas sem intervenção do usuário.

**Negativas**
- **Segredos continuam em texto plano em repouso.** A permissão protege de outros usuários, não de malware rodando como o usuário. É uma melhora, não uma solução — e a nota de [[Segurança e Hardening]] diz isso explicitamente em vez de fingir o contrário.
- **`--reveal` é atrito** para quem legitimamente precisa copiar um token. Aceitável.
- **A allowlist de `config update` precisa ser mantida** conforme o schema de config evolui. Mitigação: teste que falha se um campo novo não estiver classificado como escreível-por-agente ou não.

## Status

**Aceito.**
