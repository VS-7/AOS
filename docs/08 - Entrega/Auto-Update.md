---
tags: [entrega, update, releases]
aliases: [Auto-Update, Atualização]
fase: 9
status: em-construcao
origem: "[[Versões e Artefatos]]"
---

# Auto-Update

> Pai: [[AOS]] · Origem no original: [[Versões e Artefatos]] · Fase: 9

**Núcleo entregue.** `internal/domain/update` implementa Check/Download/Apply
como este design pede: Download recusa a release inteira
(`UPDATE_SIGNATURE_INVALID`) se a assinatura do arquivo de checksums não
verificar contra a chave embutida, antes de confiar em qualquer checksum
dele; cada asset é então conferido pelo próprio SHA-256
(`UPDATE_CHECKSUM_MISMATCH`) — nada fica staged em nenhuma falha. Apply
espera o trabalho em andamento drenar (limitado), troca os binários,
reinicia o daemon, e desfaz tudo — reiniciando de novo na versão anterior —
se a nova não ficar saudável a tempo. `internal/core/relsig` é uma
implementação Ed25519 própria (não bit-a-bit compatível com o `minisign`
real — ver o comentário do próprio pacote) em vez de uma reimplementação
adivinhada do formato do minisign. A versão única entre os três binários é
guardada em três pontos: `Download` só aceita um release que traga um asset
para cada binário instalado na plataforma (`UPDATE_NO_ASSET_FOR_PLATFORM`,
em vez de atualizar o `aos` e deixar o `aosd` para trás); `Apply` confere de
novo que o que está staged ainda cobre o que está instalado
(`UPDATE_STAGED_INCOMPLETE`). A comparação entre a versão da janela e a do
daemon que ela adota é do próprio `cmd/aos-desktop`, não do atualizador.

**Não entregue:** coordenação com `~/.mcp.json` (`aos self mcp doctor`) —
feature própria, de escopo comparável, não construída nesta rodada. A chave
de assinatura em uso é uma chave de desenvolvimento gerada por
`tools/genreleasekey`; a privada não foi commitada e precisa ser trocada por
uma chave real antes de qualquer release de verdade. Nenhum canal de release
existe ainda: com `BaseURL` vazio, `update check` responde `not-configured` —
não "atualizado", que era o que respondia, e que a janela mostrava como "você
está na versão mais recente". O `release.yml` publica o feed
(`checksums.txt.sig` e `stable.json`, via `tools/releasefeed`) e grava o
endereço nos binários (`build.UpdateBaseURL`) só quando o secret de
assinatura existe.

Duas coisas do desenho não valem como escritas acima. O daemon não reinicia a
si mesmo (`AOS_GATEWAY_SELF_RESTART`), então `Apply` recusa dentro dele antes
de tocar em qualquer arquivo e a troca roda de um terminal, onde o gateway de
fora reinicia e desfaz; onde a instalação tem a janela, o comando vem com o
aviso de fechar e abrir o AOS depois, porque a janela aberta continua na
versão anterior. E três instalações não são atualizadas binário a binário,
mas reinstaladas inteiras (`UPDATE_REINSTALL_REQUIRED`, com o motivo): o
`.app` do macOS (trocar um arquivo quebra o selo do bundle); uma pasta que a
conta não pode alterar (AppImage, Program Files, `/usr`); e o daemon de
servidor, compilado com `-tags webui` (`build.Flavour`), porque o feed publica
o `aosd` sem a interface web e trocá-lo deixaria o servidor só com a API —
esse é reinstalado com `AOS_SERVER=1 install.sh`.

O comando do terminal só reinicia o daemon que o supervisor dele iniciou — o
aplicativo ou `aos gateway`. Quem é o daemon que responde vem do próprio
`/api/health`, que diz a versão e o processo: o `Apply` confere que esse
processo é o do registro do gateway (`gateway.json`) antes de trocar
qualquer arquivo, e confere de novo depois de esperar os turnos. Ao lado de
um daemon iniciado à mão (`aosd serve`) ou por um gerenciador de serviços, o
reinício não para nada, e o install antigo seguia assim mesmo: achava a
versão anterior respondendo, dizia que tinha instalado e apagava o `.prev`;
com o gateway recusando subir um segundo daemon (`AOS_GATEWAY_NOT_OURS`),
dizia que o daemon "não voltou", sobre um daemon que nunca caiu. Agora recusa
antes (`UPDATE_DAEMON_NOT_SUPERVISED`), e o `install` do `Check`/`Status`
avisa (`unsupervised`): pare esse daemon onde ele foi iniciado, e o comando
sobe a nova versão sozinho. Um daemon do supervisor que não responde também
é recusado (`UPDATE_DAEMON_NOT_ANSWERING`, com `aos gateway restart`). E o
install só termina quando o processo que o gateway reiniciou responde **como a
versão staged**: um daemon que volta como outra versão (iniciado de outra
cópia do binário, `AOS_DAEMON_PATH`) é desfeito como uma falha de saúde, e o
`.prev` só é apagado depois dessa resposta. Com nenhum daemon rodando, o
install inicia a nova versão pelo gateway.

