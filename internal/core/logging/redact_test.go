package logging_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/logging"
)

const knownToken = "aos_0123456789abcdef0123456789abcdef"

// What auth.Service.mintToken actually produces: "aos_" and base64url, not
// hex. Every existing assertion in this file used the hex shape above, so the
// pattern could be — and was — wrong about the only token this product mints
// and no test noticed.
const mintedToken = "aos_nEoEqYKgMwSTh8YGuR-ckTMYFmboQFv4kz2T2ClCPls"

func newBuffered(t *testing.T) (*bytes.Buffer, context.Context) {
	t.Helper()
	buf := &bytes.Buffer{}
	l := logging.New(logging.Config{Level: "debug", Format: "json", Output: buf})
	return buf, logging.Into(context.Background(), l)
}

func TestSecretKeysAreDropped(t *testing.T) {
	buf, ctx := newBuffered(t)
	logging.FromContext(ctx).Info("configured",
		"api_token", knownToken,
		"password", "hunter2",
		"Authorization", "Bearer "+knownToken,
	)
	out := buf.String()
	for _, leaked := range []string{knownToken, "hunter2"} {
		if strings.Contains(out, leaked) {
			t.Errorf("log leaked %q: %s", leaked, out)
		}
	}
	if !strings.Contains(out, logging.Redacted) {
		t.Errorf("nothing was redacted: %s", out)
	}
}

func TestTokenShapeIsRedactedUnderAnyKey(t *testing.T) {
	buf, ctx := newBuffered(t)
	logging.FromContext(ctx).Info("calling provider",
		"argument", "--header=Bearer "+knownToken,
		"provider_key", "sk-abcdefghijklmnopqrstuvwxyz",
	)
	out := buf.String()
	if strings.Contains(out, knownToken) || strings.Contains(out, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("token-shaped value survived redaction: %s", out)
	}
}

func TestMessageIsRedactedToo(t *testing.T) {
	buf, ctx := newBuffered(t)
	logging.FromContext(ctx).Info("registered mcp server with token " + knownToken)
	if strings.Contains(buf.String(), knownToken) {
		t.Errorf("message leaked the token: %s", buf.String())
	}
}

func TestAmbientIdentityCorrelatesEveryLine(t *testing.T) {
	buf, ctx := newBuffered(t)
	ctx = identity.With(ctx, identity.Identity{
		RequestID: "req-1", WorkspaceID: "ws-1", AgentID: "orchestrator",
	})
	logging.FromContext(ctx).Info("first")
	logging.Component(ctx, "collections").Info("second")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for _, l := range lines {
		for _, want := range []string{`"request_id":"req-1"`, `"workspace":"ws-1"`, `"agent":"orchestrator"`} {
			if !strings.Contains(l, want) {
				t.Errorf("line missing %s: %s", want, l)
			}
		}
	}
	if !strings.Contains(lines[1], `"component":"collections"`) {
		t.Errorf("component tag missing: %s", lines[1])
	}
}

func TestGroupedAttributesAreRedacted(t *testing.T) {
	buf, ctx := newBuffered(t)
	logging.FromContext(ctx).WithGroup("provider").With("name", "openai").
		Info("configured", "api_key", knownToken)
	out := buf.String()
	if strings.Contains(out, knownToken) {
		t.Errorf("a grouped attribute leaked: %s", out)
	}
	if !strings.Contains(out, "openai") {
		t.Errorf("the group lost a non-secret attribute: %s", out)
	}
}

func TestNestedGroupValueIsRedacted(t *testing.T) {
	buf, ctx := newBuffered(t)
	logging.FromContext(ctx).Info("boot",
		slog.Group("security", slog.String("apiToken", knownToken), slog.Bool("enabled", true)))
	out := buf.String()
	if strings.Contains(out, knownToken) {
		t.Errorf("a nested group leaked: %s", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("the nested group lost a non-secret field: %s", out)
	}
}

func TestFormatSelection(t *testing.T) {
	cases := []struct {
		name     string
		cfg      logging.Config
		wantJSON bool
	}{
		{"explicit json", logging.Config{Format: "json"}, true},
		{"explicit text", logging.Config{Format: "text"}, false},
		{"auto on a tty", logging.Config{Format: "auto", TTY: true}, false},
		{"auto when piped", logging.Config{Format: "auto"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			cfg := c.cfg
			cfg.Output = buf
			logging.New(cfg).Info("hello")
			isJSON := strings.HasPrefix(strings.TrimSpace(buf.String()), "{")
			if isJSON != c.wantJSON {
				t.Fatalf("json = %v, want %v: %s", isJSON, c.wantJSON, buf.String())
			}
		})
	}
}

func TestLevelFiltering(t *testing.T) {
	for _, c := range []struct {
		level      string
		debugShown bool
		warnShown  bool
	}{
		{"debug", true, true},
		{"info", false, true},
		{"warn", false, true},
		{"warning", false, true},
		{"error", false, false},
		{"nonsense", false, true}, // an unknown level falls back to info
	} {
		buf := &bytes.Buffer{}
		l := logging.New(logging.Config{Level: c.level, Format: "json", Output: buf})
		l.Debug("dbg")
		l.Warn("wrn")
		out := buf.String()
		if strings.Contains(out, "dbg") != c.debugShown {
			t.Errorf("level %q: debug visibility wrong: %s", c.level, out)
		}
		if strings.Contains(out, "wrn") != c.warnShown {
			t.Errorf("level %q: warn visibility wrong: %s", c.level, out)
		}
	}
}

func TestFromContextWithoutALoggerStillWorks(t *testing.T) {
	// A caller that never attached a logger must not panic; it falls back to
	// the process default.
	logging.FromContext(context.Background()).Debug("no logger attached")
}

func TestNewDefaultsToStderr(t *testing.T) {
	if logging.New(logging.Config{}) == nil {
		t.Fatal("New returned nil")
	}
}

// The credential most likely to appear in this system's own logs is one it
// issued. It is base64url — letters, digits, "-" and "_" — and the redactor
// used to ask for hex, so a real token passed through untouched under any key
// the secret-key list did not happen to cover.
func TestATokenThisProductActuallyMintsIsRedacted(t *testing.T) {
	buf, ctx := newBuffered(t)
	logging.FromContext(ctx).Info("a call was authenticated", "presented", mintedToken)
	logging.FromContext(ctx).Info("and in the message: " + mintedToken)

	out := buf.String()
	if strings.Contains(out, mintedToken) {
		t.Fatalf("the token survived redaction:\n%s", out)
	}
	if !strings.Contains(out, logging.Redacted) {
		t.Fatalf("nothing was redacted at all:\n%s", out)
	}
}
