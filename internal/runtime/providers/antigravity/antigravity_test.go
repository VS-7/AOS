package antigravity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
)

// server records what the adapter sent and answers what a test wants it to.
type server struct {
	t        *testing.T
	bodies   map[string]json.RawMessage
	headers  http.Header
	answers  map[string]string
	statuses map[string]int
	hits     map[string]int
}

func newServer(t *testing.T) (*server, string) {
	t.Helper()
	s := &server{
		t:        t,
		bodies:   map[string]json.RawMessage{},
		answers:  map[string]string{},
		statuses: map[string]int{},
		hits:     map[string]int{},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path
		if i := strings.LastIndex(method, ":"); i >= 0 {
			method = method[i+1:]
		}
		raw, _ := io.ReadAll(r.Body)
		s.bodies[method] = raw
		s.headers = r.Header.Clone()
		s.hits[method]++
		if code := s.statuses[method]; code != 0 {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"error":{"message":"no"}}`))
			return
		}
		answer, ok := s.answers[method]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(ts.Close)
	return s, ts.URL + "/v1internal"
}

func (s *server) sent(method string) map[string]any {
	s.t.Helper()
	var out map[string]any
	if err := json.Unmarshal(s.bodies[method], &out); err != nil {
		s.t.Fatalf("%s body did not parse: %v — %s", method, err, s.bodies[method])
	}
	return out
}

// home writes a credential file the adapter can read, so that Auth succeeds
// without going anywhere near a real Google endpoint.
func home(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	// An expiry far in the future, so no refresh is attempted.
	body := `{"token":{"access_token":"at-1","token_type":"Bearer","refresh_token":"rt-1","expiry":"2099-01-01T00:00:00Z"},"auth_method":"consumer"}`
	if err := os.WriteFile(filepath.Join(path, "antigravity-oauth-token"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

const catalogueAnswer = `{"models":{
  "gemini-3.6-flash-low":{"displayName":"Gemini 3.6 Flash (Low)","supportsThinking":true,"thinkingBudget":1000,"minThinkingBudget":32,"maxTokens":1048576,"maxOutputTokens":65536,"quotaInfo":{"remainingFraction":1,"resetTime":"2099-01-01T00:00:00Z"}},
  "gemini-3.6-flash-high":{"displayName":"Gemini 3.6 Flash (High)","supportsThinking":true,"thinkingBudget":-1,"minThinkingBudget":32},
  "chat_20706":{"maxTokens":16384},
  "hidden-one":{"displayName":"Hidden","isInternal":true}}}`

const projectAnswer = `{"cloudaicompanionProject":"trim-involution-8n4q7","currentTier":{"id":"free-tier","name":"Antigravity"}}`

func build(t *testing.T, base string) *Provider {
	t.Helper()
	p := New(providers.Config{BaseURL: base, Home: home(t)})
	// The pacing floor is real time, and no test should spend it.
	p.guard.sleep = func(context.Context, time.Duration) error { return nil }
	return p
}

func ask(model string) agentloop.Request {
	return agentloop.Request{
		Model:    model,
		Messages: []agentloop.Message{{Role: agentloop.RoleUser, Text: "hi"}},
	}
}

// TestTheEnvelopeCarriesWhatTheServiceExpects. The body is not a Gemini
// request: it is a Gemini request nested under "request", beside the model,
// the project and the surface name.
func TestTheEnvelopeCarriesWhatTheServiceExpects(t *testing.T) {
	s, base := newServer(t)
	s.answers["fetchAvailableModels"] = catalogueAnswer
	s.answers["loadCodeAssist"] = projectAnswer
	s.answers["generateContent"] = `{"response":{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7},"modelVersion":"gemini-3.6-flash"},"traceId":"t1"}`

	p := build(t, base)
	got, err := p.Generate(context.Background(), agentloop.Request{
		Model:        "gemini-3.6-flash-low",
		Instructions: "be terse",
		Reasoning:    agentloop.ReasoningMedium,
		Messages:     []agentloop.Message{{Role: agentloop.RoleUser, Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Message.Text != "hello" {
		t.Errorf("text = %q", got.Message.Text)
	}
	if got.Model != "gemini-3.6-flash" {
		t.Errorf("model = %q, want the version the service reported", got.Model)
	}
	if got.Usage.Total != 7 {
		t.Errorf("usage = %+v", got.Usage)
	}

	body := s.sent("generateContent")
	if body["model"] != "gemini-3.6-flash-low" {
		t.Errorf("model = %v", body["model"])
	}
	if body["project"] != "trim-involution-8n4q7" {
		t.Errorf("project = %v, want the one loadCodeAssist named", body["project"])
	}
	if body["userAgent"] != "antigravity" {
		t.Errorf("userAgent = %v", body["userAgent"])
	}
	inner, ok := body["request"].(map[string]any)
	if !ok {
		t.Fatalf("request = %T, want the nested Gemini body", body["request"])
	}
	if _, has := inner["contents"]; !has {
		t.Error("the nested body carries no contents")
	}
	if _, has := inner["systemInstruction"]; !has {
		t.Error("the instructions were dropped")
	}
	if s.headers.Get("Client-Metadata") == "" {
		t.Error("Client-Metadata was not sent")
	}
	if ua := s.headers.Get("User-Agent"); !strings.HasPrefix(ua, "antigravity/") {
		t.Errorf("User-Agent = %q", ua)
	}
	if auth := s.headers.Get("Authorization"); auth != "Bearer at-1" {
		t.Errorf("Authorization = %q, want the token from the credential file", auth)
	}
}

// TestAProjectTheServiceDidNotNameIsNotInvented. The field is optional, and
// sending an empty one would be worse than sending none.
func TestAProjectTheServiceDidNotNameIsNotInvented(t *testing.T) {
	s, base := newServer(t)
	s.answers["fetchAvailableModels"] = catalogueAnswer
	s.answers["loadCodeAssist"] = `{"currentTier":{"id":"free-tier"}}`
	s.answers["generateContent"] = `{"response":{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}}`

	p := build(t, base)
	if _, err := p.Generate(context.Background(), ask("gemini-3.6-flash-low")); err != nil {
		t.Fatal(err)
	}
	if _, present := s.sent("generateContent")["project"]; present {
		t.Error("an empty project was sent")
	}
}

// TestTheProjectIsLookedUpOnceRatherThanPerCall.
func TestTheProjectIsLookedUpOnceRatherThanPerCall(t *testing.T) {
	s, base := newServer(t)
	s.answers["fetchAvailableModels"] = catalogueAnswer
	s.answers["loadCodeAssist"] = projectAnswer
	s.answers["generateContent"] = `{"response":{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}}`

	p := build(t, base)
	for range 3 {
		if _, err := p.Generate(context.Background(), ask("gemini-3.6-flash-low")); err != nil {
			t.Fatal(err)
		}
	}
	if s.hits["loadCodeAssist"] != 1 {
		t.Errorf("loadCodeAssist called %d times, want 1", s.hits["loadCodeAssist"])
	}
	if s.hits["fetchAvailableModels"] != 1 {
		t.Errorf("fetchAvailableModels called %d times, want 1 within the TTL", s.hits["fetchAvailableModels"])
	}
}

// TestACallPairsToolResultsByTheIdTheServiceGave, which is the one place this
// wire format differs from the public Gemini API in a way that matters.
func TestACallPairsToolResultsByTheIdTheServiceGave(t *testing.T) {
	s, base := newServer(t)
	s.answers["fetchAvailableModels"] = catalogueAnswer
	s.answers["loadCodeAssist"] = projectAnswer
	s.answers["generateContent"] = `{"response":{"candidates":[{"content":{"parts":[
		{"thoughtSignature":"sig-9","functionCall":{"name":"Read","args":{"path":"README.md"},"id":"call_42"}}]},
		"finishReason":"STOP"}]}}`

	p := build(t, base)
	got, err := p.Generate(context.Background(), agentloop.Request{
		Model: "gemini-3.6-flash-low",
		Messages: []agentloop.Message{
			{Role: agentloop.RoleUser, Text: "what does the readme say"},
			{Role: agentloop.RoleAssistant, ToolCalls: []agentloop.ToolCall{
				{ID: "call_7", Name: "Read", Input: json.RawMessage(`{"path":"README.md"}`), Signature: "sig-1"},
			}},
			{Role: agentloop.RoleTool, CallID: "call_7", Name: "Read", Result: json.RawMessage(`{"content":"hello"}`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(got.ToolCalls))
	}
	if got.ToolCalls[0].ID != "call_42" {
		t.Errorf("id = %q, want the service's own", got.ToolCalls[0].ID)
	}
	if got.ToolCalls[0].Signature != "sig-9" {
		t.Errorf("signature = %q; losing it fails the next request", got.ToolCalls[0].Signature)
	}
	if got.StopReason != agentloop.StopToolCalls {
		t.Errorf("stop = %q", got.StopReason)
	}

	// And the conversation it sent back carried both ids.
	inner, _ := s.sent("generateContent")["request"].(map[string]any)
	rendered, _ := json.Marshal(inner["contents"])
	for _, want := range []string{`"id":"call_7"`, `"thoughtSignature":"sig-1"`, `"functionResponse"`} {
		if !strings.Contains(string(rendered), want) {
			t.Errorf("contents missing %s: %s", want, rendered)
		}
	}
}

// TestACallWithNoIdStillPairs. A model served through this endpoint may answer
// in the public API's shape, which names no call.
func TestACallWithNoIdStillPairs(t *testing.T) {
	out := translate(mustGenerated(t, `{"candidates":[{"content":{"parts":[
		{"functionCall":{"name":"Read","args":{}}},
		{"functionCall":{"name":"Read","args":{}}}]}}]}`), "m")
	if len(out.ToolCalls) != 2 {
		t.Fatalf("calls = %d", len(out.ToolCalls))
	}
	if out.ToolCalls[0].ID == out.ToolCalls[1].ID {
		t.Errorf("two calls share the id %q", out.ToolCalls[0].ID)
	}
}

// TestTheStreamIsTheSameAnswerInPieces, thoughts and text kept apart.
func TestTheStreamIsTheSameAnswerInPieces(t *testing.T) {
	s, base := newServer(t)
	s.answers["fetchAvailableModels"] = catalogueAnswer
	s.answers["loadCodeAssist"] = projectAnswer
	s.answers["streamGenerateContent"] = strings.Join([]string{
		`data: {"response":{"candidates":[{"content":{"parts":[{"thought":true,"text":"weighing it"}]}}]}}`,
		"",
		`data: {"response":{"candidates":[{"content":{"parts":[{"text":"three hundred "}]}}]}}`,
		"",
		`data: {"response":{"candidates":[{"content":{"parts":[{"text":"and ninety one"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"totalTokenCount":42},"modelVersion":"gemini-3.6-flash"}}`,
		"", "",
	}, "\n")

	p := build(t, base)
	st, err := p.Stream(context.Background(), ask("gemini-3.6-flash-low"))
	if err != nil {
		t.Fatal(err)
	}
	var text, thinking strings.Builder
	for {
		chunk, err := st.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(chunk.Text)
		thinking.WriteString(chunk.Reasoning)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if text.String() != "three hundred and ninety one" {
		t.Errorf("streamed text = %q", text.String())
	}
	if thinking.String() != "weighing it" {
		t.Errorf("streamed reasoning = %q", thinking.String())
	}
	whole := st.Response()
	if whole.Message.Text != "three hundred and ninety one" {
		t.Errorf("assembled text = %q", whole.Message.Text)
	}
	if whole.Usage.Total != 42 {
		t.Errorf("assembled usage = %+v", whole.Usage)
	}
	if whole.StopReason != agentloop.StopEnd {
		t.Errorf("stop = %q", whole.StopReason)
	}
}

// TestReasoningIsRenderedInsideTheModelsOwnBounds. A budget outside them is a
// 400 and a lost turn, and the bounds differ per model.
func TestReasoningIsRenderedInsideTheModelsOwnBounds(t *testing.T) {
	fixed := modelInfo{Thinking: true, Budget: 1000, MinBudget: 32}
	dynamic := modelInfo{Thinking: true, Budget: -1, MinBudget: 32}

	cases := []struct {
		name  string
		info  modelInfo
		level agentloop.ReasoningLevel
		want  int
	}{
		{"none clamps to the floor, never zero", fixed, agentloop.ReasoningNone, 32},
		{"low is a quarter of the default", fixed, agentloop.ReasoningLow, 250},
		{"medium is the model's own default", fixed, agentloop.ReasoningMedium, 1000},
		{"high lets the model decide", fixed, agentloop.ReasoningHigh, -1},
		{"a dynamic model floors at its minimum", dynamic, agentloop.ReasoningNone, 32},
		{"a dynamic model stays dynamic", dynamic, agentloop.ReasoningMedium, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := budgetFor(c.info, c.level)
			if !ok {
				t.Fatal("no budget was produced")
			}
			if got != c.want {
				t.Errorf("budget = %d, want %d", got, c.want)
			}
			if got != -1 && got < c.info.MinBudget {
				t.Errorf("budget %d is below the model's floor %d", got, c.info.MinBudget)
			}
			if got > budgetCeiling {
				t.Errorf("budget %d is above the API's ceiling", got)
			}
		})
	}
}

// TestAModelTheCatalogueDoesNotDescribeGetsNoGenerationConfig. Guessing a
// budget for an unknown model is how a whole turn is lost to a 400.
func TestAModelTheCatalogueDoesNotDescribeGetsNoGenerationConfig(t *testing.T) {
	if cfg := generationConfig(modelInfo{}, false, agentloop.ReasoningHigh); cfg != nil {
		t.Errorf("an unknown model produced %v", cfg)
	}
	if cfg := generationConfig(modelInfo{Thinking: false}, true, agentloop.ReasoningHigh); cfg != nil {
		t.Errorf("a model that does not think produced %v", cfg)
	}
	cfg := generationConfig(modelInfo{Thinking: true, Budget: 1000, MinBudget: 32}, true, agentloop.ReasoningMedium)
	thinking, _ := cfg["thinkingConfig"].(map[string]any)
	if thinking["includeThoughts"] != true {
		t.Errorf("thoughts were not asked for: %v", cfg)
	}
}

// TestTheCatalogueOffersOnlyWhatAPersonCanPick. It carries routing aliases and
// the models behind inline completion, and a picker must show neither.
func TestTheCatalogueOffersOnlyWhatAPersonCanPick(t *testing.T) {
	s, base := newServer(t)
	s.answers["fetchAvailableModels"] = catalogueAnswer

	p := build(t, base)
	found, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(found))
	for _, m := range found {
		ids = append(ids, m.ID)
	}
	got := strings.Join(ids, ",")
	if got != "gemini-3.6-flash-high,gemini-3.6-flash-low" {
		t.Errorf("offered %q; the nameless and the internal must not appear", got)
	}
	for _, m := range found {
		if m.Name == "" {
			t.Errorf("%s has no display name", m.ID)
		}
	}
}

// TestAnExhaustedAllowanceIsRefusedBeforeItIsSpent, and only on a number that
// was just re-read.
func TestAnExhaustedAllowanceIsRefusedBeforeItIsSpent(t *testing.T) {
	s, base := newServer(t)
	s.answers["fetchAvailableModels"] = `{"models":{"gemini-3.6-flash-low":{"displayName":"Low","supportsThinking":true,"thinkingBudget":1000,"minThinkingBudget":32,"quotaInfo":{"remainingFraction":0,"resetTime":"2099-01-01T00:00:00Z"}}}}`
	s.answers["loadCodeAssist"] = projectAnswer
	s.answers["generateContent"] = `{"response":{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}}`

	p := build(t, base)
	_, err := p.Generate(context.Background(), ask("gemini-3.6-flash-low"))
	if err == nil {
		t.Fatal("an exhausted allowance was spent anyway")
	}
	if !strings.Contains(err.Error(), "ANTIGRAVITY_QUOTA_EXHAUSTED") {
		t.Fatalf("err = %v", err)
	}
	if s.hits["generateContent"] != 0 {
		t.Error("the request was sent before the allowance was checked")
	}
	// It re-read before refusing: once to populate, once to confirm.
	if s.hits["fetchAvailableModels"] < 2 {
		t.Errorf("catalogue read %d times; a refusal must not rest on a cached zero", s.hits["fetchAvailableModels"])
	}
}

// TestARefusalStopsTheAdapterRatherThanStartingAnArgument. This is the whole
// account-safety argument in one test: a 429 is not retried, and the next call
// does not even leave the process.
func TestARefusalStopsTheAdapterRatherThanStartingAnArgument(t *testing.T) {
	s, base := newServer(t)
	s.answers["fetchAvailableModels"] = catalogueAnswer
	s.answers["loadCodeAssist"] = projectAnswer
	s.statuses["generateContent"] = http.StatusTooManyRequests

	p := build(t, base)
	if _, err := p.Generate(context.Background(), ask("gemini-3.6-flash-low")); err == nil {
		t.Fatal("a 429 was not reported")
	}
	if s.hits["generateContent"] != 1 {
		t.Errorf("the 429 was retried %d times", s.hits["generateContent"]-1)
	}

	_, err := p.Generate(context.Background(), ask("gemini-3.6-flash-low"))
	if err == nil || !strings.Contains(err.Error(), "ANTIGRAVITY_BACKING_OFF") {
		t.Fatalf("the second call was sent anyway: %v", err)
	}
	if s.hits["generateContent"] != 1 {
		t.Errorf("generateContent was called %d times after a refusal", s.hits["generateContent"])
	}
}

// TestTheDoorReopens, and a clean answer forgives the history.
func TestTheDoorReopens(t *testing.T) {
	now := time.Now()
	g := newGuard(func() time.Time { return now })
	g.sleep = func(context.Context, time.Duration) error { return nil }

	g.observe(refusal(http.StatusTooManyRequests))
	if err := g.enter(context.Background()); err == nil {
		t.Fatal("the door stayed open after a refusal")
	}
	now = now.Add(firstCooldown + time.Second)
	if err := g.enter(context.Background()); err != nil {
		t.Fatalf("the door did not reopen: %v", err)
	}

	// Two consecutive refusals cost more than one.
	g.observe(refusal(http.StatusTooManyRequests))
	g.observe(refusal(http.StatusTooManyRequests))
	second := g.blockedUntil
	if !second.After(now.Add(firstCooldown)) {
		t.Error("consecutive refusals did not lengthen the window")
	}
	g.observe(nil)
	if !g.blockedUntil.IsZero() || g.refusals != 0 {
		t.Error("a clean answer did not clear the back-off")
	}
}

// TestABadRequestDoesNotShutTheDoor. A 400 is this build's fault, and hiding
// it behind a cooldown would make it look like a rate limit.
func TestABadRequestDoesNotShutTheDoor(t *testing.T) {
	now := time.Now()
	g := newGuard(func() time.Time { return now })
	g.observe(refusal(http.StatusBadRequest))
	if err := g.enter(context.Background()); err != nil {
		t.Fatalf("a 400 shut the door: %v", err)
	}
}

// TestCallsArePacedRatherThanBurst.
func TestCallsArePacedRatherThanBurst(t *testing.T) {
	now := time.Now()
	g := newGuard(func() time.Time { return now })
	var waited time.Duration
	g.sleep = func(_ context.Context, d time.Duration) error { waited += d; return nil }

	for range 3 {
		if err := g.enter(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if waited < 2*minInterval {
		t.Errorf("three back-to-back calls waited %s, want at least %s", waited, 2*minInterval)
	}
}

// TestTheCredentialFileIsReadInBothShapesTheClientsWrite.
func TestTheCredentialFileIsReadInBothShapesTheClientsWrite(t *testing.T) {
	shapes := map[string]string{
		"nested": `{"token":{"access_token":"a","refresh_token":"r","expiry":"2030-01-02T03:04:05Z"},"auth_method":"consumer"}`,
		"flat":   `{"access_token":"a","refresh_token":"r","expiry":"2030-01-02T03:04:05Z"}`,
	}
	for name, raw := range shapes {
		t.Run(name, func(t *testing.T) {
			got, err := parseCredential([]byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			if got.AccessToken != "a" || got.RefreshToken != "r" {
				t.Errorf("credentials = %+v", got)
			}
			if got.ExpiresAt.Year() != 2030 {
				t.Errorf("expiry = %v", got.ExpiresAt)
			}
		})
	}
}

// TestSigningInIsTheOfficialClientsJob. With no file anywhere, this says so
// and names the command, rather than opening a browser of its own.
func TestSigningInIsTheOfficialClientsJob(t *testing.T) {
	p := New(providers.Config{BaseURL: "http://127.0.0.1:1/v1internal", Home: t.TempDir()})
	_, err := p.Generate(context.Background(), agentloop.Request{Model: "m"})
	if err == nil {
		t.Fatal("a call was made with no credential")
	}
	if !strings.Contains(err.Error(), "OAUTH_FILE_MISSING") && !strings.Contains(err.Error(), "ANTIGRAVITY_NOT_SIGNED_IN") {
		t.Fatalf("err = %v", err)
	}
}

// TestThePlatformIsOneTheServiceKnows. It answers 400 naming the field for
// anything outside its enum.
func TestThePlatformIsOneTheServiceKnows(t *testing.T) {
	known := map[string]bool{
		"DARWIN_ARM64": true, "DARWIN_AMD64": true, "LINUX_AMD64": true,
		"LINUX_ARM64": true, "WINDOWS_AMD64": true, "PLATFORM_UNSPECIFIED": true,
	}
	if !known[platform()] {
		t.Errorf("platform() = %q, which this service rejects", platform())
	}
}

// TestAnApiKeyIsNotSentAsABearerToken. Every other provider treats APIKey as
// the credential; pasting one here must not put it on the wire.
func TestAnApiKeyIsNotSentAsABearerToken(t *testing.T) {
	s, base := newServer(t)
	s.answers["fetchAvailableModels"] = catalogueAnswer
	s.answers["loadCodeAssist"] = projectAnswer
	s.answers["generateContent"] = `{"response":{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}}`

	p := New(providers.Config{BaseURL: base, Home: home(t), APIKey: "sk-do-not-send"})
	p.guard.sleep = func(context.Context, time.Duration) error { return nil }
	if _, err := p.Generate(context.Background(), ask("gemini-3.6-flash-low")); err != nil {
		t.Fatal(err)
	}
	for name, values := range s.headers {
		for _, v := range values {
			if strings.Contains(v, "sk-do-not-send") {
				t.Fatalf("the API key was sent in %s", name)
			}
		}
	}
}

func mustGenerated(t *testing.T, raw string) generated {
	t.Helper()
	var g generated
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// refusal builds the error shape providers.Client produces for a status, which
// is what guard.observe reads: the transport status is the gateway one, and
// the provider's own code is an issue beside it.
func refusal(status int) error {
	return apperr.New("PROVIDER_REFUSED").
		Causer("providers.antigravity").
		Issue("status", status).
		Status(apperr.StatusBadGateway)
}

// TestRenewalWithoutTheOAuthClientSaysWhichVariablesAreMissing.
//
// The client id and secret are Google's, distributed inside the official CLI,
// and this repository deliberately does not carry them — committing another
// party's credential here would republish it, with that OAuth client's
// revocation as the blast radius for everyone using the real CLI. GitHub's
// push protection refuses it outright.
//
// What matters is that their absence is a sentence somebody can act on rather
// than a failed HTTP call: renewal is the only thing that needs them, so a
// token that has not expired never reaches this, and one that has must say
// exactly what to set.
func TestRenewalWithoutTheOAuthClientSaysWhichVariablesAreMissing(t *testing.T) {
	t.Setenv("AOS_ANTIGRAVITY_CLIENT_ID", "")
	t.Setenv("AOS_ANTIGRAVITY_CLIENT_SECRET", "")

	_, err := renew(context.Background(), "a-refresh-token")
	if err == nil {
		t.Fatal("a renewal with no OAuth client reported success")
	}

	fault, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err = %v, want a domain error", err)
	}
	if !strings.Contains(fault.Code, "ANTIGRAVITY_NO_OAUTH_CLIENT") {
		t.Errorf("code = %q", fault.Code)
	}

	// The variable names travel as structured data and in the call to action,
	// not in the message — which is the half a person actually reads.
	named := fmt.Sprint(fault.Issues["variables"]) + " " + fmt.Sprint(fault.Actions)
	for _, want := range []string{"AOS_ANTIGRAVITY_CLIENT_ID", "AOS_ANTIGRAVITY_CLIENT_SECRET"} {
		if !strings.Contains(named, want) {
			t.Errorf("the refusal does not name %s: issues=%v actions=%v", want, fault.Issues, fault.Actions)
		}
	}
}

// TestOneHalfOfThePairIsNotEnough. A half-set pair is the likelier mistake —
// somebody copies the id and stops — and it must read the same as none at all
// rather than reaching the token endpoint with a blank secret.
func TestOneHalfOfThePairIsNotEnough(t *testing.T) {
	t.Setenv("AOS_ANTIGRAVITY_CLIENT_ID", "an-id.apps.example.test")
	t.Setenv("AOS_ANTIGRAVITY_CLIENT_SECRET", "   ")

	if _, _, ok := oauthClient(); ok {
		t.Fatal("a blank secret was accepted as a configured client")
	}

	t.Setenv("AOS_ANTIGRAVITY_CLIENT_SECRET", "a-secret")
	id, secret, ok := oauthClient()
	if !ok || id != "an-id.apps.example.test" || secret != "a-secret" {
		t.Errorf("oauthClient() = %q, %q, %v", id, secret, ok)
	}
}
