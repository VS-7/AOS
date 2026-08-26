# AOS

Um sistema operacional para agentes de IA.

> **Beta.** Os binários não são assinados com certificado de desenvolvedor, e
> ainda não existe canal de atualização automática. O que isso significa na
> prática está em [Limitações do beta](#limitações-do-beta) — leia antes de
> instalar, são dois parágrafos.

## Como baixar

Os arquivos ficam em **[Releases](https://github.com/VS-7/AOS/releases/latest)**.

### macOS

```sh
curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh | sh
```

Isso instala `AOS.app` em `/Applications` e os comandos `aos` e `aosd` em
`~/.local/bin`. Depois, abra pelo Launchpad ou com `open -a AOS`.

**Instale por aqui, não pelo navegador.** O macOS marca tudo que um navegador
baixa com `com.apple.quarantine`, e um aplicativo não assinado com essa marca
não abre — desde o macOS 15 nem pelo botão direito → Abrir, só por uma visita
aos Ajustes. O `curl` não põe essa marca, então o que este comando instala
simplesmente roda. Se você já baixou o `.zip` pelo navegador, dá para desfazer:

```sh
xattr -dr com.apple.quarantine /Applications/AOS.app
```

Variáveis que o instalador aceita: `AOS_VERSION` para fixar uma versão,
`AOS_PREFIX` para escolher onde os comandos vão, `AOS_NO_CLI=1` para instalar
só o aplicativo.

> Hoje só há build para **Apple Silicon** (`darwin-arm64`). Em um Mac Intel o
> instalador para com uma mensagem dizendo que não encontrou o arquivo.

### Windows

Baixe um dos dois em [Releases](https://github.com/VS-7/AOS/releases/latest):

- `AOS-setup-<versão>-windows-amd64.exe` — instalador, com atalho no Menu
  Iniciar e desinstalação.
- `AOS-<versão>-windows-amd64.zip` — os três executáveis lado a lado, para
  quem prefere não rodar um instalador não assinado.

Em ambos os casos o **SmartScreen vai avisar** que o publicador é
desconhecido: é o que ele faz com qualquer binário sem certificado de
assinatura de código. Em "Mais informações" → "Executar assim mesmo".

Se usar o `.zip`, mantenha `aos-desktop.exe`, `aosd.exe` e `aos.exe` **na mesma
pasta** — a próxima seção explica por quê.

### Linux

O AOS conta com **GTK4 e WebKitGTK já instalados** na máquina. Essa é a única
dependência, e ela vem primeiro:

```sh
# Ubuntu 24.04+ / Debian 13+
sudo apt install libgtk-4-1 libwebkitgtk-6.0-4

# Fedora 39+
sudo dnf install gtk4 webkitgtk6.0

# Arch
sudo pacman -S gtk4 webkitgtk-6.0
```

Com isso resolvido, o mesmo comando do macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh | sh
```

Isso põe `aos-desktop`, `aosd` e `aos` em `~/.local/bin` e cria a entrada no
menu de aplicativos. Se alguma biblioteca estiver faltando, ele diz **qual** e
qual comando instala, em vez de deixar você com uma janela que não abre.

Em `arm64` o instalador traz só os comandos: a janela é compilada para
`amd64`.

Se preferir não passar um script pelo `sh`, as duas outras formas continuam
publicadas. O **AppImage** é um arquivo só, sem instalar nada, sem root:

```sh
chmod +x AOS-<versão>-linux-x86_64.AppImage
./AOS-<versão>-linux-x86_64.AppImage
```

Ou o `.tar.gz`, com os três binários soltos:

```sh
tar xzf AOS-<versão>-linux-amd64.tar.gz
cd AOS && ./aos-desktop
```

O AppImage **não** carrega o GTK e o WebKit dentro. Ele já tentou: o WebKitGTK
não é uma biblioteca só — ele executa `WebKitWebProcess` e
`WebKitNetworkProcess` a partir de um caminho compilado dentro dele, e nenhum
dos dois é dependência de link, então nenhum dos dois era copiado junto. O
resultado era um AppImage que fechava na hora. É um problema em aberto para
esse conjunto ([wails#4313](https://github.com/wailsapp/wails/issues/4313)),
não um descuido.

Se faltar alguma biblioteca, o AppImage diz **qual** e qual comando instala,
em vez de simplesmente não abrir.

## Como o sistema funciona

Três programas, um de cada vez fazendo uma coisa só:

| | |
|---|---|
| **`aosd`** | O daemon. É quem **é dono do workspace** — os arquivos, a fila de trabalho, o índice de busca. Roda em segundo plano e é o único processo que escreve. |
| **`aos-desktop`** | A janela. Não é uma segunda cópia do sistema: é uma interface que conversa com o daemon. |
| **`aos`** | O terminal. Mesma coisa: pergunta ao daemon. |

Por que separado? Porque um workspace tem um dono. Se cada janela e cada
terminal carregasse o sistema inteiro, todos escreveriam nos mesmos arquivos
ao mesmo tempo. Com um daemon, há um escritor e nenhuma corrida.

Uma consequência prática: **o daemon precisa estar ao lado de quem o chama.**
O aplicativo procura um `aosd` na mesma pasta que ele, e é por isso que os
pacotes o trazem dentro — o `.app` no macOS, o instalador no Windows, a pasta
no `.tar.gz`. Separar os arquivos quebra isso.

Outra: como tudo passa por HTTP, o cliente não precisa estar na mesma máquina
que o daemon. É o que torna possível o [modo servidor](#no-navegador-e-em-um-servidor).

## Primeiro uso

Abra o aplicativo. Ele sobe o daemon sozinho e mostra a tela de criação de
conta. É só preencher — a conta fica na sua máquina, em `~/.aos`, e nada é
enviado para lugar nenhum.

Feito isso, o terminal também funciona, sem login separado:

```sh
aos gateway status      # o daemon está de pé?
aos workspace list
aos tasks list
aos --help              # a árvore inteira de comandos
```

> Se `aos --help` mostrar só quatro comandos (`completion`, `gateway`, `help`,
> `self`), é porque ainda não há conta: o `aos` monta a árvore de comandos a
> partir do daemon, e sem conta o daemon não publica nada. Crie a conta pelo
> aplicativo e rode de novo.

Para operar o daemon sem a interface:

```sh
aos gateway start
aos gateway stop
aos gateway restart
```

## No navegador, e em um servidor

O mesmo aplicativo compila em **modo servidor**, que troca a janela nativa por
um servidor HTTP servindo a mesma interface ao navegador:

```sh
go build -tags server -o aos-web ./cmd/aos-desktop
WAILS_SERVER_HOST=127.0.0.1 WAILS_SERVER_PORT=8080 ./aos-web
```

Nesse modo o binário é Go puro — não precisa de GTK, WebKit nem de nada
gráfico — o que o torna adequado a um contêiner ou a uma VPS. Há um
`build/docker/Dockerfile.server` pronto para isso.

**Antes de expor na internet**, saiba que o daemon escuta em `127.0.0.1` por
padrão, de propósito. Expor exige `--host 0.0.0.0` **junto com**
`--require-auth`; sem os dois, ele aborta no boot em vez de servir um workspace
aberto para a rede. Há também o grupo `aos tunnel`, que publica o daemon local
via Cloudflare Tunnel sem abrir porta nenhuma.

Não há build pronto do modo servidor nos Releases ainda; hoje ele se compila
a partir do código.

## Verificando o que você baixou

Todo release traz um `checksums.txt` cobrindo todos os arquivos:

```sh
shasum -a 256 -c --ignore-missing checksums.txt   # macOS
sha256sum -c --ignore-missing checksums.txt       # Linux
```

O instalador do macOS já faz isso, e também confere a assinatura do bundle
antes de instalar.

## Limitações do beta

**Sem assinatura.** Nenhum binário tem certificado de desenvolvedor. No macOS
o instalador contorna isso (o Gatekeeper só barra o que veio com quarentena);
no Windows o SmartScreen vai avisar toda vez. Isso é uma escolha consciente
desta fase, não um descuido — mas significa que você deve baixar apenas do
repositório oficial e conferir os checksums.

**Sem atualização automática.** O sistema tem a maquinaria de auto-update
pronta, mas nenhum canal publicado, e a chave de assinatura de release em uso
ainda é de desenvolvimento. Atualizar hoje é rodar o instalador de novo. Uma
consequência a registrar: instalações feitas neste beta carregam essa chave, e
quando ela for rotacionada elas não conseguirão verificar releases assinados —
vai ser preciso reinstalar uma vez.

**Plataformas.** macOS só em Apple Silicon. Windows e Linux só em x86-64.

## Licença

[MIT](LICENSE).
