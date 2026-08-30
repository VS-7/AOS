package command_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
)

// collidingInput reproduces toolsets_call: a domain field genuinely named
// "tool", which is also the key the validation error used for its own
// metadata.
type collidingInput struct {
	Toolset string `json:"toolset" jsonschema:"Which toolset to reach." validate:"required,notblank"`
	Tool    string `json:"tool" jsonschema:"Name of the tool inside it." validate:"required,notblank"`

	command.Reasoning
}

// enumInput carries a closed set of values, declared once in the validate tag.
type enumInput struct {
	Category string `json:"category" jsonschema:"The function of the knowledge." validate:"required,oneof=decision learning fact"`
	Status   string `json:"status,omitempty" jsonschema:"Where it sits." validate:"omitempty,oneof=pending finished"`

	command.Reasoning
}

func registryWith[In any](t *testing.T, group, name string) *command.Registry {
	t.Helper()
	reg := command.NewRegistry()
	err := command.Register(reg, command.Command[In, sampleOutput]{
		Group: group, Name: name, Summary: "…",
		Handler: func(context.Context, In) (sampleOutput, error) { return sampleOutput{}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// TestSchemaTrueAnswersBeforeValidation is the whole point of the CTA every
// validation error carries: a caller that does not know the contract cannot be
// asked to satisfy it before being allowed to read it.
func TestSchemaTrueAnswersBeforeValidation(t *testing.T) {
	reg := newRegistry(t)
	d, _, _ := reg.Lookup("memories_store")

	for _, surface := range []command.Surface{
		command.SurfaceCLI, command.SurfaceHTTP, command.SurfaceMCP, command.SurfaceAgent,
	} {
		// No title, no _reasoning: exactly the payload the CTA tells a caller
		// to send, and exactly the one that used to fail validation instead.
		out, err := d.Invoke(context.Background(), surface, json.RawMessage(`{"schema":true}`))
		if err != nil {
			t.Fatalf("%s: schema:true must not be validated as a call: %v", surface, err)
		}
		detail, ok := out.(command.Detail)
		if !ok {
			t.Fatalf("%s: out = %T, want command.Detail", surface, out)
		}
		if detail.Tool != "memories_store" {
			t.Errorf("%s: tool = %q", surface, detail.Tool)
		}
		if detail.InputSchema == nil || detail.InputSchema.Properties["title"] == nil {
			t.Errorf("%s: the answer must carry the input schema", surface)
		}
		if detail.Tokens.Total == 0 {
			t.Errorf("%s: the answer must carry its own cost", surface)
		}
	}
}

// TestSchemaFalseStillExecutes: the key is only an escape hatch when it is
// true, so a caller that sends it explicitly off is running the command.
func TestSchemaFalseStillExecutes(t *testing.T) {
	reg := newRegistry(t)
	d, _, _ := reg.Lookup("memories_store")
	out, err := d.Invoke(context.Background(), command.SurfaceCLI,
		json.RawMessage(`{"schema":false,"title":"a memory"}`))
	if err != nil {
		t.Fatalf("schema:false must run the command: %v", err)
	}
	if _, isDetail := out.(command.Detail); isDetail {
		t.Fatal("schema:false answered with the schema")
	}
}

// TestSchemaTrueTravelsThroughRouting: introspection describes the published
// surface, which is the template's — so it must answer even where a workspace
// could not be resolved.
func TestSchemaTrueTravelsThroughRouting(t *testing.T) {
	template := newRegistry(t)
	routed := command.Route(template, func(context.Context) (*command.Registry, error) {
		return nil, apperr.New("TEST_NO_WORKSPACE").Status(apperr.StatusNotFound)
	})
	d, _, ok := routed.Lookup("memories_store")
	if !ok {
		t.Fatal("memories_store is not published by the routed registry")
	}
	out, err := d.Invoke(context.Background(), command.SurfaceHTTP, json.RawMessage(`{"schema":true}`))
	if err != nil {
		t.Fatalf("routing must not stand between a caller and the contract: %v", err)
	}
	if _, isDetail := out.(command.Detail); !isDetail {
		t.Fatalf("out = %T, want command.Detail", out)
	}
}

// TestValidationMetadataDoesNotCollideWithADomainField is defect #6: a command
// whose payload has a field called "tool" used to overwrite the metadata that
// says which command failed, so a generic log parser read the wrong thing for
// that one command out of ~140.
func TestValidationMetadataDoesNotCollideWithADomainField(t *testing.T) {
	reg := registryWith[collidingInput](t, "toolsets", "call")
	d, _, _ := reg.Lookup("toolsets_call")
	_, err := d.Invoke(context.Background(), command.SurfaceHTTP, json.RawMessage(`{}`))
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("error = %v", err)
	}
	if got := e.Issues[command.IssueCommand]; got != "toolsets_call" {
		t.Errorf("%s = %v, want the command that failed", command.IssueCommand, got)
	}
	if got := e.Issues["tool"]; got != "is required" {
		t.Errorf(`issue["tool"] = %v, want the field violation`, got)
	}
	if _, leaked := e.Issues["tool"].(string); leaked && e.Issues["tool"] == "toolsets_call" {
		t.Error("the metadata overwrote the field violation again")
	}
}

// TestClosedSetsArePublishedAsEnums: without this the only way to learn the
// accepted values is to send a wrong one and read the refusal.
func TestClosedSetsArePublishedAsEnums(t *testing.T) {
	reg := registryWith[enumInput](t, "memories", "store")
	d, _, _ := reg.Lookup("memories_store")
	schema := d.InputSchema()

	want := map[string][]string{
		"category": {"decision", "learning", "fact"},
		"status":   {"pending", "finished"},
	}
	for field, values := range want {
		prop := schema.Properties[field]
		if prop == nil {
			t.Fatalf("%s is not in the schema", field)
		}
		if len(prop.Enum) != len(values) {
			t.Fatalf("%s: enum = %v, want %v", field, prop.Enum, values)
		}
		for i, v := range values {
			if prop.Enum[i] != v {
				t.Errorf("%s: enum[%d] = %v, want %q", field, i, prop.Enum[i], v)
			}
		}
	}
}
