package toolset_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/domain/activity"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/toolset"
)

// --- interpolation: the security-relevant part -----------------------------

// A missing secret fails the connection, naming the variable. Substituting an
// empty string instead produces a 401 much later, from a server, with nothing
// pointing back at the unset variable that caused it.
func TestInterpolationFailsLoudOnAMissingVariable(t *testing.T) {
	_, err := toolset.Interpolate("Bearer ${env.GITHUB_TOKEN}", envOf(map[string]string{}))
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_TOOLSET_ENV_MISSING" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["variable"] != "GITHUB_TOKEN" {
		t.Fatalf("the error does not name the variable: %v", app.Issues)
	}
}

func TestInterpolationResolvesEveryOccurrence(t *testing.T) {
	e := envOf(map[string]string{"HOST": "api.example.com", "TOKEN": "abc"})
	got, err := toolset.Interpolate("https://${env.HOST}/v1?t=${env.TOKEN}&u=${env.HOST}", e)
	if err != nil {
		t.Fatal(err)
	}
	// Every occurrence, not just the first: a header that resolved once and
	// left the second placeholder literal is a request that fails opaquely.
	if got != "https://api.example.com/v1?t=abc&u=api.example.com" {
		t.Fatalf("got = %q", got)
	}
}

func TestATextWithNoPlaceholderIsReturnedUnchanged(t *testing.T) {
	e := envOf(map[string]string{})
	for _, in := range []string{"", "plain text", "$notenv", "${notenv.X}", "100% ${literal}"} {
		got, err := toolset.Interpolate(in, e)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got != in {
			t.Fatalf("%q became %q", in, got)
		}
	}
}

// A resolved secret must not end up in the error message of a later failure.
func TestAnInterpolatedValueIsNeverEchoedInAnError(t *testing.T) {
	e := envOf(map[string]string{"TOKEN": "super-secret-value"})
	ts := toolset.Toolset{
		ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Command: "does-not-exist",
		Env:     map[string]string{"AUTH": "${env.TOKEN}"},
	}
	svc := newService(t, withToolset(ts), withAdapter("gh", failingAdapter{}), withEnv(e))

	_, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "x"})
	if err == nil {
		t.Fatal("a failing connection reported success")
	}
	// A resolved secret must not travel in the message of a later failure —
	// that message goes to a log, a UI, and possibly a bug report.
	if strings.Contains(err.Error(), "super-secret-value") {
		t.Fatalf("the resolved secret leaked into the error: %v", err)
	}
}

// --- the discriminated union -----------------------------------------------

// The five types decode; an unknown one is refused rather than defaulting to
// one of them. Only stdio, http and rest-api are implemented in this slice,
// and the other two have to fail with "not available in this build" rather
// than with a decode error — the difference matters to whoever reads it.
func TestTheFiveTypesDecodeAndAnUnknownOneIsRefused(t *testing.T) {
	for _, raw := range []string{
		"mcp-server::stdio", "mcp-server::http", "rest-api", "cli", "custom",
	} {
		if _, err := toolset.ParseType(raw); err != nil {
			t.Fatalf("%q did not decode: %v", raw, err)
		}
	}
	// An unknown type is refused rather than defaulting to one of them: a
	// toolset that silently became "custom" would connect to nothing and say
	// nothing about why.
	_, err := toolset.ParseType("grpc")
	if code := codeOf(t, err); code != "AOS_TOOLSET_TYPE_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}

func TestATypeThisSliceDoesNotImplementSaysSo(t *testing.T) {
	svc := newService(t)
	_, err := svc.Call(ctx(), toolset.CallInput{ID: "cli-thing", Tool: "x"})
	if code := codeOf(t, err); code != "AOS_TOOLSET_TYPE_NOT_AVAILABLE" {
		t.Fatalf("code = %q", code)
	}
}

// --- the audit: the reason the indirection exists ---------------------------

// Every external call is recorded: which toolset, which tool, how long, and
// whether it worked. Never the payload — it can carry the user's data, and an
// audit log that stores it becomes the thing that leaks.
func TestACallIsRecordedInActivityWithoutThePayload(t *testing.T) {
	acts := &fakeActivities{}
	svc := newService(t, withActivities(acts), withAdapter("gh", okAdapter{}))

	secret := `{"token":"super-secret-value"}`
	if _, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "create_issue",
		Input: json.RawMessage(secret)}); err != nil {
		t.Fatal(err)
	}

	if len(acts.published) != 1 {
		t.Fatalf("published %d activities, want 1", len(acts.published))
	}
	rendered := fmt.Sprintf("%+v", acts.published[0])
	if strings.Contains(rendered, "super-secret-value") {
		t.Fatalf("the payload reached the audit log:\n%s", rendered)
	}
	if !strings.Contains(rendered, "create_issue") {
		t.Fatalf("the audit record does not say which tool ran:\n%s", rendered)
	}
}

