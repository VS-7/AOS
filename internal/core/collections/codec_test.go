package collections_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// probe carries every placeholder the native collections use plus one field of
// each interesting kind, so one type can exercise all thirteen models.
type probe struct {
	ID      string `yaml:"-" json:"id"      collection:"path"`
	Agent   string `yaml:"-" json:"agent"   collection:"path"`
	Skill   string `yaml:"-" json:"skill"   collection:"path"`
	Type    string `yaml:"-" json:"type"    collection:"path"`
	TaskID  string `yaml:"-" json:"taskId"  collection:"path"`
	Task    string `yaml:"-" json:"task"    collection:"path"`
	Routine string `yaml:"-" json:"routine" collection:"path"`

	Title      string    `yaml:"title"          json:"title"`
	Category   string    `yaml:"category"       json:"category"`
	Tags       []string  `yaml:"tags,omitempty" json:"tags,omitempty"`
	Confidence float64   `yaml:"confidence"     json:"confidence"`
	CreatedAt  time.Time `yaml:"createdAt"      json:"createdAt"`

	Content string `yaml:"-" json:"content" collection:"content"`
}

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

func markdownModel() collections.Model[probe] {
	m, err := collections.ModelOf[probe]("memories")
	if err != nil {
		panic(err)
	}
	return m
}

func jsonModel() collections.Model[probe] {
	m, err := collections.ModelOf[probe]("todos")
	if err != nil {
		panic(err)
	}
	return m
}

func TestMarkdownRoundTrip(t *testing.T) {
	m := markdownModel()
	key := collections.Key{"agent": "luara", "id": "a1b2"}
	want := &probe{
		ID: "luara-ignored-on-encode", Agent: "ignored",
		Title:      "Preferência de commit em inglês",
		Category:   "preference",
		Tags:       []string{"git", "estilo"},
		Confidence: 0.9,
		CreatedAt:  refTime,
		Content:    "O usuário escreve commits em inglês, imperativo.\n",
	}

	data, err := collections.Encode(want, m)
	if err != nil {
		t.Fatal(err)
	}

	// The path fields must not appear in the front matter: the path is their
	// only home, exactly as the original omits them from the schema.
	text := string(data)
	if strings.Contains(text, "ignored") {
		t.Fatalf("a path field leaked into the front matter:\n%s", text)
	}
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("missing front matter:\n%s", text)
	}

	got, err := collections.Decode(data, key, m)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "a1b2" || got.Agent != "luara" {
		t.Errorf("path fields = %q/%q, want a1b2/luara", got.ID, got.Agent)
	}
	if got.Title != want.Title || got.Category != want.Category || got.Confidence != want.Confidence {
		t.Errorf("front matter lost data: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "git" {
		t.Errorf("tags = %v", got.Tags)
	}
	if !got.CreatedAt.Equal(refTime) {
		t.Errorf("createdAt = %v", got.CreatedAt)
	}
	if got.Content != want.Content {
		t.Errorf("body = %q, want %q", got.Content, want.Content)
	}
}

// TestBodyIsPreservedVerbatim covers the cases a naive split gets wrong.
func TestBodyIsPreservedVerbatim(t *testing.T) {
	m := markdownModel()
	key := collections.Key{"agent": "a", "id": "b"}
	bodies := []string{
		"    indented first line\nsecond\n",
		"acentuação, 日本語, emoji 🧠\n",
		"---\nnot front matter, just a rule\n",
		"line\n\n\nthree newlines above\n",
		"no trailing newline",
	}
	for _, body := range bodies {
		data, err := collections.Encode(&probe{Title: "t", Content: body}, m)
		if err != nil {
			t.Fatalf("%q: %v", body, err)
		}
		got, err := collections.Decode(data, key, m)
		if err != nil {
			t.Fatalf("%q: %v", body, err)
		}
		want := body
		if !strings.HasSuffix(want, "\n") {
			want += "\n" // the encoder terminates the file
		}
		if got.Content != want {
			t.Errorf("body round trip: got %q, want %q", got.Content, want)
		}
	}
}