Por baixo, um update por vez: `Download` e `Apply` tomam uma trava de arquivo
(`update.lock`) que vale entre processos — o daemon e o `aosd update apply`
no terminal — e o registro (`state.json`) só é alterado sob outra trava, para
um `Check` não apagar o que um `Download` acabou de deixar staged. O `Check`
seguinte remove os `.prev` que um install terminado não conseguiu apagar (no
Windows, o próprio `aosd.exe` que rodou a troca), nunca os de um install que
parou no meio. O download não tem prazo total, só de progresso: desiste
quando os bytes param de chegar, e não quando quem pediu parou de esperar.
`Download` e `Apply` recusam agentes e clientes MCP
(`UPDATE_NOT_FOR_AGENTS`); `Check` e `Status` respondem a eles.

## Objetivo

Manter os três binários atualizados sem quebrar sessões em andamento nem instalar código não verificado.

## Comportamento do original

Auto-update via GitHub Releases ([[Fractal App (Electron)]]):

```yaml
owner: tryfractal
repo: fractal
provider: github
updaterCacheDirName: fractal-updater
```

A engenharia reversa observou o efeito colateral de não coordenar os componentes: na máquina analisada coexistem **três versões** — CLI 0.1.314 pelo nvm, app 0.1.400, CLI 0.1.401 pelo Homebrew — e o MCP efetivamente registrado é o mais antigo dos três, porque `~/.mcp.json` usa `"command": "fractal"` e o PATH decide ([[Versões e Artefatos]]).

Isso não é acidente de instalação: é o que acontece quando três artefatos se atualizam por canais independentes.

## Design

### Versão única para os três binários

`aos`, `aosd` e `aos-desktop` são compilados do mesmo commit e versionados juntos. Não existe combinação suportada de versões diferentes.

```go
// internal/core/build/version.go
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Compatible reports whether a client and a daemon can talk. Same minor is
// required; a mismatch produces an explicit error telling the user which
// component to update, instead of a confusing protocol failure later.
func Compatible(client, daemon string) error
```

O CLI verifica isso na primeira chamada ao daemon e falha com CTA.

### Fluxo

```go
// internal/domain/update/service.go

type Service interface {
	// Check queries the release channel. It never downloads anything.
	Check(ctx context.Context) (*Release, error)

	// Download fetches the artifact and verifies checksum and signature.
	// A failed verification leaves nothing installed.
	Download(ctx context.Context, r *Release) (Staged, error)

	// Apply swaps the binaries and restarts the daemon at a safe point.
	Apply(ctx context.Context, s Staged) error
}
```

### Verificação obrigatória

```go
// verify checks two things before anything is installed:
//  1. SHA-256 against the signed checksum file
//  2. a minisign signature over that checksum file, using a public key
//     embedded in the binary
// Neither is optional. An update channel without signature verification is a
// remote code execution channel.
//
//go:embed release-pubkey.pub
var releasePubKey []byte

func verify(artifact, checksums, sig []byte) error
```

### Aplicação segura

```go
// Apply swaps binaries at a point where nothing is lost:
//  1. wait for in-flight agent turns to finish (bounded by a grace period)
//  2. drain the job queue's active leases
//  3. write the new binaries next to the old ones and rename over
//  4. restart the daemon through the gateway
//  5. verify health; on failure, roll back to the previous binaries
//
// The previous version is kept until the new one reports healthy.
func (s *service) Apply(ctx context.Context, st Staged) error
```

### Canais

| Canal | Conteúdo |
|---|---|
| `stable` | Releases marcados como estáveis |
| `beta` | Pré-releases |
| `off` | Sem verificação automática |

Default: `stable`, com verificação diária e **notificação**, não instalação silenciosa. Instalar exige confirmação — exceto quando o usuário liga `update.auto`.

### Coordenação com o MCP

Após uma atualização, `aos self mcp doctor` roda automaticamente e avisa se algum cliente MCP registrado aponta para um binário de versão diferente — o problema exato observado no original ([[MCP Go SDK]]).

## Decisões e divergências

> [!decision] Versão única para os três binários
> A divergência mais importante desta nota. Elimina por construção a classe de problema observada na máquina analisada.

> [!decision] Assinatura obrigatória
> Um canal de atualização sem verificação de assinatura é um canal de execução remota de código. Não há modo de desabilitar.

> [!decision] Rollback automático em falha de saúde
> A versão anterior fica em disco até a nova reportar saudável.

> [!decision] Notificação por default, não instalação silenciosa
> Uma ferramenta que executa comandos na máquina do usuário não troca de versão sozinha sem avisar. Instalação automática é opt-in.

> [!decision] Distribuição por releases assinados, agnóstica de forja
> O original acopla ao GitHub. O verificador só precisa de artefatos, um arquivo de checksums e uma assinatura — servidos de onde for.

## Testes

- `Check` sem release novo não faz nada
- Checksum inválido aborta sem instalar
- Assinatura inválida aborta sem instalar
- `Apply` espera turno em andamento terminar
- Falha de saúde após restart faz rollback e o daemon volta funcional
- Cliente e daemon com minor diferente produzem erro com CTA
- `mcp doctor` detecta cliente MCP apontando para versão antiga
- Atualização com o daemon parado funciona

## Critério de pronto

- [ ] Verificação de checksum e assinatura obrigatória
- [ ] Aplicação segura com espera e rollback
- [ ] Versão única entre os três binários, verificada em runtime
- [ ] Coordenação com registros MCP
