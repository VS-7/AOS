---
tags: [frontend, theme, css]
aliases: [Temas Frontend, Tokens]
fase: 7
status: pronto
origem: "[[Theme]]"
---

# Temas

> Pai: [[Design System]] · Origem no original: [[Theme]] · Ver: [[Theme (Go)]] · Fase: 7

## Objetivo

Aplicar os 38 temas embutidos e os presets do usuário no frontend, sincronizando com a aparência nativa da janela.

## Comportamento do original

38 temas, um arquivo por tema, e o canal IPC `theme:set-appearance` que sincroniza o tema do renderer com a **aparência nativa e a vibrancy** da janela — trocar de tema muda o material da janela do sistema ([[Theme]]).

## Design

### Tokens como variáveis CSS

```css
/* styles/tokens.css — the contract every theme must satisfy */
:root {
  --bg: ...;            --bg-subtle: ...;    --bg-elevated: ...;
  --fg: ...;            --fg-muted: ...;     --fg-subtle: ...;
  --border: ...;        --border-strong: ...;
  --accent: ...;        --accent-fg: ...;
  --success: ...;       --warning: ...;      --danger: ...;
  --category-decision: ...;  /* one per memory category, for the graph */
}
```

A lista completa é a fonte do `tokens.txt` embutido em [[Theme (Go)]]. Um token novo aqui quebra o build de temas até que todos o declarem.

### Aplicação

```ts
// lib/theme.ts

// applyTheme writes the token values onto the document root. It also tells the
// native layer which appearance to use, so the window material matches — the
// desktop equivalent of the original's theme:set-appearance IPC.
export async function applyTheme(theme: Theme): Promise<void> {
  const root = document.documentElement;
  for (const [token, value] of Object.entries(theme.tokens)) {
    root.style.setProperty(`--${token}`, value);
  }
  root.dataset.appearance = theme.appearance;

  if (isDesktop()) {
    await SystemService.SetAppearance(theme.appearance);
  }
}
```

### `appearance: auto`

Segue `prefers-color-scheme`, com listener para mudança em tempo real. No desktop, também acompanha a mudança de aparência do SO.

### Sem flash

O tema ativo é gravado em `localStorage` e aplicado por um script inline no `index.html`, antes do primeiro paint. Sem isso, cada carga pisca no tema padrão antes de trocar.

## Decisões e divergências

> [!decision] Tokens semânticos, não cores nomeadas
> `--bg-elevated`, não `--gray-800`. Um tema é uma tabela de decisões de design, não uma paleta — o que permite que temas muito diferentes (AMOLED e Solarized Light) satisfaçam o mesmo contrato.

> [!decision] Cores de categoria de memória são tokens
> O grafo de memórias colore por categoria ([[Memory (Go)]]). Se essas cores forem literais, o grafo fica ilegível em metade dos temas.

> [!decision] Aparência nativa sincronizada, como no original
> Era um acerto do Electron; o Wails oferece equivalente. Ver [[Wails3 Services]].

## Testes

- Todo tema embutido define todos os tokens (verificado no Go, ver [[Theme (Go)]])
- Troca de tema não recarrega a página nem perde estado
- `auto` reage à mudança de preferência do SO em tempo real
- Nenhum flash de tema no carregamento (teste de snapshot no primeiro paint)
- Contraste WCAG AA em todos os embutidos
- Grafo de memórias legível em todos os temas (cores de categoria vindas de token)

## Critério de pronto

- [x] 38 temas aplicáveis
- [ ] Sincronização com aparência nativa no desktop
- [x] `auto` seguindo o SO
- [x] Sem flash no carregamento

## Estado — Fase 7

**Sem flash, e é a parte que exige cuidado.** Um script inline em `index.html`
lê a escolha do `localStorage` — incluindo os tokens já resolvidos — e a aplica
antes do primeiro paint. Sem os tokens guardados junto, a página ainda piscaria:
buscá-los do daemon é uma ida e volta, e o paint não espera.

`applyTheme` escreve os tokens na raiz, troca a classe `light`/`dark`, grava a
escolha e, no desktop, chama `SystemService.SetAppearance`.

**`auto` só escuta quando é `auto`.** Quem escolheu escuro não pediu para ser
trocado para claro ao amanhecer, então o listener de `prefers-color-scheme` só é
registrado com a preferência em `auto`.

**Divergência sobre os nomes dos tokens:** registrada em [[Theme (Go)]]. O
vocabulário é o do original (shadcn), não o esboço desta nota, porque a lista é
gerada de `tokens.css` e o CSS é o que os componentes portados vão ler.

**Não verificado:** o material nativo da janela mudando com o tema; a troca de
tema preservando estado (o caminho não recarrega nada, mas ninguém observou); e
o grafo de memórias legível nos 38 temas — as cores de categoria são derivadas do
accent e separadas por construção (`TestTheThirteenMemoryCategoriesEachGetTheirOwnColour`),
mas legibilidade contra a superfície não foi medida por tema.
