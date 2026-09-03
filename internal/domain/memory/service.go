package memory

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/search"
)

// defaultLimit is the original's page size for a recall.
const defaultLimit = 10

// Service is the memory aggregate: the five cognitive verbs and nothing else.
type Service struct {
	repo  Repository
	clock Clock
	ids   IDs
	index Index
	log   *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Repo  Repository
	Clock Clock
	IDs   IDs

	// Index is optional. Without it, Recall scans — which is what the original
	// does, and is correct up to a few thousand memories (ADR-0013).
	Index Index

	// Log records the one thing that cannot be returned: a supersede that
	// completed halfway.
	Log *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: d.Repo, clock: d.Clock, ids: d.IDs, index: d.Index, log: log}
}

// Recall scans the graph with filters. It is how a session begins.
func (s *Service) Recall(ctx context.Context, in RecallInput) (RecallOutput, error) {
	owner := s.resolveAgent(ctx, in.Agent)

	status := in.Status
	if status == "" {
		status = StatusActive
	}
	if !status.Valid() {
		return RecallOutput{}, errInvalidStatus(string(status))
	}
	if in.Category != "" && !in.Category.Valid() {
		return RecallOutput{}, errInvalidCategory(string(in.Category))
	}

	q := collections.Query{IncludeContent: true}
	if owner != "" {
		q.Key = collections.Key{"agent": owner}
	}
	found, err := s.repo.List(ctx, q)
	if err != nil {
		return RecallOutput{}, errReadFailed("Recall", err)
	}

	tokens := search.Tokenize(in.Query)
	mode := in.ScopesMode
	if mode == "" {
		mode = ScopesLax
	}

	matched := make([]Memory, 0, len(found))
	for _, m := range found {
		if normalizeStatus(m.Status) != status {
			continue
		}
		if in.Category != "" && m.Category != in.Category {
			continue
		}
		if len(tokens) > 0 && !search.Matches(m.Document(), tokens) {
			continue
		}
		if !matchScopes(m, in.Scopes, mode) {
			continue
		}
		matched = append(matched, m)
	}

	// The index is consulted for ordering, never for membership. It is derived
	// data that can lag behind the files, and a recall that omitted a memory
	// because the index had not caught up would be a silent wrong answer.
	indexed := s.rank(ctx, matched, in, tokens)

	total := len(matched)
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if in.Offset > 0 {
		if in.Offset >= len(matched) {
			matched = nil
		} else {
			matched = matched[in.Offset:]
		}
	}
	if limit < len(matched) {
		matched = matched[:limit]
	}

	return RecallOutput{Memories: matched, Total: total, Indexed: indexed}, nil
}

// rank orders the matches in place and reports whether the index was used.
func (s *Service) rank(ctx context.Context, matched []Memory, in RecallInput, tokens []string) bool {
	order := in.OrderBy
	if order == "" {
		if len(tokens) > 0 {
			order = "relevance"
		} else {
			order = "createdAt"
		}
	}

	if order == "relevance" && len(tokens) > 0 {
		scores, served := s.scores(ctx, matched, in, tokens)
		sort.SliceStable(matched, func(i, j int) bool {
			a, b := scores[matched[i].ID], scores[matched[j].ID]
			if a == b {
				return matched[i].ID < matched[j].ID
			}
			return a > b
		})
		if in.Desc {
			reverse(matched)
		}
		return served
	}

	less := func(i, j int) bool { return matched[i].CreatedAt.Before(matched[j].CreatedAt) }
	switch order {
	case "confidence":
		less = func(i, j int) bool { return matched[i].Confidence < matched[j].Confidence }
	case "updatedAt":
		less = func(i, j int) bool { return matched[i].UpdatedAt.Before(matched[j].UpdatedAt) }
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if less(i, j) {
			return true
		}
		if less(j, i) {
			return false
		}
		return matched[i].ID < matched[j].ID // ordering is always total
	})
	if in.Desc {
		reverse(matched)
	}
	return false
}

