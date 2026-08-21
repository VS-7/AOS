// Package botapi is the webhook surface external messaging providers call
// into — Telegram today. Mounted at /api/bot outside the authenticated
// group httpapi guards its command routes with: a provider's server has no
// session cookie or API token for this system, and proves itself with its
// own webhook secret instead — see bot.Registry.HandleWebhook.
package botapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/OWNER/aos/internal/adapters/telegramapi"
	"github.com/OWNER/aos/internal/domain/bot"
)

// maxBodyBytes bounds one webhook delivery — Telegram's own update payloads
// are small; this is a denial-of-service guard, not a real limit any
// legitimate update approaches.
const maxBodyBytes = 1 << 20

// Config is what the router is built from.
type Config struct {
	Registry *bot.Registry
	Log      *slog.Logger
}

// New builds the router. It is mounted by the caller — see httpapi's Bot
// field.
func New(cfg Config) http.Handler {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	s := &server{reg: cfg.Registry, log: cfg.Log}

	r := chi.NewRouter()
	r.Post("/{provider}/webhook/{agentID}", s.webhook)
	return r
}

type server struct {
	reg *bot.Registry
	log *slog.Logger
}

// webhook always answers 200: a provider that does not see one retries (and
// eventually gives up on) the delivery, and neither a registration this
// daemon does not have nor a downstream dispatch failure is something
// re-delivery fixes. The one exception is a body this handler could not even
// read, which is a malformed request, not a processing failure.
func (s *server) webhook(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	agentID := chi.URLParam(r, "agentID")

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "could not read the request body"})
		return
	}

	secret := r.Header.Get(telegramapi.SecretHeader)
	if err := s.reg.HandleWebhook(r.Context(), provider, agentID, secret, body); err != nil {
		s.log.Error("bot webhook did not resolve to a delivered message",
			"provider", provider, "agent", agentID, "err", err)
	}
	w.WriteHeader(http.StatusOK)
}
