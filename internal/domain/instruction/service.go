package instruction

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/slug"
	"github.com/OWNER/aos/internal/domain/event"
)

// Service is the instruction aggregate: CRUD over Repository, plus Applicable
// — the query the prompt assembler runs to decide which instructions belong
// in the trusted block of a given turn.
type Service struct {
	repo     Repository
	clock    Clock
	approver event.Approver
}

// Deps is what the service is built from.
type Deps struct {
	Repo  Repository
	Clock Clock

	// Approver is who Create and Update ask before writing anything, when
	// the caller is an agent — see requestApprovalIfAgent's own doc comment.
	Approver event.Approver
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	return &Service{repo: d.Repo, clock: d.Clock, approver: d.Approver}
}

// requestApprovalIfAgent gates a workspace-wide policy change behind a
// human's real-time approval, when the caller is an agent — never when a
// human calls Create or Update directly, which is already that human's own
// decision and needs nobody else's.
//
// This is a bespoke call, the same shape skill.Installer's own install
// approval already is, and for the same reason: nothing in this system
// today turns a plain RiskMedium classification into an actual "ask" on its
// own. agentloop.EventHooks.ApproveTool only reaches a human when a
// PreToolUse hook explicitly returns PermissionAsk, and no hook that
// converts risk into ask is registered by default — Risk is informational,
// read only once a hook has already decided to ask, to label how serious
// the pending request is. A prior version of this comment claimed the
// generic path already handled this; it does not, and instructions_create
// and instructions_update are consequential enough — workspace-wide policy,
// per ADR-0007's own "consultive" classification — not to wait for a
// system-wide mechanism that would change what every other unannotated
// mutation in the registry does, a decision this package has no business
// making on its own.
func (s *Service) requestApprovalIfAgent(ctx context.Context, op, id, reason string) error {
	if !identity.IsAgent(ctx) {
		return nil
	}
	if s.approver == nil {
		return errNotApproved(op, id, "no approval channel is available in this run mode")
	}
	res, err := s.approver.RequestApproval(ctx, event.ApprovalRequest{
		ToolName: "instructions_" + strings.ToLower(op),
		Risk:     event.RiskMedium,
		Reason:   reason,
	})
	if err != nil {
		return errNotApproved(op, id, "the approval channel failed: "+err.Error())
	}
	if !res.Approved {
		return errNotApproved(op, id, res.Reason)
	}
	return nil
}

// ListInput selects the instructions List returns. Skill, given, narrows to
// one skill's own instructions — the same filter the original's list command
// takes. Query is a naive substring match over name, description and
// content, not the weighted fuzzy search the original's schema describes:
// this package has no full-text index of its own, and a plain match is
// honest about that rather than promising ranking it cannot do.
type ListInput struct {
	Skill string `json:"skill,omitempty" jsonschema:"Filter to one skill's own instructions."`
	Query string `json:"query,omitempty" jsonschema:"Substring match over name, description and content."`

	command.Reasoning
}

// ListOutput is every instruction List found, content included — unlike the
// original, which excludes it from a list response as an optimization. This
// package accepts the cost: instructions are few and read rarely enough that
// a second round trip to fetch content a caller almost always wants next is
// the worse trade.
type ListOutput struct {
	Instructions []Instruction `json:"instructions" jsonschema:"Every instruction matching the filter."`
	Total        int           `json:"total" jsonschema:"How many there are."`
}

// List returns every instruction matching the filter.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return ListOutput{}, errReadFailed("List", err)
	}

	skill := strings.TrimSpace(in.Skill)
	query := strings.ToLower(strings.TrimSpace(in.Query))
	out := make([]Instruction, 0, len(found))
	for _, i := range found {
		if skill != "" && i.Skill != skill {
			continue
		}
		if query != "" && !matchesQuery(i, query) {
			continue
		}
		out = append(out, i.Clone())
	}
	return ListOutput{Instructions: out, Total: len(out)}, nil
}

func matchesQuery(i Instruction, query string) bool {
	return strings.Contains(strings.ToLower(i.Name), query) ||
		strings.Contains(strings.ToLower(i.Description), query) ||
		strings.Contains(strings.ToLower(i.Content), query)
}

// GetInput names one instruction.
type GetInput struct {
	ID string `json:"id" jsonschema:"Identifier of the instruction." validate:"required,notblank"`

	command.Reasoning
}

// Get reads one instruction in full.
func (s *Service) Get(ctx context.Context, in GetInput) (*Instruction, error) {
	return s.get(ctx, strings.TrimSpace(in.ID))
}

// get is the shared lookup every method resolves an instruction through, so
// a not-found instruction reads the same everywhere.
func (s *Service) get(ctx context.Context, id string) (*Instruction, error) {
	if id == "" {
		return nil, errNotFound(id)
	}
	found, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, errNotFound(id)
	}
	clone := found.Clone()
	return &clone, nil
}

// CreateInput composes a new instruction. ID is optional: left empty, it is
// derived from Name the way the original derives its own slug — see Create.
//
// Creating an instruction is a workspace-wide policy change, not a private
// one — exactly the "consultive" classification the design doc calls for
// (ADR-0007) — so an agent calling Create needs a human's real-time
// approval first; see requestApprovalIfAgent's own doc comment for why that
// check lives in this package rather than in a generic hook.
type CreateInput struct {
	ID          string   `json:"id,omitempty" jsonschema:"Identifier for the instruction. Derived from Name when omitted."`
	Name        string   `json:"name" jsonschema:"Human name of the instruction. Example: \"Feature Protocol\"." validate:"required,notblank"`
	Type        string   `json:"type,omitempty" jsonschema:"Categorization for organizational purposes. Example: standards, patterns, workflows."`
	Description string   `json:"description,omitempty" jsonschema:"What this instruction is for."`
	Skill       string   `json:"skill,omitempty" jsonschema:"The skill this instruction ships with, when it is skill-scoped."`
	Paths       []string `json:"paths,omitempty" jsonschema:"Glob patterns matching files this instruction applies to. Empty applies to the whole workspace."`
	Content     string   `json:"content,omitempty" jsonschema:"Markdown body of the instruction."`

	command.Reasoning
}

