package view_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/view"
)

// refTime is fixed so a CreatedAt/UpdatedAt in a test is reproducible and
// never depends on how fast the machine running it is.
var refTime = time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)

func ctx() context.Context { return context.Background() }

// codeOf extracts the apperr code from err, failing the test if err does not
// carry one — every refusal in this package must.
func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not *apperr.Error: %v", err, err)
	}
	return app.Code
}

// ---- fakeRepository: an in-memory view.Repository ----

type fakeRepository struct {
	mu    sync.Mutex
	views map[string]view.View
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{views: map[string]view.View{}}
}

func (r *fakeRepository) Get(_ context.Context, key collections.Key) (*view.View, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.views[key["id"]]
	if !ok {
		return nil, fmt.Errorf("fakeRepository: no view %q", key["id"])
	}
	out := v
	return &out, nil
}

func (r *fakeRepository) List(_ context.Context, _ collections.Query) ([]view.View, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]view.View, 0, len(r.views))
	for _, v := range r.views {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *fakeRepository) Create(_ context.Context, v *view.View) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.views[v.ID] = *v
	return nil
}

func (r *fakeRepository) Update(_ context.Context, v *view.View, _ collections.Version) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.views[v.ID] = *v
	return nil
}

func (r *fakeRepository) Delete(_ context.Context, key collections.Key) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.views, key["id"])
	return nil
}

// ---- fakeCollections: an in-memory view.Collections ----

type fakeCollections struct {
	schemas map[string]collection.Collection
	records map[string][]collection.Record
}

func (f *fakeCollections) Get(_ context.Context, id string) (*collection.Collection, error) {
	c, ok := f.schemas[id]
	if !ok {
		return nil, apperr.New("TEST_COLLECTION_NOT_FOUND").
			Causer("view_test.fakeCollections.Get").
			Msgf("no collection %q", id).
			Issue("collection", id).
			Status(apperr.StatusNotFound).
			CTA(apperr.CallToAction{Label: "declare the collection first"})
	}
	out := c
	return &out, nil
}

func (f *fakeCollections) ListRecords(_ context.Context, id string, q collection.RecordQuery) ([]collection.Record, error) {
	recs := append([]collection.Record(nil), f.records[id]...)
	if q.OrderBy != "" {
		sort.Slice(recs, func(i, j int) bool {
			vi, _ := recs[i].Data[q.OrderBy].(string)
			vj, _ := recs[j].Data[q.OrderBy].(string)
			if q.Desc {
				return vi > vj
			}
			return vi < vj
		})
	}
	if q.Limit > 0 && len(recs) > q.Limit {
		recs = recs[:q.Limit]
	}
	return recs, nil
}

// ---- fakeCommands: an in-memory view.Commands ----

type fakeCommands struct {
	known   map[string]bool
	invoked []string
}

func (f *fakeCommands) Has(name string) bool { return f.known[name] }

func (f *fakeCommands) Invoke(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
	f.invoked = append(f.invoked, name)
	return json.RawMessage(`{}`), nil
}

// ---- newService and its options ----

type serviceConfig struct {
	repo        *fakeRepository
	collections *fakeCollections
	commands    *fakeCommands
}

type serviceOption func(*serviceConfig)

func withCollection(c collection.Collection) serviceOption {
	return func(cfg *serviceConfig) { cfg.collections.schemas[c.ID] = c }
}

func withRecords(collectionID string, records ...map[string]any) serviceOption {
	return func(cfg *serviceConfig) {
		for _, data := range records {
			cfg.collections.records[collectionID] = append(cfg.collections.records[collectionID], collection.Record{
				ID:         fmt.Sprintf("%s-%d", collectionID, len(cfg.collections.records[collectionID])),
				Collection: collectionID,
				Data:       data,
			})
		}
	}
}

// withCommands registers the given command names as known, for the tests
// that only need Validate's registry check to pass and never inspect what
// was invoked.
func withCommands(names ...string) serviceOption {
	return func(cfg *serviceConfig) {
		for _, n := range names {
			cfg.commands.known[n] = true
		}
	}
}

// withCommandPort replaces the commands fake outright, for the tests that
// need to inspect what ExecuteAction actually dispatched.
func withCommandPort(cmds *fakeCommands) serviceOption {
	return func(cfg *serviceConfig) { cfg.commands = cmds }
}

