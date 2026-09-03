package command_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
)

type sampleInput struct {
	Title      string   `json:"title" jsonschema:"A brief summary." validate:"required,max=200"`
	Confidence float64  `json:"confidence,omitempty" jsonschema:"0..1 — how sure the caller is." validate:"gte=0,lte=1"`
	Scopes     []string `json:"scopes,omitempty" jsonschema:"Globs where this applies."`

	command.Reasoning
}

type sampleOutput struct {
	Echo string `json:"echo"`
}

func handler(_ context.Context, in sampleInput) (sampleOutput, error) {
	return sampleOutput{Echo: in.Title}, nil
}

func sampleCommand() command.Command[sampleInput, sampleOutput] {
	return command.Command[sampleInput, sampleOutput]{
		Group:    "memories",
		Name:     "store",
		Summary:  "Record a durable memory.",
		Doc:      "Record a durable memory in the knowledge graph.",
		Registry: true,
		Handler:  handler,
	}
}

func newRegistry(t *testing.T) *command.Registry {
	t.Helper()
	reg := command.NewRegistry()
	if err := command.Register(reg, sampleCommand()); err != nil {
		t.Fatal(err)
	}
	return reg
}

// TestToolNameIsThePathJoinedByUnderscore pins the naming rule the whole
// ecosystem depends on: a model that has seen memories_store elsewhere gets it
// right on the first attempt.
func TestToolNameIsThePathJoinedByUnderscore(t *testing.T) {
	reg := newRegistry(t)
	d, _, ok := reg.Lookup("memories_store")
	if !ok {
		t.Fatal("memories_store is not registered")
	}
	if got := strings.Join(d.Path(), "/"); got != "memories/store" {
		t.Errorf("path = %q", got)
	}
}

// TestSortedIsAlphabeticalAndStable: the published tool list must not shuffle
// between two runs of the same binary.
func TestSortedIsAlphabeticalAndStable(t *testing.T) {
	reg := command.NewRegistry()
	for _, name := range []string{"store", "recall", "forget", "graph", "reflect"} {
		c := sampleCommand()
		c.Name = name
		if err := command.Register(reg, c); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		"memories_forget", "memories_graph", "memories_recall",
		"memories_reflect", "memories_store",
	}
	for attempt := 0; attempt < 5; attempt++ {
		got := make([]string, 0, len(want))
		for _, d := range reg.Sorted() {
			got = append(got, d.Key())
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("attempt %d: order = %v, want %v", attempt, got, want)
			}
		}
	}
}

// TestRegistrationRefusesAnInputWithoutReasoning is the guard that makes
// "_reasoning is mandatory" true by construction rather than by discipline.
func TestRegistrationRefusesAnInputWithoutReasoning(t *testing.T) {
	type forgetful struct {
		Title string `json:"title" jsonschema:"A title."`
	}
	err := command.Register(command.NewRegistry(), command.Command[forgetful, sampleOutput]{
		Group: "memories", Name: "store",
		Handler: func(context.Context, forgetful) (sampleOutput, error) { return sampleOutput{}, nil },
	})
	if err == nil {
		t.Fatal("a command without _reasoning must not register")
	}
	if !strings.Contains(err.Error(), command.ReasoningField) {
		t.Fatalf("the error must name the missing field: %v", err)
	}
}

// TestRegistrationRefusesAFieldWithoutTags: reflection over struct tags fails
// silently otherwise — a field without a json tag becomes a flag named after
// the Go field, changing the published surface with no error at all.
func TestRegistrationRefusesAFieldWithoutTags(t *testing.T) {
	type untagged struct {
		Title string
		command.Reasoning
	}
	type undescribed struct {
		Title string `json:"title"`
		command.Reasoning
	}

	for name, register := range map[string]func(*command.Registry) error{
		"no json tag": func(r *command.Registry) error {
			return command.Register(r, command.Command[untagged, sampleOutput]{
				Group: "g", Name: "n",
				Handler: func(context.Context, untagged) (sampleOutput, error) { return sampleOutput{}, nil },
			})
		},
		"no jsonschema description": func(r *command.Registry) error {
			return command.Register(r, command.Command[undescribed, sampleOutput]{
				Group: "g", Name: "n",
				Handler: func(context.Context, undescribed) (sampleOutput, error) { return sampleOutput{}, nil },
			})
		},
	} {
		if err := register(command.NewRegistry()); err == nil {
			t.Errorf("%s: registration should have failed", name)
		}
	}
}