// A failed call is recorded too. An audit log that only holds successes is an
// audit log that hides exactly what somebody would look for.
func TestAFailedCallIsRecordedAsAFailure(t *testing.T) {
	acts := &fakeActivities{}
	svc := newService(t, withActivities(acts), withAdapter("gh", failingAdapter{}))

	if _, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "create_issue"}); err == nil {
		t.Fatal("a failing call reported success")
	}
	// An audit log that only holds successes hides exactly what somebody
	// would go looking for.
	if len(acts.published) != 1 {
		t.Fatalf("published %d activities, want 1", len(acts.published))
	}
	rendered := fmt.Sprintf("%+v", acts.published[0])
	if !strings.Contains(rendered, "failed") {
		t.Fatalf("the failure was not recorded as one:\n%s", rendered)
	}
}

// --- the rest of the surface: List, Get, UpdateConfig, Delete --------------

func TestListAndGetReturnIndependentCopies(t *testing.T) {
	svc := newService(t, withToolset(toolset.Toolset{
		ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Env: map[string]string{"A": "1"},
	}))

	got, err := svc.Get(ctx(), toolset.GetInput{ID: "gh"})
	if err != nil {
		t.Fatal(err)
	}
	got.Env["A"] = "corrupted"

	again, err := svc.Get(ctx(), toolset.GetInput{ID: "gh"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Env["A"] != "1" {
		t.Fatalf("mutating one caller's copy corrupted what the next Get returned: %v", again.Env)
	}

	all, err := svc.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ts := range all {
		if ts.ID == "gh" {
			found = true
			if ts.Env["A"] != "1" {
				t.Fatalf("List did not see the same, uncorrupted record: %+v", ts)
			}
		}
	}
	if !found {
		t.Fatalf("List did not return %q: %+v", "gh", all)
	}
}

func TestGetMissingIsNotFound(t *testing.T) {
	svc := newService(t)
	_, err := svc.Get(ctx(), toolset.GetInput{ID: "does-not-exist"})
	if code := codeOf(t, err); code != "AOS_TOOLSET_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

func TestUpdateConfigReplacesFieldsWholesaleAndPersists(t *testing.T) {
	svc := newService(t, withToolset(toolset.Toolset{
		ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Env: map[string]string{"A": "1", "B": "2"},
	}))

	newEnv := map[string]string{"C": "3"}
	updated, err := svc.UpdateConfig(ctx(), toolset.UpdateConfigInput{ID: "gh", Env: newEnv})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Env) != 1 || updated.Env["C"] != "3" {
		t.Fatalf("Env was not replaced wholesale: %v", updated.Env)
	}

	// Mutating the caller's own map, and the input map handed to
	// UpdateConfig, must not reach what was persisted.
	updated.Env["C"] = "corrupted"
	newEnv["C"] = "also corrupted"

	again, err := svc.Get(ctx(), toolset.GetInput{ID: "gh"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Env["C"] != "3" {
		t.Fatalf("the persisted record was corrupted by a caller's later mutation: %v", again.Env)
	}
}

// --- the install-time check staying true after install ---------------------

// TestUpdateConfigLocksCommandAndBaseURLForASkillOwnedToolset is the defect
// this locks shut: VerifyManifest checks a skill-declared toolset's Command
// against permissions.exec and its BaseURL against permissions.network
// exactly once, at install (internal/domain/skill/verify.go). Before this,
// nothing stopped UpdateConfig from repointing either afterward — a toolset
// that passed the check at api.example.com could be walked to any host, or
// any binary, with the install-time promise never re-verified against it.
func TestUpdateConfigLocksCommandAndBaseURLForASkillOwnedToolset(t *testing.T) {
	svc := newService(t, withToolset(toolset.Toolset{
		ID: "gh", Skill: "github-tools", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Command: "gh-mcp-server",
	}))

	newCommand := "curl"
	_, err := svc.UpdateConfig(ctx(), toolset.UpdateConfigInput{ID: "gh", Command: &newCommand})
	if code := codeOf(t, err); code != "AOS_TOOLSET_CONNECTION_LOCKED" {
		t.Fatalf("changing Command: code = %q", code)
	}

	newURL := "https://attacker.example.com"
	_, err = svc.UpdateConfig(ctx(), toolset.UpdateConfigInput{ID: "gh", BaseURL: &newURL})
	if code := codeOf(t, err); code != "AOS_TOOLSET_CONNECTION_LOCKED" {
		t.Fatalf("changing BaseURL: code = %q", code)
	}

	// The toolset must be exactly as it was — a rejected UpdateConfig must
	// not have partially applied.
	again, err := svc.Get(ctx(), toolset.GetInput{ID: "gh"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Command != "gh-mcp-server" {
		t.Fatalf("Command changed despite the rejection: %q", again.Command)
	}
}

// TestUpdateConfigStillFreelyEditsWhatVerifyManifestNeverChecked proves the
// lock is exactly Command and BaseURL, not a blanket freeze of a skill-owned
// toolset: Description, Status and Env carry no install-time promise and stay
// editable.
func TestUpdateConfigStillFreelyEditsWhatVerifyManifestNeverChecked(t *testing.T) {
	svc := newService(t, withToolset(toolset.Toolset{
		ID: "gh", Skill: "github-tools", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Command: "gh-mcp-server",
	}))

	disabled := toolset.StatusDisabled
	desc := "reconfigured description"
	updated, err := svc.UpdateConfig(ctx(), toolset.UpdateConfigInput{
		ID: "gh", Status: &disabled, Description: &desc, Env: map[string]string{"A": "1"},
	})
	if err != nil {
		t.Fatalf("editing fields VerifyManifest never checked: %v", err)
	}
	if updated.Status != toolset.StatusDisabled || updated.Description != desc || updated.Env["A"] != "1" {
		t.Fatalf("update did not apply: %+v", updated)
	}
}

// TestUpdateConfigAllowsSettingCommandOrBaseURLToTheirCurrentValue proves the
// lock compares values, not presence: a caller resending the same Command
// alongside an unrelated field it does want to change is not refused for a
// field it never actually changed.
func TestUpdateConfigAllowsSettingCommandOrBaseURLToTheirCurrentValue(t *testing.T) {
	svc := newService(t, withToolset(toolset.Toolset{
		ID: "gh", Skill: "github-tools", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Command: "gh-mcp-server",
	}))

	sameCommand := "gh-mcp-server"
	if _, err := svc.UpdateConfig(ctx(), toolset.UpdateConfigInput{ID: "gh", Command: &sameCommand}); err != nil {
		t.Fatalf("resending the unchanged Command was refused: %v", err)
	}
}

// TestUpdateConfigLeavesAToolsetWithNoSkillUnrestricted proves the lock is
// conditioned on Skill: a toolset a human configured directly makes no
// manifest promise for UpdateConfig to keep, and behaves exactly as before
// this lock existed.
func TestUpdateConfigLeavesAToolsetWithNoSkillUnrestricted(t *testing.T) {
	svc := newService(t, withToolset(toolset.Toolset{
		ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Command: "gh-mcp-server",
	}))

	newCommand := "curl"
	updated, err := svc.UpdateConfig(ctx(), toolset.UpdateConfigInput{ID: "gh", Command: &newCommand})
	if err != nil {
		t.Fatalf("a toolset with no Skill must stay unrestricted: %v", err)
	}
	if updated.Command != "curl" {
		t.Fatalf("Command = %q, want curl", updated.Command)
	}
}

func TestDeleteIsIdempotentAndThenNotFound(t *testing.T) {
	svc := newService(t, withToolset(toolset.Toolset{ID: "gh", Type: toolset.MCPStdio}))

	if _, err := svc.Delete(ctx(), toolset.DeleteInput{ID: "gh"}); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if _, err := svc.Delete(ctx(), toolset.DeleteInput{ID: "gh"}); err != nil {
		t.Fatalf("deleting what is not there must succeed: %v", err)
	}

	_, err := svc.Get(ctx(), toolset.GetInput{ID: "gh"})
	if code := codeOf(t, err); code != "AOS_TOOLSET_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

// --- the remaining branches: validation, wrapped repository failures --------

func TestTypeAndStatusValid(t *testing.T) {
	for _, ty := range []toolset.Type{toolset.MCPStdio, toolset.MCPHTTP, toolset.RESTAPI, toolset.CLI, toolset.Custom} {
		if !ty.Valid() {
			t.Errorf("%q should be valid", ty)
		}
	}
	if toolset.Type("grpc").Valid() {
		t.Error(`"grpc" should not be valid`)
	}
	if !toolset.StatusEnabled.Valid() || !toolset.StatusDisabled.Valid() {
		t.Error("enabled and disabled must both be valid statuses")
	}
	if toolset.Status("paused").Valid() {
		t.Error(`"paused" should not be a valid status`)
	}
}

func TestToolSpecCloneDoesNotAliasInputSchema(t *testing.T) {
	spec := toolset.ToolSpec{
		Name:        "create_issue",
		InputSchema: map[string]any{"properties": map[string]any{"title": map[string]any{"type": "string"}}},
	}
	clone := spec.Clone()
	clone.InputSchema["properties"].(map[string]any)["title"].(map[string]any)["type"] = "corrupted"

	if spec.InputSchema["properties"].(map[string]any)["title"].(map[string]any)["type"] != "string" {
		t.Fatal("mutating the clone's schema corrupted the original")
	}
}

func TestCallRefusesAnIncompleteRequest(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Call(ctx(), toolset.CallInput{ID: "", Tool: "x"}); codeOf(t, err) != "AOS_TOOLSET_CALL_INCOMPLETE" {
		t.Fatalf("missing id: code = %q", codeOf(t, err))
	}
	if _, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: ""}); codeOf(t, err) != "AOS_TOOLSET_CALL_INCOMPLETE" {
		t.Fatalf("missing tool: code = %q", codeOf(t, err))
	}
}

func TestCallRefusesADisabledToolset(t *testing.T) {
	svc := newService(t, withToolset(toolset.Toolset{
		ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusDisabled,
	}))
	_, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "x"})
	if code := codeOf(t, err); code != "AOS_TOOLSET_DISABLED" {
		t.Fatalf("code = %q", code)
	}
}

// connectOKCallFails connects successfully but fails the tool call itself —
// the branch a broken connection test cannot reach.
type connectOKCallFails struct{ failingAdapter }

func (connectOKCallFails) Connect(context.Context, toolset.Toolset) error { return nil }

func TestCallFailedIsDistinctFromConnectFailed(t *testing.T) {
	svc := newService(t, withAdapter("gh", connectOKCallFails{}))
	_, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "x"})
	if code := codeOf(t, err); code != "AOS_TOOLSET_CALL_FAILED" {
		t.Fatalf("code = %q", code)
	}
}

// interpolatingAdapter records the Toolset it was connected with, so a test
// can assert that every field — not only Env — was interpolated.
type interpolatingAdapter struct {
	okAdapter
	got toolset.Toolset
}

func (a *interpolatingAdapter) Connect(_ context.Context, ts toolset.Toolset) error {
	a.got = ts
	return nil
}

func TestCallInterpolatesCommandArgsBaseURLAndHeadersTooNotOnlyEnv(t *testing.T) {
	adapter := &interpolatingAdapter{}
	e := envOf(map[string]string{"BIN": "true", "FLAG": "-x", "HOST": "example.com", "H": "hv"})
	svc := newService(t, withEnv(e), withAdapter("gh", adapter), withToolset(toolset.Toolset{
		ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Command: "${env.BIN}", Args: []string{"${env.FLAG}"},
		BaseURL: "https://${env.HOST}", Headers: map[string]string{"X": "${env.H}"},
	}))
	if _, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "x"}); err != nil {
		t.Fatal(err)
	}
	if adapter.got.Command != "true" || adapter.got.Args[0] != "-x" ||
		adapter.got.BaseURL != "https://example.com" || adapter.got.Headers["X"] != "hv" {
		t.Fatalf("not every field was interpolated: %+v", adapter.got)
	}
}

