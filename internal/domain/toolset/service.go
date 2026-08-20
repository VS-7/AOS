package toolset

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/activity"
)

// Service is the toolset aggregate: configuration in Repository, execution
// through Adapters, and one audit record per Call regardless of outcome.
type Service struct {
	repo       Repository
	adapters   Adapters
	activities Activities
	env        EnvResolver
	clock      Clock
	log        *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Repo       Repository
	Adapters   Adapters
	Activities Activities

	// Env resolves ${env.VAR} placeholders in a Toolset's configuration.
	Env EnvResolver

	Clock Clock
	Log   *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		repo: d.Repo, adapters: d.Adapters, activities: d.Activities,
		env: d.Env, clock: d.Clock, log: log,
	}
}

// List returns every configured toolset, each independent of what the
// repository still holds.
func (s *Service) List(ctx context.Context) ([]Toolset, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return nil, errReadFailed("List", err)
	}
	out := make([]Toolset, len(found))
	for i := range found {
		out[i] = found[i].Clone()
	}
	return out, nil
}

// GetInput names one toolset.
type GetInput struct {
	ID string `json:"id" jsonschema:"Identifier of the toolset."`
}

// Get reads one toolset's configuration.
func (s *Service) Get(ctx context.Context, in GetInput) (*Toolset, error) {
	return s.get(ctx, strings.TrimSpace(in.ID))
}

// get is the shared lookup List, Get, Call, UpdateConfig and Delete all
// resolve a toolset through, so a not-found toolset reads the same everywhere.
func (s *Service) get(ctx context.Context, id string) (*Toolset, error) {
	if id == "" {
		return nil, errNotFound(id)
	}
	found, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		// A repository failure and a missing record are both "cannot use
		// this toolset" from the caller's side; the repository's own logs
		// carry the distinction if it matters later.
		return nil, errNotFound(id)
	}
	clone := found.Clone()
	return &clone, nil
}

// CallInput names a tool call: which toolset, which tool, and its argument
// payload.
type CallInput struct {
	ID    string          `json:"id" jsonschema:"Identifier of the toolset to call."`
	Tool  string          `json:"tool" jsonschema:"Name of the tool, as published by the connected target."`
	Input json.RawMessage `json:"input,omitempty" jsonschema:"Arguments for the tool. Opaque to this service — call toolsets_get with schema:true first."`
}

// CallOutput is what a tool call returned.
type CallOutput struct {
	Result     json.RawMessage `json:"result" jsonschema:"Whatever the tool returned. Opaque to this service."`
	DurationMS int64           `json:"durationMs" jsonschema:"How long the call took, end to end, in milliseconds."`
}

// Call is the single path through which this system executes something
// outside its own process. It resolves the toolset, picks the Adapter its
// Type maps to, interpolates ${env.VAR} placeholders, connects, calls, closes
// — and records exactly one activity for the attempt, success or failure,
// through a defer that runs regardless of which of those steps failed.
//
// The toolset's tools are reached only this way: they are never registered
// into the agent's own tool list, which is the whole reason this boundary is
// one function instead of one per connection type.
func (s *Service) Call(ctx context.Context, in CallInput) (CallOutput, error) {
	id := strings.TrimSpace(in.ID)
	tool := strings.TrimSpace(in.Tool)
	if id == "" || tool == "" {
		return CallOutput{}, errCallIncomplete(id, tool)
	}

	ts, err := s.get(ctx, id)
	if err != nil {
		return CallOutput{}, err
	}
	if ts.Status == StatusDisabled {
		return CallOutput{}, errDisabled(id)
	}

	factory, ok := s.adapters[ts.Type]
	if !ok {
		return CallOutput{}, errTypeNotAvailable(ts.Type)
	}

	start := s.clock.Now()
	var callErr error
	// The audit record is written from a defer so that every return path
	// below — interpolation failing, Connect failing, Call failing, or none
	// of those — produces exactly one entry. A log that only reaches this
	// point on success would hide precisely the calls someone would go
	// looking for.
	defer func() {
		s.audit(ctx, id, tool, s.clock.Now().Sub(start), callErr)
	}()

	resolved, ierr := interpolateToolset(*ts, s.env)
	if ierr != nil {
		callErr = ierr
		return CallOutput{}, callErr
	}

	adapter := factory()
	defer func() { _ = adapter.Close() }()

	if cerr := adapter.Connect(ctx, resolved); cerr != nil {
		callErr = errConnectFailed(id, cerr)
		return CallOutput{}, callErr
	}

	result, cerr := adapter.Call(ctx, tool, in.Input)
	if cerr != nil {
		callErr = errCallFailed(id, tool, cerr)
		return CallOutput{}, callErr
	}

	return CallOutput{Result: result, DurationMS: s.clock.Now().Sub(start).Milliseconds()}, nil
}