func TestReasoningIsRequiredOnToolSurfacesOnly(t *testing.T) {
	reg := newRegistry(t)
	d, _, _ := reg.Lookup("memories_store")
	payload := json.RawMessage(`{"title":"a memory"}`)

	// A human at a terminal does not justify the command they just typed.
	if _, err := d.Invoke(context.Background(), command.SurfaceCLI, payload); err != nil {
		t.Errorf("the CLI must not require _reasoning: %v", err)
	}
	if _, err := d.Invoke(context.Background(), command.SurfaceHTTP, payload); err != nil {
		t.Errorf("HTTP must not require _reasoning: %v", err)
	}

	// A model must say why it is calling.
	for _, surface := range []command.Surface{command.SurfaceMCP, command.SurfaceAgent} {
		_, err := d.Invoke(context.Background(), surface, payload)
		if err == nil {
			t.Fatalf("%s must require _reasoning", surface)
		}
		e, ok := apperr.As(err)
		if !ok {
			t.Fatalf("%s: %v", surface, err)
		}
		if got := e.Issues[command.ReasoningField]; got != command.ReasoningRejection {
			t.Errorf("%s: the rejection message is not the original's: %v", surface, got)
		}
	}
}

// TestAnEmptyReasoningIsARejectedCall reproduces the original's wording, which
// is what the model has learned to satisfy.
func TestAnEmptyReasoningIsARejectedCall(t *testing.T) {
	reg := newRegistry(t)
	d, _, _ := reg.Lookup("memories_store")
	_, err := d.Invoke(context.Background(), command.SurfaceMCP,
		json.RawMessage(`{"title":"a memory","_reasoning":"   "}`))
	if err == nil {
		t.Fatal("whitespace is not a reason")
	}
}

// TestValidationErrorCarriesTheIntrospectionPath follows the rule the master
// prompt gives the agent: do not retry blindly, inspect the contract, then fix.
// The error carries that path already built.
func TestValidationErrorCarriesTheIntrospectionPath(t *testing.T) {
	reg := newRegistry(t)
	d, _, _ := reg.Lookup("memories_store")
	_, err := d.Invoke(context.Background(), command.SurfaceAgent,
		json.RawMessage(`{"confidence":5,"_reasoning":"why not"}`))
	if err == nil {
		t.Fatal("expected a validation failure")
	}
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("error = %v", err)
	}
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Error("a bad payload is a 400")
	}
	if len(e.Actions) == 0 || e.Actions[0].Tool != "memories_store" {
		t.Fatalf("the CTA must point at the tool's own schema: %+v", e.Actions)
	}
	if _, asked := e.Actions[0].Input.(map[string]any)["schema"]; !asked {
		t.Errorf("the CTA must ask for schema:true, got %+v", e.Actions[0].Input)
	}
	// The issues name the fields by their JSON name — the name every surface uses.
	if _, reported := e.Issues["title"]; !reported {
		t.Errorf("the missing required field is not reported: %v", e.Issues)
	}
	if _, reported := e.Issues["confidence"]; !reported {
		t.Errorf("the out-of-range field is not reported: %v", e.Issues)
	}
}

