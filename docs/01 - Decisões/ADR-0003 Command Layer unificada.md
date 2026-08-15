---
tags: [adr, decisao, command, superficies]
aliases: [ADR-0003, Command Layer, Ponte de Superfícies]
fase: 2
status: especificado
origem: "[[Ponte CLI para MCP]]"
---

# ADR-0003 — Command Layer unificada

> Pai: [[AOS]] · Origem no original: [[Ponte CLI para MCP]] · Detalhe técnico: [[Command Layer]] · Fase: 2

## Contexto

O sistema tem cinco consumidores da mesma capacidade: terminal humano, agente externo via MCP, agente interno, HTTP e documentação. O original resolve isso com `FractalCommand` + o framework `incur`, e o [[Guia de Reimplementação]] é categórico:

> **Uma definição de comando → quatro superfícies.** Sem isso, você mantém CLI, MCP, tools do agente e docs em sincronia manual — e elas divergem em semanas. Se for reimplementar apenas uma coisa deste sistema, reimplemente [[Ponte CLI para MCP]].

A consequência prática observada no original é forte: o registry de tools do agente interno é literalmente `FractalTasksCommands.toTools()` — **o agente nativo e o Claude Code externo usam exatamente as mesmas ferramentas, com os mesmos schemas e as mesmas descrições**. Não existe divergência possível porque não existe segunda definição.

O risco de não fazer isso é conhecido e mensurável: 23 grupos × ~6 comandos = ~140 capacidades × 5 superfícies = 700 pontos de sincronização manual.

## Decisão

Um pacote `internal/core/command` com um registry heterogêneo de comandos tipados. Cada comando é declarado uma vez:

```go
type Command[In any, Out any] struct {
	Group    string   // "memories"
	Name     string   // "store"
	Summary  string   // uma linha, vira o help curto do cobra
	Doc      string   // Markdown completo, vira a description da tool MCP
	Examples []Example
	Local    bool     // true = sem --base-url/--token (gateway)
	Registry bool     // false = fora do registry de tools do agente
	Handler  func(ctx context.Context, in In) (Out, error)
}
```

E projeta-se em cinco superfícies por reflexão sobre `In`:

| Superfície | Mecanismo | Nota |
|---|---|---|
| CLI | flags do `pflag` derivadas das tags `json` e `jsonschema` | [[CLI cobra]] |
| MCP | `mcp.AddTool` com schema inferido pelo SDK oficial | [[MCP Go SDK]] |
| Tool do agente | mesmo `Descriptor` no registry do loop, execução in-process | [[Agent Loop]] |
| HTTP | `POST /api/{group}/{name}` com body = `In` | [[HTTP chi]] |
| `SKILL.md` | geração a partir de `Doc` + `Examples` | [[Especificação da Skill]] |

**Duas projeções de tool, selecionáveis por configuração:** plana (`memories_store`) e composta (`Memory({action:"store", input:{...}})`). O original migrou de ~150 tools planas para compostas entre 0.1.314 e 0.1.401, quebrando integrações no caminho ([[Ferramentas MCP]]). Implementar as duas e permitir escolha por config evita repetir a quebra — ver [[ADR-0011 Superfície de tools versionada]].

**`_reasoning` obrigatório em toda tool**, validado como não-vazio, exatamente como no original.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Definir cada superfície à mão** | Honesto sobre o esforço inicial (menor), desonesto sobre o custo total. Divergência é questão de semanas. |
| **Geração de código (`go:generate`) a partir de um IDL** | Elimina reflexão em runtime e dá erros em tempo de compilação. Descartado por acoplar o ciclo de desenvolvimento a um passo de geração e por exigir manter um gerador — que também pode divergir. A reflexão acontece uma vez, no boot, sobre ~140 structs: o custo é irrelevante. |
| **Usar só o schema JSON como fonte** | Perde a tipagem do handler. O ganho da abordagem é `Handler func(ctx, In) (Out, error)` ser type-safe no ponto onde a regra de negócio vive. |
| **Adotar um framework CLI que já gere MCP** | Não existe equivalente maduro do `incur` em Go. Construir a camada é o trabalho. |

## Consequências

**Positivas**
- Adicionar uma capacidade é escrever um `Command` e registrá-lo. As cinco superfícies aparecem sozinhas.
- Um teste de CI compara a `SKILL.md` commitada com a gerada e falha na divergência. Documentação que mente vira build vermelho.
- A cobertura de teste da camada de comando cobre as cinco superfícies de uma vez: testar o handler testa tudo.

**Negativas**
- **Reflexão sobre struct tags é frágil em silêncio.** Um campo sem tag `json` gera uma flag com nome derivado do campo Go, o que muda a superfície sem erro. Mitigação: um teste de registry que percorre todos os comandos e falha se algum campo de `In` não tiver `json` e `jsonschema`. Ver [[Command Layer]], seção Testes.
- **Tipos complexos em argv.** Um `[]Rule` não existe em linha de comando. Solução idêntica à do original: campo aceita string JSON, decodificada após o parse (`--rules '[{"type":"always",...}]'`).
- **O registry heterogêneo precisa de uma interface não-genérica** (`Descriptor`) porque Go não permite `[]Command[any, any]`. Custo: um `json.RawMessage` no ponto de invocação. Aceitável e contido em um arquivo.

## Status

**Aceito.** É a peça de maior alavancagem do sistema — ver [[Command Layer]] para o design completo.
