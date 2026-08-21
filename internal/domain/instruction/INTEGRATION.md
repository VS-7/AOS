# Integration notes for the `instruction` domain

Not applied here on purpose — `internal/app/wire.go`, `internal/app/runtime.go`,
`internal/runtime/prompt/*` and `frontend/src/lib/command-map.ts` are shared
files another pass integrates centrally, to avoid an N-way merge conflict
across the domains built alongside this one. Three separate changes are
needed.

## 1. `internal/app/wire.go`

Import:

```go
"github.com/OWNER/aos/internal/domain/instruction"
```

In `newRepoSet`/`repoSet` (mirrors every other native collection):

```go
instructionModel, err := collections.ModelOf[instruction.Instruction]("instructions")
if err != nil {
	return repoSet{}, err
}
```

```go
instructions: fscollections.New(root, instructionModel,
	fscollections.WithLock[instruction.Instruction](lock),
	fscollections.WithIndex[instruction.Instruction](index),
),
```

(add `instructions *fscollections.Repo[instruction.Instruction]` to the
`repoSet` struct — no `WithPublisher`: nothing subscribes to instruction
changes over the realtime hub yet, matching `agents`/`tasks`/etc., not the
ecosystem four)

Service construction, near the other native domains:

```go
instructionSvc := instruction.NewService(instruction.Deps{
	Repo: repos.instructions, Clock: clock,
})
```

Registration, alongside the others:

```go
instruction.Register(reg, instructionSvc)
```

`App` struct field:

```go
Instructions *instruction.Service
```

...and in the `New` function's final `return &App{...}`:

```go
Instructions: instructionSvc,
```

## 2. Prompt assembly — wiring instructions into the trusted block

This is the part that gives the domain its actual purpose (see `entity.go`'s
package doc): `internal/runtime/prompt.Inventory` **already has** an
`Instructions []string` field, reserved with a comment anticipating exactly
this task (`internal/runtime/prompt/inventory.go:23`). It is currently never
populated. Two changes:

**a) `internal/app/runtime.go`**, the `reader` struct (used to build
`prompt.NewAssembler`'s `Reader` port):

```go
type reader struct {
	workspaces   *workspace.Service
	agents       *agent.Service
	memories     *memory.Service
	instructions *instruction.Service // add this
}
```

In `reader.Inventory`, alongside the existing `if r.agents != nil { ... }`
block — list only *active* instructions applicable to the whole turn (no
specific file paths known yet at this point, so call `Applicable(ctx, nil)`,
which returns every unscoped instruction plus nothing path-scoped — the
per-file, `PostToolUse`-triggered narrowing the original does is a further
refinement, not required to close this integration point) and collect only
**names**, matching every other inventory list's "names only, content fetched
on demand" rule:

```go
if r.instructions != nil {
	if list, err := r.instructions.Applicable(ctx, nil); err == nil {
		for _, i := range list {
			out.Instructions = append(out.Instructions, i.Name)
		}
	}
}
```

Then in `wire.go`, change the `reader{...}` construction to pass
`instructions: instructionSvc`.

**b) `internal/runtime/prompt/builder.go` + `base.md`** — render the trusted
instructions block. The trust-attribute mechanism already exists and is used
three times in `builder.go` (`Attr("trust", string(TrustTrusted))` at lines
148, 164, 175) — follow the same shape for an `<instructions trust="trusted">`
(or similar; match whatever XML/tag convention the other inventory blocks
already use in `builder.go`) block that lists `Inventory.Instructions` by
name. `base.md` line 30 already introduces the concept in prose
("**Instructions** — durable behavioral rules that shape ALL agents..."); it
does not need a content change, only the structured block needs to start
being emitted. There is no test for this yet — the design doc's own "Testes"
section calls for one ("No prompt, bloco de instruções sai com
`trust=\"trusted\"`"); add it alongside `internal/runtime/prompt/prompt_test.go`'s
existing inventory-block tests.

Fetching an instruction's full `Content` on demand (not from the inventory)
is the same `instructions_get` command this package already registers — no
further plumbing needed there.

## 3. `frontend/src/lib/command-map.ts`

Remove `"instruction"` from `DORMANT_DOMAINS`.

Add entries (mirroring the existing `view.*`/`toolset.*` shape — see
`COMMAND_MAP`'s existing entries for the exact `renameIn`/`wrapOut` idiom used
there):

```ts
"instruction.list":   "instructions_list",
"instruction.get":    { key: "instructions_get",    renameIn: { instruction: "id" } },
"instruction.create": "instructions_create",
"instruction.update": { key: "instructions_update",  renameIn: { instruction: "id" } },
"instruction.delete": { key: "instructions_delete",  renameIn: { instruction: "id" } },
```

The `renameIn: { instruction: "id" }` mapping matters: the original TypeScript
schemas (`_extracted/v401/server/src/features/instruction/schemas/instruction.schema.ts`)
name the path parameter `instruction`, not `id`, for `get`/`update`/`delete` —
this package's Go `GetInput`/`UpdateInput`/`DeleteInput` use `ID` for
consistency with every other domain in this codebase (per ADR-0016, Faixa 3:
domain names are inherited, but the Go command surface's own field-naming
convention wins over the original's inconsistency where the two conflict),
so the frontend adapter is where that rename belongs, exactly as it already
does for `view.*`'s `view: "id"` mapping.

**Frontend UI**: `frontend/src/features/instruction/` has only 1 file
(a stub/barrel) — the original product's instruction UI was not fully ported.
This backend lands without a rich frontend panel. Building one is out of
scope for this task.