func newService(t *testing.T, opts ...serviceOption) *view.Service {
	t.Helper()
	cfg := &serviceConfig{
		repo: newFakeRepository(),
		collections: &fakeCollections{
			schemas: map[string]collection.Collection{},
			records: map[string][]collection.Record{},
		},
		commands: &fakeCommands{known: map[string]bool{}},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return view.NewService(view.Deps{
		Repo:        cfg.repo,
		Collections: cfg.collections,
		Commands:    cfg.commands,
		Clock:       clockx.Fixed{At: refTime},
	})
}

// contactsSchema declares name, email, stage — nothing else. Several tests
// rely on that exact set: "salary" is deliberately absent from it.
func contactsSchema() collection.Collection {
	return collection.Collection{
		ID:     "contacts",
		Name:   "Contacts",
		Scope:  collection.ScopeWorkspace,
		Format: collection.FormatJSON,
		Fields: []collection.Field{
			{Name: "name", Type: collection.TypeString, Required: true},
			{Name: "email", Type: collection.TypeString},
			{Name: "stage", Type: collection.TypeEnum, Enum: []string{"lead", "won"}},
		},
	}
}

// mustCreateView writes a view whose tree is a bare Stack — every one of its
// props is nullable, so it validates with nothing further — over source, and
// fails the test if the write is refused.
func mustCreateView(t *testing.T, svc *view.Service, id string, source view.Source) *view.View {
	t.Helper()
	v, err := svc.Create(ctx(), view.CreateInput{
		ID:     id,
		Name:   id,
		Title:  id,
		Source: source,
		Tree:   view.Node{Component: "Stack"},
	})
	if err != nil {
		t.Fatalf("mustCreateView(%q): %v", id, err)
	}
	return v
}

// mustCreateViewWithAction writes a view whose tree is a single Button
// carrying action, against the "contacts" collection.
func mustCreateViewWithAction(t *testing.T, svc *view.Service, id string, action view.Action) *view.View {
	t.Helper()
	v, err := svc.Create(ctx(), view.CreateInput{
		ID:     id,
		Name:   id,
		Title:  id,
		Source: view.Source{Collection: "contacts"},
		Tree: view.Node{
			Component: "Button",
			Props:     map[string]any{"label": action.Label},
			Actions:   []view.Action{action},
		},
	})
	if err != nil {
		t.Fatalf("mustCreateViewWithAction(%q): %v", id, err)
	}
	return v
}

// A view that would render blank is never written. Each refusal has to name
// what is wrong, because the reader is an agent that will try again
// immediately and needs to know what to change.
func TestAnUnknownComponentIsRefusedNamingIt(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "SuperTable"},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_COMPONENT_UNKNOWN" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["component"] != "SuperTable" {
		t.Fatalf("the error does not name the component: %v", app.Issues)
	}
}

func TestAMissingRequiredPropIsRefusedNamingIt(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	// Heading declares `text` as required in the catalog.
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "Heading", Props: map[string]any{"level": "h2"}},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_PROP_REQUIRED" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["prop"] != "text" {
		t.Fatalf("the error does not name the prop: %v", app.Issues)
	}
}

func TestAPropOfTheWrongTypeIsRefusedNamingIt(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "Heading", Props: map[string]any{"text": 7}},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_PROP_WRONG_TYPE" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["prop"] != "text" || app.Issues["expected"] != "string" {
		t.Fatalf("the error does not say what was expected: %v", app.Issues)
	}
}

// A bind points a prop at a field of the source collection. Pointing it at a
// field the collection does not declare renders nothing, silently.
func TestABindToAFieldTheCollectionDoesNotHaveIsRefused(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	// contacts declares name, email, stage. "salary" renders nothing, silently.
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "Text", Bind: map[string]string{"text": "salary"}},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_BIND_UNKNOWN_FIELD" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["field"] != "salary" {
		t.Fatalf("the error does not name the field: %v", app.Issues)
	}
}

