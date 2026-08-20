package collections_test

import (
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/collections"
)

// A dynamic collection has no Go struct: its fields were declared by an agent
// at runtime and live in a map. The engine still has to round-trip it, because
// every guarantee around a record — atomic write, the per-file lock, the CAS
// check — is the same one the native collections get.
func TestARecordRoundTripsThroughMarkdown(t *testing.T) {
	model := collections.Model[collections.Record]{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
		Format:   collections.FormatMarkdown,
	}
	in := &collections.Record{
		Key:     collections.Key{"id": "ada"},
		Fields:  map[string]any{"name": "Ada Lovelace", "stage": "won", "score": 42},
		Content: "Conheceu o Babbage numa festa.",
	}

	raw, err := collections.Encode(in, model)
	if err != nil {
		t.Fatal(err)
	}
	out, err := collections.Decode(raw, collections.Key{"id": "ada"}, model)
	if err != nil {
		t.Fatal(err)
	}

	if out.Key["id"] != "ada" {
		t.Fatalf("key = %v, want id=ada", out.Key)
	}
	if out.Fields["name"] != "Ada Lovelace" || out.Fields["stage"] != "won" {
		t.Fatalf("fields = %v", out.Fields)
	}
	if out.Content != "Conheceu o Babbage numa festa." {
		t.Fatalf("content = %q", out.Content)
	}
}

// The key lives in the path and nowhere else — the same rule Encode applies to
// a native record's `collection:"path"` fields. A key duplicated into the front
// matter is a second source of truth that drifts the first time a file moves.
func TestARecordsKeyIsNotWrittenIntoTheFrontMatter(t *testing.T) {
	model := collections.Model[collections.Record]{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
		Format:   collections.FormatMarkdown,
	}
	raw, err := collections.Encode(&collections.Record{
		Key:    collections.Key{"id": "ada"},
		Fields: map[string]any{"name": "Ada"},
	}, model)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); strings.Contains(got, "id: ada") {
		t.Fatalf("the key was duplicated into the front matter:\n%s", got)
	}
}

func TestARecordRoundTripsThroughJSON(t *testing.T) {
	model := collections.Model[collections.Record]{
		Name:     "deals",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/deals/records/{id}.json")},
		Format:   collections.FormatJSON,
	}
	in := &collections.Record{
		Key:    collections.Key{"id": "d1"},
		Fields: map[string]any{"title": "Contrato anual", "amount": 1200.5, "open": true},
	}

	raw, err := collections.Encode(in, model)
	if err != nil {
		t.Fatal(err)
	}
	out, err := collections.Decode(raw, collections.Key{"id": "d1"}, model)
	if err != nil {
		t.Fatal(err)
	}
	if out.Fields["title"] != "Contrato anual" || out.Fields["open"] != true {
		t.Fatalf("fields = %v", out.Fields)
	}
}

// KeyOf and FieldOf are what List, Query.Filters and the write path call. A
// Record that answered neither would be a record the engine cannot index.
func TestKeyOfAndFieldOfReadARecordsMap(t *testing.T) {
	r := &collections.Record{
		Key:    collections.Key{"id": "ada", "collection": "contacts"},
		Fields: map[string]any{"stage": "won"},
	}

	k := collections.KeyOf(r)
	if k["id"] != "ada" || k["collection"] != "contacts" {
		t.Fatalf("KeyOf = %v", k)
	}
	if v, ok := collections.FieldOf(r, "stage"); !ok || v != "won" {
		t.Fatalf("FieldOf(stage) = %v, %v", v, ok)
	}
	if _, ok := collections.FieldOf(r, "nope"); ok {
		t.Fatal("FieldOf reported a field that does not exist")
	}
}

// WithoutBody is what List returns when IncludeContent is off: the answer to
// "what is here", not "what does it say".
func TestWithoutBodyDropsARecordsContent(t *testing.T) {
	r := &collections.Record{
		Key:     collections.Key{"id": "ada"},
		Fields:  map[string]any{"name": "Ada"},
		Content: "um corpo longo",
	}
	light := collections.WithoutBody(r)
	if light.Content != "" {
		t.Fatalf("content = %q, want empty", light.Content)
	}
	if r.Content == "" {
		t.Fatal("WithoutBody mutated the original")
	}
	if light.Fields["name"] != "Ada" {
		t.Fatalf("WithoutBody dropped the fields too: %v", light.Fields)
	}
}
