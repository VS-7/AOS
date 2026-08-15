---
tags: [adr, decisao, seguranca, sandbox]
aliases: [ADR-0006, Allowlist, Sandbox]
fase: 5
status: especificado
origem: "[[Sandbox]]"
---

# ADR-0006 — Allowlist no sandbox

> Pai: [[AOS]] · Origem no original: [[Sandbox]] · Detalhe técnico: [[Sandbox (Go)]] · Fase: 5

## Contexto

O original controla execução de comandos com uma **blocklist por basename**:

```ts
const blockedCommands = ["rm","rmdir","del","format","mkfs","dd",
                         "shutdown","reboot","poweroff","halt"];
const isBlocked = blockedCommands.includes(path.basename(command));
```

A própria nota de engenharia reversa documenta os contornos triviais ([[Sandbox]]):

```
bash -c "rm -rf /"                        → basename é bash
/bin/sh -c 'dd if=...'
python -c "import shutil; shutil.rmtree(...)"
find . -delete
node -e "require('fs').rmSync(...)"
git clean -fdx
```

E existe `runCommand({ shell: true })` na API. A conclusão da análise é direta:

> A proteção real do Fractal contra destruição não é técnica: é o [[System Prompt (BASE)]] mais os hooks que podem negar a chamada. Quem depender do sandbox como fronteira de segurança dura estará confiando em algo que ele não é.

Uma blocklist que não bloqueia é pior que nenhuma: ela produz uma sensação de contenção que não existe, e o usuário concede permissão `execute` acreditando estar protegido.

## Decisão

Substituir a blocklist por **allowlist de executáveis + política declarada por agente**, com três camadas.

**Camada 1 — política por agente.** O frontmatter do [[Agent (Go)]] declara o que pode:

```yaml
sandbox:
  permissions: [read, write, execute]
  exec:
    policy: allowlist          # allowlist | deny-all
    allow: [git, go, task, npm, pnpm, node, rg, ls, cat, sed]
    denyArgs:                  # padrões rejeitados mesmo em binário permitido
      - "git push --force*"
      - "git clean*"
```

Default para um agente sem bloco `sandbox`: `permissions: [read]`, `exec.policy: deny-all`. **Somente leitura, sem execução** — mais restritivo que o original, que já defaultava para `["read"]` mas com execução liberada assim que a permissão fosse concedida.

**Camada 2 — resolução real do executável.** A verificação não é sobre a string: resolve-se o binário em disco (`exec.LookPath`) e compara-se o caminho canônico, não o basename. `./rm` e `/bin/rm` são o mesmo alvo.

**Camada 3 — shell explícito e restrito.** `shell: true` não existe. Executar via shell exige que o próprio shell esteja na allowlist (`bash`, `sh`) **e** que a política do agente declare `exec.allowShell: true`. Quando permitido, a linha de comando passa pelo mesmo filtro `denyArgs`. Um agente que precisa de shell é uma decisão consciente do dono do workspace, registrada em arquivo versionado.

```go
// internal/runtime/sandbox/exec.go
type ExecPolicy struct {
	Policy     string   `yaml:"policy"`     // "allowlist" | "deny-all"
	Allow      []string `yaml:"allow"`
	DenyArgs   []string `yaml:"denyArgs"`
	AllowShell bool     `yaml:"allowShell"`
}

// VerifyExec resolves the binary on disk and matches the canonical path
// against the allowlist. Basename comparison is never used.
func (p ExecPolicy) VerifyExec(name string, args []string) error
```

Mantidas do original, sem alteração: raiz confinada (worktree ou workspace), **resolução de path antes da checagem de contenção** (fecha path traversal), `.git` read-only, diretório de spillover read-only, timeout default de 120 s, e os campos `originalSize`/`omittedSize` que dizem ao agente quanto da saída foi cortado.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Manter a blocklist** | Reproduzir um defeito conhecido é falha de execução, não fidelidade. |
| **Container por execução (Docker/OCI)** | Isolamento verdadeiro. **Descartado como padrão**: exige runtime de container instalado, quebra o caso de uso principal (agente operando o repositório local do usuário, com o toolchain do usuário) e adiciona latência de segundos por chamada. Fica registrado como opção futura para toolsets de terceiros. |
| **Sandbox do SO** (`seatbelt` no macOS, `seccomp`/`landlock` no Linux, AppContainer no Windows) | Contenção real no nível do kernel. **Adiado**: três implementações distintas, comportamento divergente e nenhuma cobertura decente em Go puro sem CGO. Reabrir quando houver demanda de execução não confiável. Ver [[Segurança e Hardening]]. |
| **Allowlist + política por agente** | Escolhido. Contenção proporcional ao modelo de ameaça real: o agente é semi-confiável, o risco é ação destrutiva não intencional, e o dono do workspace decide o alcance. |

## Consequências

**Positivas**
- O agente **não pode** executar um binário que o dono do workspace não listou. `bash -c "rm -rf /"` falha na camada 3, e `rm -rf /` falha na camada 1.
- A política é um arquivo versionado: revisável em code review, auditável em `git log`.
- Combinada com [[ADR-0007 Canal real de aprovação de tool]], uma execução fora da allowlist pode escalar para aprovação humana em vez de simplesmente falhar.

**Negativas**
- **Atrito real de uso.** Um agente que precisa rodar um comando novo é bloqueado até alguém editar a política. Mitigação: o erro carrega um CTA com a linha exata a adicionar (`AOS_SANDBOX_EXEC_NOT_ALLOWED` com `cta: "adicione 'pytest' em sandbox.exec.allow do agente X"`). Ver [[Estratégia de Erros]].
- **Allowlist não impede mau uso do que está permitido.** `git` permitido inclui `git reset --hard`. Daí a lista `denyArgs` e a manutenção do prompt-mestre como camada de política — a proteção é em profundidade, não em uma única fronteira.
- **Manutenção da lista default.** Precisamos de um conjunto inicial sensato por tipo de workspace, senão a primeira experiência é uma sequência de bloqueios.

## Status

**Aceito.** Corrige o defeito #6 da lista de anti-padrões.
