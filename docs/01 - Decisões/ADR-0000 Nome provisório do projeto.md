---
tags: [adr, decisao, identidade]
aliases: [ADR-0000, Nome do Projeto, Codinome AOS]
fase: 0
status: especificado
origem: "[[PROMPT — Reconstrução em Go]]"
---

# ADR-0000 — Nome provisório do projeto

> Pai: [[AOS]] · Fase: 0

## Contexto

O sistema a ser construído reproduz a função do Fractal ([[Fractal OS]]) mas **não é** o Fractal: é um produto independente, em outra linguagem, com decisões de segurança e arquitetura deliberadamente divergentes. Usar o nome do original seria incorreto de fato e problemático de direito — o Fractal é marca da Nubler Digital Ltda, com licença proprietária ([[Versões e Artefatos]]).

O nome definitivo ainda não foi escolhido. Bloquear a construção até que exista um nome é desperdício; espalhar um nome que será trocado depois cria uma dívida de renomeação em centenas de arquivos.

## Decisão

Adotar o codinome **`AOS`** (*Agent Operating System*) como **marcador explícito**, com três regras que garantem renomeação em uma operação:

1. **Um único módulo Go:** `github.com/OWNER/aos`. `OWNER` é literal e greppável — não é um nome real.
2. **Nenhum nome derivado no código.** Proibido `AosService`, `AOSError`, `aosConfig`. Tipos são nomeados pelo domínio (`WorkspaceService`, `AppError`, `Config`). O nome do produto aparece só em: `go.mod`, imports, constantes de branding em `internal/core/build`, e nomes de binário no `Taskfile.yml`.
3. **Prefixo de erro parametrizado.** Códigos de erro usam `AOS_` como prefixo, gerado a partir de uma constante única (`apperr.Prefix`), nunca literal espalhado. Ver [[Estratégia de Erros]].

Constante única de branding:

```go
// internal/core/build/brand.go
package build

// Brand holds every user-visible identifier of the product.
// Renaming the product means editing this file and the module path — nothing else.
const (
	Name        = "aos"        // binary name, config dir suffix, MCP server name
	DisplayName = "AOS"        // window title, CLI header
	ErrorPrefix = "AOS"        // error code prefix: AOS_MEMORY_NOT_FOUND
	StateDir    = ".aos"       // ~/.aos and <workspace>/.aos
	EnvPrefix   = "AOS"        // AOS_SERVER_PORT, AOS_TOKEN
	Port        = 5326
)
```

Procedimento de renomeação, quando o nome existir:

```bash
go mod edit -module github.com/<owner>/<name>
grep -rl 'github.com/OWNER/aos' --include='*.go' . | xargs sed -i '' 's|github.com/OWNER/aos|github.com/<owner>/<name>|g'
# editar internal/core/build/brand.go
task test
```

## Alternativas consideradas

| Alternativa | Por que foi descartada |
|---|---|
| Escolher um nome agora | Nome de produto é decisão do dono, não do arquiteto. Um nome ruim escolhido sob pressão custa mais que o marcador. |
| Usar `fractal` temporariamente | Confunde os dois sistemas no grafo de notas, no `~/.fractal` do usuário (que já existe na máquina e seria corrompido) e no registro MCP. Risco concreto, não teórico. |
| Nome genérico permanente (`agentd`) | Colide com binários existentes e não é registrável como marca. |
| Placeholder sem semântica (`xyz`) | Torna o código ilegível durante meses de construção. |

## Consequências

**Positivas**
- Construção começa hoje; renomeação é uma tarefa de 10 minutos com teste de regressão.
- O diretório de estado `~/.aos` **não colide** com o `~/.fractal` da instalação existente do original. As duas ferramentas coexistem na mesma máquina durante o desenvolvimento — o que é necessário para comparação funcional.
- O prefixo `AOS_` nos códigos de erro não colide com `FRACTAL_`, o que evita confusão em logs de máquinas com as duas ferramentas.

**Negativas**
- Toda documentação externa gerada antes da renomeação (`SKILL.md`, completions, OpenAPI) precisa ser regenerada. Como tudo é gerado do registry ([[Especificação da Skill]]), isso é automático.
- O nome de tool MCP muda com o nome do produto se o servidor for renomeado no `~/.mcp.json`. Os **nomes de tool** em si (`memories_store`) são independentes do nome do produto e não mudam — ver [[ADR-0016 Compatibilidade de nomes com o original]].

**Neutras**
- Nomes de coleção, campos de frontmatter e formato de arquivo não carregam o nome do produto, então arquivos criados antes da renomeação continuam válidos depois.

## Status

**Aceito.** Vigente até que o nome definitivo seja escolhido, quando esta ADR passa a `Substituída` e uma ADR de renomeação registra o nome final e a data.