// audit records one Call attempt. It never carries the input payload — that
// can hold the user's data, and an audit log that stores it becomes the thing
// that leaks — only which toolset, which tool, how long, and whether it
// worked.
func (s *Service) audit(ctx context.Context, id, tool string, dur time.Duration, callErr error) {
	outcome := "succeeded"
	if callErr != nil {
		outcome = "failed"
	}
	_, err := s.activities.Publish(ctx, activity.PublishInput{
		Namespace: "toolset",
		Event:     "call",
		Title:     fmt.Sprintf("Toolset %s: %s %s", id, tool, outcome),
		Data: map[string]any{
			"toolset":    id,
			"tool":       tool,
			"outcome":    outcome,
			"durationMs": dur.Milliseconds(),
		},
	})
	if err != nil {
		s.log.Error("failed to publish toolset call audit",
			"toolset", id, "tool", tool, "outcome", outcome, "err", err)
	}
}

// UpdateConfigInput changes the connection configuration of a toolset. A nil
// pointer leaves that field unchanged; Args, Env and Headers, when non-nil,
// replace the field wholesale — there is no per-key merge, so removing one
// header means sending the whole remaining set.
type UpdateConfigInput struct {
	ID string `json:"id" jsonschema:"Identifier of the toolset to reconfigure."`

	Description *string           `json:"description,omitempty" jsonschema:"New description. Omit to leave unchanged."`
	Status      *Status           `json:"status,omitempty" jsonschema:"New lifecycle status: enabled or disabled. Omit to leave unchanged."`
	Command     *string           `json:"command,omitempty" jsonschema:"New command for mcp-server::stdio. Omit to leave unchanged."`
	Args        []string          `json:"args,omitempty" jsonschema:"New argument list, replacing the old one wholesale."`
	Env         map[string]string `json:"env,omitempty" jsonschema:"New environment, replacing the old one wholesale."`
	BaseURL     *string           `json:"baseUrl,omitempty" jsonschema:"New base URL. Omit to leave unchanged."`
	Headers     map[string]string `json:"headers,omitempty" jsonschema:"New headers, replacing the old ones wholesale."`
}

// UpdateConfig changes the describable parts of a toolset's configuration.
func (s *Service) UpdateConfig(ctx context.Context, in UpdateConfigInput) (*Toolset, error) {
	id := strings.TrimSpace(in.ID)
	current, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.Status != nil {
		current.Status = *in.Status
	}
	if in.Command != nil {
		current.Command = strings.TrimSpace(*in.Command)
	}
	if in.Args != nil {
		current.Args = cloneSlice(in.Args)
	}
	if in.Env != nil {
		current.Env = cloneStrings(in.Env)
	}
	if in.BaseURL != nil {
		current.BaseURL = strings.TrimSpace(*in.BaseURL)
	}
	if in.Headers != nil {
		current.Headers = cloneStrings(in.Headers)
	}
	current.UpdatedAt = s.clock.Now()

	// The repository is handed its own independent copy: current.Clone()
	// rather than current itself, so that what the repository ends up
	// storing does not alias the Args/Env/Headers this function is about to
	// return to the caller. Without this, a caller mutating the toolset it
	// got back would corrupt what every later Get and List reads.
	toWrite := current.Clone()
	if err := s.repo.Update(ctx, &toWrite, collections.Version{}); err != nil {
		return nil, errWriteFailed("UpdateConfig", err)
	}
	return current, nil
}

// DeleteInput names one toolset to remove.
type DeleteInput struct {
	ID string `json:"id" jsonschema:"Identifier of the toolset to delete."`
}

// DeleteOutput confirms what was removed.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the toolset that was deleted."`
}

// Delete removes a toolset's configuration. It is idempotent, like the
// repository underneath it: deleting what is already gone succeeds rather
// than erroring, so a retried delete after a dropped response is safe.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := strings.TrimSpace(in.ID)
	if err := s.repo.Delete(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: id}, nil
}