// Create declares a new instruction. The id is the slugified name when one is
// not given explicitly, matching the original's own derivation
// (`name.toLowerCase().replace(/\s+/g, "-")`) closely enough that a workspace
// migrating from the original keeps the same ids — slug.Generate is stricter
// about punctuation, which only ever produces a cleaner id, never a
// different one for the names the original actually accepted.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Instruction, error) {
	name := strings.TrimSpace(in.Name)
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = slug.Generate(name)
	}
	if id == "" {
		return nil, errNameRequired()
	}

	if _, err := s.get(ctx, id); err == nil {
		return nil, errAlreadyExists(id)
	}

	reason := fmt.Sprintf("declare the workspace-wide instruction %q, which shapes every agent's behavior", name)
	if err := s.requestApprovalIfAgent(ctx, "Create", id, reason); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	instr := Instruction{
		ID:          id,
		Name:        name,
		Type:        strings.TrimSpace(in.Type),
		Description: in.Description,
		Skill:       in.Skill,
		Paths:       clonePaths(in.Paths),
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
		Content:     in.Content,
	}
	if err := s.repo.Create(ctx, &instr); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	return &instr, nil
}

// UpdateInput changes an existing instruction. A nil pointer leaves that
// field unchanged; Paths, given at all, replaces the field wholesale — the
// same "no per-key merge" rule toolset.UpdateConfigInput already documents.
type UpdateInput struct {
	ID string `json:"id" jsonschema:"Identifier of the instruction to update." validate:"required,notblank"`

	Name        *string  `json:"name,omitempty" jsonschema:"New name. Omit to leave unchanged."`
	Type        *string  `json:"type,omitempty" jsonschema:"New categorization. Omit to leave unchanged."`
	Description *string  `json:"description,omitempty" jsonschema:"New description. Omit to leave unchanged."`
	Paths       []string `json:"paths,omitempty" jsonschema:"New glob patterns, replacing the old ones wholesale."`
	Content     *string  `json:"content,omitempty" jsonschema:"New body. Omit to leave unchanged."`
	Active      *bool    `json:"active,omitempty" jsonschema:"New lifecycle state. Omit to leave unchanged."`

	command.Reasoning
}

// Update changes the describable parts of an instruction. Like Create, it
// asks a human before writing when the caller is an agent — see
// requestApprovalIfAgent's own doc comment.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Instruction, error) {
	id := strings.TrimSpace(in.ID)
	current, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.Type != nil {
		current.Type = strings.TrimSpace(*in.Type)
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.Paths != nil {
		current.Paths = clonePaths(in.Paths)
	}
	if in.Content != nil {
		current.Content = *in.Content
	}
	if in.Active != nil {
		current.Active = *in.Active
	}
	current.UpdatedAt = s.clock.Now()

	reason := fmt.Sprintf("change the workspace-wide instruction %q, which shapes every agent's behavior", id)
	if err := s.requestApprovalIfAgent(ctx, "Update", id, reason); err != nil {
		return nil, err
	}

	toWrite := current.Clone()
	if err := s.repo.Update(ctx, &toWrite, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	return current, nil
}

// DeleteInput names one instruction to remove.
type DeleteInput struct {
	ID string `json:"id" jsonschema:"Identifier of the instruction to delete." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput confirms what was removed.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the instruction that was deleted."`
}

// Delete removes an instruction. Idempotent, like every native record's
// delete in this codebase: deleting what is already gone succeeds.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := strings.TrimSpace(in.ID)
	if err := s.repo.Delete(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: id}, nil
}

// Applicable returns every active instruction that applies to at least one of
// paths — or that has no Paths at all, which means workspace-wide, the
// default and the common case per the design doc's own decision. It is what
// internal/runtime/prompt calls to build the trusted instruction block for a
// turn; see internal/domain/instruction/INTEGRATION.md for the exact wiring.
//
// Unlike memory.ScopesMode, there is no strict variant: an instruction with
// no Paths is *always* workspace-wide, never excluded, because that is what
// "no paths given" means for a policy rule — there is no reading of the
// original or the design doc where an unscoped instruction should sometimes
// not apply.
func (s *Service) Applicable(ctx context.Context, paths []string) ([]Instruction, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return nil, errReadFailed("Applicable", err)
	}
	out := make([]Instruction, 0, len(found))
	for _, i := range found {
		if !i.Active {
			continue
		}
		if len(i.Paths) == 0 || matchesAnyPath(i.Paths, paths) {
			out = append(out, i.Clone())
		}
	}
	return out, nil
}

// matchesAnyPath reports whether any of candidates matches any of patterns.
// An invalid pattern matches nothing rather than failing the whole call — the
// same defensive rule memory.matchScopes already applies to its own globs.
func matchesAnyPath(patterns, candidates []string) bool {
	for _, pattern := range patterns {
		for _, candidate := range candidates {
			if ok, err := doublestar.Match(pattern, candidate); err == nil && ok {
				return true
			}
		}
	}
	return false
}

func clonePaths(p []string) []string {
	if p == nil {
		return nil
	}
	out := make([]string, len(p))
	copy(out, p)
	return out
}
