# Integration notes for the `instruction` domain

**Done**, all three sections below. `wire.go` builds `instructionSvc` and
exposes it on `App.Instructions`; `internal/app/runtime.go`'s `reader`
struct carries an `instructions *instruction.Service` field and populates
`Inventory.Instructions` with every applicable instruction's name
(`TestTheAssembledPromptCarriesEveryInventoryCategory`, `internal/app`); the
prompt's trusted block renders them (see
`docs/04 - Domínio/Instruction (Go).md`'s own "Estado atual" for the
approval-gating work layered on top of this). `frontend/src/lib/command-map.ts`
has `instruction.list`/`.getById`/`.create`/`.update`/`.delete`
(`instruction` is not in `DORMANT_DOMAINS`).

**Still open, not a wiring gap:** `frontend/src/features/instruction/` has
only 1 file (interfaces, no page component) — same situation as `tunnel`.
The backend and command mapping are both ready for a settings panel; none
exists yet.