// An action names a command from the registry. A view is not a parallel path
// to mutation: the same validation and the same authorisation apply.
func TestAnActionNamingAnUnregisteredCommandIsRefused(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()), withCommands("collections_records-delete"))
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{
			Component: "Button",
			Props:     map[string]any{"label": "Apagar tudo"},
			Actions:   []view.Action{{Label: "Apagar tudo", Command: "database_drop"}},
		},
	})
	// A view is not a parallel path to mutation: an action is a Descriptor
	// from the registry, with the same authorisation as any other surface.
	if code := codeOf(t, err); code != "AOS_VIEW_ACTION_COMMAND_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}

// A component that does not accept children may not have any.
func TestChildrenUnderALeafComponentAreRefused(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	// Badge declares no slots, so nothing can nest under it.
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{
			Component: "Badge",
			Children:  []view.Node{{Component: "Text", Props: map[string]any{"text": "x"}}},
		},
	})
	if code := codeOf(t, err); code != "AOS_VIEW_CHILDREN_NOT_ACCEPTED" {
		t.Fatalf("code = %q", code)
	}
}

// The refusal has to reach the deepest node, not only the root.
func TestAnInvalidNodeDeepInTheTreeIsStillRefused(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "Stack", Children: []view.Node{
			{Component: "Card", Children: []view.Node{
				{Component: "Stack", Children: []view.Node{
					{Component: "NotAComponent"},
				}},
			}},
		}},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_COMPONENT_UNKNOWN" {
		t.Fatalf("code = %q", app.Code)
	}
	// A view of thirty nodes whose error does not locate the bad one is an
	// error the agent cannot act on.
	at, _ := app.Issues["at"].(string)
	if !strings.Contains(at, "children") {
		t.Fatalf("the error does not say where in the tree: %v", app.Issues)
	}
}

// Render resolves the source and returns the tree with the data attached —
// which is what the frontend's renderer consumes.
func TestRenderAttachesTheCollectionsRecords(t *testing.T) {
	svc := newService(t,
		withCollection(contactsSchema()),
		withRecords("contacts",
			map[string]any{"name": "Ada", "stage": "won"},
			map[string]any{"name": "Grace", "stage": "lead"},
		))
	mustCreateView(t, svc, "crm", view.Source{Collection: "contacts", Sort: []view.SortSpec{{Field: "name"}}})

	out, err := svc.Render(ctx(), view.RenderInput{ID: "crm"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Records) != 2 {
		t.Fatalf("rendered %d records, want 2", len(out.Records))
	}
	if out.Records[0].Data["name"] != "Ada" {
		t.Fatalf("the sort declared in Source was not applied: %v", out.Records)
	}
	if out.View.ID != "crm" {
		t.Fatal("the rendered payload does not carry the view itself")
	}
}

// Scaffold is what lets an agent produce a valid view without guessing: it
// infers components from the field types.
func TestScaffoldProducesAViewThatValidates(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	v, err := svc.Scaffold(ctx(), view.ScaffoldInput{Collection: "contacts", Kind: view.KindTable})
	if err != nil {
		t.Fatal(err)
	}
	// The proof is not that it produced something; it is that what it produced
	// survives the same validation everything else goes through.
	if _, err := svc.Create(ctx(), view.CreateInput{ID: "scaffolded", Source: v.Source, Tree: v.Tree}); err != nil {
		t.Fatalf("Scaffold produced a view its own service refuses: %v", err)
	}
}

// Every component Scaffold can emit must exist in the generated catalog. This
// is the test that covers the difference between an embedded JSON catalog and
// a generated .gen.go: the compiler is not checking it, so this is.
func TestEveryComponentScaffoldCanEmitIsInTheCatalog(t *testing.T) {
	for _, name := range view.ScaffoldComponents() {
		if _, ok := view.LookupComponent(name); !ok {
			t.Fatalf("Scaffold emits %q, which is not in the catalog", name)
		}
	}
}

// Scaffold also composes for Board and Detail — the two kinds with no single
// catalog component of their own — and what it produces must validate too.
func TestScaffoldForDetailProducesAViewThatValidates(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	v, err := svc.Scaffold(ctx(), view.ScaffoldInput{Collection: "contacts", Kind: view.KindDetail})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx(), view.CreateInput{ID: "scaffolded-detail", Source: v.Source, Tree: v.Tree}); err != nil {
		t.Fatalf("Scaffold(Detail) produced a view its own service refuses: %v", err)
	}
}

