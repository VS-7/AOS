package skill

import (
	"context"
	"strings"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "skills",
	Tool:    "Skills",
	Summary: "Installable packages of capability.",
	Doc: `A skill is not documentation: it brings agents, memories, routines,
collections, views and toolsets together as one unit, verified against its
own declared manifest and applied only once a person has agreed.

## Commands
- **list** — every skill installed in this workspace
- **install** — verify a package's manifest, ask a person, and apply it
- **create** — the same operation, named for a script or an agent assembling
  a package from a local directory rather than a person clicking install
- **update** — turn a skill's hooks and toolsets on or off, without removing it
- **delete** — uninstall a skill and everything it brought

## When to use
- **A capability that should persist across sessions, with its own agents and
  data:** install it as a skill rather than improvising the pieces by hand

## When NOT to use
- Not for a single collection or view with no agent behind it — declare that
  directly instead of packaging it`,
	Hint: `Content beyond what a package's manifest declares is refused at install, not
trimmed and applied anyway — a manifest is a promise this system checks, not
takes on faith.

install and create never authorise themselves: a person is asked before
anything is written, every time, regardless of which name called it.`,
}

// ListInput takes nothing — List has no filter — but every command input
// still carries a reason.
type ListInput struct {
	command.Reasoning
}

// InstallRequest is the wire shape of skills_install and skills_create.
//
// It is a distinct type from InstallInput (install.go) on purpose:
// InstallInput.AcceptedAll is a Go func, not something a JSON schema can
// describe or a model can set — that is what keeps "an agent calling
// skills_install does not authorise itself" true by construction (ADR-0007).
// Every call arriving through this surface leaves AcceptedAll nil, which
// Install reads as "ask a person" — there is no field here that could ever
// set it to anything else.
type InstallRequest struct {
	Source string `json:"source" jsonschema:"Where the package is. A local directory today." validate:"required,notblank"`
	Ref    string `json:"ref,omitempty" jsonschema:"Version to install — a tag or a commit. Empty installs whatever the source currently holds."`

	command.Reasoning
}

// UpdateInput turns a skill's live behaviour on or off. Nothing else about
// an installed skill is mutable in place — content, permissions and
// inventory change by uninstalling and reinstalling, not by editing a field.
type UpdateInput struct {
	ID     string `json:"id" jsonschema:"Identifier of the skill to update." validate:"required,notblank"`
	Active *bool  `json:"active,omitempty" jsonschema:"Turn this skill's hooks and toolsets on (true) or off (false). Omit to leave unchanged."`

	command.Reasoning
}

// DeleteInput names the skill to uninstall.
type DeleteInput struct {
	ID string `json:"id" jsonschema:"Identifier of the skill to uninstall." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput confirms what was removed.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the skill that was uninstalled."`
}

// Register declares the group on the registry.
func Register(reg *command.Registry, inst *Installer) {
	reg.DescribeGroup(GroupDoc)

	listHandler := func(ctx context.Context, _ ListInput) (ListOutput, error) {
		return inst.List(ctx)
	}
	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "skills",
		Name:    "list",
		Summary: "List every installed skill.",
		Doc:     "Every skill installed in this workspace, active or not, with what it brought.",
		Examples: []command.Example{
			{Description: "everything installed", Input: ListInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List skills", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     listHandler,
	})

	// install and create share one handler: the same verified, consented
	// write, reached under the name a person clicking a button expects and
	// the name a script or an agent installing from a local directory
	// expects. Neither ever sets AcceptedAll — see InstallRequest's own doc.
	installHandler := func(ctx context.Context, in InstallRequest) (*Skill, error) {
		return inst.Install(ctx, InstallInput{Source: in.Source, Ref: in.Ref})
	}
	command.MustRegister(reg, command.Command[InstallRequest, *Skill]{
		Group:   "skills",
		Name:    "install",
		Summary: "Install a skill package.",
		Doc: `Installs a capability: the agents, collections, views, routines and
memories it ships, as one unit.

The package's manifest is verified against what it actually contains before
anything is written, and a person is asked before it is applied. An agent
does not authorise this on its own.`,
		Examples: []command.Example{
			{Description: "install from a local directory", Input: InstallRequest{Source: "./skills/crm"}},
			{Description: "install a specific version", Input: InstallRequest{Source: "./skills/crm", Ref: "v1.2.0"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Install a skill", DestructiveHint: true, OpenWorldHint: true},
		Handler:     installHandler,
	})
	command.MustRegister(reg, command.Command[InstallRequest, *Skill]{
		Group:   "skills",
		Name:    "create",
		Summary: "Install a skill package, from a script or an agent assembling one.",
		Doc: `The same operation as skills_install, under the name a script or an agent
building a package from a local directory reaches for. A person is still
asked before anything is written — this name changes who is calling, not
what is checked.`,
		Examples: []command.Example{
			{Description: "install a package just assembled on disk", Input: InstallRequest{Source: "./skills/crm"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Install a skill", DestructiveHint: true, OpenWorldHint: true},
		Handler:     installHandler,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Skill]{
		Group:   "skills",
		Name:    "update",
		Summary: "Turn a skill's live behaviour on or off.",
		Doc: `Enable or disable a skill without removing it: its agents, memories and
configuration stay in place, only its hooks and toolsets stop being live.

Nothing else about an installed skill changes here — content, permissions
and inventory are what an install verified and a person consented to; to
change any of those, uninstall and install again.`,
		Examples: []command.Example{
			{Description: "disable a skill", Input: UpdateInput{ID: "crm", Active: falsePtr()}},
			{Description: "re-enable it", Input: UpdateInput{ID: "crm", Active: truePtr()}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Update a skill"},
		Handler:     inst.Update,
	})

	deleteHandler := func(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
		id := strings.TrimSpace(in.ID)
		if err := inst.Uninstall(ctx, UninstallInput{ID: id}); err != nil {
			return DeleteOutput{}, err
		}
		return DeleteOutput{ID: id}, nil
	}
	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "skills",
		Name:    "delete",
		Summary: "Uninstall a skill.",
		Doc: `Removes a skill and everything it brought: its agents, memories,
routines, collections, views, hooks and toolset connections.

Hooks and toolsets are torn down first, so nothing keeps intercepting a tool
call or holding a connection on behalf of a directory about to disappear.`,
		Examples: []command.Example{
			{Description: "uninstall a skill", Input: DeleteInput{ID: "crm"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Uninstall a skill", DestructiveHint: true},
		Handler:     deleteHandler,
	})
}

func truePtr() *bool  { v := true; return &v }
func falsePtr() *bool { v := false; return &v }

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, string) (*Skill, error)       = (*Installer)(nil).Get
	_ func(context.Context) (ListOutput, error)           = (*Installer)(nil).List
	_ func(context.Context, InstallInput) (*Skill, error) = (*Installer)(nil).Install
	_ func(context.Context, UninstallInput) error         = (*Installer)(nil).Uninstall
	_ func(context.Context, UpdateInput) (*Skill, error)  = (*Installer)(nil).Update
)