func TestDecodeAcceptsHandWrittenFiles(t *testing.T) {
	m := markdownModel()
	key := collections.Key{"agent": "a", "id": "b"}

	t.Run("no front matter at all", func(t *testing.T) {
		got, err := collections.Decode([]byte("just a note\n"), key, m)
		if err != nil {
			t.Fatal(err)
		}
		if got.Content != "just a note\n" {
			t.Errorf("content = %q", got.Content)
		}
		if got.ID != "b" {
			t.Errorf("path fields must still be filled: %q", got.ID)
		}
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		raw := "---\r\ntitle: Windows\r\n---\r\n\r\nbody\r\n"
		got, err := collections.Decode([]byte(raw), key, m)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "Windows" || got.Content != "body\n" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("UTF-8 BOM", func(t *testing.T) {
		raw := append([]byte{0xEF, 0xBB, 0xBF}, []byte("---\ntitle: BOM\n---\n\nbody\n")...)
		got, err := collections.Decode(raw, key, m)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "BOM" {
			t.Errorf("title = %q", got.Title)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		if _, err := collections.Decode(nil, key, m); err != nil {
			t.Fatalf("an empty record is empty, not broken: %v", err)
		}
	})
}

func TestDecodeRejectsBrokenFrontMatter(t *testing.T) {
	_, err := collections.Decode([]byte("---\ntitle: [unclosed\n---\n\nbody\n"),
		collections.Key{"agent": "a", "id": "b"}, markdownModel())
	if err == nil {
		t.Fatal("invalid YAML must be reported, not ignored")
	}
	if !strings.Contains(err.Error(), "COLLECTION_DECODE_FAILED") {
		t.Fatalf("error = %v", err)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	m := jsonModel()
	key := collections.Key{"taskId": "t-1", "id": "td-9"}
	want := &probe{TaskID: "leaked", ID: "leaked", Title: "write the test", Confidence: 1}

	data, err := collections.Encode(want, m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "leaked") {
		t.Fatalf("path fields leaked into the JSON body:\n%s", data)
	}
	got, err := collections.Decode(data, key, m)
	if err != nil {
		t.Fatal(err)
	}
	if got.TaskID != "t-1" || got.ID != "td-9" || got.Title != "write the test" {
		t.Fatalf("got %+v", got)
	}
}

func TestEncodeDoesNotMutateItsInput(t *testing.T) {
	m := markdownModel()
	v := &probe{ID: "keep", Agent: "keep", Content: "body"}
	if _, err := collections.Encode(v, m); err != nil {
		t.Fatal(err)
	}
	if v.ID != "keep" || v.Agent != "keep" || v.Content != "body" {
		t.Fatalf("Encode cleared fields on the caller's value: %+v", v)
	}
}

func TestEncodeIsByteStable(t *testing.T) {
	m := markdownModel()
	v := &probe{Title: "t", Tags: []string{"a", "b"}, CreatedAt: refTime, Content: "body\n"}
	first, err := collections.Encode(v, m)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, err := collections.Encode(v, m)
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != string(first) {
			t.Fatalf("encoding is not stable:\n%s\n---\n%s", first, again)
		}
	}
}

func TestKeyOfReadsThePathFieldsBack(t *testing.T) {
	k := collections.KeyOf(&probe{ID: "a", Agent: "luara"})
	if k["id"] != "a" || k["agent"] != "luara" {
		t.Fatalf("key = %s", k)
	}
}

func TestWithoutBodyKeepsEverythingElse(t *testing.T) {
	v := &probe{Title: "t", Content: "a long body"}
	stripped := collections.WithoutBody(v)
	if stripped.Content != "" {
		t.Error("the body should be gone")
	}
	if stripped.Title != "t" {
		t.Error("everything else should stay")
	}
	if v.Content != "a long body" {
		t.Error("the caller's value must not be touched")
	}
}