func TestInvalidJSONIsReportedWithTheSameCTA(t *testing.T) {
	reg := newRegistry(t)
	d, _, _ := reg.Lookup("memories_store")
	_, err := d.Invoke(context.Background(), command.SurfaceAgent, json.RawMessage(`{"title":`))
	e, ok := apperr.As(err)
	if !ok || len(e.Actions) == 0 {
		t.Fatalf("error = %v", err)
	}
	if e.Code != "AOS_COMMAND_INVALID_INPUT" {
		t.Errorf("code = %q", e.Code)
	}
}

func TestSchemaIsComputedOnceAtRegistration(t *testing.T) {
	reg := newRegistry(t)
	d, _, _ := reg.Lookup("memories_store")
	first := d.InputSchema()
	if first == nil {
		t.Fatal("no schema")
	}
	if d.InputSchema() != first {
		t.Error("the schema is being recomputed per call")
	}
	if first.Properties["title"].Description != "A brief summary." {
		t.Errorf("the jsonschema tag did not become the description: %+v", first.Properties["title"])
	}
	if first.Properties[command.ReasoningField] == nil {
		t.Error("_reasoning must be part of the published schema")
	}
}

func TestPrivilegeBoundary(t *testing.T) {
	reg := command.NewRegistry()
	open := sampleCommand()
	if err := command.Register(reg, open); err != nil {
		t.Fatal(err)
	}
	closed := sampleCommand()
	closed.Group, closed.Name, closed.Registry = "gateway", "restart", false
	if err := command.Register(reg, closed); err != nil {
		t.Fatal(err)
	}

	for _, d := range reg.AgentTools() {
		if d.Key() == "gateway_restart" {
			t.Fatal("the agent must not reach the gateway: it operates the domain, not the installation")
		}
	}
	if len(reg.All()) != 2 {
		t.Fatalf("the command is still published on the other surfaces: %d", len(reg.All()))
	}
}

func TestAliasResolvesAndAnnouncesTheNewName(t *testing.T) {
	reg := newRegistry(t)
	reg.MustAlias(command.Alias{
		From: "memories_create", To: "memories_store",
		Since: "0.4.0", RemoveAt: "1.0.0", Deprecated: true,
	})

	d, notice, ok := reg.Lookup("memories_create")
	if !ok {
		t.Fatal("the alias does not resolve")
	}
	if d.Key() != "memories_store" {
		t.Fatalf("resolved to %q", d.Key())
	}
	if notice == nil || notice.Use != "memories_store" {
		t.Fatalf("notice = %+v", notice)
	}
	if !strings.Contains(notice.Message, "1.0.0") {
		t.Errorf("the notice must say when the old name stops working: %q", notice.Message)
	}

	// The notice travels in the envelope, so the model relearns by itself.
	raw, err := command.MarshalEnvelope(sampleOutput{Echo: "x"}, notice)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"_deprecated"`) {
		t.Errorf("envelope = %s", raw)
	}
}

func TestAnAliasCannotShadowARealCommand(t *testing.T) {
	reg := newRegistry(t)
	if err := reg.Alias(command.Alias{From: "memories_store", To: "memories_store"}); err == nil {
		t.Fatal("an alias over a real command would hide it")
	}
}

func TestRegistryRefusesDuplicatesAndLateAdditions(t *testing.T) {
	reg := newRegistry(t)
	if err := command.Register(reg, sampleCommand()); err == nil {
		t.Error("the same key must not register twice")
	}
	reg.Freeze()
	other := sampleCommand()
	other.Name = "recall"
	if err := command.Register(reg, other); err == nil {
		t.Error("a frozen registry must not accept a new command: the published list is a contract")
	}
}

func TestGroupsAreOrderedAndCarryTheirDocumentation(t *testing.T) {
	reg := newRegistry(t)
	reg.DescribeGroup(command.GroupDoc{Name: "memories", Tool: "Memory", Summary: "Remember things."})

	groups := reg.Groups()
	if len(groups) != 1 {
		t.Fatalf("%d groups", len(groups))
	}
	if groups[0].Tool != "Memory" || groups[0].Summary != "Remember things." {
		t.Fatalf("group = %+v", groups[0].GroupDoc)
	}
}

func TestEnvelopeCarriesCallsToActionOnSuccess(t *testing.T) {
	raw, err := command.MarshalEnvelope(withSuggestion{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"_cta"`) || !strings.Contains(string(raw), "start the task") {
		t.Fatalf("envelope = %s", raw)
	}
}

