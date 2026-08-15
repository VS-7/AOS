package config

import (
	"context"
	"errors"
	"sort"

	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/patch"
)

// Service is the configuration API of the domain.
type Service interface {
	// Get always redacts for agents. Humans on a TTY may pass Reveal, which the
	// transport only sets after an interactive confirmation.
	Get(ctx context.Context, in GetInput) (Config, error)
	Update(ctx context.Context, in UpdateInput) (Config, error)

	// Raw is internal-only: it never crosses a transport boundary.
	Raw(ctx context.Context) (Config, error)
}

type service struct{ store Store }

// NewService wires the configuration service over a store.
func NewService(store Store) Service { return &service{store: store} }

func (s *service) Raw(ctx context.Context) (Config, error) {
	c, err := s.store.Load(ctx)
	if err != nil {
		return Config{}, errLoadFailed(err)
	}
	return Normalize(c), nil
}

func (s *service) Get(ctx context.Context, in GetInput) (Config, error) {
	c, err := s.Raw(ctx)
	if err != nil {
		return Config{}, err
	}
	if !in.Reveal {
		return Redact(c), nil
	}
	// Reveal is refused for an agent regardless of what the caller asked for.
	// There is no code path by which config_get returns a secret to a model.
	if identity.IsAgent(ctx) {
		return Config{}, errRevealDenied()
	}
	return c, nil
}

func (s *service) Update(ctx context.Context, in UpdateInput) (Config, error) {
	current, err := s.Raw(ctx)
	if err != nil {
		return Config{}, err
	}

	byAgent := identity.IsAgent(ctx)
	for _, path := range sortedKeys(in.Set) {
		if byAgent && !AgentWritable(path) {
			return Config{}, errFieldForbidden(path)
		}
	}

	next, err := applyPatch(current, in.Set)
	if err != nil {
		return Config{}, err
	}
	next = Normalize(next)
	if err := s.store.Save(ctx, next); err != nil {
		return Config{}, errSaveFailed(err)
	}
	// Update always returns the redacted view: a caller that legitimately needs
	// a secret asks for it explicitly through Get with Reveal.
	return Redact(next), nil
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// applyPatch sets dotted paths on a copy of c.
//
// The paths a caller writes are the paths the file shows, because both come
// from the same JSON tags — there is no second naming scheme to keep in sync.
func applyPatch(c Config, set map[string]any) (Config, error) {
	out, err := patch.Apply(c, set)
	if err == nil {
		return out, nil
	}
	var unknown *patch.UnknownPathError
	if errors.As(err, &unknown) {
		return Config{}, errUnknownField(unknown.Path)
	}
	var bad *patch.ValueError
	if errors.As(err, &bad) {
		return Config{}, errBadValue(bad.Path, bad)
	}
	return Config{}, err
}
