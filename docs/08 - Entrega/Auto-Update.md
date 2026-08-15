---
tags: [entrega, update, releases]
aliases: [Auto-Update, Atualização]
fase: 9
status: especificado
origem: "[[Versões e Artefatos]]"
---

# Auto-Update

> Pai: [[AOS]] · Origem no original: [[Versões e Artefatos]] · Fase: 9

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