type withSuggestion struct {
	ID string `json:"id"`
}

func (withSuggestion) CallsToAction() []apperr.CallToAction {
	return []apperr.CallToAction{{Label: "start the task", Tool: "tasks_start"}}
}

// TestDescriptorExposesEverythingTheSurfacesNeed walks the accessors that the
// CLI, MCP and the documentation generator each read. They are trivial, and
// that is the point: a surface that cannot see a field cannot publish it.
func TestDescriptorExposesEverythingTheSurfacesNeed(t *testing.T) {
	reg := command.NewRegistry()
	c := sampleCommand()
	c.Aliases = []string{"put"}
	c.Local = true
	c.Examples = []command.Example{{Description: "a preference", Input: sampleInput{Title: "x"}}}
	c.Annotations = command.Annotations{Title: "Store a memory", DestructiveHint: true}
	if err := command.Register(reg, c); err != nil {
		t.Fatal(err)
	}
	d, _, _ := reg.Lookup("memories_store")

	if d.Name() != "store" || d.Group() != "memories" {
		t.Errorf("name/group = %q/%q", d.Name(), d.Group())
	}
	if len(d.Aliases()) != 1 || d.Aliases()[0] != "put" {
		t.Errorf("aliases = %v", d.Aliases())
	}
	if d.Summary() == "" || d.Doc() == "" {
		t.Error("a tool without a summary or documentation is unusable by a model")
	}
	if len(d.Examples()) != 1 {
		t.Errorf("examples = %v", d.Examples())
	}
	if !d.Local() {
		t.Error("Local was not carried through")
	}
	if !d.Annotations().DestructiveHint {
		t.Error("the annotations drive the approval risk level and must survive")
	}
	if d.InputType() == nil || d.InputType().Kind().String() != "struct" {
		t.Errorf("input type = %v", d.InputType())
	}
}

func TestSurfaceNames(t *testing.T) {
	for surface, want := range map[command.Surface]string{
		command.SurfaceCLI:   "cli",
		command.SurfaceHTTP:  "http",
		command.SurfaceMCP:   "mcp",
		command.SurfaceAgent: "agent",
	} {
		if got := surface.String(); got != want {
			t.Errorf("%d = %q, want %q", surface, got, want)
		}
	}
}

func TestGroupOfAndAliasesAreReadable(t *testing.T) {
	reg := newRegistry(t)
	reg.DescribeGroup(command.GroupDoc{Name: "memories", Tool: "Memory"})
	reg.MustAlias(command.Alias{From: "memories_create", To: "memories_store", Deprecated: true})

	if doc, ok := reg.GroupOf("memories"); !ok || doc.Tool != "Memory" {
		t.Fatalf("group = %+v %v", doc, ok)
	}
	if _, ok := reg.GroupOf("nope"); ok {
		t.Error("an undescribed group must not be reported as described")
	}
	if aliases := reg.Aliases(); len(aliases) != 1 || aliases["memories_create"].To != "memories_store" {
		t.Fatalf("aliases = %v", aliases)
	}
	if reg.Len() != 1 {
		t.Errorf("len = %d", reg.Len())
	}
}

// TestANonDeprecatedAliasCarriesNoNotice: an alias can exist as a convenience
// without telling the model to stop using it.
func TestANonDeprecatedAliasCarriesNoNotice(t *testing.T) {
	reg := newRegistry(t)
	reg.MustAlias(command.Alias{From: "memories_put", To: "memories_store"})
	_, notice, ok := reg.Lookup("memories_put")
	if !ok {
		t.Fatal("the alias does not resolve")
	}
	if notice != nil {
		t.Errorf("notice = %+v", notice)
	}
}