func TestCallFailsLoudWhenAHeaderPlaceholderIsMissing(t *testing.T) {
	svc := newService(t, withAdapter("gh", okAdapter{}), withToolset(toolset.Toolset{
		ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Headers: map[string]string{"X": "${env.MISSING}"},
	}))
	_, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "x"})
	if code := codeOf(t, err); code != "AOS_TOOLSET_ENV_MISSING" {
		t.Fatalf("code = %q", code)
	}
}

// erroringRepo fails every operation, so List/Update/Delete's wrapping of a
// repository failure — as opposed to a not-found — is exercised.
type erroringRepo struct{}

func (erroringRepo) Get(context.Context, collections.Key) (*toolset.Toolset, error) {
	return nil, errors.New("disk on fire")
}

func (erroringRepo) List(context.Context, collections.Query) ([]toolset.Toolset, error) {
	return nil, errors.New("disk on fire")
}

func (erroringRepo) Update(context.Context, *toolset.Toolset, collections.Version) error {
	return errors.New("disk on fire")
}

func (erroringRepo) Delete(context.Context, collections.Key) error {
	return errors.New("disk on fire")
}

func TestRepositoryFailuresAreWrapped(t *testing.T) {
	svc := toolset.NewService(toolset.Deps{
		Repo: erroringRepo{}, Adapters: toolset.Adapters{}, Activities: &fakeActivities{},
		Env: envOf(map[string]string{}), Clock: &fixedClock{at: time.Now()},
	})

	if _, err := svc.List(ctx()); codeOf(t, err) != "AOS_TOOLSET_READ_FAILED" {
		t.Fatalf("List: code = %q", codeOf(t, err))
	}
	if _, err := svc.Delete(ctx(), toolset.DeleteInput{ID: "gh"}); codeOf(t, err) != "AOS_TOOLSET_WRITE_FAILED" {
		t.Fatalf("Delete: code = %q", codeOf(t, err))
	}
}

