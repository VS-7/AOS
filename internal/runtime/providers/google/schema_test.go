package google

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/OWNER/aos/internal/runtime/agentloop"
)

// schemaOf renders what geminiParameters would send for a raw JSON Schema.
func schemaOf(t *testing.T, raw string) map[string]any {
	t.Helper()
	var s jsonschema.Schema
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("the fixture does not parse: %v", err)
	}
	return geminiParameters(&s)
}

// The two shapes that made every tool-carrying turn fail with 400 "Invalid
// JSON payload received". Both come straight out of the registry's generated
// schemas, so neither is hypothetical.
func TestDropsFieldsTheSchemaProtoDoesNotHave(t *testing.T) {
	out := schemaOf(t, `{
		"type": "object",
		"additionalProperties": false,
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"properties": {"id": {"type": "string", "additionalProperties": false}},
		"required": ["id"]
	}`)

	if _, present := out["additionalProperties"]; present {
		t.Error("additionalProperties survived; the API rejects the whole request over it")
	}
	if _, present := out["$schema"]; present {
		t.Error("$schema survived")
	}
	properties := out["properties"].(map[string]any)
	id := properties["id"].(map[string]any)
	if _, present := id["additionalProperties"]; present {
		t.Error("additionalProperties survived on a nested property — the sweep has to recurse")
	}
	if id["type"] != "string" {
		t.Errorf("the nested type was lost: %v", id["type"])
	}
}

func TestCollapsesUnionTypeIntoNullable(t *testing.T) {
	out := schemaOf(t, `{
		"type": "object",
		"properties": {"note": {"type": ["string", "null"]}}
	}`)

	note := out["properties"].(map[string]any)["note"].(map[string]any)
	if note["type"] != "string" {
		t.Errorf("type = %v, want the single non-null member; a list is what the proto refuses", note["type"])
	}
	if note["nullable"] != true {
		t.Error("nullable was not set, so the optionality of the field was silently lost")
	}
}

func TestInlinesRefSoAParameterIsNeverEmptied(t *testing.T) {
	out := schemaOf(t, `{
		"type": "object",
		"$defs": {"Point": {"type": "object", "properties": {"x": {"type": "number"}}}},
		"properties": {"at": {"$ref": "#/$defs/Point"}}
	}`)

	at := out["properties"].(map[string]any)["at"].(map[string]any)
	inner, ok := at["properties"].(map[string]any)
	if !ok || inner["x"] == nil {
		t.Fatalf("the $ref was not inlined, so the parameter reached the model shapeless: %#v", at)
	}
}

func TestRequiredNeverNamesADroppedProperty(t *testing.T) {
	// `gone` is not a schema object, so it cannot be rendered; leaving it in
	// `required` would describe a parameter the model cannot see.
	out := schemaOf(t, `{
		"type": "object",
		"properties": {"kept": {"type": "string"}},
		"required": ["kept", "gone"]
	}`)

	required, _ := out["required"].([]any)
	for _, name := range required {
		if name == "gone" {
			t.Error("required still names a property that was not rendered")
		}
	}
	if len(required) != 1 || required[0] != "kept" {
		t.Errorf("required = %v, want [kept]", required)
	}
}

func TestSelfReferentialSchemaTerminates(t *testing.T) {
	// Legal JSON Schema; inlining it without a depth bound would not return.
	out := schemaOf(t, `{
		"type": "object",
		"$defs": {"Node": {"type": "object", "properties": {"next": {"$ref": "#/$defs/Node"}}}},
		"properties": {"root": {"$ref": "#/$defs/Node"}}
	}`)
	if out == nil {
		t.Fatal("a cyclic schema rendered nothing at all")
	}
}

// The signature round-trip. Gemini 3 refuses a follow-up request whose
// functionCall parts lost their thought_signature, which is the second model
// call of every tool-using turn — the one that reads the tool's result. So a
// dropped signature does not degrade anything, it fails the turn.
func TestThoughtSignatureSurvivesTheRoundTrip(t *testing.T) {
	answer := translate(mustParse(t, `{
		"candidates": [{"content": {"parts": [
			{"functionCall": {"name": "memories_recall", "args": {}}, "thoughtSignature": "sig-abc"}
		]}}]
	}`), "gemini-3-flash-preview")

	if len(answer.ToolCalls) != 1 {
		t.Fatalf("parsed %d tool calls, want 1", len(answer.ToolCalls))
	}
	if answer.ToolCalls[0].Signature != "sig-abc" {
		t.Fatalf("signature = %q, want it read off the response", answer.ToolCalls[0].Signature)
	}

	// And it has to go back out on the call it belongs to.
	rendered := contents([]agentloop.Message{answer.Message})
	parts := rendered[0]["parts"].([]map[string]any)
	if parts[0]["thoughtSignature"] != "sig-abc" {
		t.Fatalf("the signature was not sent back: %#v", parts[0])
	}
}

func TestAToolCallWithNoSignatureSendsNoEmptyOne(t *testing.T) {
	answer := translate(mustParse(t, `{
		"candidates": [{"content": {"parts": [
			{"functionCall": {"name": "memories_recall", "args": {}}}
		]}}]
	}`), "gemini-3-flash-preview")

	rendered := contents([]agentloop.Message{answer.Message})
	parts := rendered[0]["parts"].([]map[string]any)
	if _, present := parts[0]["thoughtSignature"]; present {
		t.Error("an empty signature was sent; the field should be absent entirely")
	}
}

func mustParse(t *testing.T, raw string) generated {
	t.Helper()
	var g generated
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		t.Fatalf("the fixture does not parse: %v", err)
	}
	return g
}
