package collection_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/collection"
)

// at is the instant every fixed clock in this file reports. Task 4's own
// hook test (TestDeclarativeHooksNormaliseAndStamp) fixes it in-line already;
// this is the same value, declared once for the service tests below.
var at = time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

func crm() collection.Collection {
	return collection.Collection{
		ID:     "contacts",
		Name:   "Contacts",
		Scope:  collection.ScopeWorkspace,
		Format: collection.FormatMarkdown,
		Fields: []collection.Field{
			{Name: "name", Type: collection.TypeString, Required: true},
			{Name: "email", Type: collection.TypeString, Unique: true},
			{Name: "stage", Type: collection.TypeEnum, Enum: []string{"lead", "won", "lost"}},
			{Name: "score", Type: collection.TypeNumber},
			{Name: "active", Type: collection.TypeBoolean},
			{Name: "closedAt", Type: collection.TypeDate},
			{Name: "tags", Type: collection.TypeList},
			{Name: "owner", Type: collection.TypeRef, Ref: "agents"},
		},
	}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not an apperr: %v", err, err)
	}
	return app.Code
}

func TestAValidRecordPasses(t *testing.T) {
	err := collection.Validate(crm(), map[string]any{
		"name": "Ada", "email": "ada@example.com", "stage": "won",
		"score": 42.0, "active": true, "tags": []any{"vip"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAMissingRequiredFieldIsRefusedNamingIt(t *testing.T) {
	err := collection.Validate(crm(), map[string]any{"email": "a@b.c"}, nil)
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_REQUIRED" {
		t.Fatalf("code = %q", code)
	}
	var app *apperr.Error
	_ = errors.As(err, &app)
	if app.Issues["field"] != "name" {
		t.Fatalf("the error does not name the field: %v", app.Issues)
	}
}

func TestAValueOutsideTheEnumIsRefused(t *testing.T) {
	err := collection.Validate(crm(), map[string]any{"name": "Ada", "stage": "maybe"}, nil)
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_NOT_IN_ENUM" {
		t.Fatalf("code = %q", code)
	}
}

func TestAValueOfTheWrongTypeIsRefused(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"string given a number": {"name": 7},
		"number given a string": {"name": "Ada", "score": "muito"},
		"boolean given a word":  {"name": "Ada", "active": "sim"},
		"date given nonsense":   {"name": "Ada", "closedAt": "ontem"},
		"list given a scalar":   {"name": "Ada", "tags": "vip"},
	} {
		err := collection.Validate(crm(), data, nil)
		if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_WRONG_TYPE" {
			t.Fatalf("%s: code = %q", name, code)
		}
	}
}

// Unique is checked against what is already stored. Without the existing
// records there is nothing to compare against, which is why they are a
// parameter rather than something Validate goes and fetches — the domain does
// not do IO.
func TestADuplicateUniqueValueIsRefused(t *testing.T) {
	existing := []map[string]any{{"name": "Ada", "email": "ada@example.com"}}
	err := collection.Validate(crm(), map[string]any{"name": "Grace", "email": "ada@example.com"}, existing)
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_NOT_UNIQUE" {
		t.Fatalf("code = %q", code)
	}
}

func TestAFieldNotDeclaredIsRefused(t *testing.T) {
	err := collection.Validate(crm(), map[string]any{"name": "Ada", "salary": 100}, nil)
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_UNDECLARED" {
		t.Fatalf("code = %q", code)
	}
}

