package collection

import (
	"context"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Record is one row under a collection: the domain's view of a
// collections.Record, translated back and forth by RecordService.
//
// It carries its own identity and timestamps, which collections.Record — the
// engine's type — does not: an engine record is a Key, a Fields map and a
// body, with no opinion about time. A collection that wants a queryable
// "created at" or "updated at" field declares one and a setTimestamp hook
// stamps it, the same as any other field; CreatedAt and UpdatedAt here are a
// best-effort convenience read from the record's storage version, not a
// second, competing source of truth for that field.
type Record struct {
	ID         string         `json:"id" jsonschema:"Identifier of this record, unique within its collection."`
	Collection string         `json:"collection" jsonschema:"Id of the collection this record belongs to."`
	Data       map[string]any `json:"data" jsonschema:"The record's fields, as declared by its collection's schema."`

	// Content is the Markdown body, for a collection of format md. It is
	// never one of Data's keys — the body is content, not a field, and
	// declaring every note-taking collection a "content" field would be
	// describing the format twice.
	Content string `json:"content,omitempty" jsonschema:"The Markdown body, for a collection of format md. Empty for json, and for a record with no body."`

	CreatedAt time.Time `json:"createdAt" jsonschema:"When the record was written. Best-effort: read from the file's storage version, not tracked separately."`
	UpdatedAt time.Time `json:"updatedAt" jsonschema:"When the record was last written. Best-effort, from the same source as createdAt."`
}

// RecordQuery selects the records List returns. It is the domain's shape of
// collections.Query — filters, ordering, limit and offset — kept separate so
// that a caller of RecordService never has to import the engine package to
// list a collection's rows.
type RecordQuery struct {
	Filters map[string]any `json:"filters,omitempty" jsonschema:"Field values a record must match, by field name."`
	OrderBy string         `json:"orderBy,omitempty" jsonschema:"Name of a declared field to order by. Empty orders by path, which is stable."`
	Desc    bool           `json:"desc,omitempty" jsonschema:"Reverse the order."`
	Limit   int            `json:"limit,omitempty" jsonschema:"Maximum number of records to return. 0 means no limit."`
	Offset  int            `json:"offset,omitempty" jsonschema:"Number of matching records to skip, for pagination."`
}

func (q RecordQuery) toQuery() collections.Query {
	return collections.Query{
		Filters: q.Filters,
		OrderBy: q.OrderBy,
		Desc:    q.Desc,
		Limit:   q.Limit,
		Offset:  q.Offset,
	}
}

// RecordService is the aggregate for the rows stored under a collection,
// reachable as Service.Records().
type RecordService interface {
	List(ctx context.Context, collectionID string, q RecordQuery) ([]Record, error)
	Get(ctx context.Context, collectionID, id string) (*Record, error)

	// Create is CreateWithContent with an empty body. A json-format collection
	// has no body to give it; a md-format one that never calls
	// CreateWithContent simply never writes to it, the same as any optional
	// field.
	Create(ctx context.Context, collectionID string, data map[string]any) (*Record, error)
	CreateWithContent(ctx context.Context, collectionID string, data map[string]any, content string) (*Record, error)

	Update(ctx context.Context, collectionID, id string, data map[string]any) (*Record, error)
	Delete(ctx context.Context, collectionID, id string) error
}

// recordService is the only implementation of RecordService.
type recordService struct {
	decls Repository
	repos RecordRepositories
	clock Clock
	ids   IDs
}

// declaration fetches the collection a record is being written against.
//
// A missing declaration is refused here, naming what was asked for, rather
// than creating the collection implicitly: an implicit collection is a typo
// that becomes a schema. Any error the Repository port reports — a wrong id
// is by far the likely one — becomes the same refusal, matching how
// Service.Get already treats a lookup miss.
func (s *recordService) declaration(ctx context.Context, collectionID string) (Collection, error) {
	id := strings.TrimSpace(collectionID)
	c, err := s.decls.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return Collection{}, errNotFound(id, nil)
	}
	return *c, nil
}

