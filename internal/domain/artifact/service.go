package artifact

import (
	"context"
	"log/slog"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/pathx"
)

// defaultEntrypoint is what Create scaffolds when none is given — Files.Ensure
// is the one that actually writes the placeholder page there.
const defaultEntrypoint = "index.html"

// Service is the artifact aggregate: configuration in Repository, files on
// disk through Files, and the password an artifact's by_password visibility
// checks against through PasswordHasher.
type Service struct {
	repo   Repository
	files  Files
	hasher PasswordHasher
	clock  Clock
	ids    IDs
	log    *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Repo   Repository
	Files  Files
	Hasher PasswordHasher

	Clock Clock
	IDs   IDs
	Log   *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		repo: d.Repo, files: d.Files, hasher: d.Hasher,
		clock: d.Clock, ids: d.IDs, log: log,
	}
}

// ListInput takes nothing — List has no filter — but every command input
// still carries a reason.
type ListInput struct {
	command.Reasoning
}

// List returns every artifact registered in the workspace, each independent
// of what the repository still holds.
func (s *Service) List(ctx context.Context, _ ListInput) ([]Artifact, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return nil, errReadFailed("List", err)
	}
	out := make([]Artifact, len(found))
	for i := range found {
		out[i] = found[i].Clone()
		out[i].URLs = s.urlsFor(out[i].ID)
	}
	return out, nil
}

// urlsFor computes where a client reaches id's own files. Local always
// points at internal/transport/artifactapi's mount — see URLs' own doc
// comment. Tunnel stays nil: computing a real one needs this service to
// know whether a tunnel is active, which would mean threading a
// tunnel.Service dependency through Deps for a field nothing reads yet
// (the frontend this was ported alongside only ever reads .local — see
// URLs' own doc) — a real follow-up, not a silent omission.
func (s *Service) urlsFor(id string) *URLs {
	return &URLs{Local: "/v/artifacts/" + id + "/"}
}

// GetInput names one artifact.
type GetInput struct {
	ID string `json:"id" jsonschema:"Identifier of the artifact." validate:"required,notblank"`

	command.Reasoning
}

// Get reads one artifact's configuration.
func (s *Service) Get(ctx context.Context, in GetInput) (*Artifact, error) {
	return s.get(ctx, strings.TrimSpace(in.ID))
}

// get is the shared lookup List, Get, Update, SetPassword and Delete all
// resolve an artifact through, so a not-found artifact reads the same
// everywhere.
func (s *Service) get(ctx context.Context, id string) (*Artifact, error) {
	if id == "" {
		return nil, errNotFound(id)
	}
	found, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, errNotFound(id)
	}
	clone := found.Clone()
	clone.URLs = s.urlsFor(clone.ID)
	return &clone, nil
}

// CreateInput describes a new artifact. Entrypoint is optional: Create
// scaffolds a minimal HTML file at defaultEntrypoint when it is empty,
// matching the original's behaviour.
type CreateInput struct {
	ID          string     `json:"id,omitempty" jsonschema:"Identifier for the new artifact. Generated when omitted."`
	Name        string     `json:"name" jsonschema:"Human-readable name." validate:"required,notblank"`
	Description string     `json:"description,omitempty" jsonschema:"What this artifact is."`
	Entrypoint  string     `json:"entrypoint,omitempty" jsonschema:"HTML file served as the artifact's root. A minimal one is scaffolded when omitted."`
	Visibility  Visibility `json:"visibility,omitempty" jsonschema:"One of: private, workspace, by_password. Defaults to private."`
	Skill       string     `json:"skill,omitempty" jsonschema:"Skill that owns this artifact, if any."`

	command.Reasoning
}

// Create registers a new artifact, scaffolding its entrypoint when the
// caller supplies none.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Artifact, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = s.ids.New()
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = Private
	}
	if !visibility.Valid() {
		return nil, errInvalidVisibility(string(visibility))
	}
	entrypoint := strings.TrimSpace(in.Entrypoint)
	if entrypoint == "" {
		entrypoint = defaultEntrypoint
	}

	actual, err := s.files.Ensure(ctx, id, entrypoint)
	if err != nil {
		return nil, errScaffoldFailed(id, err)
	}

	now := s.clock.Now()
	a := &Artifact{
		ID:          id,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Entrypoint:  actual,
		Visibility:  visibility,
		Skill:       strings.TrimSpace(in.Skill),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	a.URLs = s.urlsFor(a.ID)
	return a, nil
}

// UpdateInput changes the describable parts of an artifact. A nil pointer
// leaves that field unchanged.
type UpdateInput struct {
	ID string `json:"id" jsonschema:"Identifier of the artifact to update." validate:"required,notblank"`

	Name        *string     `json:"name,omitempty" jsonschema:"New name. Omit to leave unchanged."`
	Description *string     `json:"description,omitempty" jsonschema:"New description. Omit to leave unchanged."`
	Entrypoint  *string     `json:"entrypoint,omitempty" jsonschema:"New entrypoint file. Omit to leave unchanged."`
	Visibility  *Visibility `json:"visibility,omitempty" jsonschema:"New visibility. Omit to leave unchanged."`

	command.Reasoning
}

