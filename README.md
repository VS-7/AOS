# AOS

**Um sistema operacional para agentes de IA.**

Você define agentes — cada um com papel, instruções e memória própria — e eles
trabalham no seu repositório. Cada tarefa roda isolada em seu próprio worktree
do Git, então nada é sobrescrito enquanto você continua trabalhando.

Tudo fica na sua máquina. O workspace são arquivos no seu disco, em Markdown,
versionáveis junto com o código. Não há serviço no meio.

Três formas de usar, sobre o mesmo sistema: **aplicativo**, **terminal** e
**navegador**.

> **Beta.** Os binários ainda não têm assinatura de desenvolvedor e não há
> atualização automática. [O que isso muda para você](#limitações-do-beta).

---

## O que dá para fazer

| | |
|---|---|
| **Agentes** | Papel, instruções e memória por agente. Eles lembram entre sessões. |
| **Tarefas** | O trabalho que os agentes executam, cada uma em um worktree isolado. |
| **Conversas** | Canais para falar com os agentes, com histórico e anexos. |
| **Rotinas** | Automações por agenda ou por gatilho — inclusive webhook. |
| **Coleções** | Registros estruturados, guardados como Markdown no seu repositório. |
| **Metas e projetos** | O que os agentes perseguem, e sob o que está agrupado. |
| **Skills e ferramentas** | O que cada agente pode usar, declarado e permissionado. |
| **Marketplace** | Plugins e skills prontos para instalar. |
| **MCP** | Seus agentes ficam disponíveis para qualquer cliente que fale MCP. |

---

## Instalar

### macOS e Linux

```sh
curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh | sh
```

No Linux o AOS precisa de **GTK4 e WebKitGTK** instalados. Se faltar, o
instalador diz qual pacote e qual comando da sua distribuição — ele não deixa
você com uma janela que não abre.

> Instale por este comando, não baixando pelo navegador: o macOS bloqueia
> aplicativos sem assinatura que vieram de download. O `curl` não recebe essa
> marca. [Por quê](INSTALL.md#macos).

### Windows

Baixe em [Releases](https://github.com/VS-7/AOS/releases/latest):

- **`AOS-setup-<versão>-windows-amd64.exe`** — instalador, com atalho no Menu
  Iniciar.
- `AOS-<versão>-windows-amd64.zip` — os executáveis soltos.

O SmartScreen vai avisar que o publicador é desconhecido. Em **Mais
informações → Executar assim mesmo**.

### Servidor (VPS)

Sem nada gráfico no servidor: o daemon carrega a interface dentro dele e você
acessa pelo navegador.

```sh
curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh | AOS_SERVER=1 sh
```

Ele instala os binários, cria o workspace e escreve um serviço do systemd —
nada disso precisa de root. Depois:

```sh
systemctl --user enable --now aos
loginctl enable-linger "$USER"
```

O daemon escuta em `127.0.0.1:5326` e fica lá. Para abrir na internet, um
proxy reverso na frente — com Caddy é uma linha:

```caddyfile
aos.example.com {
    reverse_proxy 127.0.0.1:5326
}
```

📖 **[Guia de instalação completo](INSTALL.md)** — requisitos por
distribuição, AppImage, verificação de checksums, variáveis do instalador e
solução de problemas.

---

## Primeiro uso

Abra o aplicativo. Ele sobe o daemon sozinho e mostra a tela de criação de
conta. A conta fica em `~/.aos`, na sua máquina, e nada é enviado para lugar
nenhum.

Feito isso, o terminal funciona sem login separado:

```sh
aos tasks list
aos agents list
aos --help          # a árvore inteira de comandos
```

---

## Como é montado

Três programas, cada um com uma função:

| | |
|---|---|
| **`aosd`** | O daemon. É o dono do workspace — os arquivos, a fila de trabalho, a busca. Roda em segundo plano e é o único que escreve. |
| **`aos-desktop`** | A janela. Uma interface que conversa com o daemon. |
| **`aos`** | O terminal. Mesma coisa, pela linha de comando. |

Um workspace tem um dono só. Se cada janela e cada terminal carregasse o
sistema inteiro, todos escreveriam nos mesmos arquivos ao mesmo tempo — com um
daemon, há um escritor e nenhuma disputa.

Como tudo passa por HTTP, o cliente não precisa estar na mesma máquina que o
daemon. É o que torna possível o modo servidor acima.

> Os pacotes trazem o `aosd` junto com o aplicativo, e ele precisa estar **na
> mesma pasta**. Separar os arquivos quebra isso.

---

## Limitações do beta

**Sem assinatura.** Nenhum binário tem certificado de desenvolvedor. No macOS
o instalador contorna; no Windows o SmartScreen avisa toda vez. Baixe apenas
do repositório oficial e [confira os checksums](INSTALL.md#verificando-o-download).

**Sem atualização automática.** Atualizar hoje é rodar o instalador de novo.
Instalações feitas neste beta carregam uma chave de assinatura de
desenvolvimento; quando ela for rotacionada, será preciso reinstalar uma vez.

**Plataformas.** A janela: macOS em Apple Silicon, Windows e Linux em x86-64.
O servidor e o terminal também em `arm64` no Linux.

---

## Licença

[MIT](LICENSE).
