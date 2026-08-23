package app

import (
	"context"
	_ "embed"
	"strings"

	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/domain/job"
)

// releasePubKey is the Ed25519 public key (relsig.GenerateKey's format)
// every release checksums file's signature is verified against — generated
// by tools/genreleasekey. The matching private key is never committed; see
// that tool's own doc comment.
//
//go:embed release-pubkey.pub
var releasePubKeyRaw string

func releasePublicKey() string { return strings.TrimSpace(releasePubKeyRaw) }

// updateSupervisor adapts gateway.Service to update.DaemonSupervisor — the
// narrow slice update.Apply needs (restart, health), not the whole
// Start/Stop/Status surface. update does not import gateway directly, the
// same discipline internal/domain/tunnel's own Config port documents.
type updateSupervisor struct{ svc *gateway.Service }

func (u updateSupervisor) Restart(ctx context.Context) error {
	_, err := u.svc.Restart(ctx, gateway.RestartInput{})
	return err
}

func (u updateSupervisor) Healthy(ctx context.Context) bool {
	state, err := u.svc.Status(ctx, gateway.StatusInput{})
	return err == nil && state.Healthy
}

// updateActiveWork adapts job.Queue to update.ActiveWork: how many turns
// are claimed — actively running, not merely queued — right now. A nil
// queue (the daemon started without one; see wire.go's own comment on why
// that is allowed to happen) reports zero: nothing can be in flight through
// a queue that never opened.
type updateActiveWork struct{ queue job.Queue }

func (u updateActiveWork) Count(ctx context.Context) (int, error) {
	if u.queue == nil {
		return 0, nil
	}
	jobs, err := u.queue.List(ctx, job.Filter{Status: job.Claimed})
	if err != nil {
		return 0, err
	}
	return len(jobs), nil
}
