package template_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/liquidengine"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/template"
)

func ctx() context.Context { return context.Background() }

func codeOf(t *testing.T, err error) string {
	t.Helper()
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err is not *apperr.Error: %v", err)
	}
	return e.Code
}

// fixedClock advances by a millisecond on every read, so CreatedAt/UpdatedAt
// are deterministic and never equal by accident.
type fixedClock struct{ at time.Time }

func (c *fixedClock) Now() time.Time {
	c.at = c.at.Add(time.Millisecond)
	return c.at
}

func newService(t *testing.T) *template.Service {
	t.Helper()
	return template.NewService(template.Deps{
		Repo:   fakes.NewRepo[template.Template]("templates"),
		Engine: liquidengine.New(),
		Clock:  &fixedClock{},
	})
}

// --- round trip --------------------------------------------------------

func TestRoundTripWithDeclaredVariables(t *testing.T) {
	svc := newService(t)

	created, err := svc.Create(ctx(), template.CreateInput{
		ID:      "plan",
		Name:    "plan",
		Content: "## {{ name }}\n\n{{ summary }}",
		Variables: []template.Variable{
			{Name: "name", Type: "string", Required: true},
			{Name: "summary", Type: "string", Default: "(no summary)"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "plan" || len(created.Variables) != 2 {
		t.Fatalf("created = %+v", created)
	}

	got, err := svc.Get(ctx(), template.GetInput{ID: "plan"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != created.Content {
		t.Fatalf("get did not round-trip Content: %+v", got)
	}

	listed, err := svc.List(ctx(), template.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if listed.Total != 1 || listed.Templates[0].ID != "plan" {
		t.Fatalf("list = %+v", listed)
	}

	updated, err := svc.Update(ctx(), template.UpdateInput{
		ID:      "plan",
		Content: strPtr("## {{ name }} (revised)"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Content != "## {{ name }} (revised)" {
		t.Fatalf("updated.Content = %q", updated.Content)
	}
	if !updated.UpdatedAt.After(created.CreatedAt) {
		t.Fatalf("UpdatedAt did not advance: created=%v updated=%v", created.CreatedAt, updated.UpdatedAt)
	}

	if _, err := svc.Delete(ctx(), template.DeleteInput{ID: "plan"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx(), template.GetInput{ID: "plan"}); codeOf(t, err) != "AOS_TEMPLATE_NOT_FOUND" {
		t.Fatalf("get after delete: code = %v", err)
	}

	// Deleting what is already gone succeeds — idempotent, like every other
	// native record's repository.
	if _, err := svc.Delete(ctx(), template.DeleteInput{ID: "plan"}); err != nil {
		t.Fatalf("second delete should be idempotent: %v", err)
	}
}

// --- write-time validation ----------------------------------------------

func TestCreateRefusesContentThatDoesNotParseAsLiquid(t *testing.T) {
	svc := newService(t)
	_, err := svc.Create(ctx(), template.CreateInput{
		ID: "broken", Name: "broken", Content: "{% if x %}no endif",
	})
	if code := codeOf(t, err); code != "AOS_TEMPLATE_CONTENT_INVALID" {
		t.Fatalf("code = %q", code)
	}

	// Nothing was written: get finds nothing, the round-trip guarantee for
	// a refusal that happens before persistence.
	if _, err := svc.Get(ctx(), template.GetInput{ID: "broken"}); codeOf(t, err) != "AOS_TEMPLATE_NOT_FOUND" {
		t.Fatalf("a refused create should not have persisted anything: %v", err)
	}
}

func TestUpdateRefusesNewContentThatDoesNotParseAsLiquid(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(ctx(), template.CreateInput{ID: "t", Name: "t", Content: "fine"}); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Update(ctx(), template.UpdateInput{ID: "t", Content: strPtr("{% if %}")})
	if code := codeOf(t, err); code != "AOS_TEMPLATE_CONTENT_INVALID" {
		t.Fatalf("code = %q", code)
	}

	// The old content is untouched.
	got, err := svc.Get(ctx(), template.GetInput{ID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "fine" {
		t.Fatalf("a refused update should not have changed Content: %q", got.Content)
	}
}

func TestCreateWithoutAnIDIsRefused(t *testing.T) {
	svc := newService(t)
	_, err := svc.Create(ctx(), template.CreateInput{Name: "no id", Content: "x"})
	if code := codeOf(t, err); code != "AOS_TEMPLATE_ID_REQUIRED" {
		t.Fatalf("code = %q", code)
	}
}

// --- render, end to end over the real Liquid engine ----------------------

func TestRenderEndToEndWithTheRealEngine(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(ctx(), template.CreateInput{
		ID: "greet", Name: "greet", Content: "Hello, {{ name }}!",
		Variables: []template.Variable{{Name: "name", Required: true}},
	}); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Render(ctx(), template.RenderInput{ID: "greet", Variables: map[string]any{"name": "Ada"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Output != "Hello, Ada!" {
		t.Fatalf("out.Output = %q", out.Output)
	}
}

func TestRenderRefusesAMissingRequiredVariableWithACTAToGet(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(ctx(), template.CreateInput{
		ID: "greet", Name: "greet", Content: "Hello, {{ name }}!",
		Variables: []template.Variable{{Name: "name", Required: true}},
	}); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Render(ctx(), template.RenderInput{ID: "greet"})
	app, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_TEMPLATE_VARIABLE_MISSING" {
		t.Fatalf("code = %q", app.Code)
	}
	if len(app.Actions) == 0 || app.Actions[0].Tool != "templates_get" {
		t.Fatalf("CTA does not point at templates_get: %+v", app.Actions)
	}
}

// A memory or another persisted record containing a Liquid-looking string
// must not be able to reach config or the process environment through a
// template render — variables are exactly and only what the caller passed
// through RenderInput.Variables, never anything this service adds on its own
// (ADR-0014's allowlist rule, applied here even though this package is the
// one place that IS meant to render, unlike the prompt path).
func TestRenderNeverSeesConfigOrEnvironment(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(ctx(), template.CreateInput{
		ID: "leak", Name: "leak", Content: "{{ config.security.secret }}{{ env.HOME }}",
	}); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Render(ctx(), template.RenderInput{ID: "leak"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.Output, "secret") || out.Output != "" {
		t.Fatalf("rendered something from names never in scope: %q", out.Output)
	}
}

func TestRenderOfAMissingTemplateFails(t *testing.T) {
	svc := newService(t)
	_, err := svc.Render(ctx(), template.RenderInput{ID: "nope"})
	if code := codeOf(t, err); code != "AOS_TEMPLATE_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

// --- golden filter behaviour ---------------------------------------------

// TestFilterGoldens documents osteele/liquid's actual output for a handful of
// standard Liquid filters. No live LiquidJS was available in this
// environment to diff against directly, so this is not a proof of parity —
// it is a golden that catches a future regression in this package's own
// dependency, and a place to record a real divergence if one is found later
// (ADR-0014 warns osteele/liquid does not cover 100% of LiquidJS's filters).
func TestFilterGoldens(t *testing.T) {
	svc := newService(t)
	cases := []struct {
		name, content, want string
		vars                map[string]any
	}{
		{"upcase", "{{ name | upcase }}", "ADA", map[string]any{"name": "ada"}},
		{"downcase", "{{ name | downcase }}", "ada", map[string]any{"name": "ADA"}},
		{"append", "{{ name | append: '!' }}", "Ada!", map[string]any{"name": "Ada"}},
		{"default", "{{ missing | default: 'x' }}", "x", map[string]any{}},
		{"size", "{{ list | size }}", "3", map[string]any{"list": []any{1, 2, 3}}},
		{"join", "{{ list | join: ', ' }}", "a, b", map[string]any{"list": []any{"a", "b"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := "golden-" + c.name
			if _, err := svc.Create(ctx(), template.CreateInput{ID: id, Name: id, Content: c.content}); err != nil {
				t.Fatal(err)
			}
			out, err := svc.Render(ctx(), template.RenderInput{ID: id, Variables: c.vars})
			if err != nil {
				t.Fatal(err)
			}
			if out.Output != c.want {
				t.Fatalf("%s: got %q, want %q", c.name, out.Output, c.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
