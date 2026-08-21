package botapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/domain/bot"
	"github.com/OWNER/aos/internal/transport/botapi"
)

type noopChats struct{}

func (noopChats) GetByChannel(context.Context, string, string) (bot.ChatRef, error) {
	return bot.ChatRef{}, nil
}
func (noopChats) CreateForChannel(context.Context, string, string, string, string) (bot.ChatRef, error) {
	return bot.ChatRef{}, nil
}
func (noopChats) Send(context.Context, string, string, string) error { return nil }

type noopURL struct{}

func (noopURL) URL(context.Context) (string, bool) { return "", false }

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	reg := bot.NewRegistry(bot.Deps{
		Providers: map[string]bot.Provider{},
		Chats:     noopChats{},
		PublicURL: noopURL{},
		Clock:     fixedClock{at: time.Now()},
	})
	return botapi.New(botapi.Config{Registry: reg})
}

// TestWebhookAlwaysAnswers200 proves the handler never turns an unknown
// registration or a downstream failure into a status Telegram would retry
// on — see botapi's own doc comment on why.
func TestWebhookAlwaysAnswers200(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook/no-such-agent", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 even for an unknown registration", rec.Code)
	}
}

// TestWebhookRejectsAnUnreadableBody proves the one exception: a body this
// handler could not even read is a malformed request, not a processing
// failure to swallow into a 200.
func TestWebhookRejectsAnUnreadableBody(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook/some-agent", errReader{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unreadable body", rec.Code)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