// List returns the records of one collection matching q.
func (s *recordService) List(ctx context.Context, collectionID string, q RecordQuery) ([]Record, error) {
	c, err := s.declaration(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	repo, err := s.repos.For(c)
	if err != nil {
		return nil, err
	}
	rows, err := repo.List(ctx, q.toQuery())
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for i := range rows {
		// Timestamps are not stamped here: List answers "what is here", and a
		// Version lookup per row would be a file stat for every record in the
		// collection to fill in a field nothing in this method's tests reads.
		out = append(out, recordFrom(collectionID, &rows[i]))
	}
	return out, nil
}

// Get reads one record.
func (s *recordService) Get(ctx context.Context, collectionID, id string) (*Record, error) {
	c, err := s.declaration(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	repo, err := s.repos.For(c)
	if err != nil {
		return nil, err
	}
	key := collections.Key{"id": strings.TrimSpace(id)}
	rec, err := repo.Get(ctx, key)
	if err != nil {
		return nil, errRecordNotFound(collectionID, id)
	}
	out := recordFrom(collectionID, rec)
	stampVersion(ctx, repo, key, &out)
	return &out, nil
}

// Create writes a new record with no body. See CreateWithContent for the
// order it follows.
func (s *recordService) Create(ctx context.Context, collectionID string, data map[string]any) (*Record, error) {
	return s.CreateWithContent(ctx, collectionID, data, "")
}

// CreateWithContent writes a new record, in the order the schema it is
// validated against exists for:
//
//  1. fetch the declaration — absent, refuse, naming what was asked for;
//  2. run the declared hooks, which normalise the data before anything
//     downstream sees it;
//  3. fetch what is already stored, but only when some field declares
//     Unique — an unconditional scan would make every collection pay for a
//     constraint most do not declare;
//  4. validate the normalised data against the schema and what is stored;
//  5. write.
func (s *recordService) CreateWithContent(ctx context.Context, collectionID string, data map[string]any, content string) (*Record, error) {
	c, err := s.declaration(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	repo, err := s.repos.For(c)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	normalised, err := ApplyHooks(c, data, now, "create")
	if err != nil {
		return nil, err
	}

	existing, err := existingFields(ctx, repo, c, "")
	if err != nil {
		return nil, err
	}
	if err := Validate(c, normalised, existing); err != nil {
		return nil, err
	}

	key := collections.Key{"id": s.ids.New()}
	rec := &collections.Record{Key: key, Fields: normalised, Content: content}
	if err := repo.Create(ctx, rec); err != nil {
		return nil, err
	}

	out := recordFrom(collectionID, rec)
	stampVersion(ctx, repo, key, &out)
	return &out, nil
}

// Update revalidates and rewrites a record.
//
// A refused update leaves the stored record untouched by construction: repo.Update
// is the last statement in the method, so nothing between fetching the current
// record and validating the new data can have written anything.
func (s *recordService) Update(ctx context.Context, collectionID, id string, data map[string]any) (*Record, error) {
	c, err := s.declaration(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	repo, err := s.repos.For(c)
	if err != nil {
		return nil, err
	}
	key := collections.Key{"id": strings.TrimSpace(id)}
	current, err := repo.Get(ctx, key)
	if err != nil {
		return nil, errRecordNotFound(collectionID, id)
	}

	now := s.clock.Now()
	normalised, err := ApplyHooks(c, data, now, "update")
	if err != nil {
		return nil, err
	}

	// The record being updated must not be compared against itself: keeping
	// the unique value it already holds is not a collision.
	existing, err := existingFields(ctx, repo, c, current.Key["id"])
	if err != nil {
		return nil, err
	}
	if err := Validate(c, normalised, existing); err != nil {
		return nil, err
	}

	// Content is not one of Update's parameters — it is not a field, and this
	// call has no way to change it — so the stored body carries over
	// untouched.
	rec := &collections.Record{Key: current.Key, Fields: normalised, Content: current.Content}
	if err := repo.Update(ctx, rec, collections.Version{}); err != nil {
		return nil, err
	}

	out := recordFrom(collectionID, rec)
	stampVersion(ctx, repo, key, &out)
	return &out, nil
}

// Delete removes a record.
func (s *recordService) Delete(ctx context.Context, collectionID, id string) error {
	c, err := s.declaration(ctx, collectionID)
	if err != nil {
		return err
	}
	repo, err := s.repos.For(c)
	if err != nil {
		return err
	}
	return repo.Delete(ctx, collections.Key{"id": strings.TrimSpace(id)})
}

// recordFrom translates the engine's record into the domain's.
func recordFrom(collectionID string, rec *collections.Record) Record {
	data := rec.Fields
	if data == nil {
		data = map[string]any{}
	}
	return Record{
		ID:         rec.Key["id"],
		Collection: collectionID,
		Data:       data,
		Content:    rec.Content,
	}
}

// stampVersion fills CreatedAt and UpdatedAt from the repository's storage
// version, when the repository can report one. Both come from the same
// modification time: unlike a declared field a setTimestamp hook stamps, the
// engine does not separately track when a file was first written versus last
// rewritten, and reporting one borrowed value as two is more honest than
// leaving the more visible of the two blank.
func stampVersion(ctx context.Context, repo RecordRepo, key collections.Key, r *Record) {
	versioned, ok := repo.(collections.Versioned)
	if !ok {
		return
	}
	v, err := versioned.Version(ctx, key)
	if err != nil {
		return
	}
	r.CreatedAt = v.ModTime
	r.UpdatedAt = v.ModTime
}

// existingFields lists the stored Fields of a collection's records, but only
// when the schema declares at least one Unique field — an unconditional List
// would make every collection pay for a constraint most do not declare.
// skipID excludes one record (the one being updated) from its own check.
func existingFields(ctx context.Context, repo RecordRepo, c Collection, skipID string) ([]map[string]any, error) {
	if !hasUniqueField(c) {
		return nil, nil
	}
	rows, err := repo.List(ctx, collections.Query{})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		if skipID != "" && rows[i].Key["id"] == skipID {
			continue
		}
		out = append(out, rows[i].Fields)
	}
	return out, nil
}

func hasUniqueField(c Collection) bool {
	for _, f := range c.Fields {
		if f.Unique {
			return true
		}
	}
	return false
}
