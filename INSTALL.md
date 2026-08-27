# Instalação

O caminho curto está no [README](README.md). Este documento é o resto: os
requisitos de cada plataforma, as formas alternativas de instalar, como
verificar o que você baixou, e o que fazer quando algo não abre.

- [macOS](#macos)
- [Windows](#windows)
- [Linux](#linux)
- [Servidor / VPS](#servidor--vps)
- [Variáveis do instalador](#variáveis-do-instalador)
- [Verificando o download](#verificando-o-download)
- [Quando não abre](#quando-não-abre)

---

## macOS

```sh
curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh | sh
```

Instala `AOS.app` em `/Applications` e os comandos `aos` e `aosd` em
`~/.local/bin`. Abra pelo Launchpad ou com `open -a AOS`.

### Por que não baixar pelo navegador

O macOS marca tudo que um navegador baixa com `com.apple.quarantine`. Um
aplicativo sem assinatura de desenvolvedor que carrega essa marca **não abre** —
desde o macOS 15 nem pelo botão direito → Abrir, só depois de uma visita aos
Ajustes do Sistema.

O `curl` não põe essa marca. É por isso que o instalador existe, e é a única
diferença entre um aplicativo que roda e um que o Gatekeeper recusa.

Se você já baixou o `.zip` pelo navegador, dá para desfazer:

```sh
xattr -dr com.apple.quarantine /Applications/AOS.app
```

O instalador também confere a assinatura ad-hoc do bundle antes de instalar —
ela não é uma declaração de confiança, é o que permite o aplicativo rodar em
Apple Silicon. Um download corrompido é recusado ali, e não na hora de abrir.

> Só há build para **Apple Silicon** (`darwin-arm64`). Em um Mac Intel o
> instalador para com a mensagem de que não encontrou o arquivo.

---

## Windows

Baixe em [Releases](https://github.com/VS-7/AOS/releases/latest):

| Arquivo | Quando usar |
|---|---|
| `AOS-setup-<versão>-windows-amd64.exe` | Instalador, com atalho no Menu Iniciar e desinstalação. |
| `AOS-<versão>-windows-amd64.zip` | Os três executáveis lado a lado, para quem prefere não rodar um instalador não assinado. |

Nos dois casos o **SmartScreen avisa** que o publicador é desconhecido — é o
que ele faz com qualquer binário sem certificado de assinatura de código. Em
**Mais informações → Executar assim mesmo**.

Se usar o `.zip`, mantenha `aos-desktop.exe`, `aosd.exe` e `aos.exe` **na mesma
pasta**: o aplicativo procura o daemon ao lado dele.

---

## Linux

### Requisito: GTK4 e WebKitGTK

É a única dependência, e ela vem antes de tudo:

```sh
# Ubuntu 24.04+ / Debian 13+
sudo apt install libgtk-4-1 libwebkitgtk-6.0-4

# Fedora 39+
sudo dnf install gtk4 webkitgtk6.0

# Arch
sudo pacman -S gtk4 webkitgtk-6.0
```

### Instalador

```sh
curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh | sh
```

Põe `aos-desktop`, `aosd` e `aos` em `~/.local/bin` e cria a entrada no menu de
aplicativos. Se faltar alguma biblioteca, ele diz **qual** e qual comando
instala.

Em `arm64` o instalador traz só os comandos de terminal: a janela é compilada
para `amd64`. Para usar em `arm64`, veja [servidor](#servidor--vps).

### AppImage

Um arquivo só, sem instalar nada, sem root:

```sh
chmod +x AOS-<versão>-linux-x86_64.AppImage
./AOS-<versão>-linux-x86_64.AppImage
```

O AppImage **não carrega o GTK e o WebKit dentro** — os requisitos acima valem
igual. Se faltar biblioteca, ele diz qual antes de fechar.

<details>
<summary>Por que ele não empacota o WebKit</summary>

Já tentou. O WebKitGTK não é uma biblioteca só: ele executa
`WebKitWebProcess` e `WebKitNetworkProcess` a partir de um caminho compilado
dentro do `libwebkitgtk`, e faz `dlopen` de um injected bundle a partir de
outro. Nenhum dos três é dependência de link, então nenhum era copiado junto —
enquanto a biblioteca que precisa deles era. Sobrava um WebKit empacotado que
não conseguia subir, ao lado do WebKit do sistema com o qual ele se recusa a
casar versão. O resultado era um AppImage que fechava na hora.

A saída usual, `linuxdeploy-plugin-gtk`, é um plugin de GTK+2/3 e não se
aplica: o AOS usa GTK4 com `webkitgtk-6.0`. É um problema em aberto para esse
conjunto — [wails#4313](https://github.com/wailsapp/wails/issues/4313) — não um
descuido.

</details>

### Tarball

Os três binários soltos:

```sh
tar xzf AOS-<versão>-linux-amd64.tar.gz
cd AOS && ./aos-desktop
```

---

## Servidor / VPS

Aqui o `aosd` carrega a interface dentro dele e você acessa pelo navegador.
Nada gráfico é instalado no servidor — é Go puro, sem GTK e sem WebKit.

```sh
curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh | AOS_SERVER=1 sh
```

> A variável vai em `sh`, não em `curl`: prefixar a busca define a variável
> para o download, não para o script que o download produz.

O instalador põe `aosd` e `aos` em `~/.local/bin`, cria o workspace em `~/aos`
(`AOS_WORKSPACE` muda isso) e escreve
`~/.config/systemd/user/aos.service`. Nada disso precisa de root.

```sh
systemctl --user enable --now aos
loginctl enable-linger "$USER"
```

O `enable-linger` é o que impede o serviço de morrer quando sua sessão SSH
fecha.

### Colocando na internet

O daemon escuta em `127.0.0.1:5326` e **fica lá**, de propósito: expor direto
exige ligar a segurança antes, e sem isso ele aborta no boot em vez de servir
um workspace aberto para a rede.

O caminho recomendado é um proxy reverso terminando TLS na frente. Com Caddy é
um arquivo inteiro:

```caddyfile
aos.example.com {
    reverse_proxy 127.0.0.1:5326
}
```

O Caddy resolve o certificado sozinho. A primeira página pede para criar a
conta, e enquanto ela não existir nada mais responde.

Se preferir não abrir porta nenhuma nem apontar DNS, o grupo `aos tunnel`
publica o daemon por Cloudflare Tunnel.

### O binário de servidor é outro arquivo

Ele vem como `AOS-server-<versão>-linux-<arch>.tar.gz`, separado do desktop. É
o mesmo `aosd`, com a interface compilada dentro; o `aosd` que acompanha o
desktop não a tem, porque a janela já carrega a própria cópia — uma segunda no
mesmo pacote seriam 14 MB repetidos.

A interface vai comprimida: 53 MB viram 14 MB dentro do binário, e é assim que
ela chega ao navegador — o pedaço principal são 2 MB na rede em vez de 7,6 MB.

Servidor existe para `amd64` e `arm64`.

---

## Variáveis do instalador

| Variável | O que faz |
|---|---|
| `AOS_VERSION` | Instala uma versão específica em vez da última. |
| `AOS_PREFIX` | Onde os binários vão. Padrão: `~/.local/bin`. |
| `AOS_NO_CLI=1` | Instala só o aplicativo, sem os comandos de terminal. |
| `AOS_SERVER=1` | Instala o daemon headless e o serviço do systemd (Linux). |
| `AOS_WORKSPACE` | O diretório que o servidor serve. Padrão: `~/aos`. |

---

## Verificando o download

Todo release traz um `checksums.txt` cobrindo todos os arquivos:

```sh
shasum -a 256 -c --ignore-missing checksums.txt   # macOS
sha256sum -c --ignore-missing checksums.txt       # Linux
```

O instalador já faz isso sozinho, e se recusa a instalar quando algo não
confere.

---

## Quando não abre

**Linux, a janela não aparece e nada é dito.** Falta GTK4 ou WebKitGTK. Rode
`aos-desktop` pelo terminal: ele nomeia a biblioteca que falta.

**macOS diz que o aplicativo está danificado.** Veio do navegador, com
quarentena. Rode `xattr -dr com.apple.quarantine /Applications/AOS.app` ou
reinstale pelo `curl`.

**`aos --help` mostra só quatro comandos** (`completion`, `gateway`, `help`,
`self`). Ainda não há conta: o `aos` monta a árvore de comandos a partir do
daemon, e sem conta o daemon não publica nada. Crie a conta pelo aplicativo e
rode de novo.

**O aplicativo abre e diz que não alcança o daemon.** O `aosd` precisa estar na
mesma pasta que o executável do aplicativo. Se você separou os arquivos do
`.zip` ou do `.tar.gz`, junte-os de volta.

Para operar o daemon à mão:

```sh
aos gateway status
aos gateway start
aos gateway stop
aos gateway restart
```
