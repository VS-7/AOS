package collection_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
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