// scores asks the index to rank, and falls back to the shared scoring function.
//
// Both use the same tokeniser, so the two paths cannot disagree about what a
// query means — only about how fast the answer arrives. The bool reports which
// path actually ran, which is what the caller is told: "an index answered this"
// is a claim, and a broken index that silently degraded must not make it.
func (s *Service) scores(ctx context.Context, matched []Memory, in RecallInput, tokens []string) (map[string]float64, bool) {
	out := make(map[string]float64, len(matched))
	if s.index != nil {
		hits, err := s.index.Search(ctx, search.Query{
			Collection:  "memories",
			Text:        in.Query,
			Consistency: search.Eventual,
		})
		if err == nil {
			for _, h := range hits {
				out[h.ID] = h.Score
			}
			// A memory the index has not caught up with still has to be ranked.
			for _, m := range matched {
				if _, ok := out[m.ID]; !ok {
					out[m.ID] = search.Score(m.Document(), tokens)
				}
			}
			return out, true
		}
		// ADR-0013: a broken index degrades to scanning, with a warning, rather
		// than taking the whole search down.
		s.log.Warn("search index unavailable, scanning instead", "err", err)
	}
	for _, m := range matched {
		out[m.ID] = search.Score(m.Document(), tokens)
	}
	return out, false
}

// Reflect reads one memory in full.
func (s *Service) Reflect(ctx context.Context, in ReflectInput) (*Memory, error) {
	owner := s.resolveAgent(ctx, in.Agent)
	id := strings.TrimSpace(in.Memory)

	if owner != "" {
		got, err := s.repo.Get(ctx, collections.Key{"agent": owner, "id": id})
		if err == nil {
			return got, nil
		}
	}
	// Without an ambient agent there is no key to build, so the memory is found
	// by scanning. This is the human-terminal path: a person inspecting the
	// graph does not know which agent owns which trace.
	found, err := s.repo.List(ctx, collections.Query{IncludeContent: true})
	if err != nil {
		return nil, errReadFailed("Reflect", err)
	}
	for i := range found {
		if found[i].ID == id && (owner == "" || found[i].Agent == owner) {
			return &found[i], nil
		}
	}
	return nil, errNotFound(owner, id)
}

// Store records durable knowledge and applies the supersede protocol.
//
// The order is deliberate. Every supersede target is verified to exist before
// anything is written, so a lineage never points at nothing. The replacement is
// written next. Only then are the old ones deprecated — because if that step
// fails, the knowledge still exists and the inconsistency is visible, whereas
// deprecating first could leave the agent with neither.
func (s *Service) Store(ctx context.Context, in StoreInput) (StoreOutput, error) {
	owner := s.resolveAgent(ctx, in.Agent)
	if owner == "" {
		return StoreOutput{}, errAgentRequired("Store")
	}
	if !in.Category.Valid() {
		return StoreOutput{}, errInvalidCategory(string(in.Category))
	}

	confidence := 1.0
	if in.Confidence != nil {
		confidence = *in.Confidence
	}
	if confidence < 0 || confidence > 1 {
		return StoreOutput{}, errInvalidConfidence(confidence)
	}

	var expires *time.Time
	if in.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, in.ExpiresAt)
		if err != nil {
			return StoreOutput{}, errInvalidExpiry(in.ExpiresAt, err)
		}
		expires = &parsed
	}

	for _, sup := range in.Supersedes {
		if len(strings.TrimSpace(sup.Reason)) < 5 {
			return StoreOutput{}, errReasonTooShort(sup.ID)
		}
		if _, err := s.repo.Get(ctx, collections.Key{"agent": owner, "id": sup.ID}); err != nil {
			return StoreOutput{}, errSupersedeTargetMissing(sup.ID)
		}
	}

	now := s.clock.Now()
	m := &Memory{
		ID:          s.ids.New(),
		Agent:       owner,
		Title:       in.Title,
		Description: in.Description,
		Category:    in.Category,
		Tags:        in.Tags,
		Confidence:  confidence,
		Links:       in.Links,
		Supersedes:  in.Supersedes,
		Status:      StatusActive,
		Scopes:      in.Scopes,
		ExpiresAt:   expires,
		CreatedAt:   now,
		UpdatedAt:   now,
		// The original strips leading whitespace so the body sits flush
		// against the front matter; a body that starts with a blank line
		// renders as one on every surface that shows it.
		Content: strings.TrimLeft(in.Content, " \t\n\r"),
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return StoreOutput{}, errWriteFailed("Store", err)
	}
	s.reindex(ctx, m)

	out := StoreOutput{Memory: *m}
	for _, sup := range in.Supersedes {
		if _, err := s.deprecate(ctx, owner, sup.ID, sup.Reason, m.ID); err != nil {
			// Reported, not hidden. Without a transaction across files this is
			// a real state the system can end up in, and the caller is the only
			// one who can resolve it.
			s.log.Error("supersede incomplete",
				"replacement", m.ID, "superseded", sup.ID, "err", err)
			out.Incomplete = append(out.Incomplete, sup.ID)
			continue
		}
		out.Deprecated = append(out.Deprecated, sup.ID)
	}
	return out, nil
}

