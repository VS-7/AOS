---
tags: [critico, seguranca, sandbox, runtime]
aliases: [Sandbox Go, Contenção, Permissões]
fase: 5
status: especificado
origem: "[[Sandbox]]"
---

# Sandbox (Go) ★

> Pai: [[AOS]] · Origem no original: [[Sandbox]] · Decisão: [[ADR-0006 Allowlist no sandbox]] · Fase: 5

## Objetivo

Confinar o acesso do agente a filesystem e execução de comandos dentro de uma raiz declarada, com permissões explícitas e uma allowlist de executáveis — corrigindo a blocklist contornável do original.

## Comportamento do original

`FractalAgentSandboxService` (710 linhas). O que está **certo** e reproduzimos sem alteração ([[Sandbox]]):

1. **Raiz confinada** — workspace, ou a worktree Git quando a [[Task (Go)]] executa em branch isolada.
2. **Resolução de path antes da checagem de contenção** — a normalização de `..` acontece primeiro, o que fecha o vetor clássico de path traversal.
3. **`.git` read-only** — o agente lê o histórico (o prompt-mestre manda reconstruir a timeline com `git log`) mas nunca escreve. Operações Git legítimas passam pelo comando `git`, não por manipulação de arquivo.
4. **Diretório de spillover read-only** — leitura permitida, escrita negada.
5. **Permissão default `["read"]`** — somente leitura, salvo declaração explícita.
6. **`originalSize` / `omittedSize`** no resultado de comando — o agente sabe **quanto** foi cortado, em vez de assumir que viu tudo.
7. **Timeout default de 120 s**, com `timeoutMs: null` desabilitando explicitamente.
8. **PATH mínimo no Windows** — `System32` + `WindowsPowerShell`, sem diretórios do usuário.
9. **`glob` sempre exclui `.git`.**

O que está **errado** e corrigimos: a blocklist por basename, contornável com `bash -c`, `python -c`, `find -delete`, `git clean -fdx` e `node -e`. Ver [[ADR-0006 Allowlist no sandbox]].

## Design em Go

### Interfaces segregadas

```go
// internal/runtime/sandbox/port.go
package sandbox

type FileReader interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	Stat(ctx context.Context, path string) (FileInfo, error)
}

type FileWriter interface {
	WriteFile(ctx context.Context, path string, data []byte) error
	Mkdir(ctx context.Context, path string) error
	Remove(ctx context.Context, path string) error
}

type Globber interface {
	Glob(ctx context.Context, pattern string, opts GlobOptions) ([]string, error)
}

type CommandRunner interface {
	Run(ctx context.Context, c Command) (Result, error)
	Start(ctx context.Context, c Command) (Handle, error) // long-running jobs
}
```

Um `*Sandbox` implementa as quatro. Cada tool recebe só a de que precisa — a tool `Read` **não pode** escrever porque não tem o método ([[SOLID no Go]], ISP).

### Construção

```go
// internal/runtime/sandbox/sandbox.go

type Options struct {
	WorkspacePath string
	WorktreePath  string      // when set, becomes the root — task isolation
	Permissions   Permissions // default: read only
	Exec          ExecPolicy
	TmpDir        string      // ~/.aos/tmp — readable, never writable
	Timeout       time.Duration
	MaxOutputChars int
}

type Permissions struct {
	Read, Write, Delete, Execute bool
}

func New(o Options) (*Sandbox, error)
```

### Contenção de path — a ordem é a segurança