// failingActivities always errors on Publish, so Call's audit failure — logged
// rather than allowed to fail the call that already happened — is exercised.
type failingActivities struct{}

func (failingActivities) Publish(context.Context, activity.PublishInput) (*activity.Activity, error) {
	return nil, errors.New("log is full")
}

func TestAnAuditPublishFailureDoesNotFailTheCallItRecords(t *testing.T) {
	svc := toolset.NewService(toolset.Deps{
		Repo:       reposeeded(t),
		Adapters:   toolset.Adapters{toolset.MCPStdio: func() toolset.Adapter { return okAdapter{} }},
		Activities: failingActivities{},
		Env:        envOf(map[string]string{}),
		Clock:      &fixedClock{at: time.Now()},
	})
	if _, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "x"}); err != nil {
		t.Fatalf("a broken audit sink must not fail the call that already succeeded: %v", err)
	}
}

func reposeeded(t *testing.T) toolset.Repository {
	t.Helper()
	repo := fakes.NewRepo[toolset.Toolset]("toolsets")
	ts := toolset.Toolset{ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled}
	if err := repo.Create(context.Background(), &ts); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestUpdateConfigChangesEveryScalarField(t *testing.T) {
	svc := newService(t, withToolset(toolset.Toolset{ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled}))

	desc, cmd, base := "new description", "new-command", "https://new.example.com"
	disabled := toolset.StatusDisabled
	updated, err := svc.UpdateConfig(ctx(), toolset.UpdateConfigInput{
		ID: "gh", Description: &desc, Status: &disabled, Command: &cmd, BaseURL: &base,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != desc || updated.Status != disabled || updated.Command != cmd || updated.BaseURL != base {
		t.Fatalf("not every field was applied: %+v", updated)
	}
}

// --- test doubles and helpers ------------------------------------------------

func ctx() context.Context { return context.Background() }

func envOf(m map[string]string) *env.Resolver { return env.New(env.Map(m)) }

func codeOf(t *testing.T, err error) string {
	t.Helper()
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err is not *apperr.Error: %v", err)
	}
	return e.Code
}

type okAdapter struct{}

func (okAdapter) Connect(context.Context, toolset.Toolset) error { return nil }
func (okAdapter) ListTools(context.Context) ([]toolset.ToolSpec, error) {
	return []toolset.ToolSpec{{Name: "create_issue"}}, nil
}

func (okAdapter) Call(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"ok":true}`), nil
}
func (okAdapter) Close() error { return nil }

type failingAdapter struct{}

func (failingAdapter) Connect(context.Context, toolset.Toolset) error {
	return errors.New("connection refused")
}

func (failingAdapter) ListTools(context.Context) ([]toolset.ToolSpec, error) {
	return nil, errors.New("not connected")
}

func (failingAdapter) Call(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return nil, errors.New("not connected")
}
func (failingAdapter) Close() error { return nil }

// fakeActivities records every publish without a store behind it, which is
// all a test needs to inspect what Call chose to record.
type fakeActivities struct {
	published []activity.PublishInput
}

func (f *fakeActivities) Publish(_ context.Context, in activity.PublishInput) (*activity.Activity, error) {
	f.published = append(f.published, in)
	return &activity.Activity{ID: fmt.Sprintf("a-%d", len(f.published))}, nil
}

// fixedClock advances by a millisecond on every read, so Call's duration is
// deterministic and never zero without depending on wall-clock speed.
type fixedClock struct {
	at time.Time
}

func (c *fixedClock) Now() time.Time {
	c.at = c.at.Add(time.Millisecond)
	return c.at
}

// serviceBuild collects what newService assembles a *toolset.Service from.
// Defaults are seeded so the tests the brief describes — which reference
// toolset "gh" and "cli-thing" without always configuring them explicitly —
// have something to resolve against.
type serviceBuild struct {
	toolsets   map[string]toolset.Toolset
	adapterIDs map[string]toolset.Adapter
	activities *fakeActivities
	env        toolset.EnvResolver
}

type serviceOption func(*serviceBuild)

// withToolset adds or replaces a toolset in the fixture, keyed by its ID.
func withToolset(ts toolset.Toolset) serviceOption {
	return func(b *serviceBuild) { b.toolsets[ts.ID] = ts }
}

// withAdapter registers a as the adapter for whatever Type the toolset with
// this id has. It is resolved by ID rather than by Type directly so a test
// reads as "this toolset talks to this fake", the pairing that matters to a
// reader — Adapters is keyed by Type underneath because that is what
// Service.Call actually dispatches on.
func withAdapter(id string, a toolset.Adapter) serviceOption {
	return func(b *serviceBuild) { b.adapterIDs[id] = a }
}

func withActivities(acts *fakeActivities) serviceOption {
	return func(b *serviceBuild) { b.activities = acts }
}

func withEnv(e toolset.EnvResolver) serviceOption {
	return func(b *serviceBuild) { b.env = e }
}

func defaultToolsets() map[string]toolset.Toolset {
	return map[string]toolset.Toolset{
		"gh":        {ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled, Command: "true"},
		"cli-thing": {ID: "cli-thing", Type: toolset.CLI, Status: toolset.StatusEnabled},
	}
}

func newService(t *testing.T, opts ...serviceOption) *toolset.Service {
	t.Helper()
	b := &serviceBuild{
		toolsets:   defaultToolsets(),
		adapterIDs: map[string]toolset.Adapter{},
	}
	for _, o := range opts {
		o(b)
	}

	repo := fakes.NewRepo[toolset.Toolset]("toolsets")
	for _, ts := range b.toolsets {
		v := ts
		if err := repo.Create(context.Background(), &v); err != nil {
			t.Fatal(err)
		}
	}

	adapters := toolset.Adapters{}
	for id, a := range b.adapterIDs {
		ts, ok := b.toolsets[id]
		if !ok {
			t.Fatalf("withAdapter(%q, ...): no toolset with that id was configured", id)
		}
		fixed := a
		adapters[ts.Type] = func() toolset.Adapter { return fixed }
	}

	acts := b.activities
	if acts == nil {
		acts = &fakeActivities{}
	}
	envResolver := b.env
	if envResolver == nil {
		envResolver = envOf(map[string]string{})
	}

	return toolset.NewService(toolset.Deps{
		Repo:       repo,
		Adapters:   adapters,
		Activities: acts,
		Env:        envResolver,
		Clock:      &fixedClock{at: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)},
	})
}
