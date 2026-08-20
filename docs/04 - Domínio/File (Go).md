---
tags: [dominio, file, filesystem]
aliases: [File Go, Arquivos]
fase: 7
status: especificado
origem: "[[File]]"
---

# File (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[File]] · Fase: 7

## Objetivo

Acesso a arquivos do workspace pela API HTTP — o backend do explorador de arquivos da UI.

## Comportamento do original

A feature **não** tem grupo de comando nem tools MCP ([[File]]). A separação é intencional:

| Caminho | Quem usa | Proteção |
|---|---|---|
| `FileService` (`/api/file`) | UI web / usuário humano | Autenticação HTTP |
| `fs.toolset` | Agente | [[Sandbox (Go)]] com permissões e contenção de raiz |

O agente acessa filesystem pelas tools nativas (`Read`, `Write`, `Edit`, `Glob`, `Grep`, `Bash`), nunca por esta API.

## Design em Go

```go
// internal/domain/file/service.go

// Service backs the UI file explorer. It is deliberately NOT exposed as
// commands or MCP tools: the agent's filesystem path is the sandbox, and
// having two doors to the same resource with different rules is how one of
// them gets forgotten.
type Service interface {
	Tree(ctx context.Context, in TreeInput) (Tree, error)
	Read(ctx context.Context, in ReadInput) (Content, error)
	Write(ctx context.Context, in WriteInput) error
	Move(ctx context.Context, from, to string) error
	Delete(ctx context.Context, path string) error

	// Diff powers the task worktree review screen.
	Diff(ctx context.Context, in DiffInput) (Diff, error)
}

type Content struct {
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	Text      string `json:"text,omitempty"`
	Base64    string `json:"base64,omitempty"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
}
```

### Contenção

```go
// Every path goes through the same resolution the sandbox uses: normalize,
// resolve symlinks, then check containment. The two implementations share the
// helper precisely so they cannot drift apart.
func (s *service) resolve(ctx context.Context, p string) (string, error) {
	root := s.workspaceRoot(ctx)
	if wt := s.worktreeFor(ctx); wt != "" {
		root = wt
	}
	return pathx.ResolveInside(root, p)
}
```

`pathx.ResolveInside` é o mesmo helper de [[Sandbox (Go)]]. Duas cópias da lógica de contenção seriam duas chances de errar.

## Decisões e divergências

> [!decision] Continua fora de CLI e MCP
> Herdado sem alteração. A fronteira de privilégio é deliberada: o agente passa pelo sandbox, o humano pela autenticação HTTP.

> [!decision] Helper de contenção compartilhado
> No original, `FileService` e `SandboxService` implementam contenção separadamente. Uma única implementação em `internal/core/pathx`, testada uma vez, usada nos dois.

> [!decision] `pathx.ResolveInside` fica atrás da porta `FS`, não direto no domínio
> O esboço acima (`s.resolve` chamando `pathx.ResolveInside` diretamente) é o que a interface *comunica*, mas `internal/architecture` tem um teste (`TestDomainTestsDoNotTouchIO`) que proíbe teste de domínio tocar disco real. Como `pathx.Resolve` chama `filepath.EvalSymlinks` (I/O real), um teste de `file.Service` com FS fake ainda cairia em disco se o domínio chamasse `pathx` diretamente por operação. Solução: `FS.Resolve(ctx, root, p)` entra na porta — o adapter real (`osfile`) delega para `pathx.ResolveInside`; um fake de teste faz a mesma contenção em memória, sem disco. `Service.root()` continua chamando `pathx.Root` diretamente (não é por-operação, e resolver o root em si — antes de qualquer FS — não tem como ser injetado sem reintroduzir o mesmo problema de galinha-e-ovo que `pathx.Root` existe para resolver). Contenção continua uma implementação só; só o *ponto de chamada* por-arquivo mudou de lugar.

> [!decision] Leitura com teto e detecção de binário
> Arquivo grande é truncado com marcação explícita; binário volta como base64 com limite. Sem isso, abrir um arquivo de 500 MB na UI derruba o navegador.

## Testes

- Path traversal negado nas cinco operações
- Symlink apontando para fora da raiz negado
- Worktree de task confina o acesso à branch
- Arquivo maior que o teto vem truncado, com `Truncated: true`
- Binário detectado e devolvido em base64
- `Diff` de worktree contra a base

## Critério de pronto

- [x] Explorador de arquivos funcionando na UI — backend em `internal/domain/file`, HTTP em `/api/file`, e a UI (árvore, menu de contexto, editor Monaco) portada no branch do frontend. Pendência conhecida: a árvore não se atualiza sozinha — `files:changed` está mapeado para `collection.changed`, que nada no Go publica ainda; ver [[Collection (Go)]] na Fase 8
- [x] Contenção compartilhada com o sandbox — `internal/core/pathx`
- [x] Sem superfície CLI nem MCP — `Service` não é `command.Descriptor`; roteado direto em `internal/transport/fileapi`, fora do registry

### Escopo desta fase (backend)

`Service` implementa exatamente a interface Go do topo: `Tree`, `Read`, `Write`, `Move`, `Delete`, `Diff` — não os 11 endpoints do `FractalFileController` original (`list`/`search`/`explorer`/`changes`/`content` foram deliberadamente consolidados ou descartados; `Write` também faz o papel do `create` original, já que a interface Go não tem um método `Create` separado). Ficou de fora, sinalizado como pendência e não como decisão definitiva:

- Contexto de explorer `task`/`branch` (ver `FractalFileExplorerContextSchema` no original) — `Tree`/`Read`/`Write`/`Diff` hoje só enxergam a raiz do workspace. `Diff` sempre compara contra `HEAD` do repositório do workspace.
- A lista de mudanças (`changes`) e o snapshot completo de git status do explorer original — `Diff` responde por caminho único, sob demanda; não há endpoint que liste todos os arquivos alterados de uma vez.
- Busca por nome (`search`) — não portada.
