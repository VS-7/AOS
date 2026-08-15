package config_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/config"
)

// fakeStore keeps the configuration in memory. Domain tests run on fakes: no
// disk, no network, and a failure means the rule under test is wrong.
type fakeStore struct {
	cfg      config.Config
	saveErr  error
	loadErr  error
	saves    int
	lastSave config.Config
}

func (f *fakeStore) Load(context.Context) (config.Config, error) {
	if f.loadErr != nil {
		return config.Config{}, f.loadErr
	}
	return f.cfg, nil
}

func (f *fakeStore) Save(_ context.Context, c config.Config) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saves++
	f.cfg, f.lastSave = c, c
	return nil
}

func configured() config.Config {
	c := config.Default()
	c.Security.Secret = strings.Repeat("a", 128)
	c.Security.APIToken = "aos_0123456789abcdef0123456789abcdef"
	c.Security.Password = "correct horse battery staple"
	c.Tunnel.Token = "cf_tunnel_token_value"
	c.Agents.Providers = []config.Provider{
		{ID: "openai", Key: "sk-liveprovidersecretvalue"},
		{ID: "codex", Key: ""},
	}
	return c
}

func newService(c config.Config) (config.Service, *fakeStore) {
	store := &fakeStore{cfg: c}
	return config.NewService(store), store
}

func agentCtx() context.Context {
	return identity.With(context.Background(), identity.Identity{AgentID: "orchestrator"})
}

func humanCtx() context.Context {
	return identity.With(context.Background(), identity.Identity{UserID: "vitor"})
}

// TestNoSecretIsReachableWithoutReveal is the fix for defect #7: config_get was
// exposed over MCP with no filtering, so an agent could read security.secret,
// the API token and every provider key.
func TestNoSecretIsReachableWithoutReveal(t *testing.T) {
	original := configured()
	svc, _ := newService(original)

	got, err := svc.Get(agentCtx(), config.GetInput{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{
		original.Security.Secret,
		original.Security.APIToken,
		original.Security.Password,
		original.Tunnel.Token,
		original.Agents.Providers[0].Key,
	} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("redacted config still contains %q: %s", secret, raw)
		}
	}
	// The fingerprint keeps "which key is configured" answerable.
	if !strings.HasPrefix(got.Agents.Providers[0].Key, config.RedactedMark) {
		t.Errorf("provider key was not fingerprinted: %q", got.Agents.Providers[0].Key)
	}
	if got.Agents.Providers[1].Key != "" {
		t.Errorf("an unset key must stay empty, got %q", got.Agents.Providers[1].Key)
	}
}

// TestEverySecretFieldIsRedacted walks the type rather than a sample, so a
// secret field added later cannot slip through untested.
func TestEverySecretFieldIsRedacted(t *testing.T) {
	paths := config.SecretPaths()
	if len(paths) == 0 {
		t.Fatal("no secret fields found — the tag scan is broken")
	}
	want := []string{
		"security.password", "security.secret", "security.apiToken",
		"agents.providers[].key", "tunnel.token",
	}
	for _, w := range want {
		found := false
		for _, p := range paths {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("secret path %q not detected; found %v", w, paths)
		}
	}

	// Redaction must leave every non-secret field untouched.
	before := configured()
	after := config.Redact(before)
	if !reflect.DeepEqual(before.Region, after.Region) || before.General != after.General {
		t.Error("redaction changed a non-secret field")
	}
	if after.Security.Enabled != before.Security.Enabled {
		t.Error("redaction changed security.enabled")
	}
}

func TestRevealIsRefusedForAgents(t *testing.T) {
	svc, _ := newService(configured())
	_, err := svc.Get(agentCtx(), config.GetInput{Reveal: true})
	if err == nil {
		t.Fatal("an agent must never receive unredacted secrets")
	}
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("error = %v", err)
	}
}

