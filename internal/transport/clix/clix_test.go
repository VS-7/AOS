package clix_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/transport/clix"
)

type rule struct {
	Type        string `json:"type" jsonschema:"When the rule applies."`
	Instruction string `json:"instruction" jsonschema:"What to do."`
}

type demoInput struct {
	ID         string   `json:"id" cli:"arg" jsonschema:"The record to act on." validate:"required"`
	Title      string   `json:"title,omitempty" jsonschema:"A short title."`
	Confidence float64  `json:"confidence,omitempty" jsonschema:"How sure the caller is."`
	Count      int      `json:"count,omitempty" jsonschema:"How many."`
	Force      bool     `json:"force,omitempty" jsonschema:"Skip the confirmation."`
	Scopes     []string `json:"scopes,omitempty" jsonschema:"Globs where this applies."`
	Rules      []rule   `json:"rules,omitempty" jsonschema:"Structured rules; on a terminal, JSON."`

	command.Reasoning
}

type demoOutput struct {
	Received demoInput `json:"received"`
}

// registry builds a registry with one command that records what it received.
func registry(t *testing.T, opts ...func(*command.Command[demoInput, demoOutput])) (*command.Registry, *demoInput) {
	t.Helper()
	var seen demoInput
	c := command.Command[demoInput, demoOutput]{
		Group:    "records",
		Name:     "touch",
		Summary:  "Touch a record.",
		Doc:      "Touch a record so that something observable happens.",
		Registry: true,
		Handler: func(_ context.Context, in demoInput) (demoOutput, error) {
			seen = in
			return demoOutput{Received: in}, nil
		},
	}
	for _, o := range opts {
		o(&c)
	}
	reg := command.NewRegistry()
	if err := command.Register(reg, c); err != nil {
		t.Fatal(err)
	}
	reg.DescribeGroup(command.GroupDoc{Name: "records", Summary: "Records."})
	return reg, &seen
}

func run(t *testing.T, reg *command.Registry, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	root := clix.NewRoot(clix.Config{
		Registry: reg, Out: &out, Err: &errOut,
		IsTTY: func() bool { return false },
	})
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), errOut.String(), err
}

