package command_test

import (
	"context"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/OWNER/aos/internal/core/command"
)

// enumeratedInput carries a type that declares its own closed set, which is
// how a domain publishes one without repeating it in a tag.
type enumeratedInput struct {
	Category sampleCategory `json:"category,omitempty" jsonschema:"The function of the knowledge."`

	command.Reasoning
}

type sampleCategory string

func (sampleCategory) EnumValues() []string { return []string{"decision", "learning"} }

// TestManifestDescribesTheWholeSurface pins what a client that is not the CLI
// reads to learn what this installation publishes — the answer `self tools`
// and `self llms` were missing 95% of (defect #1).
func TestManifestDescribesTheWholeSurface(t *testing.T) {
	reg := command.NewRegistry()
	reg.DescribeGroup(command.GroupDoc{
		Name: "memories", Tool: "Memory", Summary: "Durable knowledge.", Doc: "Long form.", Hint: "A hint.",
	})
	for _, name := range []string{"store", "recall"} {
		c := sampleCommand()
		c.Name = name
		if err := command.Register(reg, c); err != nil {
			t.Fatal(err)
		}
	}

	m := command.ManifestOf(reg, "1.2.3")
	if m.Version != "1.2.3" {
		t.Errorf("version = %q", m.Version)
	}
	if len(m.Groups) != 1 {
		t.Fatalf("groups = %d", len(m.Groups))
	}
	g := m.Groups[0]
	if g.Name != "memories" || g.Tool != "Memory" || g.Doc != "Long form." || g.Hint != "A hint." {
		t.Errorf("group = %+v — the documentation a model reads must travel with it", g)
	}

	commands := m.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands = %d", len(commands))
	}
	// Alphabetical within the group, as every other surface publishes them.
	if commands[0].Name != "recall" || commands[1].Name != "store" {
		t.Errorf("order = %q, %q", commands[0].Name, commands[1].Name)
	}
	first := commands[0]
	if first.Key != "memories_recall" || first.Group != "memories" {
		t.Errorf("command = %+v", first)
	}
	if !first.Registry {
		t.Error("whether a command is in the agent's registry is part of the description")
	}
	if first.InputSchema == nil || first.InputSchema.Properties["title"] == nil {
		t.Error("a manifest without the input schema cannot teach the contract")
	}
}

func TestManifestOfAnEmptyRegistryIsEmpty(t *testing.T) {
	m := command.ManifestOf(command.NewRegistry(), "")
	if len(m.Groups) != 0 || len(m.Commands()) != 0 {
		t.Errorf("manifest = %+v", m)
	}
}

// TestSchemaWithoutReasoningDropsOnlyTheOuterField: on the composite shape
// `_reasoning` travels next to `action`, so the per-action schema must not
// demand it a second time — and must keep everything else exactly as it was.
func TestSchemaWithoutReasoningDropsOnlyTheOuterField(t *testing.T) {
	reg := newRegistry(t)
	d, _, _ := reg.Lookup("memories_store")

	trimmed := command.SchemaWithoutReasoning(d)
	if _, present := trimmed.Properties[command.ReasoningField]; present {
		t.Error("_reasoning is still a property of the action schema")
	}
	for _, name := range trimmed.Required {
		if name == command.ReasoningField {
			t.Error("_reasoning is still required by the action schema")
		}
	}
	if trimmed.Properties["title"] == nil {
		t.Error("the rest of the contract was dropped with it")
	}
	// The original is untouched: the flat surface still publishes the field it
	// genuinely takes.
	if _, present := d.InputSchema().Properties[command.ReasoningField]; !present {
		t.Error("trimming the copy mutated the registry's own schema")
	}
}

func TestSchemaWithoutReasoningToleratesNoSchema(t *testing.T) {
	if got := command.SchemaWithoutReasoning(schemaless{}); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// schemaless is a descriptor with no schema at all, which nothing in the tree
// produces — the guard exists so that a future one cannot panic here.
type schemaless struct{ command.Descriptor }

func (schemaless) InputSchema() *jsonschema.Schema { return nil }

// TestATypeDeclaringItsOwnSetPublishesIt covers the other half of defect #8:
// a domain type owns its closed set, and the schema reads it rather than a
// second copy in a tag.
func TestATypeDeclaringItsOwnSetPublishesIt(t *testing.T) {
	reg := registryWith[enumeratedInput](t, "memories", "recall")
	d, _, _ := reg.Lookup("memories_recall")
	prop := d.InputSchema().Properties["category"]
	if prop == nil {
		t.Fatal("category is not in the schema")
	}
	if len(prop.Enum) != 2 || prop.Enum[0] != "decision" || prop.Enum[1] != "learning" {
		t.Errorf("enum = %v, want the values the type declares", prop.Enum)
	}
}

// TestSchemaKeyIsReservedAtRegistration: a command that declared its own
// `schema` field would be the one command nobody could inspect, and it would
// look from the outside like introspection had quietly stopped working.
func TestSchemaKeyIsReservedAtRegistration(t *testing.T) {
	type shadowing struct {
		Schema string `json:"schema" jsonschema:"Something else entirely."`
		command.Reasoning
	}
	err := command.Register(command.NewRegistry(), command.Command[shadowing, sampleOutput]{
		Group: "g", Name: "n",
		Handler: func(context.Context, shadowing) (sampleOutput, error) { return sampleOutput{}, nil },
	})
	if err == nil {
		t.Fatal("a command may not declare a field named schema")
	}
}

// TestAMalformedPayloadIsNotASchemaRequest: the reserved key is read out of a
// payload that parses. One that does not has to reach the decoder, which is
// what says why it is wrong.
func TestAMalformedPayloadIsNotASchemaRequest(t *testing.T) {
	reg := newRegistry(t)
	d, _, _ := reg.Lookup("memories_store")
	for _, payload := range []string{`{"schema":`, `{"schema":"yes"}`, `[]`, ``} {
		out, err := d.Invoke(context.Background(), command.SurfaceCLI, []byte(payload))
		if _, isDetail := out.(command.Detail); isDetail {
			t.Errorf("%q was treated as a request for the contract", payload)
		}
		_ = err
	}
}