// ExecuteAction does not execute anything itself: it resolves the declared
// action and dispatches through the registry.
func TestExecuteActionDispatchesThroughTheRegistry(t *testing.T) {
	cmds := &fakeCommands{known: map[string]bool{"collections_records-delete": true}}
	svc := newService(t, withCollection(contactsSchema()), withCommandPort(cmds))
	mustCreateViewWithAction(t, svc, "crm", view.Action{
		Label: "Apagar", Command: "collections_records-delete",
		Input: map[string]any{"collection": "contacts"},
	})

	if _, err := svc.ExecuteAction(ctx(), view.ExecuteActionInput{
		ID: "crm", Label: "Apagar", Input: map[string]any{"id": "ada"},
	}); err != nil {
		t.Fatal(err)
	}
	// The view resolved and dispatched; it did not do the work itself.
	if len(cmds.invoked) != 1 || cmds.invoked[0] != "collections_records-delete" {
		t.Fatalf("invoked = %v", cmds.invoked)
	}
}

// An action the view does not declare cannot be executed, even by naming a
// real command — otherwise the view is a way to call anything.
func TestExecuteActionRefusesAnActionTheViewDoesNotDeclare(t *testing.T) {
	cmds := &fakeCommands{known: map[string]bool{"collections_records-delete": true}}
	svc := newService(t, withCollection(contactsSchema()), withCommandPort(cmds))
	mustCreateView(t, svc, "crm", view.Source{Collection: "contacts"})

	// Naming a real command is not enough. If it were, the view would be a way
	// to call anything in the registry from a button that was never declared.
	_, err := svc.ExecuteAction(ctx(), view.ExecuteActionInput{ID: "crm", Label: "Apagar"})
	if code := codeOf(t, err); code != "AOS_VIEW_ACTION_NOT_DECLARED" {
		t.Fatalf("code = %q", code)
	}
	if len(cmds.invoked) != 0 {
		t.Fatalf("it dispatched anyway: %v", cmds.invoked)
	}
}

// --- CRUD surface not exercised by the refusal and rendering tests above ---

func TestListReturnsEveryCreatedView(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	mustCreateView(t, svc, "crm-a", view.Source{Collection: "contacts"})
	mustCreateView(t, svc, "crm-b", view.Source{Collection: "contacts"})

	out, err := svc.List(ctx(), view.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 || len(out.Views) != 2 {
		t.Fatalf("List returned %d views, want 2", out.Total)
	}
}

func TestGetOnAMissingViewIsRefusedNamingIt(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	_, err := svc.Get(ctx(), view.GetInput{ID: "nope"})
	if code := codeOf(t, err); code != "AOS_VIEW_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

func TestDeleteRemovesAView(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	mustCreateView(t, svc, "crm", view.Source{Collection: "contacts"})

	if err := svc.Delete(ctx(), view.DeleteInput{ID: "crm"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx(), view.GetInput{ID: "crm"}); err == nil {
		t.Fatal("the view still exists after Delete")
	}
}

// Create refuses an empty id up front: it is also the view's file name.
func TestCreateRefusesAnEmptyID(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	_, err := svc.Create(ctx(), view.CreateInput{
		Source: view.Source{Collection: "contacts"},
		Tree:   view.Node{Component: "Stack"},
	})
	if code := codeOf(t, err); code != "AOS_VIEW_ID_REQUIRED" {
		t.Fatalf("code = %q", code)
	}
}

// Mutating what List, Get and Create hand back must not corrupt what the
// service holds underneath — the same defect this codebase has already shipped
// four times over, for a map or a slice that crossed a cache boundary
// uncopied.
func TestReturnedViewsAreImmutableToTheCaller(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	mustCreateView(t, svc, "crm", view.Source{Collection: "contacts"})

	got, err := svc.Get(ctx(), view.GetInput{ID: "crm"})
	if err != nil {
		t.Fatal(err)
	}
	got.Tree.Component = "Corrupted"

	again, err := svc.Get(ctx(), view.GetInput{ID: "crm"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Tree.Component == "Corrupted" {
		t.Fatal("mutating a View returned by Get corrupted what the service holds")
	}
}
