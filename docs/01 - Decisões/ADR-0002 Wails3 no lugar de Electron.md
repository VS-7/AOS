---
tags: [adr, decisao, desktop, wails]
aliases: [ADR-0002, Wails3, Desktop]
fase: 7
status: especificado
origem: "[[Fractal App (Electron)]]"
---

# ADR-0002 — Wails3 no lugar de Electron

> Pai: [[AOS]] · Origem no original: [[Fractal App (Electron)]] · Fase: 7

## Contexto

O desktop do original é Electron: **578 MB** de bundle, dos quais o Chromium e o Electron Framework respondem pela maior parte, mais 161 MB do binário do servidor embutido e ~55 diretórios `.lproj` de localização do Chromium ([[Fractal App (Electron)]]).

A superfície IPC real é pequena — nove canais: controles de janela sem frame, health check, `shell.openPath`, revelar no Finder, abrir URL externa, diálogo de arquivo, sincronização de tema/vibrancy e report de crash. Ou seja: **paga-se um navegador inteiro para obter nove chamadas de sistema e uma janela transparente**.

Há ainda um defeito de segurança de origem estrutural:

```ts
app.commandLine.appendSwitch("ignore-certificate-errors");
```

Aplicado ao processo inteiro no binário de produção — toda requisição HTTPS do renderer aceita certificado inválido ([[Autenticação e Credenciais]], risco 8).

## Decisão

**Wails v3** com WebView nativo do sistema operacional (WKWebView no macOS, WebView2 no Windows, WebKitGTK no Linux) e frontend **React 19 + TypeScript**.

O bootstrap alvo, verificado contra a documentação atual do Wails v3:

```go
package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name: "AOS",
		Services: []application.Service{
			application.NewService(&WorkspaceService{}),
			application.NewService(&AgentService{}),
			application.NewService(&ChatService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "AOS",
		Width:  1440,
		Height: 900,
	})

	if err := app.Run(); err != nil {
		panic(err)
	}
}
```

Bindings TypeScript são gerados com `wails3 generate bindings` e importados no React a partir de `../bindings/`. Ver [[Wails3 Services]] e [[React 19 e Bindings]].

**Decisão de acoplamento — o desktop não embute o daemon.** No original, o Electron faz `spawn` do `fractal-server` e o supervisiona ([[Fractal App (Electron)]]). Aqui o desktop usa o mesmo [[Gateway (Go)]] que o CLI usa: chama `gateway.EnsureRunning(ctx)` no start. Consequências: uma única implementação de supervisão de processo, e app e CLI coexistem sem disputar a porta — comportamento que o original só obtém por um health check ad-hoc antes do spawn.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Manter Electron** | Único ganho real: paridade de API com a implementação existente e vibrancy do macOS bem documentada. Custo: ~500 MB, um segundo runtime JS na máquina e uma superfície de segurança que já falhou uma vez no original. |
| **Tauri v2** | Tecnicamente equivalente ao Wails no resultado (WebView nativo, binário pequeno). Descartado porque o backend é Go: com Tauri, a camada nativa seria Rust e o daemon Go rodaria como processo separado obrigatoriamente, sem a possibilidade de compartilhar tipos e serviços em processo. |
| **Só web, sem desktop** | Perde bandeja, notificações nativas, deep link `aos://` e o seletor de arquivos nativo. O produto original tem superfície visual dedicada — abandoná-la é reduzir escopo, não simplificar. |

## Consequências

**Positivas**
- Bundle estimado em **10–25 MB** contra 578 MB. O binário do daemon deixa de ser duplicado dentro do app.
- Sem `ignore-certificate-errors`: o WebView usa a política TLS do sistema. Se em algum momento for necessário confiar em certificado local, o escopo será `localhost`, declarado em código e revisado.
- Services Wails são structs Go comuns — os mesmos serviços de domínio, sem camada IPC própria. Ver [[Wails3 Services]].

**Negativas**
- **Diferença de motor de renderização por plataforma.** WKWebView, WebView2 e WebKitGTK não são idênticos. Mitigação: o design system evita APIs de ponta e a matriz de teste visual cobre as três plataformas ([[Design System]]).
- **Vibrancy e materiais nativos do macOS têm API diferente da do Electron.** Reimplementar a sincronização tema ↔ aparência nativa exige trabalho novo — ver [[Temas]].
- **Ecossistema menor que o do Electron.** Menos receitas prontas para casos exóticos (auto-update assinado, instaladores). Ver [[Auto-Update]] e [[Empacotamento Wails3]].

**Neutras**
- Deep linking (`fractal://` no original, `aos://` aqui) existe nas duas plataformas; a API muda, o comportamento não.

## Status

**Aceito.**