func TestLookupOfAnUnknownKeyFails(t *testing.T) {
	reg := newRegistry(t)
	if _, _, ok := reg.Lookup("memories_obliterate"); ok {
		t.Fatal("an unknown key must not resolve")
	}
	// An alias pointing at nothing must not resolve either.
	reg.MustAlias(command.Alias{From: "memories_ghost", To: "memories_missing"})
	if _, _, ok := reg.Lookup("memories_ghost"); ok {
		t.Fatal("an alias to a missing command must not resolve")
	}
}

func TestRegisterRejectsAnIncompleteCommand(t *testing.T) {
	reg := command.NewRegistry()
	if err := command.Register(reg, command.Command[sampleInput, sampleOutput]{
		Name: "store", Handler: handler,
	}); err == nil {
		t.Error("a command without a group must not register")
	}
	if err := command.Register(reg, command.Command[sampleInput, sampleOutput]{
		Group: "memories", Name: "store",
	}); err == nil {
		t.Error("a command without a handler must not register")
	}
}

func TestMustRegisterPanicsOnAProgrammingError(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a malformed command must fail at boot, not at first use")
		}
	}()
	command.MustRegister(command.NewRegistry(), command.Command[sampleInput, sampleOutput]{
		Group: "memories", Name: "store",
	})
}

func TestMustAliasPanicsOnAShadow(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an alias over a real command must fail at boot")
		}
	}()
	reg := newRegistry(t)
	reg.MustAlias(command.Alias{From: "memories_store", To: "memories_store"})
}

func TestInvokeWithAnEmptyPayload(t *testing.T) {
	reg := command.NewRegistry()
	type empty struct{ command.Reasoning }
	if err := command.Register(reg, command.Command[empty, sampleOutput]{
		Group: "memories", Name: "graph",
		Handler: func(context.Context, empty) (sampleOutput, error) {
			return sampleOutput{Echo: "graph"}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	d, _, _ := reg.Lookup("memories_graph")

	// A command whose input is only the reasoning still runs from the CLI,
	// where the payload is an empty object.
	out, err := d.Invoke(context.Background(), command.SurfaceCLI, nil)
	if err != nil {
		t.Fatalf("nil payload: %v", err)
	}
	if out.(sampleOutput).Echo != "graph" {
		t.Errorf("out = %+v", out)
	}
	if _, err := d.Invoke(context.Background(), command.SurfaceCLI, []byte("null")); err != nil {
		t.Fatalf("null payload: %v", err)
	}
}

// A json.RawMessage is "whatever JSON the caller sends, kept verbatim". The
// inference library sees the underlying []byte and publishes an array of
// integers 0-255 — so `toolsets_call.input` told every model that the
// arguments of an external tool were a byte array, and no model following the
// schema could call one. The daemon accepted objects regardless, which is why
// nothing caught it: the contract was wrong only where a model could read it.
func TestARawMessageFieldIsPublishedAsAnythingRatherThanBytes(t *testing.T) {
	type In struct {
		Input json.RawMessage `json:"input,omitempty" jsonschema:"Arguments for the tool."`
		command.Reasoning
	}

	reg := command.NewRegistry()
	if err := command.Register(reg, command.Command[In, struct{}]{
		Group: "probe", Name: "call", Summary: "Probe.",
		Handler: func(context.Context, In) (struct{}, error) { return struct{}{}, nil },
	}); err != nil {
		t.Fatal(err)
	}
	d, _, ok := reg.Lookup("probe_call")
	if !ok {
		t.Fatal("the command was not registered")
	}

	field := d.InputSchema().Properties["input"]
	if field == nil {
		t.Fatal("the schema does not mention input at all")
	}
	if field.Type == "array" || field.Items != nil {
		t.Errorf("input is published as %q with items %+v, want an open schema", field.Type, field.Items)
	}
	if field.Description == "" {
		t.Error("the description was dropped, which is the model's only guidance about the shape")
	}
}