```go
// internal/runtime/sandbox/path.go

// resolve normalizes BEFORE containment is checked. Order matters: normalizing
// first collapses "..", so a traversal attempt becomes a path that simply falls
// outside the root and is rejected. Symlinks are resolved too — a symlink
// inside the root pointing outside is not an escape hatch.
func (s *Sandbox) resolve(p string) (string, error) {
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(s.root, p)
	}
	abs = filepath.Clean(abs)

	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		real = abs // a file about to be created: validate its parent instead
		if parent, perr := filepath.EvalSymlinks(filepath.Dir(abs)); perr == nil {
			real = filepath.Join(parent, filepath.Base(abs))
		}
	}
	return real, nil
}

func (s *Sandbox) checkFS(p string, op Op) error {
	real, err := s.resolve(p)
	if err != nil {
		return err
	}

	inRoot := within(real, s.root)
	inTmp := within(real, s.tmpDir)

	if !inRoot && !inTmp {
		return errPathOutside(real, s.root)
	}
	if inTmp && op != OpRead {
		return errTmpReadOnly(real)
	}
	if isGitPath(real) && op != OpRead {
		return errGitReadOnly(real)
	}
	if !s.perms.allows(op) {
		return errPermissionDenied(op, real)
	}
	return nil
}

// within compares cleaned paths with a separator guard, so "/a/bc" is not
// considered inside "/a/b".
func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
```

O tratamento de symlink é **adição nossa**: o original resolve `..` mas a engenharia reversa não registra resolução de link simbólico. Um link dentro da raiz apontando para fora seria escape.

### Execução — allowlist em três camadas

```go
// internal/runtime/sandbox/exec.go

type ExecPolicy struct {
	Policy     Policy   // PolicyAllowlist | PolicyDenyAll
	Allow      []string // binary names or absolute paths
	DenyArgs   []string // glob patterns matched against the full command line
	AllowShell bool
}

// VerifyExec applies three layers, in order:
//  1. the agent must hold the execute permission at all;
//  2. the binary is resolved on disk (LookPath) and its canonical path is
//     matched against the allowlist — never the basename, which is what makes
//     the original's blocklist trivially bypassable;
//  3. the rendered command line is matched against DenyArgs, and a shell is
//     only reachable when AllowShell is set.
func (s *Sandbox) VerifyExec(name string, args []string) error {
	if !s.perms.Execute {
		return errExecPermissionRequired()
	}

	bin, err := exec.LookPath(name)
	if err != nil {
		return errExecNotFound(name)
	}
	real, err := filepath.EvalSymlinks(bin)
	if err != nil {
		return err
	}

	if !s.exec.allows(real, filepath.Base(real)) {
		return apperr.New("SANDBOX_EXEC_NOT_ALLOWED").
			Causer("sandbox.VerifyExec").
			Msgf("command %q is not in this agent's allowlist", name).
			Issue("resolved", real).
			CTA(apperr.CallToAction{
				Label:   "add the binary to the agent's policy",
				Command: fmt.Sprintf("add %q under sandbox.exec.allow in the agent file", filepath.Base(real)),
			})
	}

	if isShell(real) && !s.exec.AllowShell {
		return errShellNotAllowed(real)
	}

	line := strings.Join(append([]string{name}, args...), " ")
	for _, pat := range s.exec.DenyArgs {
		if ok, _ := doublestar.Match(pat, line); ok {
			return errDeniedArgs(pat, line)
		}
	}
	return nil
}
```

### Execução com limites

```go
type Command struct {
	Name    string
	Args    []string
	Dir     string        // must resolve inside the root
	Env     []string      // filtered: no secrets leak into child processes
	Timeout time.Duration // 0 = default 120s; explicit -1 = no timeout
	Stdin   []byte
}

type Result struct {
	ExitCode int
	TimedOut bool
	Stdout   Stream
	Stderr   Stream
}

// Stream carries what the agent sees plus how much it did not see, mirroring
// the original's originalSize/omittedSize. Without this the agent assumes it
// saw everything.
type Stream struct {
	Content      string
	OriginalSize int
	OmittedSize  int
}

func (s *Sandbox) Run(ctx context.Context, c Command) (Result, error) {
	if err := s.VerifyExec(c.Name, c.Args); err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeoutFor(c))
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Dir = s.resolveDir(c.Dir)
	cmd.Env = s.filterEnv(c.Env)
	cmd.WaitDelay = 5 * time.Second // SIGKILL after the SIGTERM grace period
	// ...
}
```

**`filterEnv` é adição nossa.** O processo filho recebe um ambiente mínimo, sem `AOS_TOKEN`, sem chaves de provider, sem segredos do workspace. O original passa o ambiente adiante.