func TestRevealWorksForAHuman(t *testing.T) {
	original := configured()
	svc, _ := newService(original)
	got, err := svc.Get(humanCtx(), config.GetInput{Reveal: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Security.Secret != original.Security.Secret {
		t.Error("a human with --reveal should see the value")
	}
}

func TestAgentMayWriteAllowlistedFields(t *testing.T) {
	svc, store := newService(config.Default())
	_, err := svc.Update(agentCtx(), config.UpdateInput{
		Set: map[string]any{"region.timezone": "America/Sao_Paulo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.lastSave.Region.Timezone != "America/Sao_Paulo" {
		t.Fatalf("timezone = %q", store.lastSave.Region.Timezone)
	}
}

func TestAgentMayNotWriteSecurityOrProviderKeys(t *testing.T) {
	for _, path := range []string{
		"security.enabled", "security.apiToken", "tunnel.token", "user.email",
	} {
		svc, store := newService(config.Default())
		_, err := svc.Update(agentCtx(), config.UpdateInput{Set: map[string]any{path: "x"}})
		if err == nil {
			t.Errorf("%s: an agent must not be able to write this", path)
			continue
		}
		if !errors.Is(err, apperr.ErrForbidden) {
			t.Errorf("%s: error = %v", path, err)
		}
		e, _ := apperr.As(err)
		if len(e.Actions) == 0 {
			t.Errorf("%s: a 403 must carry a CTA", path)
		}
		if store.saves != 0 {
			t.Errorf("%s: a rejected update must not write", path)
		}
	}
}

func TestHumanMayWriteTheSameFieldTheAgentCannot(t *testing.T) {
	svc, store := newService(config.Default())
	if _, err := svc.Update(humanCtx(), config.UpdateInput{
		Set: map[string]any{"security.enabled": false},
	}); err != nil {
		t.Fatal(err)
	}
	if store.lastSave.Security.Enabled {
		t.Fatal("the update was not applied")
	}
}

func TestUnknownFieldIsRejected(t *testing.T) {
	svc, _ := newService(config.Default())
	_, err := svc.Update(humanCtx(), config.UpdateInput{Set: map[string]any{"region.planet": "earth"}})
	if err == nil || !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateNeverReturnsSecrets(t *testing.T) {
	original := configured()
	svc, store := newService(original)
	got, err := svc.Update(humanCtx(), config.UpdateInput{Set: map[string]any{"region.city": "Salvador"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Security.Secret == original.Security.Secret {
		t.Error("Update returned the secret in full")
	}
	if !strings.HasPrefix(got.Security.Secret, config.RedactedMark) {
		t.Errorf("secret was not fingerprinted: %q", got.Security.Secret)
	}
	// What was written to the store keeps the real value: redaction is a
	// property of the response, never of the stored configuration.
	if store.lastSave.Security.Secret != original.Security.Secret {
		t.Error("Update persisted a redacted secret over the real one")
	}
}

// TestRedactDoesNotMutateItsInput is the defect the deep copy exists for: a
// shallow copy shares the providers backing array, so redacting the response
// would blank the live key the daemon needs to reach the provider.
func TestRedactDoesNotMutateItsInput(t *testing.T) {
	original := configured()
	before := original.Agents.Providers[0].Key

	_ = config.Redact(original)

	if original.Agents.Providers[0].Key != before {
		t.Fatalf("Redact mutated its input: %q became %q", before, original.Agents.Providers[0].Key)
	}
	if original.Security.Secret == "" || strings.HasPrefix(original.Security.Secret, config.RedactedMark) {
		t.Fatal("Redact mutated a struct field of its input")
	}
}

// TestSafeDefaults records the three deliberate divergences from the original's
// defaults, so that a future change to Default() has to face them explicitly.
func TestSafeDefaults(t *testing.T) {
	d := config.Default()
	if !d.Security.Enabled {
		t.Error("security must be enabled by default (ADR-0009)")
	}
	if d.Telemetry.Enabled {
		t.Error("telemetry must be opt-in")
	}
	if d.MCP.ToolShape != config.ToolShapeComposite {
		t.Errorf("tool shape = %q, want composite (ADR-0011)", d.MCP.ToolShape)
	}
}

// TestNormalizeFillsNewFieldsWithoutLosingOldValues is the upgrade path: a
// config written by an older build keeps working.
func TestNormalizeFillsNewFieldsWithoutLosingOldValues(t *testing.T) {
	old := config.Config{}
	old.User.Name = "Vitor"
	old.Region.Timezone = "America/Sao_Paulo"

	got := config.Normalize(old)
	if got.User.Name != "Vitor" || got.Region.Timezone != "America/Sao_Paulo" {
		t.Fatalf("normalize lost data: %+v", got)
	}
	if got.Region.Language == "" || got.MCP.ToolShape == "" || got.Agents.Models == nil {
		t.Fatalf("normalize left new fields empty: %+v", got)
	}
}

func TestLoadFailurePropagatesAsAppError(t *testing.T) {
	store := &fakeStore{loadErr: errors.New("disk gone")}
	svc := config.NewService(store)
	_, err := svc.Get(context.Background(), config.GetInput{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if e, ok := apperr.As(err); !ok || e.CauserName == "" {
		t.Fatalf("error is not a classified app error: %v", err)
	}
}
