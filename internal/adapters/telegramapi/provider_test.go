package telegramapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/bot"
)

// fakeTelegram is a minimal stand-in for api.telegram.org: it records every
// call and answers {"ok": true}, so a test proves what this package sent
// without a real network.
type fakeTelegram struct {
	calls []call
	fail  bool
}

type call struct {
	path string
	body map[string]any
}

func (f *fakeTelegram) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.calls = append(f.calls, call{path: r.URL.Path, body: body})
		if f.fail {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "boom"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": true})
	}
}

func newTestProvider(t *testing.T, ft *fakeTelegram) (*Provider, func()) {
	t.Helper()
	srv := httptest.NewServer(ft.handler())
	return NewWithClient(srv.URL, srv.Client()), srv.Close
}

func TestRegisterWebhookCallsSetWebhookWithTheURLAndSecret(t *testing.T) {
	ft := &fakeTelegram{}
	p, closeSrv := newTestProvider(t, ft)
	defer closeSrv()

	err := p.RegisterWebhook(context.Background(), "https://example.com/hook", "s3cr3t", map[string]any{"token": "tok"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ft.calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(ft.calls))
	}
	if !strings.Contains(ft.calls[0].path, "/bottok/setWebhook") {
		t.Fatalf("path = %q, want it to name the token and setWebhook", ft.calls[0].path)
	}
	if ft.calls[0].body["url"] != "https://example.com/hook" || ft.calls[0].body["secret_token"] != "s3cr3t" {
		t.Fatalf("body = %+v", ft.calls[0].body)
	}
}

func TestUnregisterWebhookCallsDeleteWebhook(t *testing.T) {
	ft := &fakeTelegram{}
	p, closeSrv := newTestProvider(t, ft)
	defer closeSrv()

	if err := p.UnregisterWebhook(context.Background(), map[string]any{"token": "tok"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ft.calls[0].path, "/deleteWebhook") {
		t.Fatalf("path = %q", ft.calls[0].path)
	}
}

func TestSendSplitsALongMessageAcrossMultipleCalls(t *testing.T) {
	ft := &fakeTelegram{}
	p, closeSrv := newTestProvider(t, ft)
	defer closeSrv()

	text := strings.Repeat("x", maxChars-10) + "\n\n" + strings.Repeat("y", 100)
	if err := p.Send(context.Background(), "tok", bot.Outbound{ChatID: "chat-1", Text: text}); err != nil {
		t.Fatal(err)
	}
	if len(ft.calls) < 2 {
		t.Fatalf("got %d sendMessage calls, want at least 2 for a message over the limit", len(ft.calls))
	}
	for _, c := range ft.calls {
		if c.body["chat_id"] != "chat-1" {
			t.Fatalf("chat_id = %v, want chat-1", c.body["chat_id"])
		}
	}
}

func TestSendReportsHowManyChunksLandedBeforeAFailure(t *testing.T) {
	ft := &fakeTelegram{fail: true}
	p, closeSrv := newTestProvider(t, ft)
	defer closeSrv()

	err := p.Send(context.Background(), "tok", bot.Outbound{ChatID: "chat-1", Text: "hi"})
	if err == nil {
		t.Fatal("want an error when Telegram refuses the call")
	}
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("TELEGRAMAPI_SEND_PARTIAL").Code {
		t.Fatalf("want TELEGRAMAPI_SEND_PARTIAL, got %v", err)
	}
}

func TestSetTypingOnCallsSendChatAction(t *testing.T) {
	ft := &fakeTelegram{}
	p, closeSrv := newTestProvider(t, ft)
	defer closeSrv()

	if err := p.SetTyping(context.Background(), "tok", "chat-1", true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ft.calls[0].path, "sendChatAction") {
		t.Fatalf("path = %q", ft.calls[0].path)
	}
}

func TestSetTypingOffMakesNoCall(t *testing.T) {
	ft := &fakeTelegram{}
	p, closeSrv := newTestProvider(t, ft)
	defer closeSrv()

	if err := p.SetTyping(context.Background(), "tok", "chat-1", false); err != nil {
		t.Fatal(err)
	}
	if len(ft.calls) != 0 {
		t.Fatalf("got %d calls, want 0: Telegram's typing indicator has no explicit off", len(ft.calls))
	}
}

func TestParseRejectsAWrongWebhookSecret(t *testing.T) {
	p := New()
	_, err := p.Parse(context.Background(), "wrong", "right", []byte(`{"message":{"text":"hi"}}`))
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("TELEGRAMAPI_WEBHOOK_SECRET_MISMATCH").Code {
		t.Fatalf("want TELEGRAMAPI_WEBHOOK_SECRET_MISMATCH, got %v", err)
	}
}

func TestParseAcceptsTheRightSecretAndDecodesTheMessage(t *testing.T) {
	p := New()
	body := `{"message":{"chat":{"id":555},"from":{"id":42},"text":"hello there"}}`

	in, err := p.Parse(context.Background(), "right", "right", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if in.ChatID != "555" || in.UserID != "42" || in.Text != "hello there" {
		t.Fatalf("got %+v", in)
	}
}

func TestParseRejectsAnUpdateWithNoMessage(t *testing.T) {
	p := New()
	_, err := p.Parse(context.Background(), "s", "s", []byte(`{}`))
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("TELEGRAMAPI_UPDATE_UNSUPPORTED").Code {
		t.Fatalf("want TELEGRAMAPI_UPDATE_UNSUPPORTED, got %v", err)
	}
}