### PATH mínimo

```go
// minimalPath returns a controlled PATH so the agent cannot reach arbitrary
// user-installed binaries that are not in the allowlist.
func minimalPath() string {
	switch runtime.GOOS {
	case "windows":
		return `C:\Windows\System32;C:\Windows;C:\Windows\System32\WindowsPowerShell\v1.0`
	default:
		return "/usr/bin:/bin:/usr/sbin:/sbin"
	}
}
```

Binários da allowlist fora desse PATH são resolvidos por caminho absoluto no momento da verificação e invocados assim.

### Política declarada no agente

```yaml
# .aos/agents/luara/agent.md — frontmatter
sandbox:
  permissions: [read, write, execute]
  exec:
    policy: allowlist
    allow: [git, go, task, node, npm, pnpm, rg, ls, cat]
    denyArgs:
      - "git push --force*"
      - "git clean*"
      - "* --no-verify*"
    allowShell: false
```

Ausente o bloco: `permissions: [read]`, `policy: deny-all`.

## Decisões e divergências

> [!decision] Allowlist substitui blocklist
> A mudança central. Ver [[ADR-0006 Allowlist no sandbox]] para a análise dos contornos que motivam.

> [!decision] Symlinks resolvidos
> Adição. Sem `EvalSymlinks`, um link dentro da raiz apontando para fora é escape silencioso.

> [!decision] Ambiente filtrado no processo filho
> Adição. Um `env` num shell permitido não deve imprimir o token da API.

> [!decision] O sandbox não é fronteira de segurança dura — e nós dizemos isso
> A engenharia reversa conclui, sobre o original: *"Quem depender do sandbox como fronteira de segurança dura estará confiando em algo que ele não é."* Com allowlist a situação melhora muito, mas um binário permitido continua podendo fazer o que sabe fazer. A proteção é em profundidade: allowlist + `denyArgs` + hooks + aprovação humana + prompt-mestre. Isolamento real exige sandbox do SO ou container, registrado como evolução em [[Segurança e Hardening]].

## Testes

- **Traversal:** `../../etc/passwd`, `./a/../../b`, path absoluto fora da raiz — todos negados com `AOS_SANDBOX_PATH_OUTSIDE`.
- **Symlink:** link dentro da raiz apontando para `/etc` é negado.
- **Prefixo:** raiz `/a/b` não contém `/a/bc` (teste do bug clássico de comparação por prefixo de string).
- **`.git`:** leitura permitida, escrita e remoção negadas, inclusive em subdiretórios.
- **Tmp:** leitura permitida, escrita negada.
- **Allowlist — os contornos do original:** `bash -c "rm -rf /"`, `/bin/sh -c dd`, `python -c shutil.rmtree`, `find . -delete`, `node -e fs.rmSync`, `git clean -fdx` — **todos negados**. Teste tabular explícito, cada linha citando o contorno que endereça.
- **Resolução real:** `./rm` e `/bin/rm` são reconhecidos como o mesmo binário.
- **`denyArgs`:** `git push --force origin main` negado com `git` permitido.
- **Shell:** negado sem `allowShell`; permitido com, e ainda sujeito a `denyArgs`.
- **Timeout:** comando que dorme 5 s com timeout de 1 s é morto; `TimedOut` verdadeiro; processo filho não sobrevive (verificado por `ps`).
- **Truncagem:** saída maior que o limite preenche `OmittedSize` corretamente.
- **Env filtrado:** `env` num shell permitido não contém `AOS_TOKEN` nem chaves de provider.
- **Contrato:** a suíte roda contra o sandbox real e contra o fake em memória ([[Testes de Contrato de Port]]).

## Critério de pronto

- [ ] Todos os contornos documentados do original negados, com teste nomeando cada um
- [ ] Symlinks resolvidos antes da checagem de contenção
- [ ] Política por agente lida do frontmatter e aplicada
- [ ] Ambiente filtrado no processo filho
- [ ] `originalSize` / `omittedSize` presentes em todo resultado de comando
- [ ] Suíte de contrato verde para as quatro interfaces
