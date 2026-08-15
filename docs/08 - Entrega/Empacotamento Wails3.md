---
tags: [entrega, wails, empacotamento, assinatura]
aliases: [Empacotamento, Packaging]
fase: 9
status: especificado
origem: "[[Fractal App (Electron)]]"
---

# Empacotamento Wails3

> Pai: [[AOS]] · Origem no original: [[Fractal App (Electron)]] · Fase: 9

## Objetivo

Produzir aplicativos instaláveis e assinados para macOS, Windows e Linux, a partir do binário Wails.

## Comportamento do original

`Fractal.app` de 578 MB, com o Electron Framework, ~55 diretórios `.lproj` do Chromium, o binário do servidor de 161 MB embutido, `icon.icns` e `app-update.yml` ([[Fractal App (Electron)]]).

## Design

### macOS

```
AOS.app/
└── Contents/
    ├── Info.plist          # CFBundleShortVersionString, LSMinimumSystemVersion 11.0
    ├── MacOS/aos-desktop   # o binário Wails
    └── Resources/
        ├── icon.icns
        └── (nada mais — o frontend está embutido no binário)
```

O daemon **não é embutido**: o desktop usa o [[Gateway (Go)]], que resolve o `aosd` instalado ao lado ou o baixa na primeira execução. Isso evita duplicar 45 MB dentro do app e mantém uma única supervisão ([[ADR-0002 Wails3 no lugar de Electron]]).

```bash
# Sign, notarize, staple — required for distribution outside the App Store.
codesign --deep --force --options runtime --sign "$DEVELOPER_ID" AOS.app
xcrun notarytool submit AOS.zip --keychain-profile "$PROFILE" --wait
xcrun stapler staple AOS.app
```

Entitlements mínimos, listados e justificados:

| Entitlement | Por quê |
|---|---|
| `com.apple.security.network.client` | Chamadas a providers de LLM |
| `com.apple.security.network.server` | Daemon local em loopback |
| `com.apple.security.files.user-selected.read-write` | Seletor de arquivos |

Sem `com.apple.security.cs.allow-unsigned-executable-memory` e sem `disable-library-validation` — o Electron frequentemente precisa; o WebView nativo não.

### Windows

```powershell
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /a aos-desktop.exe
```

Instalador MSI via WiX, com WebView2 como dependência declarada (presente por padrão no Windows 11; o instalador baixa o runtime evergreen quando ausente).

### Linux

| Formato | Nota |
|---|---|
| `.deb` / `.rpm` | Dependência: `libwebkit2gtk-4.1` |
| AppImage | Sem dependências, mas WebKitGTK precisa estar no sistema |
| Flatpak | Sandbox do Flatpak com permissões declaradas |

### Ícones e identidade visual

Um único SVG mestre gera todos os tamanhos e formatos (`.icns`, `.ico`, PNGs) por `task gen-icons`. Enquanto o nome não existir ([[ADR-0000 Nome provisório do projeto]]), o ícone é um marcador — trocá-lo é substituir um arquivo e rodar a task.

## Decisões e divergências

> [!decision] Daemon fora do bundle
> O original embute um binário de 161 MB dentro do app e o supervisiona com lógica própria. Aqui, uma supervisão só, e o app pesa o que o app pesa.

> [!decision] Entitlements mínimos e justificados
> Cada um tem uma linha explicando por que existe. Sem isso, entitlements viram cargo cult e a revisão da Apple fica mais difícil do que precisa.

> [!decision] Assinatura obrigatória no release
> Um binário não assinado que executa comandos na máquina do usuário é pedir para ser bloqueado pelo Gatekeeper e pelo SmartScreen — e com razão.

> [!decision] Empacotamento depois do nome
> Assinatura, bundle id, ícone e instalador carregam a identidade do produto. A Fase 9 pressupõe o nome definido; até lá, os artefatos usam o marcador.

## Testes

- App instala e abre nas três plataformas (CI com matriz)
- macOS: `spctl --assess` aprova o app notarizado
- Windows: assinatura válida, SmartScreen não bloqueia
- Linux: `.deb` instala e o app abre em Ubuntu LTS
- Primeira execução sem `aosd` instalado resolve a situação (baixa ou orienta)
- Desinstalação remove os arquivos do app e **preserva** `~/.aos`

## Critério de pronto

- [ ] Bundles assinados para as três plataformas
- [ ] Notarização macOS aprovada
- [ ] Instaladores testados em máquina limpa
- [ ] Desinstalação preservando dados do usuário
