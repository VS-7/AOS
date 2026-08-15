---
tags: [adr, decisao, persistencia, concorrencia]
aliases: [ADR-0012, Escrita Atômica, Lock]
fase: 1
status: em-construcao
origem: "[[Modelo de Persistência]]"
---

# ADR-0012 — Escrita atômica e lock por arquivo

> Pai: [[AOS]] · Origem no original: [[Modelo de Persistência]] · Detalhe técnico: [[Collections Engine]] · Fase: 1

## Contexto

O original grava registros com escrita direta e **sem lock** (defeito #17). Três fontes de concorrência real, todas documentadas na engenharia reversa, colidem nesse ponto:

1. **Worker de jobs com concorrência 20** ([[Jobs e Queues]]) — vários jobs do mesmo workspace em paralelo.
2. **"Agent universes"** ([[Memory]]) — múltiplas instâncias paralelas do mesmo agente compartilhando o mesmo grafo de memória, sem escopo privado. O aviso do próprio produto: *"What you store here, every parallel self sees."*
3. **Nove processos MCP simultâneos** observados na máquina ([[Instalação Local Observada]]), todos clientes do mesmo servidor.

Somam-se dois problemas distintos:

- **Escrita não atômica** — um crash no meio de um `write` deixa um `.memory.md` truncado, com frontmatter YAML inválido. O registro fica ilegível e a coleção inteira falha ao carregar.
- **Escrita concorrente** — dois writers no mesmo caminho produzem intercalação ou perda silenciosa (last-write-wins sem detecção).

O [[Gateway]] tem o mesmo problema em outra forma: não usa lock de PID, apenas o par `pid` + liveness, e duas invocações simultâneas de `start()` podem ambas passar pela checagem (defeito #18).

## Decisão

**Escrita atômica sempre**, via temp + rename no mesmo diretório:

```go
// internal/core/collections/atomic.go

// WriteFileAtomic writes data to path atomically: it creates a temp file in the
// same directory (same filesystem, so rename is atomic), fsyncs it, renames over
// the target, then fsyncs the directory so the rename itself is durable.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once the rename succeeds

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return fsyncDir(dir)
}
```

**Lock por caminho, hierárquico:**

```go
// internal/core/collections/lock.go

// PathLock serializes writers per canonical path within the process, and across
// processes via an advisory lock file. In-process locks are keyed by the cleaned
// absolute path so that "./a/../b.md" and "b.md" contend on the same mutex.
type PathLock struct{ /* sharded map[string]*sync.Mutex + flock handle */ }

func (l *PathLock) With(ctx context.Context, path string, fn func() error) error
```

- **Intra-processo:** mapa fragmentado de mutexes por caminho canônico. Barato e suficiente para o caso comum (um daemon, N goroutines).
- **Inter-processo:** lock advisory por arquivo (`flock` em Unix, `LockFileEx` no Windows) via `github.com/gofrs/flock`. Necessário porque o CLI pode escrever direto quando o daemon está parado.

**Lockfile no gateway:** `~/.aos/runtime/gateway/gateway.lock` com `flock` exclusivo, adquirido antes de qualquer checagem de estado. Corrige o defeito #18.

**Detecção de escrita concorrente lógica (CAS):** operações de update carregam o `mtime` + tamanho lidos e falham com `AOS_COLLECTION_CONFLICT` se o arquivo mudou entre leitura e escrita. Sem isso, dois agentes editando a mesma memória perdem uma edição sem que ninguém saiba.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Não fazer nada, como o original** | Reproduzir um defeito conhecido. A concorrência aqui não é teórica: 20 workers e N instâncias paralelas do mesmo agente. |
| **Lock global de coleção** | Simples e correto, mas serializa escritas independentes. Com 20 workers, vira gargalo. |
| **Journal / WAL próprio** | Recuperação real de crash. Descartado: complexidade de banco no lugar onde a escolha foi *não* ter banco ([[ADR-0004 Collections em Markdown]]). O `rename` atômico já dá a garantia essencial — o arquivo está inteiro ou não está. |
| **CAS por hash de conteúdo** | Mais preciso que `mtime`+tamanho, mas exige reler o arquivo inteiro para validar. Adotado apenas onde o conteúdo é pequeno e o conflito é caro (memórias). |

## Consequências

**Positivas**
- Um `.memory.md` nunca fica truncado. Crash no meio de uma escrita deixa o arquivo anterior intacto.
- Dois agentes paralelos editando o mesmo registro produzem um erro explícito com CTA (`recarregue e reaplique`), não perda silenciosa.
- O gateway deixa de ter janela de corrida no `start`.

**Negativas**
- **Custo de `fsync`.** Duas sincronizações por escrita (arquivo + diretório) custam alguns milissegundos em disco lento. Aceitável para a taxa de escrita real; configurável (`AOS_FSYNC=off`) para ambientes de teste.
- **Locks advisory não são obrigatórios.** Um editor de texto do usuário salvando o mesmo arquivo ignora o lock. É inerente ao modelo de arquivos editáveis à mão — e por isso o watcher recarrega e o CAS detecta a divergência.
- **Deadlock por ordem de locks** em operações multi-arquivo. Mitigação: locks sempre adquiridos em ordem lexicográfica de caminho, verificado em revisão e coberto por teste com `-race`.

## Status

**Aceito.** Corrige os defeitos #17 e #18.