func TestEveryCommandOfTheRegistryBecomesACommand(t *testing.T) {
	reg, _ := registry(t)
	out, _, err := run(t, reg, "records", "touch", "r-1", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"id": "r-1"`) {
		t.Fatalf("output = %s", out)
	}
}

// TestPositionalArgumentAndFlagsReachTheHandler covers the mapping the whole
// terminal surface rests on: json tags become flags, `cli:"arg"` becomes a
// positional.
func TestPositionalArgumentAndFlagsReachTheHandler(t *testing.T) {
	reg, seen := registry(t)
	_, _, err := run(t, reg, "records", "touch", "r-1",
		"--title", "a title",
		"--confidence", "0.9",
		"--count", "3",
		"--force",
		"--scopes", "src/**,docs/**",
		"--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if seen.ID != "r-1" || seen.Title != "a title" {
		t.Errorf("received %+v", seen)
	}
	if seen.Confidence != 0.9 || seen.Count != 3 || !seen.Force {
		t.Errorf("typed flags did not arrive: %+v", seen)
	}
	if len(seen.Scopes) != 2 || seen.Scopes[0] != "src/**" {
		t.Errorf("scopes = %v", seen.Scopes)
	}
}

// TestComplexTypesTravelAsJSON: a []Rule does not exist on a command line. The
// original solves it the same way — the field takes a JSON string.
func TestComplexTypesTravelAsJSON(t *testing.T) {
	reg, seen := registry(t)
	_, _, err := run(t, reg, "records", "touch", "r-1",
		"--rules", `[{"type":"always","instruction":"review the diff"}]`,
		"--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if len(seen.Rules) != 1 || seen.Rules[0].Instruction != "review the diff" {
		t.Fatalf("rules = %+v", seen.Rules)
	}
}

// TestTheSamePayloadProducesTheSameInputThroughBothPaths is the round trip the
// examples in --help depend on: the line rendered for a payload must parse back
// into that payload.
func TestTheSamePayloadProducesTheSameInputThroughBothPaths(t *testing.T) {
	reg, seen := registry(t)
	d, _, _ := reg.Lookup("records_touch")

	payload := demoInput{
		ID: "r-1", Title: "a title", Confidence: 0.5, Count: 2, Force: true,
		Scopes: []string{"a", "b"},
		Rules:  []rule{{Type: "always", Instruction: "do it"}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	argv, err := clix.CommandLineFor(d, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, reg, append(argv, "--format", "json")...); err != nil {
		t.Fatalf("argv %v: %v", argv, err)
	}

	viaCLI, _ := json.Marshal(*seen)
	viaAPI, _ := json.Marshal(payload)
	if string(viaCLI) != string(viaAPI) {
		t.Fatalf("the rendered line does not parse back into the payload.\ncli: %s\napi: %s", viaCLI, viaAPI)
	}
}

// TestReasoningIsNotAFlag: the reasoning of a human is the command they typed.
func TestReasoningIsNotAFlag(t *testing.T) {
	reg, _ := registry(t)
	_, _, err := run(t, reg, "records", "touch", "r-1", "--_reasoning", "because")
	if err == nil {
		t.Fatal("_reasoning must not be a flag on the terminal surface")
	}
	// And a call without it still works.
	if _, _, err := run(t, reg, "records", "touch", "r-1", "--format", "json"); err != nil {
		t.Fatalf("the CLI must not require _reasoning: %v", err)
	}
}

func TestTransportFlagsArePresentExceptOnLocalCommands(t *testing.T) {
	remote, _ := registry(t)
	out, _, err := run(t, remote, "records", "touch", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--base-url", "--workspace", "--agent"} {
		if !strings.Contains(out, flag) {
			t.Errorf("a remote command must offer %s", flag)
		}
	}
	if !hasFlag(out, "token") {
		t.Error("a remote command must offer --token")
	}

	local, _ := registry(t, func(c *command.Command[demoInput, demoOutput]) { c.Local = true })
	out, _, err = run(t, local, "records", "touch", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "--base-url") {
		t.Error("a local command must not offer --base-url: it never talks to an API")
	}
	if hasFlag(out, "token") {
		t.Error("a local command must not offer --token: it never talks to an API")
	}
	// The token-budget flags are global and stay: they are about output size,
	// not about reaching a remote API.
	if !strings.Contains(out, "--token-limit") {
		t.Error("the global paging flags must remain on a local command")
	}
}

// hasFlag looks for an exact flag rather than a prefix, so that --token-limit
// does not read as --token.
func hasFlag(help, name string) bool {
	for _, line := range strings.Split(help, "\n") {
		for _, field := range strings.Fields(line) {
			if strings.TrimSuffix(field, ",") == "--"+name {
				return true
			}
		}
	}
	return false
}

func TestSchemaFlagPrintsTheContractWithoutRunning(t *testing.T) {
	reg, seen := registry(t)
	out, _, err := run(t, reg, "records", "touch", "--schema")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(out), &schema); err != nil {
		t.Fatalf("--schema did not print JSON: %s", out)
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["title"]; !ok {
		t.Errorf("schema is missing a field: %v", props)
	}
	if _, ok := props[command.ReasoningField]; !ok {
		t.Errorf("the published schema must mention %s", command.ReasoningField)
	}
	if seen.ID != "" {
		t.Error("--schema must not execute the command")
	}
}

func TestTokenPaginationCutsOnALineBoundary(t *testing.T) {
	reg, _ := registry(t)
	full, _, err := run(t, reg, "records", "touch", "r-1", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	page, _, err := run(t, reg, "records", "touch", "r-1", "--format", "json", "--token-limit", "5")
	if err != nil {
		t.Fatal(err)
	}
	if len(page) >= len(full) {
		t.Fatalf("the page is not shorter:\n%s", page)
	}
	for _, line := range strings.Split(strings.TrimRight(page, "\n"), "\n") {
		if !strings.Contains(full, line) {
			t.Fatalf("a line was cut in the middle: %q", line)
		}
	}
}

func TestTokenCountReportsTheSizeInsteadOfTheResult(t *testing.T) {
	reg, _ := registry(t)
	out, _, err := run(t, reg, "records", "touch", "r-1", "--format", "json", "--token-count")
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("--token-count did not print a number: %q", out)
	}
	if n <= 0 {
		t.Fatalf("token count = %d", n)
	}
}

func TestFilterOutputSelectsPaths(t *testing.T) {
	reg, _ := registry(t)
	out, _, err := run(t, reg, "records", "touch", "r-1", "--title", "kept",
		"--format", "json", "--filter-output", "data")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "kept") {
		t.Fatalf("output = %s", out)
	}
}

// TestDefaultFormatFollowsTheConsumer: without a terminal the consumer is a
// program, and JSON is more predictable to parse than TOON.
func TestDefaultFormatFollowsTheConsumer(t *testing.T) {
	reg, _ := registry(t)

	var human bytes.Buffer
	root := clix.NewRoot(clix.Config{
		Registry: reg, Out: &human, Err: &human,
		IsTTY: func() bool { return true },
	})
	root.SetArgs([]string{"records", "touch", "r-1"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(strings.TrimSpace(human.String()), "{") {
		t.Errorf("a human should get TOON: %s", human.String())
	}

	program, _, err := run(t, reg, "records", "touch", "r-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(program), "{") {
		t.Errorf("a program should get JSON: %s", program)
	}
}

// TestErrorsReachTheAgentWithTheirCallToAction: a failure is an instruction for
// the next step, and it is worthless if the surface swallows it.
func TestErrorsReachTheAgentWithTheirCallToAction(t *testing.T) {
	reg := command.NewRegistry()
	err := command.Register(reg, command.Command[demoInput, demoOutput]{
		Group: "records", Name: "fail", Summary: "Always fails.",
		Handler: func(context.Context, demoInput) (demoOutput, error) {
			return demoOutput{}, apperr.New("RECORD_LOCKED").
				Causer("records.fail").
				Msgf("the record is locked").
				Status(apperr.StatusConflict).
				CTA(apperr.CallToAction{Label: "wait and retry", Tool: "records_touch"})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, stderr, runErr := run(t, reg, "records", "fail", "r-1", "--format", "json")
	if runErr == nil {
		t.Fatal("a failing command must exit non-zero")
	}
	if !clix.Silent(runErr) {
		t.Error("the error was already rendered; it must not be printed twice")
	}
	var payload map[string]any
	if jerr := json.Unmarshal([]byte(stderr), &payload); jerr != nil {
		t.Fatalf("stderr is not the structured error: %s", stderr)
	}
	if payload["code"] != "AOS_RECORD_LOCKED" {
		t.Errorf("code = %v", payload["code"])
	}
	if _, ok := payload["cta"]; !ok {
		t.Errorf("the call to action did not survive: %v", payload)
	}
}

// TestBuiltinsCannotShadowADomainGroup covers defect #15: in the original the
// builtin `skills` group hides the domain one, and its create/install/discovery
// operations cannot be reached from the CLI at all.
func TestBuiltinsCannotShadowADomainGroup(t *testing.T) {
	reg := command.NewRegistry()
	err := command.Register(reg, command.Command[demoInput, demoOutput]{
		Group: "skills", Name: "install", Summary: "Install a skill.",
		Handler: func(context.Context, demoInput) (demoOutput, error) { return demoOutput{}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	reg.DescribeGroup(command.GroupDoc{Name: "skills", Summary: "Skills."})

	out, _, herr := run(t, reg, "skills", "--help")
	if herr != nil {
		t.Fatal(herr)
	}
	if !strings.Contains(out, "install") {
		t.Fatalf("the domain group is not reachable:\n%s", out)
	}

	// The built-ins are still there, under their own namespace.
	out, _, herr = run(t, reg, "self", "--help")
	if herr != nil {
		t.Fatal(herr)
	}
	for _, want := range []string{"completions", "tools", "llms"} {
		if !strings.Contains(out, want) {
			t.Errorf("self is missing %s:\n%s", want, out)
		}
	}
}

func TestCompletionsAreGeneratedForFourShells(t *testing.T) {
	reg, _ := registry(t)
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		out, _, err := run(t, reg, "self", "completions", shell)
		if err != nil {
			t.Errorf("%s: %v", shell, err)
			continue
		}
		if len(out) < 100 {
			t.Errorf("%s: completion script looks empty", shell)
		}
	}
}

func TestSelfToolsListsThePublishedSurface(t *testing.T) {
	reg, _ := registry(t)
	out, _, err := run(t, reg, "self", "tools", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "records_touch") {
		t.Fatalf("output = %s", out)
	}
}

func TestHelpRendersDocumentationAndPasteableExamples(t *testing.T) {
	reg, _ := registry(t, func(c *command.Command[demoInput, demoOutput]) {
		c.Examples = []command.Example{
			{Description: "touch a record", Input: demoInput{ID: "r-1", Title: "a title"}},
		}
	})
	out, _, err := run(t, reg, "records", "touch", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Touch a record so that something observable happens.") {
		t.Errorf("the documentation is missing:\n%s", out)
	}
	if !strings.Contains(out, "aos records touch r-1 --title 'a title'") {
		t.Errorf("the example is not a pasteable line:\n%s", out)
	}
}

// shadowInput has fields whose names collide with the transport flags. Both
// meanings are legitimate: `--agent` on a transport is "act as", and on
// `memories recall` it is "whose memories".
type shadowInput struct {
	Agent     string `json:"agent,omitempty" jsonschema:"Whose records to read."`
	Workspace string `json:"workspace,omitempty" jsonschema:"Which workspace to read."`

	command.Reasoning
}

type shadowOutput struct {
	Agent     string `json:"agent"`
	Workspace string `json:"workspace"`
}

func shadowRegistry(t *testing.T) *command.Registry {
	t.Helper()
	reg := command.NewRegistry()
	err := command.Register(reg, command.Command[shadowInput, shadowOutput]{
		Group:    "records",
		Name:     "scan",
		Summary:  "Scan records.",
		Doc:      "Scan the records of one agent.",
		Registry: true,
		Handler: func(_ context.Context, in shadowInput) (shadowOutput, error) {
			return shadowOutput{Agent: in.Agent, Workspace: in.Workspace}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	reg.DescribeGroup(command.GroupDoc{Name: "records", Summary: "Records."})
	return reg
}

// TestADomainFieldMayShareATransportFlagName is a regression, and it was found
// by the parity suite rather than reasoned about: `memories forget` has an
// `agent` field, and registering it beside the `--agent` transport flag made
// pflag panic. The failure was not a confusing command — it was the whole
// command tree failing to build, so every command died because one had an
// ordinary field name.
func TestADomainFieldMayShareATransportFlagName(t *testing.T) {
	reg := shadowRegistry(t)

	// Building the tree at all is half the assertion.
	out, _, err := run(t, reg, "records", "scan", "--agent", "luara", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	// And the command's own field is what receives the value.
	if !strings.Contains(out, `"agent": "luara"`) {
		t.Fatalf("the domain field did not receive the flag: %s", out)
	}
}

// TestTheShadowedFlagKeepsTheDomainDescription: the help is where a person
// finds out which of the two meanings is in force, so it has to say the right
// one. The transport flags the input does not claim are untouched.
func TestTheShadowedFlagKeepsTheDomainDescription(t *testing.T) {
	out, _, err := run(t, shadowRegistry(t), "records", "scan", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range clix.TransportFlags() {
		if !hasFlag(out, name) {
			t.Errorf("--%s is missing entirely", name)
		}
	}
	if !strings.Contains(out, "Whose records to read") {
		t.Errorf("--agent does not carry the domain description:\n%s", out)
	}
	if strings.Contains(out, "X-Agent-ID") {
		t.Errorf("--agent still advertises the transport meaning:\n%s", out)
	}
	if !strings.Contains(out, "Bearer") {
		t.Errorf("--token lost its transport description:\n%s", out)
	}
}
