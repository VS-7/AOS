package app

import (
	"context"

	"github.com/OWNER/aos/internal/domain/routine"
)

// routineWebhooks is what the daemon's webhook route fires routines through.
// Until it existed a webhook trigger minted a token with nowhere to send it:
// FireWebhook was reachable from no transport at all.
//
// It resolves a webhook to the routines of the workspace it
// names, the same routing every command gets (scopeFor). The sender names it
// in the fire URL's query string; one that names none reaches the primary.
type routineWebhooks struct{ app *App }

func (h routineWebhooks) VerifyWebhook(ctx context.Context, in routine.WebhookInput) error {
	target, err := h.app.scopeFor(ctx)
	if err != nil {
		return err
	}
	return target.Routines.VerifyWebhook(ctx, in)
}

func (h routineWebhooks) FireWebhook(ctx context.Context, in routine.WebhookInput) (*routine.Run, error) {
	target, err := h.app.scopeFor(ctx)
	if err != nil {
		return nil, err
	}
	return target.Routines.FireWebhook(ctx, in)
}
