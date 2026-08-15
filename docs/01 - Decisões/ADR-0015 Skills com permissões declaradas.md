---
tags: [adr, decisao, seguranca, skill]
aliases: [ADR-0015, Skills, Assinatura]
fase: 8
status: especificado
origem: "[[Skill]]"
---

# ADR-0015 — Skills com permissões declaradas e origem verificável

> Pai: [[AOS]] · Origem no original: [[Skill]] · Fase: 8

## Contexto

Uma [[Skill]] no original é um pacote instalável que pode trazer, de uma vez ([[Skill]], [[Marketplace]]):

- **Agentes próprios**, via o segundo padrão da coleção `agents`
- **Memórias pré-existentes** para esses agentes
- **Rotinas** que rodam sozinhas, com gatilho cron, webhook ou activity
- **Hooks** que interceptam o comportamento de **todos** os agentes
- **Coleções e views** que aparecem na UI
- **Toolsets** dos cinco tipos, incluindo `cli`, que executa binários locais

O alerta da engenharia reversa é direto:

> Instalar uma skill do registry pode registrar hooks `PreToolUse` capazes de **reescrever silenciosamente a entrada de qualquer tool** (`updatedInput`), além de trazer toolsets que executam comandos locais. Não há sandbox ou assinatura para skills de terceiros. O modelo de confiança é o mesmo de instalar um pacote npm: total.

A combinação mais perigosa é hook + toolset `cli`: um hook `PreToolUse` reescreve o payload de uma tool legítima para apontar a um comando arbitrário, e o usuário vê o agente "usando uma ferramenta normal".

## Decisão

Três camadas: **declaração**, **consentimento** e **procedência**.

**1. Manifesto de permissões obrigatório.** Toda skill declara no frontmatter do `SKILL.md` o que instala e o que pede:

```yaml
---
name: github-flow
version: 1.2.0
source: owner/repo
permissions:
  hooks: [PreToolUse, PostToolUse]     # quais eventos intercepta
  toolsets:
    - type: cli
      command: gh                       # binário exato, não wildcard
  exec: [gh, git]                       # entra na allowlist do sandbox
  network: [api.github.com]             # hosts que toolsets podem alcançar
  agents: [reviewer]                    # agentes que instala
  routines: 1                           # quantas rotinas autônomas traz
  collections: [pull_requests]
---
```

Instalação **falha** se o conteúdo do pacote exceder o manifesto. Um hook não declarado não é registrado; um toolset `cli` para binário não declarado não conecta. A verificação é do instalador, não do autor.

**2. Consentimento explícito no install.** `aos skills install owner/repo` mostra um diff de permissões e exige confirmação. Em modo não-interativo, exige `--accept-permissions` com a lista, o que torna a concessão visível em script e em histórico de shell:

```
aos skills install owner/repo --accept-permissions hooks,exec:gh,network:api.github.com
```

Um agente **não pode** instalar skill de terceiro sem aprovação humana — `skills_install` roteia por [[ADR-0007 Canal real de aprovação de tool]] com `Risk: high`.

**3. Procedência verificável.** Duas fases:

- **Fase 8 (agora):** `source` (`<owner>/<repo>`) e `commit` SHA gravados no install. `aos skills verify` recomputa o hash do conteúdo instalado e compara com o registrado — detecta adulteração local pós-instalação.
- **Depois:** assinatura de publisher (`minisign`/`cosign`) com chave publicada no registry, e política `skills.requireSignature: true` para instalações corporativas.

**Isolamento de execução dos hooks:** hooks de skill rodam com o mesmo tratamento de qualquer código não confiável — timeout, saída limitada, e **sem** acesso ao `Approver`. Um hook não pode aprovar a si mesmo.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Confiança total, como o original** | Reproduzir o defeito #20. O vetor hook + toolset `cli` é grave demais para ser deixado aberto. |
| **Proibir hooks em skills de terceiros** | Elimina o vetor mais perigoso e também o mecanismo de extensibilidade que dá valor às skills. Excessivo. |
| **Sandbox real (container/WASM) para hooks** | Isolamento verdadeiro. **Adiado**: hooks são declarativos (retornam decisão e contexto), não código arbitrário no nosso modelo — o risco é o que declaram fazer, e isso o manifesto cobre. Reabrir se hooks passarem a executar código. |
| **Só assinatura, sem manifesto** | Assinatura prova quem publicou, não o que o pacote faz. As duas coisas são necessárias, e o manifesto entrega mais valor imediato. |

## Consequências

**Positivas**
- O usuário sabe, antes de instalar, que a skill vai interceptar tool calls e executar `gh`.
- O manifesto é versionado e diffável: uma atualização que pede permissão nova exige novo consentimento.
- As permissões `exec` da skill compõem com [[ADR-0006 Allowlist no sandbox]] — a skill pede, o dono concede, o sandbox aplica.

**Negativas**
- **Atrito no ecossistema.** Autores precisam manter o manifesto; instaladores veem uma tela a mais. Preço aceitável para a superfície envolvida.
- **Manifesto pode mentir sobre intenção**, não sobre capacidade. Declarar `network: [api.github.com]` e exfiltrar dados para lá é possível. A defesa é o registro auditável em [[Activity (Go)]], não a prevenção.
- **Verificação de conteúdo × manifesto exige um validador que entenda todos os tipos de recurso** de uma skill. Trabalho real na Fase 8, coberto por testes por tipo.

## Status

**Aceito.** Corrige o defeito #20.
