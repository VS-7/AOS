package prompt_test

import (
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/runtime/prompt"
)

// TestTheDialect locks the rules the original's XMLParser defines, because the
// assembled document is only comparable to the original's if the serializer
// agrees with it on every one of them.
func TestTheDialect(t *testing.T) {
	cases := []struct {
		name string
		in   any
		tag  string
		want string
	}{
		{
			name: "a short string is one line",
			in:   "hello", tag: "greeting",
			want: "  <greeting>hello</greeting>",
		},
		{
			name: "a long string gets its own lines",
			in:   strings.Repeat("x", 90), tag: "long",
			want: "  <long>\n    " + strings.Repeat("x", 90) + "\n  </long>",
		},
		{
			name: "an attribute comes from an @ key and text from a hash",
			in:   prompt.Object{prompt.Attr("count", "3"), prompt.Body("decisions")},
			tag:  "category",
			want: `  <category count="3">decisions</category>`,
		},
		{
			name: "an underscore key is bookkeeping and does not reach the model",
			in:   prompt.Object{{Key: "_internal", Value: prompt.Text("secret")}, {Key: "shown", Value: prompt.Text("yes")}},
			tag:  "block",
			want: "  <block>\n    <shown>yes</shown>\n  </block>",
		},
		{
			name: "a list of names repeats its own tag rather than nesting an item level",
			in:   prompt.Strings([]string{"a", "b"}), tag: "names",
			want: "  <names>a</names>\n  <names>b</names>",
		},
		{
			name: "a list of objects nests under the singular of the tag",
			in: prompt.List{
				prompt.Object{{Key: "id", Value: prompt.Text("1")}},
				prompt.Object{{Key: "id", Value: prompt.Text("2")}},
			},
			tag:  "entries",
			want: "  <entries>\n    <entry>\n      <id>1</id>\n    </entry>\n    <entry>\n      <id>2</id>\n    </entry>\n  </entries>",
		},
		{
			name: "a plural in ies becomes a y",
			in:   prompt.List{prompt.Object{{Key: "id", Value: prompt.Text("1")}}},
			tag:  "memories",
			want: "  <memories>\n    <memory>\n      <id>1</id>\n    </memory>\n  </memories>",
		},
		{
			name: "an empty list disappears",
			in:   prompt.List{}, tag: "names",
			want: "",
		},
		{
			name: "an empty object is still an element",
			in:   prompt.Object{prompt.Attr("kind", "data")}, tag: "block",
			want: `  <block kind="data"></block>`,
		},
		{
			name: "an empty string disappears",
			in:   "", tag: "nothing",
			want: "",
		},
		{
			name: "markup in a value is escaped",
			in:   `<b>&"</b>`, tag: "value",
			want: `  <value>&lt;b&gt;&amp;&quot;&lt;/b&gt;</value>`,
		},
		{
			name: "markup in an attribute is escaped too",
			in:   prompt.Object{prompt.Attr("title", `a "quoted" <tag>`), prompt.Body("x")},
			tag:  "block",
			want: `  <block title="a &quot;quoted&quot; &lt;tag&gt;">x</block>`,
		},
		{
			name: "a map is emitted with its keys sorted, so two runs agree",
			in:   map[string]any{"zulu": "z", "alpha": "a"}, tag: "block",
			want: "  <block>\n    <alpha>a</alpha>\n    <zulu>z</zulu>\n  </block>",
		},
		{
			name: "numbers and booleans are values, not errors",
			in:   map[string]any{"count": 3, "enabled": true, "ratio": 0.5},
			tag:  "block",
			want: "  <block>\n    <count>3</count>\n    <enabled>true</enabled>\n    <ratio>0.5</ratio>\n  </block>",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := prompt.Encode(c.in, c.tag, 1)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("\n got: %q\nwant: %q", got, c.want)
			}
		})
	}
}

// TestAValueTheDialectCannotCarryIsAnError, rather than a silent omission that
// leaves a hole in the prompt nobody notices.
func TestAValueTheDialectCannotCarryIsAnError(t *testing.T) {
	if _, err := prompt.Encode(struct{ A int }{1}, "block", 1); err == nil {
		t.Fatal("an arbitrary struct was serialized")
	}
	if _, err := prompt.Encode([]any{struct{}{}}, "block", 1); err == nil {
		t.Fatal("a list of arbitrary structs was serialized")
	}
	if _, err := prompt.Encode(map[string]any{"k": struct{}{}}, "block", 1); err == nil {
		t.Fatal("a map of arbitrary structs was serialized")
	}
}

// TestARootlessValueEmitsItsChildrenBare, which is how the builder joins the
// top-level blocks of the document.
func TestARootlessValueEmitsItsChildrenBare(t *testing.T) {
	got, err := prompt.Encode(prompt.Object{
		{Key: "one", Value: prompt.Text("1")},
		{Key: "two", Value: prompt.Text("2")},
	}, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != "  <one>1</one>\n  <two>2</two>" {
		t.Fatalf("got %q", got)
	}
}