// Forget deprecates a memory, preserving its lineage. There is no Delete: the
// history of what an agent believed is not something the system offers to erase.
func (s *Service) Forget(ctx context.Context, in ForgetInput) (*Memory, error) {
	owner := s.resolveAgent(ctx, in.Agent)
	if owner == "" {
		return nil, errAgentRequired("Forget")
	}
	if len(strings.TrimSpace(in.Reason)) < 5 {
		return nil, errReasonTooShort(in.Memory)
	}
	return s.deprecate(ctx, owner, strings.TrimSpace(in.Memory), in.Reason, "")
}

// deprecate is the single place a memory changes status, so that Forget and the
// supersede protocol cannot drift apart.
func (s *Service) deprecate(ctx context.Context, owner, id, reason, replacedBy string) (*Memory, error) {
	current, err := s.repo.Get(ctx, collections.Key{"agent": owner, "id": id})
	if err != nil {
		return nil, errNotFound(owner, id)
	}
	if current.Status == StatusDeprecated {
		return nil, errAlreadyDeprecated(id)
	}

	now := s.clock.Now()
	current.Status = StatusDeprecated
	current.DeprecatedAt = &now
	current.DeprecatedReason = reason
	// DeprecatedBy holds the memory that replaced this one, which is what the
	// field is documented to mean. The original writes the *agent* slug here,
	// so its lineage records who deprecated rather than what superseded, and
	// the chain cannot be walked.
	current.DeprecatedBy = replacedBy
	current.UpdatedAt = now

	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return nil, errWriteFailed("Forget", err)
	}
	s.reindex(ctx, current)
	return current, nil
}

// reindex keeps the derived index in step. A failure is logged and swallowed:
// the files are the truth, the index is rebuildable, and refusing a write
// because a cache did not update would be the wrong trade.
func (s *Service) reindex(ctx context.Context, m *Memory) {
	if s.index == nil {
		return
	}
	if err := s.index.Upsert(ctx, m.Document()); err != nil {
		s.log.Warn("could not index memory", "memory", m.ID, "err", err)
	}
}

// resolveAgent picks whose memories are addressed: the one named, otherwise the
// ambient identity, lowercased as the slug always is.
func (s *Service) resolveAgent(ctx context.Context, explicit string) string {
	if named := strings.ToLower(strings.TrimSpace(explicit)); named != "" {
		return named
	}
	if actor, kind := identity.Actor(ctx); kind == identity.ActorAgent {
		return strings.ToLower(actor)
	}
	return ""
}

// normalizeStatus treats an empty status as active, which is what a memory
// written by hand without the field means.
func normalizeStatus(s Status) Status {
	if s == "" {
		return StatusActive
	}
	return s
}

func reverse(v []Memory) {
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
}
