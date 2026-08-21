package template

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
)

// blockingEngine never returns from Render on its own — it waits for the
// test to release it, or for the caller to stop waiting, whichever comes
// first. It stands in for a template with a pathological loop: real Liquid
// has no built-in way to loop forever, so this is the deterministic
// equivalent, not a live osteele/liquid infinite loop.
type blockingEngine struct {
	release chan struct{}
}

func (blockingEngine) Validate(string) error { return nil }

func (e blockingEngine) Render(source string, vars map[string]any) (string, error) {
	<-e.release
	return "too late", nil
}

func TestRenderStopsWaitingAtTheTimeoutRatherThanHangingForever(t *testing.T) {
	old := renderTimeout
	renderTimeout = 20 * time.Millisecond
	defer func() { renderTimeout = old }()

	eng := blockingEngine{release: make(chan struct{})}
	defer close(eng.release) // let the abandoned goroutine finish, so the test process can exit cleanly

	_, err := render(context.Background(), eng, "slow", "{{ x }}", nil)
	app, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err is %T, want *apperr.Error", err)
	}
	if app.Code != "AOS_TEMPLATE_RENDER_TIMEOUT" {
		t.Fatalf("code = %q", app.Code)
	}
}

// capEngine returns a fixed string, long enough to exceed whatever
// maxOutputBytes the test shrinks itself to.
type capEngine struct{ out string }

func (capEngine) Validate(string) error                           { return nil }
func (e capEngine) Render(string, map[string]any) (string, error) { return e.out, nil }

func TestRenderRejectsOutputOverTheCap(t *testing.T) {
	old := maxOutputBytes
	maxOutputBytes = 8
	defer func() { maxOutputBytes = old }()

	eng := capEngine{out: "this is far more than eight bytes"}
	_, err := render(context.Background(), eng, "big", "{{ x }}", nil)
	app, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err is %T, want *apperr.Error", err)
	}
	if app.Code != "AOS_TEMPLATE_OUTPUT_TOO_LARGE" {
		t.Fatalf("code = %q", app.Code)
	}
}

type failEngine struct{ err error }

func (failEngine) Validate(string) error                           { return nil }
func (e failEngine) Render(string, map[string]any) (string, error) { return "", e.err }

func TestRenderWrapsAnEngineFailure(t *testing.T) {
	_, err := render(context.Background(), failEngine{err: errors.New("bad filter")}, "t", "{{ x }}", nil)
	app, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err is %T, want *apperr.Error", err)
	}
	if app.Code != "AOS_TEMPLATE_RENDER_FAILED" {
		t.Fatalf("code = %q", app.Code)
	}
}

// --- validateVariables -------------------------------------------------

func TestValidateVariablesFillsAMissingOptionalFromItsDefault(t *testing.T) {
	declared := []Variable{{Name: "greeting", Default: "hi"}}
	got, err := validateVariables("t", declared, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if got["greeting"] != "hi" {
		t.Fatalf("got = %v", got)
	}
}

func TestValidateVariablesRefusesAMissingRequiredWithNoDefault(t *testing.T) {
	declared := []Variable{{Name: "name", Required: true}}
	_, err := validateVariables("t", declared, map[string]any{})
	app, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err is %T, want *apperr.Error", err)
	}
	if app.Code != "AOS_TEMPLATE_VARIABLE_MISSING" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["variable"] != "name" {
		t.Fatalf("the error does not name the variable: %v", app.Issues)
	}
	if len(app.Actions) == 0 || app.Actions[0].Tool != "templates_get" {
		t.Fatalf("CTA does not point at templates_get: %+v", app.Actions)
	}
}

func TestValidateVariablesPassesThroughWhatTheCallerGaveOverADefault(t *testing.T) {
	declared := []Variable{{Name: "name", Default: "fallback"}}
	got, err := validateVariables("t", declared, map[string]any{"name": "given"})
	if err != nil {
		t.Fatal(err)
	}
	if got["name"] != "given" {
		t.Fatalf("got = %v", got)
	}
}

func TestHasDelimitersRecognisesBothLiquidSyntaxForms(t *testing.T) {
	cases := map[string]bool{
		"plain text":             false,
		"":                       false,
		"{{ name }}":             true,
		"{% if x %}y{% endif %}": true,
	}
	for in, want := range cases {
		if got := hasDelimiters(in); got != want {
			t.Fatalf("hasDelimiters(%q) = %v, want %v", in, got, want)
		}
	}
}
