package format_test

import (
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/transport/clix/format"
)

func render(t *testing.T, v any, opts format.Options) string {
	t.Helper()
	got, err := format.Render(v, opts)
	if err != nil {
		t.Fatal(err)
	}
	return got.Text
}

type gatewayStatus struct {
	Status    string `json:"status"`
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	StartedAt string `json:"startedAt"`
	Version   string `json:"version"`
}

// TestTOONMatchesTheOriginalFlatObject compares against output captured from
// the original binary installed on this machine:
//
//	$ fractal gateway status --format toon
//	status: running
//	pid: 45655
//	port: 5326
//	startedAt: "2026-08-14T22:02:56.980Z"
//	version: 0.1.400
func TestTOONMatchesTheOriginalFlatObject(t *testing.T) {
	got := render(t, gatewayStatus{
		Status: "running", PID: 45655, Port: 5326,
		StartedAt: "2026-08-14T22:02:56.980Z", Version: "0.1.400",
	}, format.Options{Format: format.TOON})

	want := strings.Join([]string{
		"status: running",
		"pid: 45655",
		"port: 5326",
		`startedAt: "2026-08-14T22:02:56.980Z"`,
		"version: 0.1.400",
	}, "\n")

	if got != want {
		t.Fatalf("TOON output differs.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

type suggestion struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
}

type notFound struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	CTA     struct {
		Description string       `json:"description"`
		Commands    []suggestion `json:"commands"`
	} `json:"cta"`
}

// TestTOONMatchesTheOriginalTabularArray compares against:
//
//	$ fractal themes list --format toon
//	code: COMMAND_NOT_FOUND
//	message: 'themes' is not a command for 'fractal'.
//	cta:
//	  description: "Suggested command:"
//	  commands[1]{command,description}:
//	    fractal --help,see all available commands
//
// The tabular form is where TOON earns its name: the field names are written
// once instead of once per row.
func TestTOONMatchesTheOriginalTabularArray(t *testing.T) {
	var v notFound
	v.Code = "COMMAND_NOT_FOUND"
	v.Message = "'themes' is not a command for 'fractal'."
	v.CTA.Description = "Suggested command:"
	v.CTA.Commands = []suggestion{{Command: "fractal --help", Description: "see all available commands"}}

	want := strings.Join([]string{
		"code: COMMAND_NOT_FOUND",
		"message: 'themes' is not a command for 'fractal'.",
		"cta:",
		`  description: "Suggested command:"`,
		"  commands[1]{command,description}:",
		"    fractal --help,see all available commands",
	}, "\n")

	got := render(t, v, format.Options{Format: format.TOON})
	if got != want {
		t.Fatalf("TOON output differs.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestTOONFallsBackToAListWhenTheArrayIsNotUniform compares against:
//
//	cta:
//	  description: "Suggested commands:"
//	  commands[2]:
//	    - command: fractal config --format toon
//	    - command: fractal --help
//	      description: see all available commands
func TestTOONFallsBackToAListWhenTheArrayIsNotUniform(t *testing.T) {
	var v notFound
	v.Code = "COMMAND_NOT_FOUND"
	v.Message = "'config get' is not a command for 'fractal'. Did you mean 'config'?"
	v.CTA.Description = "Suggested commands:"
	v.CTA.Commands = []suggestion{
		{Command: "fractal config --format toon"},
		{Command: "fractal --help", Description: "see all available commands"},
	}

	want := strings.Join([]string{
		"code: COMMAND_NOT_FOUND",
		"message: 'config get' is not a command for 'fractal'. Did you mean 'config'?",
		"cta:",
		`  description: "Suggested commands:"`,
		"  commands[2]:",
		"    - command: fractal config --format toon",
		"    - command: fractal --help",
		"      description: see all available commands",
	}, "\n")

	got := render(t, v, format.Options{Format: format.TOON})
	if got != want {
		t.Fatalf("TOON output differs.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestFieldOrderIsTheDeclarationOrder: the original prints a gateway status as
// status, pid, port… — the struct order, not the alphabetical order a Go map
// would give.
func TestFieldOrderIsTheDeclarationOrder(t *testing.T) {
	got := render(t, gatewayStatus{Status: "running", PID: 1, Port: 2, Version: "3"},
		format.Options{Format: format.TOON})
	lines := strings.Split(got, "\n")
	for i, want := range []string{"status", "pid", "port", "startedAt", "version"} {
		if !strings.HasPrefix(lines[i], want+":") {
			t.Fatalf("line %d is %q, want the field %q", i, lines[i], want)
		}
	}
}

func TestQuotingRules(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"running", "running"},
		{"Suggested command:", `"Suggested command:"`},             // a colon would split the line
		{"2026-08-14T22:02:56.980Z", `"2026-08-14T22:02:56.980Z"`}, // colons again
		{"0.1.400", "0.1.400"},                                     // two dots: not a number, no ambiguity
		{"5326", `"5326"`},                                         // this one would read back as a number
		{"true", `"true"`},                                         // not a boolean
		{"", `""`},                                                 // empty is not absent
		{" padded ", `" padded "`},
		{"'themes' is not a command.", "'themes' is not a command."},
	}
	for _, c := range cases {
		got := render(t, map[string]string{"v": c.value}, format.Options{Format: format.TOON})
		want := "v: " + c.want
		if got != want {
			t.Errorf("%q rendered as %q, want %q", c.value, got, want)
		}
	}
}

func TestEveryFormatRenders(t *testing.T) {
	v := []suggestion{
		{Command: "aos agents list", Description: "list the agents"},
		{Command: "aos agents get", Description: "read one agent"},
	}
	for _, f := range format.All {
		got := render(t, v, format.Options{Format: f})
		if got == "" {
			t.Errorf("%s produced nothing", f)
		}
		switch f {
		case format.JSON:
			if !strings.HasPrefix(got, "[") {
				t.Errorf("json = %q", got)
			}
		case format.JSONL:
			if lines := strings.Split(got, "\n"); len(lines) != 2 {
				t.Errorf("jsonl has %d lines, want one per record", len(lines))
			}
		case format.MD:
			if !strings.HasPrefix(got, "| command") {
				t.Errorf("md = %q", got)
			}
		case format.TOON:
			if !strings.HasPrefix(got, "[2]{command,description}:") {
				t.Errorf("toon = %q", got)
			}
		}
	}
}

func TestParseRejectsAnUnknownFormat(t *testing.T) {
	if _, err := format.Parse("xml"); err == nil {
		t.Fatal("expected an error")
	}
	for _, f := range format.All {
		if got, err := format.Parse(string(f)); err != nil || got != f {
			t.Errorf("%s: %v %v", f, got, err)
		}
	}
}

type memory struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

func TestFilterSelectsPaths(t *testing.T) {
	v := struct {
		Total    int      `json:"total"`
		Memories []memory `json:"memories"`
	}{
		Total: 3,
		Memories: []memory{
			{ID: "a", Title: "first", Category: "decision", Tags: []string{"x"}},
			{ID: "b", Title: "second", Category: "fact", Tags: []string{"y"}},
			{ID: "c", Title: "third", Category: "fact", Tags: []string{"z"}},
		},
	}

	t.Run("single scalar key returns the scalar", func(t *testing.T) {
		got := render(t, v, format.Options{Format: format.TOON, Filter: "total"})
		if got != "3" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("several keys keep the structure", func(t *testing.T) {
		got := render(t, v.Memories[0], format.Options{Format: format.TOON, Filter: "id,title"})
		want := "id: a\ntitle: first"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("slice cuts an array", func(t *testing.T) {
		got := render(t, v, format.Options{Format: format.JSON, Filter: "memories[0,2]"})
		if strings.Contains(got, `"c"`) {
			t.Fatalf("the slice did not cut: %s", got)
		}
		if !strings.Contains(got, `"a"`) || !strings.Contains(got, `"b"`) {
			t.Fatalf("the slice cut too much: %s", got)
		}
	})

	t.Run("a typo does not hide the answer", func(t *testing.T) {
		got := render(t, v.Memories[0], format.Options{Format: format.TOON, Filter: "["})
		if !strings.Contains(got, "title: first") {
			t.Fatalf("a malformed filter should render everything, got %q", got)
		}
	})
}