// Update changes the describable parts of an artifact's configuration.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Artifact, error) {
	id := strings.TrimSpace(in.ID)
	current, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.Entrypoint != nil {
		current.Entrypoint = strings.TrimSpace(*in.Entrypoint)
	}
	if in.Visibility != nil {
		if !in.Visibility.Valid() {
			return nil, errInvalidVisibility(string(*in.Visibility))
		}
		current.Visibility = *in.Visibility
	}
	current.UpdatedAt = s.clock.Now()

	toWrite := current.Clone()
	if err := s.repo.Update(ctx, &toWrite, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	current.URLs = s.urlsFor(current.ID)
	return current, nil
}

// DeleteInput names one artifact to remove.
type DeleteInput struct {
	ID string `json:"id" jsonschema:"Identifier of the artifact to delete." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput confirms what was removed.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the artifact that was deleted."`
}

// Delete removes an artifact's configuration and its files. It is
// idempotent: deleting what is already gone succeeds rather than erroring.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := strings.TrimSpace(in.ID)
	// An id that is not a name is refused here rather than resolved
	// somewhere surprising. The adapter guards its own joins too — that is
	// the layer that actually stops ".." from reaching the workspace's whole
	// .aos — but reporting "deleted" for an id that never named an artifact
	// is an answer nobody can act on.
	if _, err := pathx.Segment(id); err != nil {
		return DeleteOutput{}, errUnsafeID(id)
	}
	if err := s.repo.Delete(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	if err := s.files.Remove(ctx, id); err != nil {
		s.log.Warn("artifact record deleted but its files could not be removed",
			"id", id, "err", err)
	}
	return DeleteOutput{ID: id}, nil
}

// SetPasswordInput names an artifact and the plaintext password to set on it.
type SetPasswordInput struct {
	ID       string `json:"id" jsonschema:"Identifier of the artifact." validate:"required,notblank"`
	Password string `json:"password" jsonschema:"New plaintext password, hashed before it is stored." validate:"required,notblank"`

	command.Reasoning
}

// SetPasswordOutput is the URL to share once the password is set.
type SetPasswordOutput struct {
	URL string `json:"url" jsonschema:"Shareable URL for this artifact."`
}

// SetPassword hashes password with argon2id and persists the hash, so a
// shared by_password link survives a daemon restart — the original generates
// this secret fresh on every boot and never writes it down, which is defect
// #19: a link shared before a restart stops working after one.
func (s *Service) SetPassword(ctx context.Context, in SetPasswordInput) (SetPasswordOutput, error) {
	id := strings.TrimSpace(in.ID)
	current, err := s.get(ctx, id)
	if err != nil {
		return SetPasswordOutput{}, err
	}

	hash, herr := s.hasher.Hash(in.Password)
	if herr != nil {
		return SetPasswordOutput{}, errHashFailed(id, herr)
	}

	current.PasswordHash = hash
	current.UpdatedAt = s.clock.Now()
	toWrite := current.Clone()
	if err := s.repo.Update(ctx, &toWrite, collections.Version{}); err != nil {
		return SetPasswordOutput{}, errWriteFailed("SetPassword", err)
	}
	// Was "/v/{workspace}/artifacts/" + id + "/" + current.Entrypoint — a
	// literal, never-interpolated "{workspace}" left over from the design
	// doc's own sketch, predating internal/transport/artifactapi's real
	// route (single-workspace, no such segment — see that package's doc).
	// s.urlsFor is the one place this daemon computes an artifact's URL now.
	return SetPasswordOutput{URL: s.urlsFor(id).Local}, nil
}

// AccessRequest is what a caller of Authorize brings: whatever it was able
// to establish about who is asking, before this package decides whether that
// is enough for a.
type AccessRequest struct {
	// Authenticated is whether the request carries a valid session for the
	// workspace that owns a — required for Workspace and, alone, still not
	// enough for ByPassword.
	Authenticated bool

	// Password is the plaintext password offered for a ByPassword artifact,
	// empty when none was given.
	Password string
}

// Authorize decides whether req may read a, per its Visibility.
func (s *Service) Authorize(ctx context.Context, a *Artifact, req AccessRequest) error {
	switch a.Visibility {
	case Private, Workspace:
		if !req.Authenticated {
			return errUnauthorized(a.ID)
		}
		return nil
	case ByPassword:
		if a.PasswordHash == "" {
			return errPasswordRequired(a.ID)
		}
		if req.Password == "" {
			return errUnauthorized(a.ID)
		}
		ok, err := s.hasher.Verify(req.Password, a.PasswordHash)
		if err != nil {
			return errHashFailed(a.ID, err)
		}
		if !ok {
			return errUnauthorized(a.ID)
		}
		return nil
	default:
		return errInvalidVisibility(string(a.Visibility))
	}
}
