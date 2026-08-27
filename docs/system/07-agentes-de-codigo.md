# 07 · Agentes de código

> [Índice](../README.md) · Anterior: [Canais](06-canais.md) · Próximo: [Operação](08-operacao.md)

Como plugar o AOS em Claude Code, Codex, Cursor, Gemini CLI, OpenCode — ou
em qualquer agente que leia um `SKILL.md` ou fale MCP.

São duas peças, e elas se completam:

| | O que dá | Como chega |
|---|---|---|
| **Skill** | O *conhecimento*: quando usar o AOS, o protocolo de início de sessão, o de memória, as regras duras e uma referência por grupo de comando. | Arquivos em `<dir de skills>/aos/` |
| **MCP** | A *capacidade*: as 27 tools que executam de fato. | `aos --mcp` (stdio) ou `http://127.0.0.1:5326/mcp` |

Só a skill: o agente sabe operar mas precisa chamar o `aos` pelo terminal.
Só o MCP: o agente tem as ferramentas mas descobre o contrato tateando.
As duas: ele sabe o que fazer e tem com o quê.

## Instalar a skill

```sh
aos self skill install --all              # em todo agente detectado na máquina
aos self skill install --to claude-code   # em um específico
aos self skill install --dir ./.claude/skills   # num diretório qualquer (por projeto)
aos self skill targets                    # o que existe nesta máquina, e onde
aos self skill show tasks                 # ler uma referência sem instalar nada
```

No aplicativo: **Configurações → Developers → Add skill to…**

A skill vem **compilada dentro dos binários**: funciona sem rede e é sempre a
que corresponde aos comandos que este build realmente tem — uma referência
`tasks.md` que descreve o grupo `tasks` deste daemon, não de outro.

### Onde ela é instalada

| Agente | Diretório |
|---|---|
| Claude Code | `~/.claude/skills/aos/` |
| Codex | `~/.codex/skills/aos/` |
| Cursor | `~/.cursor/skills/aos/` |
| Gemini CLI | `~/.gemini/skills/aos/` |
| OpenCode | `~/.config/opencode/skills/aos/` |
| Convenção entre clientes | `~/.agents/skills/aos/` |

`--all` escreve só onde o agente parece instalado (o diretório de configuração
existe), e nunca sobrescreve uma skill de terceiro que ocupe o mesmo nome.

Para escopo de projeto, `--dir <repo>/.claude/skills` ou
`<repo>/.agents/skills` — o que estiver no repositório vale para quem clona.

## Ligar o MCP

**stdio** — o cliente lança o processo:

```jsonc
{
  "mcpServers": {
    "aos": { "command": "aos", "args": ["--mcp"] }
  }
}
```

Se o daemon não estiver rodando, o próprio `aos --mcp` o sobe. A credencial
vem de `~/.aos/local.token`; `AOS_TOKEN` sobrepõe.

**HTTP** — o cliente conecta:

```jsonc
{
  "mcpServers": {
    "aos": {
      "type": "http",
      "url": "http://127.0.0.1:5326/mcp",
      "headers": { "Authorization": "Bearer aos_..." }
    }
  }
}
```

O token sai de **Configurações → Developers → REST API**, que também gera o
`mcp.json` pronto para copiar.

### Por cliente

| Cliente | Onde configurar |
|---|---|
| **Claude Code** | `claude mcp add aos -- aos --mcp`, ou `~/.claude.json` |
| **Codex** | `~/.codex/config.toml`, seção `[mcp_servers.aos]` |
| **Cursor** | Settings → MCP, ou `~/.cursor/mcp.json` |
| **Gemini CLI** | `~/.gemini/settings.json`, chave `mcpServers` |
| **OpenCode** | `opencode.json`, chave `mcp` |
| Outros | Qualquer um que aceite `command`/`args` ou `type: http` |

## Diga ao AOS qual agente você é

Um agente de código que opera este workspace **é** um dos agentes dele — e a
memória pertence a um agente. Diga qual com `AOS_AGENT_ID`:

```jsonc
{
  "mcpServers": {
    "aos": {
      "command": "aos",
      "args": ["--mcp"],
      "env": { "AOS_AGENT_ID": "atlas" }
    }
  }
}
```

Sem isso, `agents_me` não tem quem responder e `memories_store` recusa a
chamada dizendo exatamente isso. O terminal lê a mesma variável:

```sh
AOS_AGENT_ID=atlas aos memories recall --set limit=20 --reason "início de sessão"
```

Crie o agente antes, se ele ainda não existir:

```sh
aos agents create --set id=meu-agente --set name="Meu Agente" --reason "…"
```

## Como um agente deve usar o AOS

É o que a skill ensina. Em resumo:

**Início de sessão — obrigatório**

1. `agents_me` — quem eu sou neste workspace
2. `memories_recall` (limite 20) — o que eu já sei
3. `workspace_introspect` — o que existe

**Memória.** Recupere antes de gravar; se já há traço, linke ou supersede.
Calibre a confiança com honestidade: 0.9–1.0 verificado, 0.6–0.8 inferência
forte, abaixo de 0.6 palpite. Memórias são globais entre instâncias
paralelas suas.

**Tools compostas.** Na primeira chamada de cada ação, passe `schema: true`
no mesmo nível de `action` para receber descrição, exemplos e schema. Depois
você conhece o contrato.

**Regras duras.** Todo call precisa de `_reasoning`. Use `set-status` para
mover tarefas, nunca `update`. Em modo tarefa, comunique-se pelos comentários
da tarefa. Só mova para `in_review` com evidência. Um comando fora da
allowlist do agente falha, e o erro diz exatamente o que pedir ao dono do
workspace.

## Verificar

```sh
aos self skill targets        # a skill está onde deveria?
aos self tools                # a superfície publicada, na ordem publicada
aos agents me --format json   # a identidade responde?
aos gateway status            # o daemon está de pé?
```

Do lado do cliente MCP: se `tools/list` traz 27 tools compostas (`Agent`,
`Memory`, `Task`, …), está ligado.

## Quando algo não funciona

**`agents_me` ou `memories_store` recusa por falta de identidade.**
Falta `AOS_AGENT_ID`. Veja a seção acima.

**O cliente MCP não lista tool nenhuma.** O daemon não subiu ou não há conta.
Rode `aos gateway status`; se não houver conta, crie no aplicativo — é o que
escreve a credencial que o terminal lê.

**`aos <grupo> <comando>` diz que o comando não existe.** A árvore vem do
daemon. O erro traz o código `AOS_CLI_SURFACE_UNAVAILABLE` e o que fazer.

**A skill está desatualizada.** Ela é gerada do registro a cada build.
Reinstale depois de atualizar: `aos self skill install --all`.

**Uma tool falha por validação.** Não repita às cegas: leia o erro (ele traz
o campo e a ação corretiva) e inspecione o contrato com `schema: true`.

> Próximo: [08 · Operação](08-operacao.md)
