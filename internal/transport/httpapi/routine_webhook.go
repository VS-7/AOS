package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/OWNER/aos/internal/core/safe"
	"github.com/OWNER/aos/internal/domain/routine"
)

// RoutineWebhookPath is where a routine's webhook trigger is fired, relative
// to the daemon's origin. The id is the routine's; the workspace goes in the
// query string (`?workspace=`), the same way ambientIdentity reads it for any
// link, because the sender carries no header of this system's.
const RoutineWebhookPath = "/api/hooks/routines/{id}"

// maxWebhookBytes bounds one delivery. A webhook announces that something
// happened; it does not carry the thing.
const maxWebhookBytes = 1 << 20

// RoutineWebhooks is the slice of the routine aggregate the webhook route
// needs, resolved to whichever workspace the request names.
type RoutineWebhooks interface {
	VerifyWebhook(ctx context.Context, in routine.WebhookInput) error
	FireWebhook(ctx context.Context, in routine.WebhookInput) (*routine.Run, error)
}

// routineWebhook fires a routine from outside.
//
// It is not a command route, and cannot be one: every command answers to the
// session or API token authenticate checks, and a deploy script or a form
// service holds neither. The routine's own webhook token is the only
// credential, verified in constant time by the domain.
//
// It is read from the Authorization header and nowhere else — not the query
// string, for the reason ambientIdentity gives, and not the X-Auth-Token header
// or the session cookie the rest of the API accepts, which would offer the
// daemon's own credential to the one surface a stranger reaches. And it is
// checked before anything is done for the caller: no token is refused before
// a byte of the body is buffered, and a wrong one before the body is read at
// all. Verifying opens the workspace the request names.
//
// The answer is 202 as soon as the token is good. A run is a whole model turn,
// and a sender kept waiting that long times out and delivers again, which is
// a second run. The run is recorded like any other and appears in the
// routine's history.
func (s *Server) routineWebhook(w http.ResponseWriter, r *http.Request) {
	in := routine.WebhookInput{
		ID:    chi.URLParam(r, "id"),
		Token: webhookToken(r),
	}
	if in.Token == "" {
		writeError(w, r, errWebhookNoToken())
		return
	}
	if err := s.cfg.RoutineWebhooks.VerifyWebhook(r.Context(), in); err != nil {
		writeError(w, r, err)
		return
	}

	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, r, errBodyTooLarge(maxWebhookBytes))
			return
		}
		writeError(w, r, errWebhookUnreadable(err))
		return
	}
	in.Payload = webhookPayload(raw)

	// Detached from the request's cancellation, which comes with this answer,
	// but not from its values: the workspace the run belongs to rides on them.
	runCtx := context.WithoutCancel(r.Context())
	log := loggerFrom(r, s.cfg.Log)
	s.background.Add(1)
	safe.Go(runCtx, "httpapi.routineWebhook", func(ctx context.Context) error {
		defer s.background.Done()
		if _, err := s.cfg.RoutineWebhooks.FireWebhook(ctx, in); err != nil {
			// The run records its own failure; this line is for whoever reads
			// the daemon log wondering why a delivery did nothing.
			log.Warn("a routine fired by webhook did not succeed", "routine", in.ID, "err", err)
		}
		return nil
	})

	writeJSON(w, http.StatusAccepted, map[string]any{
		"data": map[string]any{"accepted": true, "routine": in.ID},
	})
}

// webhookToken is the bearer credential in r's Authorization header, or "".
func webhookToken(r *http.Request) string {
	rest, ok := cutPrefixFold(r.Header.Get("Authorization"), "bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(rest)
}

// webhookPayload is what the routine is handed. A JSON object arrives as its
// fields; anything else — form fields, plain text, an array — arrives whole
// under "body", since a sender's format is not this system's to refuse.
func webhookPayload(raw []byte) map[string]any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(trimmed), &object); err == nil && object != nil {
		return object
	}
	return map[string]any{"body": trimmed}
}
