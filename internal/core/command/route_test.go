package command_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/OWNER/aos/internal/core/command"
)

type routedIn struct {
	command.Reasoning
	Name string `json:"name" jsonschema:"The value echoed back, so a test can tell one registry from another."`
}

type routedOut struct {
	Answer string `json:"answer"`
}

// registryAnswering builds a registry whose one command reports which
// registry it belongs to, so a test can tell which one actually ran.
func registryAnswering(t *testing.T, answer string) *command.Registry {
	t.Helper()
	r := command.NewRegistry()
	command.MustRegister(r, command.Command[routedIn, routedOut]{
		Group:   "probe",
		Name:    "ask",
		Summary: "reports which registry answered",
		Handler: func(_ context.Context, in routedIn) (routedOut, error) {
			return routedOut{Answer: answer + ":" + in.Name}, nil
		},
	})
	return r
}

func invoke(t *testing.T, r *command.Registry, key string) (routedOut, error) {
	t.Helper()
	d, _, ok := r.Lookup(key)
	if !ok {
		t.Fatalf("%s is not in the registry", key)
	}
	raw, err := d.Invoke(context.Background(), command.SurfaceHTTP,
		json.RawMessage(`{"_reasoning":"a test is exercising the routing","name":"x"}`))
	if err != nil {
		return routedOut{}, err
	}
	out, ok := raw.(routedOut)
	if !ok {
		t.Fatalf("result is %T, not the command's own output type", raw)
	}
	return out, nil
}

// TestRoutedInvokeReachesTheRegistryTheContextNames is the whole point: one
// published surface, several backing registries, and the call lands in the one
// this caller is scoped to.
func TestRoutedInvokeReachesTheRegistryTheContextNames(t *testing.T) {
	alpha := registryAnswering(t, "alpha")
	beta := registryAnswering(t, "beta")

	which := alpha
	routed := command.Route(alpha, func(context.Context) (*command.Registry, error) {
		return which, nil
	})

	got, err := invoke(t, routed, "probe_ask")
	if err != nil {
		t.Fatal(err)
	}
	if got.Answer != "alpha:x" {
		t.Fatalf("answer = %q, want the alpha registry", got.Answer)
	}

	which = beta
	got, err = invoke(t, routed, "probe_ask")
	if err != nil {
		t.Fatal(err)
	}
	if got.Answer != "beta:x" {
		t.Fatalf("answer = %q, want the beta registry", got.Answer)
	}
}

// TestRoutedKeepsEveryDescribedShape: the routed registry is what the CLI, MCP
// and HTTP surfaces publish their tool list from, so it has to look exactly
// like the one it was built over — same keys, same schemas, same docs.
func TestRoutedKeepsEveryDescribedShape(t *testing.T) {
	template := registryAnswering(t, "alpha")
	routed := command.Route(template, func(context.Context) (*command.Registry, error) {
		return template, nil
	})

	if routed.Len() != template.Len() {
		t.Fatalf("routed has %d commands, template has %d", routed.Len(), template.Len())
	}

	want, _, _ := template.Lookup("probe_ask")
	got, _, ok := routed.Lookup("probe_ask")
	if !ok {
		t.Fatal("probe_ask is missing from the routed registry")
	}
	if got.Key() != want.Key() || got.Group() != want.Group() || got.Name() != want.Name() {
		t.Errorf("identity diverged: %s/%s/%s", got.Key(), got.Group(), got.Name())
	}
	if got.Summary() != want.Summary() {
		t.Errorf("summary = %q, want %q", got.Summary(), want.Summary())
	}
	if got.InputType() != want.InputType() {
		t.Errorf("input type = %v, want %v", got.InputType(), want.InputType())
	}
	if got.InputSchema() == nil {
		t.Error("the routed descriptor published no input schema")
	}
	if got.InRegistry() != want.InRegistry() {
		t.Error("registry visibility diverged")
	}
}

// TestRoutedSurfacesAResolutionFailure: a caller naming a workspace that does
// not exist gets that error, not a command that quietly ran somewhere else.
func TestRoutedSurfacesAResolutionFailure(t *testing.T) {
	template := registryAnswering(t, "alpha")
	refusal := errors.New("no such workspace")
	routed := command.Route(template, func(context.Context) (*command.Registry, error) {
		return nil, refusal
	})

	if _, err := invoke(t, routed, "probe_ask"); !errors.Is(err, refusal) {
		t.Fatalf("error = %v, want the resolver's own refusal", err)
	}
}

// TestRoutedIsFrozen — the published tool list must not be a moving target,
// and a routed registry is a published surface like any other.
func TestRoutedIsFrozen(t *testing.T) {
	template := registryAnswering(t, "alpha")
	routed := command.Route(template, func(context.Context) (*command.Registry, error) {
		return template, nil
	})

	err := command.Register(routed, command.Command[routedIn, routedOut]{
		Group: "probe", Name: "later",
		Handler: func(context.Context, routedIn) (routedOut, error) { return routedOut{}, nil },
	})
	if err == nil {
		t.Fatal("a command was added to a routed registry after boot")
	}
}
