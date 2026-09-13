package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/model"
	"github.com/OWNER/aos/internal/runtime/providers"
)

// catalogTTL is how long one provider's answer is reused.
//
// Long enough that opening the settings screen, changing a slot and looking
// again is one round of calls rather than four; short enough that a model
// released this morning is offered this afternoon without restarting anything.
// A changed credential does not wait for it — see catalogEntry.credential.
const catalogTTL = 5 * time.Minute

// catalogFailureTTL is how long a provider that could not be asked is not
// asked again.
//
// Short, because the usual fixes — a key pasted again, a login refreshed — are
// made and retried within seconds, and a failure served past them would make
// the fix look like it did not work. Not zero, because the settings screen asks
// on every render: a provider that cannot answer was asked again each time,
// which for a file-based credential meant another renewal attempt against
// somebody else's file and another warning in the log. A changed credential
// does not wait for this either.
const catalogFailureTTL = 30 * time.Second

// catalogTimeout bounds one provider's answer.
//
// The shared provider client allows ten minutes, which is right for a reasoning
// model working through a hard question and absurd for a list of names. A
// settings screen that hangs for ten minutes on an unreachable host is
// indistinguishable from one that is broken.
const catalogTimeout = 20 * time.Second

// modelCatalog answers the model domain's Catalog port from the live
// configuration and the provider registry.
//
// It lives here rather than in the domain for the usual reason — it does
// network I/O and reads credentials — and it caches for a reason that is not
// usual: every call costs somebody else's rate limit. A settings screen that
// re-renders asks again, and four providers times every render is a way to get
// an installation throttled by doing nothing.
type modelCatalog struct {
	config config.Service
	home   string
	clock  clockx.Clock

	mu     sync.Mutex
	cached map[string]catalogEntry
}

type catalogEntry struct {
	models  []model.Model
	fetched time.Time

	// err is a failed answer, kept for catalogFailureTTL rather than
	// catalogTTL.
	err error

	// credential fingerprints the key the answer was fetched with, so that
	// correcting a wrong key shows the right catalogue immediately instead of
	// serving the failure until the TTL expires. It is a hash: this struct
	// outlives the call, and there is no reason for a second copy of a secret
	// to exist in memory when a comparison is all that is needed.
	credential string
}

func newModelCatalog(cfg config.Service, home string, clock clockx.Clock) *modelCatalog {
	return &modelCatalog{config: cfg, home: home, clock: clock, cached: map[string]catalogEntry{}}
}

// Connected lists the providers this installation holds a credential for.
func (c *modelCatalog) Connected(ctx context.Context) ([]string, error) {
	// Get, not Raw: this only needs the ids, and the redacted view carries
	// them. Reaching for the unredacted configuration to answer a question
	// that does not need a secret is how a secret ends up somewhere it was
	// never meant to go.
	current, err := c.config.Get(ctx, config.GetInput{})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(current.Agents.Providers))
	for _, p := range current.Agents.Providers {
		if p.ID != "" {
			ids = append(ids, p.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// Models asks one provider what it serves, reusing a recent answer.
func (c *modelCatalog) Models(ctx context.Context, provider string) ([]model.Model, error) {
	// Raw, for the same reason internal/app's models.For needs it: the
	// adapter has to authenticate with the real key, and Get would hand it a
	// fingerprint to send as a bearer token.
	current, err := c.config.Raw(ctx)
	if err != nil {
		return nil, err
	}
	key := keyFor(current, provider)
	fingerprint := fingerprintOf(key)

	if hit, ok := c.fresh(provider, fingerprint); ok {
		if hit.err != nil {
			return nil, repeatedFailure{hit.err}
		}
		return hit.models, nil
	}

	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()

	found, err := providers.Models(ctx, provider, providers.Config{APIKey: key, Home: c.home})
	if err != nil {
		// Kept only briefly: see catalogFailureTTL.
		c.mu.Lock()
		c.cached[provider] = catalogEntry{fetched: c.clock.Now(), credential: fingerprint, err: err}
		c.mu.Unlock()
		return nil, err
	}

	models := make([]model.Model, 0, len(found))
	for _, m := range found {
		models = append(models, model.Model{ID: m.ID, Name: m.Name})
	}

	c.mu.Lock()
	c.cached[provider] = catalogEntry{models: models, fetched: c.clock.Now(), credential: fingerprint}
	c.mu.Unlock()
	return models, nil
}

// fresh returns a cached answer — a catalogue, or the failure to read one —
// when it was fetched recently with this same credential.
func (c *modelCatalog) fresh(provider, fingerprint string) (catalogEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.cached[provider]
	if !ok || entry.credential != fingerprint {
		return catalogEntry{}, false
	}
	ttl := catalogTTL
	if entry.err != nil {
		ttl = catalogFailureTTL
	}
	if c.clock.Now().Sub(entry.fetched) >= ttl {
		return catalogEntry{}, false
	}
	// A copy of the models: the caller owns what it receives, and the domain
	// sorts and filters what it is given.
	if entry.models != nil {
		models := make([]model.Model, len(entry.models))
		copy(models, entry.models)
		entry.models = models
	}
	return entry, true
}

// repeatedFailure is a failure answered from the cache: the same error, marked
// so the model service reports it without logging it again.
type repeatedFailure struct{ error }

func (r repeatedFailure) Is(target error) bool { return target == model.ErrRepeated }
func (r repeatedFailure) Unwrap() error        { return r.error }

// fingerprintOf hashes a credential so the cache can tell one from another
// without holding it.
//
// An empty key is named rather than hashed, so that reading a cache entry in a
// debugger says "this provider has no key" instead of a digest of nothing —
// which is the normal state for the two providers whose credential is another
// tool's file on this machine, not an anomaly worth decoding.
func fingerprintOf(key string) string {
	if key == "" {
		return "none"
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8])
}