// A ref pointing at a collection that does not exist cannot render: the view
// has nothing to resolve the relation against. Refusing at declaration time is
// cheaper than refusing at render time.
func TestARefToAnUnknownCollectionIsRefusedAtDeclaration(t *testing.T) {
	c := crm()
	c.Fields = append(c.Fields, collection.Field{Name: "deal", Type: collection.TypeRef, Ref: "does-not-exist"})
	err := collection.ValidateSchema(c, func(name string) bool { return name == "agents" })
	if code := codeOf(t, err); code != "AOS_COLLECTION_REF_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}

func TestARefFieldWithoutARefTargetIsRefused(t *testing.T) {
	c := crm()
	c.Fields = []collection.Field{{Name: "owner", Type: collection.TypeRef}}
	err := collection.ValidateSchema(c, func(string) bool { return true })
	if code := codeOf(t, err); code != "AOS_COLLECTION_REF_MISSING" {
		t.Fatalf("code = %q", code)
	}
}

// Hooks are declarative — a small set of named actions with parameters, never
// source code. The original accepts code strings and writes them into a
// generated schema.ts, which is an agent producing executable code in the
// workspace with no sandbox and no review.
func TestDeclarativeHooksNormaliseAndStamp(t *testing.T) {
	c := crm()
	c.Fields = append(c.Fields, collection.Field{Name: "slug", Type: collection.TypeString},
		collection.Field{Name: "createdAt", Type: collection.TypeDate})
	c.Hooks = []collection.Hook{
		{On: "create", Action: collection.ActionSetTimestamp, Field: "createdAt"},
		{On: "create", Action: collection.ActionSlugify, Field: "slug", From: "name"},
		{On: "create", Action: collection.ActionDefaultTo, Field: "stage", Value: "lead"},
	}
	at := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	out, err := collection.ApplyHooks(c, map[string]any{"name": "Ada Lovelace"}, at, "create")
	if err != nil {
		t.Fatal(err)
	}
	if out["slug"] != "ada-lovelace" {
		t.Fatalf("slug = %v", out["slug"])
	}
	if out["stage"] != "lead" {
		t.Fatalf("stage = %v", out["stage"])
	}
	if out["createdAt"] != at.Format(time.RFC3339) {
		t.Fatalf("createdAt = %v", out["createdAt"])
	}
}

// defaultTo must not overwrite a value somebody supplied — that is the whole
// difference between a default and an assignment.
func TestDefaultToDoesNotOverwriteAGivenValue(t *testing.T) {
	c := crm()
	c.Hooks = []collection.Hook{{On: "create", Action: collection.ActionDefaultTo, Field: "stage", Value: "lead"}}
	out, err := collection.ApplyHooks(c, map[string]any{"name": "Ada", "stage": "won"}, time.Unix(0, 0), "create")
	if err != nil {
		t.Fatal(err)
	}
	if out["stage"] != "won" {
		t.Fatalf("stage = %v, want the supplied value kept", out["stage"])
	}
}

// A hook naming an action this build does not implement is a refusal, not a
// silent skip: a normalisation that quietly did not happen is a record that
// looks right and is not.
func TestAnUnknownHookActionIsRefused(t *testing.T) {
	c := crm()
	c.Hooks = []collection.Hook{{On: "create", Action: "runShellCommand", Field: "name"}}
	_, err := collection.ApplyHooks(c, map[string]any{"name": "Ada"}, time.Unix(0, 0), "create")
	if code := codeOf(t, err); code != "AOS_COLLECTION_HOOK_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}

// fakeRepo is the in-memory Repository the service tests run on: the domain
// does not touch IO, so nothing here opens a file.
type fakeRepo struct {
	items map[string]collection.Collection

	// failCreate, when set, is what Create returns instead of storing.
	failCreate error

	// onDelete fires once Delete has removed the record, and
	// onUnregisterObserved fires at the start of Delete, before anything is
	// removed — together they are what proves Delete unregisters before it
	// touches the store.
	onDelete             func()
	onUnregisterObserved func()
}

func (r *fakeRepo) Get(_ context.Context, key collections.Key) (*collection.Collection, error) {
	c, ok := r.items[key["id"]]
	if !ok {
		return nil, collections.NotFoundError("collections", key)
	}
	out := c
	return &out, nil
}

func (r *fakeRepo) List(_ context.Context, _ collections.Query) ([]collection.Collection, error) {
	out := make([]collection.Collection, 0, len(r.items))
	for _, c := range r.items {
		out = append(out, c)
	}
	return out, nil
}

func (r *fakeRepo) Create(_ context.Context, v *collection.Collection) error {
	if r.failCreate != nil {
		return r.failCreate
	}
	if r.items == nil {
		r.items = map[string]collection.Collection{}
	}
	r.items[v.ID] = *v
	return nil
}

func (r *fakeRepo) Update(_ context.Context, v *collection.Collection, _ collections.Version) error {
	if r.items == nil {
		r.items = map[string]collection.Collection{}
	}
	r.items[v.ID] = *v
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, key collections.Key) error {
	if r.onUnregisterObserved != nil {
		r.onUnregisterObserved()
	}
	delete(r.items, key["id"])
	if r.onDelete != nil {
		r.onDelete()
	}
	return nil
}

func ctx() context.Context { return context.Background() }

func mustCreate(t *testing.T, svc *collection.Service, id string) *collection.Collection {
	t.Helper()
	got, err := svc.Create(ctx(), collection.CreateInput{
		ID: id, Name: id, Format: collection.FormatMarkdown,
		Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// A collection is registered only after it is stored. A name that resolved to
// nothing on disk would be a collection you can write to and never read back.
func TestCreateRegistersOnlyAfterItPersisted(t *testing.T) {
	repo := &fakeRepo{failCreate: errors.New("the disk is full")}
	reg := collections.NewRegistry()
	svc := collection.NewService(collection.Deps{Repo: repo, Registry: reg, Clock: clockx.Fixed{At: at}})

	_, err := svc.Create(ctx(), collection.CreateInput{
		ID: "contacts", Name: "Contacts", Format: collection.FormatMarkdown,
		Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
	})
	if err == nil {
		t.Fatal("a failing write reported success")
	}
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the collection was registered even though nothing was stored")
	}
}

// Deleting unregisters before removing, so nothing can write a record whose
// declaration is on its way out.
func TestDeleteUnregistersBeforeRemoving(t *testing.T) {
	var order []string
	reg := collections.NewRegistry()
	repo := &fakeRepo{onDelete: func() { order = append(order, "removed") }}
	svc := collection.NewService(collection.Deps{Repo: repo, Registry: reg, Clock: clockx.Fixed{At: at}})

	mustCreate(t, svc, "contacts")
	repo.onUnregisterObserved = func() { order = append(order, "unregistered") }

	if err := svc.Delete(ctx(), collection.DeleteInput{ID: "contacts"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the collection is still registered after being deleted")
	}
	// Nothing may write a record whose declaration is on its way out.
	if len(order) != 2 || order[0] != "unregistered" {
		t.Fatalf("order = %v, want unregister before remove", order)
	}
}

// A reserved name is refused by the registry, and the refusal reaches the
// caller unchanged rather than being restated here.
func TestCreatingACollectionNamedAgentsIsRefused(t *testing.T) {
	svc := collection.NewService(collection.Deps{
		Repo: &fakeRepo{}, Registry: collections.NewRegistry(), Clock: clockx.Fixed{At: at},
	})
	_, err := svc.Create(ctx(), collection.CreateInput{
		ID: "agents", Name: "Agents", Format: collection.FormatMarkdown,
		Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
	})
	// The registry refuses it, and the refusal reaches the caller unchanged
	// rather than being restated here — one list of reserved names, in one
	// place.
	if code := codeOf(t, err); code != "AOS_COLLECTION_NAME_RESERVED" {
		t.Fatalf("code = %q", code)
	}
}

// An id that is not a safe path segment is refused before anything touches the
// filesystem.
func TestAnIdThatIsNotAPathSegmentIsRefused(t *testing.T) {
	svc := collection.NewService(collection.Deps{
		Repo: &fakeRepo{}, Registry: collections.NewRegistry(), Clock: clockx.Fixed{At: at},
	})
	// An id is a directory name. One with a separator or a dot-dot in it is a
	// path traversal, and it has to be refused before anything is resolved.
	for _, id := range []string{"", ".", "..", "../escape", "with/slash", "With Caps", "acentuação"} {
		_, err := svc.Create(ctx(), collection.CreateInput{
			ID: id, Name: "x", Format: collection.FormatMarkdown,
			Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
		})
		if err == nil {
			t.Fatalf("%q was accepted as a collection id", id)
		}
		if code := codeOf(t, err); code != "AOS_COLLECTION_NAME_INVALID" {
			t.Fatalf("%q: code = %q, want AOS_COLLECTION_NAME_INVALID", id, code)
		}
	}
}

func newTestService() (*collection.Service, *fakeRepo, *collections.Registry) {
	repo := &fakeRepo{}
	reg := collections.NewRegistry()
	svc := collection.NewService(collection.Deps{Repo: repo, Registry: reg, Clock: clockx.Fixed{At: at}})
	return svc, repo, reg
}

// TestListReturnsEveryDeclaredCollection.
func TestListReturnsEveryDeclaredCollection(t *testing.T) {
	svc, _, _ := newTestService()
	mustCreate(t, svc, "contacts")
	mustCreate(t, svc, "deals")

	out, err := svc.List(ctx(), collection.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 || len(out.Collections) != 2 {
		t.Fatalf("List = %+v, want 2 collections", out)
	}
}

// TestGetOnAnUnknownCollectionIsNotFound.
func TestGetOnAnUnknownCollectionIsNotFound(t *testing.T) {
	svc, _, _ := newTestService()
	_, err := svc.Get(ctx(), collection.GetInput{ID: "does-not-exist"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

// TestDeleteOnAnUnknownCollectionIsNotFound: nothing to unregister for a
// declaration that was never created.
func TestDeleteOnAnUnknownCollectionIsNotFound(t *testing.T) {
	svc, _, _ := newTestService()
	err := svc.Delete(ctx(), collection.DeleteInput{ID: "does-not-exist"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

// TestDeletingANativeNameIsRefusedByTheRegistry covers Delete's unregister
// step failing: the registry refuses to let go of a name it owns, and the
// refusal reaches the caller unchanged, the same as Create's does.
func TestDeletingANativeNameIsRefusedByTheRegistry(t *testing.T) {
	svc, repo, _ := newTestService()
	// Create fails to register "agents" (it is native) but the write to the
	// repository already went through, which is enough to exercise Delete's
	// own guard against removing a native declaration.
	_, _ = svc.Create(ctx(), collection.CreateInput{
		ID: "agents", Name: "Agents", Format: collection.FormatMarkdown,
		Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
	})
	if repo.items == nil || len(repo.items) != 1 {
		t.Fatalf("test setup: expected the failed create to have still written a record")
	}
	err := svc.Delete(ctx(), collection.DeleteInput{ID: "agents"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_NATIVE_NOT_REMOVABLE" {
		t.Fatalf("code = %q", code)
	}
}

// TestASchemaWithNoFieldsIsRefused.
func TestASchemaWithNoFieldsIsRefused(t *testing.T) {
	c := crm()
	c.Fields = nil
	err := collection.ValidateSchema(c, func(string) bool { return true })
	if code := codeOf(t, err); code != "AOS_COLLECTION_NO_FIELDS" {
		t.Fatalf("code = %q", code)
	}
}

// TestAFieldWithNoNameIsRefused.
func TestAFieldWithNoNameIsRefused(t *testing.T) {
	c := crm()
	c.Fields = []collection.Field{{Type: collection.TypeString}}
	err := collection.ValidateSchema(c, func(string) bool { return true })
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_UNNAMED" {
		t.Fatalf("code = %q", code)
	}
}

// TestADuplicateFieldNameIsRefused: the second declaration would hide the
// first, silently.
func TestADuplicateFieldNameIsRefused(t *testing.T) {
	c := crm()
	c.Fields = []collection.Field{
		{Name: "name", Type: collection.TypeString},
		{Name: "name", Type: collection.TypeNumber},
	}
	err := collection.ValidateSchema(c, func(string) bool { return true })
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_DUPLICATED" {
		t.Fatalf("code = %q", code)
	}
}

// TestAnUnknownFieldTypeIsRefused.
func TestAnUnknownFieldTypeIsRefused(t *testing.T) {
	c := crm()
	c.Fields = []collection.Field{{Name: "x", Type: "currency"}}
	err := collection.ValidateSchema(c, func(string) bool { return true })
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_TYPE_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}

// TestAnEmptyEnumIsRefused: an enum with no values accepts nothing.
func TestAnEmptyEnumIsRefused(t *testing.T) {
	c := crm()
	c.Fields = []collection.Field{{Name: "stage", Type: collection.TypeEnum}}
	err := collection.ValidateSchema(c, func(string) bool { return true })
	if code := codeOf(t, err); code != "AOS_COLLECTION_ENUM_EMPTY" {
		t.Fatalf("code = %q", code)
	}
}

// TestASkillScopedCollectionGetsATwoPatternDescriptor: the second pattern is
// what lets a skill-shipped collection be read without being writable by
// anything other than the skill's own install.
func TestASkillScopedCollectionGetsATwoPatternDescriptor(t *testing.T) {
	c := crm()
	c.Scope = collection.ScopeSkill
	c.Format = collection.FormatJSON
	desc, err := collection.DescriptorFor(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(desc.Patterns) != 2 {
		t.Fatalf("patterns = %d, want 2", len(desc.Patterns))
	}
}

// TestComputeFromHookCopiesAFieldsValue.
func TestComputeFromHookCopiesAFieldsValue(t *testing.T) {
	c := crm()
	c.Hooks = []collection.Hook{{On: "create", Action: collection.ActionComputeFrom, Field: "stage", From: "name"}}
	out, err := collection.ApplyHooks(c, map[string]any{"name": "won"}, at, "create")
	if err != nil {
		t.Fatal(err)
	}
	if out["stage"] != "won" {
		t.Fatalf("stage = %v", out["stage"])
	}
}

// TestCreateRefusesAnIncoherentSchema: Create runs ValidateSchema, not only
// validName — a schema with no fields is refused before anything is written.
func TestCreateRefusesAnIncoherentSchema(t *testing.T) {
	svc, _, _ := newTestService()
	_, err := svc.Create(ctx(), collection.CreateInput{ID: "empty", Name: "Empty", Format: collection.FormatMarkdown})
	if code := codeOf(t, err); code != "AOS_COLLECTION_NO_FIELDS" {
		t.Fatalf("code = %q", code)
	}
}

// fakeRecordRepo is the in-memory collection.RecordRepo one collection's
// records live in. Like fakeRepo, it does no IO: the domain's tests run on
// fakes, never on a filesystem.
type fakeRecordRepo struct {
	items map[string]collections.Record
}

func (r *fakeRecordRepo) Get(_ context.Context, key collections.Key) (*collections.Record, error) {
	rec, ok := r.items[key["id"]]
	if !ok {
		return nil, collections.NotFoundError("records", key)
	}
	out := rec
	return &out, nil
}

// List applies Filters by simple equality — enough to prove RecordQuery
// actually reaches collections.Query, without reimplementing the ordering and
// pagination the real engine's List already has its own tests for.
func (r *fakeRecordRepo) List(_ context.Context, q collections.Query) ([]collections.Record, error) {
	out := make([]collections.Record, 0, len(r.items))
	for _, rec := range r.items {
		match := true
		for field, want := range q.Filters {
			if rec.Fields[field] != want {
				match = false
				break
			}
		}
		if match {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *fakeRecordRepo) Create(_ context.Context, v *collections.Record) error {
	if r.items == nil {
		r.items = map[string]collections.Record{}
	}
	r.items[v.Key["id"]] = *v
	return nil
}

func (r *fakeRecordRepo) Update(_ context.Context, v *collections.Record, _ collections.Version) error {
	if r.items == nil {
		r.items = map[string]collections.Record{}
	}
	r.items[v.Key["id"]] = *v
	return nil
}

func (r *fakeRecordRepo) Delete(_ context.Context, key collections.Key) error {
	delete(r.items, key["id"])
	return nil
}

// fakeRecordRepos is the in-memory collection.RecordRepositories: it hands out
// one fakeRecordRepo per collection id, the same way the real
// internal/adapters/fscollections implementation hands out one Repo per
// collection's Model[collections.Record].
type fakeRecordRepos struct {
	byCollection map[string]*fakeRecordRepo
}

func (f *fakeRecordRepos) For(c collection.Collection) (collection.RecordRepo, error) {
	if f.byCollection == nil {
		f.byCollection = map[string]*fakeRecordRepo{}
	}
	repo, ok := f.byCollection[c.ID]
	if !ok {
		repo = &fakeRecordRepo{}
		f.byCollection[c.ID] = repo
	}
	return repo, nil
}

// newRecordService seeds a fakeRepo with c already declared — Task 5's tests
// are about records, not about declaring collections, which Task 4 already
// covers above — and wires a fresh fakeRecordRepos and a sequence of ids
// behind it.
func newRecordService(t *testing.T, c collection.Collection) collection.RecordService {
	t.Helper()
	repo := &fakeRepo{items: map[string]collection.Collection{c.ID: c}}
	svc := collection.NewService(collection.Deps{
		Repo: repo, Registry: collections.NewRegistry(), Clock: clockx.Fixed{At: at},
		RecordRepos: &fakeRecordRepos{}, IDs: &ids.Sequence{Prefix: "rec"},
	})
	return svc.Records()
}

// A record is validated against its collection's declaration before it is
// stored, and normalised by the collection's hooks. Neither is optional: the
// point of declaring a schema is that what is stored matches it.
func TestCreatingARecordValidatesAndNormalises(t *testing.T) {
	c := crm()
	c.Fields = append(c.Fields, collection.Field{Name: "createdAt", Type: collection.TypeDate})
	c.Hooks = []collection.Hook{{On: "create", Action: collection.ActionSetTimestamp, Field: "createdAt"}}
	svc := newRecordService(t, c)

	rec, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada", "stage": "lead"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Data["createdAt"] != at.Format(time.RFC3339) {
		t.Fatalf("the hook did not run: %v", rec.Data)
	}
	// And the refusal path, from the same service: validation is not optional.
	if _, err := svc.Create(ctx(), "contacts", map[string]any{"stage": "lead"}); err == nil {
		t.Fatal("a record missing a required field was stored")
	}
}

// The unique check needs what is already stored, so the service is what
// fetches it and hands it to Validate — the domain function stays pure.
func TestCreatingASecondRecordWithADuplicateUniqueValueIsRefused(t *testing.T) {
	svc := newRecordService(t, crm())

	if _, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada", "email": "a@b.c"}); err != nil {
		t.Fatal(err)
	}
	// The unique check needs what is already stored, so the service is what
	// fetches it and hands it to Validate — the domain function stays pure.
	_, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Grace", "email": "a@b.c"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_NOT_UNIQUE" {
		t.Fatalf("code = %q", code)
	}
}

// A record in a collection nobody declared is refused, naming it, rather than
// creating the collection implicitly. An implicit collection is a typo that
// becomes a schema.
func TestARecordInAnUnknownCollectionIsRefused(t *testing.T) {
	svc := newRecordService(t, crm())

	// An implicit collection is a typo that becomes a schema.
	_, err := svc.Create(ctx(), "contatos", map[string]any{"name": "Ada"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
	var app *apperr.Error
	_ = errors.As(err, &app)
	if app.Issues["collection"] != "contatos" {
		t.Fatalf("the error does not name what was asked for: %v", app.Issues)
	}
}

// Update revalidates. A record that was valid when written and invalid after
// an edit is exactly what validation is for.
func TestUpdatingARecordRevalidates(t *testing.T) {
	svc := newRecordService(t, crm())
	rec, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada", "stage": "lead"})
	if err != nil {
		t.Fatal(err)
	}

	// A record that was valid when written and invalid after an edit is
	// exactly what validation is for.
	_, err = svc.Update(ctx(), "contacts", rec.ID, map[string]any{"name": "Ada", "stage": "talvez"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_NOT_IN_ENUM" {
		t.Fatalf("code = %q", code)
	}
	// And the stored record is untouched: a refused update is not a partial one.
	again, err := svc.Get(ctx(), "contacts", rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Data["stage"] != "lead" {
		t.Fatalf("stage = %v, want the refused update to have changed nothing", again.Data["stage"])
	}
}

// The Markdown body of a record is content, not a field: it round-trips
// without being declared in the schema.
func TestARecordsBodyIsNotAField(t *testing.T) {
	svc := newRecordService(t, crm())

	// The Markdown body is content, not a column: it round-trips without being
	// declared, and declaring every note-taking collection a "content" field
	// would be describing the format twice.
	rec, err := svc.CreateWithContent(ctx(), "contacts",
		map[string]any{"name": "Ada"}, "Conheceu o Babbage numa festa.")
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.Get(ctx(), "contacts", rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Content != "Conheceu o Babbage numa festa." {
		t.Fatalf("content = %q", again.Content)
	}
	if _, isField := again.Data["content"]; isField {
		t.Fatal("the body leaked into the fields")
	}
}

// TestListingRecordsReturnsWhatWasCreated, ordered and filtered as asked —
// RecordQuery is only worth having if it actually reaches the engine's Query.
func TestListingRecordsReturnsWhatWasCreated(t *testing.T) {
	svc := newRecordService(t, crm())
	for _, name := range []string{"Ada", "Grace"} {
		if _, err := svc.Create(ctx(), "contacts", map[string]any{"name": name}); err != nil {
			t.Fatal(err)
		}
	}

	all, err := svc.List(ctx(), "contacts", collection.RecordQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("List = %d records, want 2", len(all))
	}

	filtered, err := svc.List(ctx(), "contacts", collection.RecordQuery{Filters: map[string]any{"name": "Ada"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].Data["name"] != "Ada" {
		t.Fatalf("filtered List = %+v, want just Ada", filtered)
	}
}

// TestListingAnUnknownCollectionIsRefused: List goes through the same
// declaration lookup Create does, and refuses the same way.
func TestListingAnUnknownCollectionIsRefused(t *testing.T) {
	svc := newRecordService(t, crm())
	_, err := svc.List(ctx(), "contatos", collection.RecordQuery{})
	if code := codeOf(t, err); code != "AOS_COLLECTION_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

// TestGettingAnUnknownRecordIsNotFound names both the collection and the
// record, so an agent that typed the id wrong knows which one missed.
func TestGettingAnUnknownRecordIsNotFound(t *testing.T) {
	svc := newRecordService(t, crm())
	_, err := svc.Get(ctx(), "contacts", "does-not-exist")
	if code := codeOf(t, err); code != "AOS_COLLECTION_RECORD_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

// TestUpdatingAnUnknownRecordIsNotFound: the same refusal Get gives, since
// there is nothing to revalidate against.
func TestUpdatingAnUnknownRecordIsNotFound(t *testing.T) {
	svc := newRecordService(t, crm())
	_, err := svc.Update(ctx(), "contacts", "does-not-exist", map[string]any{"name": "Ada"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_RECORD_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

// TestDeletingARecordRemovesIt.
func TestDeletingARecordRemovesIt(t *testing.T) {
	svc := newRecordService(t, crm())
	rec, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx(), "contacts", rec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx(), "contacts", rec.ID); err == nil {
		t.Fatal("the record was still readable after being deleted")
	}
}

// TestDeletingFromAnUnknownCollectionIsRefused: Delete goes through the same
// declaration lookup the other operations do.
func TestDeletingFromAnUnknownCollectionIsRefused(t *testing.T) {
	svc := newRecordService(t, crm())
	err := svc.Delete(ctx(), "contatos", "any-id")
	if code := codeOf(t, err); code != "AOS_COLLECTION_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

// TestCreateWithContentRunsUpdateHooksToo: an unknown hook action is refused
// on update the same way it is on create — hooks are not a create-only pass.
func TestUpdateRefusesAnUnknownHookAction(t *testing.T) {
	c := crm()
	c.Hooks = []collection.Hook{{On: "update", Action: "runShellCommand", Field: "name"}}
	svc := newRecordService(t, c)

	rec, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Update(ctx(), "contacts", rec.ID, map[string]any{"name": "Ada Lovelace"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_HOOK_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}

// TestRecordTimestampsAreAProjectionOfDeclaredFields: CreatedAt/UpdatedAt on
// the domain Record are read back from the collection's own declared
// createdAt/updatedAt fields — the same fields a setTimestamp hook stamps and
// Validate already enforces — not derived from anything the storage layer
// tracks on its own.
func TestRecordTimestampsAreAProjectionOfDeclaredFields(t *testing.T) {
	c := crm()
	c.Fields = append(c.Fields,
		collection.Field{Name: "createdAt", Type: collection.TypeDate},
		collection.Field{Name: "updatedAt", Type: collection.TypeDate},
	)
	c.Hooks = []collection.Hook{
		{On: "create", Action: collection.ActionSetTimestamp, Field: "createdAt"},
		{On: "create", Action: collection.ActionSetTimestamp, Field: "updatedAt"},
	}
	svc := newRecordService(t, c)

	rec, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if !rec.CreatedAt.Equal(at) || !rec.UpdatedAt.Equal(at) {
		t.Fatalf("CreatedAt = %v, UpdatedAt = %v, want both %v", rec.CreatedAt, rec.UpdatedAt, at)
	}
}

// TestRecordTimestampsAreZeroWhenNotDeclared: a collection that never
// declared createdAt/updatedAt fields has nothing to report, and reports it
// honestly as zero rather than borrowing a file's modification time.
func TestRecordTimestampsAreZeroWhenNotDeclared(t *testing.T) {
	svc := newRecordService(t, crm())
	rec, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if !rec.CreatedAt.IsZero() || !rec.UpdatedAt.IsZero() {
		t.Fatalf("CreatedAt = %v, UpdatedAt = %v, want both zero", rec.CreatedAt, rec.UpdatedAt)
	}
}

// TestRecordsDataIsIndependentOfStorage: a caller mutating the Data map a
// service call handed back must not corrupt what the repository holds for
// the next reader. recordFrom must copy rec.Fields, never hand out the map
// itself — the same defect WithoutBody guards against for the engine's own
// in-memory index.
func TestRecordsDataIsIndependentOfStorage(t *testing.T) {
	svc := newRecordService(t, crm())
	rec, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada"})
	if err != nil {
		t.Fatal(err)
	}

	rec.Data["name"] = "mutated by the caller"

	again, err := svc.Get(ctx(), "contacts", rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Data["name"] != "Ada" {
		t.Fatalf("stored name = %v, want the caller's mutation to not have reached storage", again.Data["name"])
	}
}

// countingRecordRepo wraps fakeRecordRepo to count List calls: it is the
// proof that existingFields' conditional actually skips the scan, not merely
// that Validate tolerates a nil slice of existing records.
type countingRecordRepo struct {
	*fakeRecordRepo
	listCalls int
}

func (r *countingRecordRepo) List(ctx context.Context, q collections.Query) ([]collections.Record, error) {
	r.listCalls++
	return r.fakeRecordRepo.List(ctx, q)
}

type countingRecordRepos struct {
	repo *countingRecordRepo
}

func (f *countingRecordRepos) For(collection.Collection) (collection.RecordRepo, error) {
	return f.repo, nil
}

// TestCreatingARecordSkipsTheExistingScanWithoutAUniqueField: an unconditional
// List on every Create would make every collection pay for a constraint most
// do not declare. This is what proves the skip actually happens, rather than
// just trusting the conditional's own comment.
func TestCreatingARecordSkipsTheExistingScanWithoutAUniqueField(t *testing.T) {
	c := crm()
	for i := range c.Fields {
		c.Fields[i].Unique = false
	}
	repo := &fakeRepo{items: map[string]collection.Collection{c.ID: c}}
	repos := &countingRecordRepos{repo: &countingRecordRepo{fakeRecordRepo: &fakeRecordRepo{}}}
	svc := collection.NewService(collection.Deps{
		Repo: repo, Registry: collections.NewRegistry(), Clock: clockx.Fixed{At: at},
		RecordRepos: repos, IDs: &ids.Sequence{Prefix: "rec"},
	}).Records()

	if _, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada"}); err != nil {
		t.Fatal(err)
	}
	if repos.repo.listCalls != 0 {
		t.Fatalf("List was called %d times; a collection with no Unique field must not pay for the scan", repos.repo.listCalls)
	}
}
