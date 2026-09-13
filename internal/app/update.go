package app

import (
	"context"
	_ "embed"
	"strings"

	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/auth"
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
//
// It holds two copies of the gateway because the update service runs in two
// kinds of process. Inside the daemon, the gateway refuses to restart the
// process it lives in (AOS_GATEWAY_SELF_RESTART), and it was the only copy
// this adapter had: Apply swapped the binaries, the restart was refused, the
// rollback's restart was refused the same way, and the answer was "rollback
// ALSO failed — the daemon may be down". In a process run from a terminal —
// `aosd update apply` — the daemon is another process, and the outside
// gateway restarts it exactly the way `aos gateway restart` does.
//
// serving is App.serving: true once Serve runs, for the primary and every
// workspace it built alike. Asked on every call rather than fixed at wiring,
// because New runs before anyone knows whether Serve will follow.
type updateSupervisor struct {
	inside  *gateway.Service
	outside *gateway.Service
	serving func() bool
}

// CanRestart is false in the daemon itself, which cannot restart onto new
// binaries and still be there to roll them back.
func (u updateSupervisor) CanRestart(context.Context) bool {
	return u.outside != nil && !u.serving()
}

func (u updateSupervisor) Restart(ctx context.Context) error {
	svc := u.inside
	if u.CanRestart(ctx) {
		svc = u.outside
	}
	_, err := svc.Restart(ctx, gateway.RestartInput{})
	return err
}

func (u updateSupervisor) Healthy(ctx context.Context) bool {
	svc := u.inside
	if u.CanRestart(ctx) {
		svc = u.outside
	}
	state, err := svc.Status(ctx, gateway.StatusInput{})
	return err == nil && state.Healthy
}

// updateOperators answers update.Operators from the account the request
// carries.
//
// Installing an update replaces the programs every account on this machine
// runs, so it is an administrator's call: update_download and update_apply
// were open to any valid token, a member's included.
//
// A call with no account on it is let through. There are two ways to get
// one, and neither is somebody the check could turn away: the installation
// switched authentication off, which lets every loopback caller do anything
// by the owner's own choice; or the command runs inside a process started
// from a terminal — `aosd update apply` — by someone who can already replace
// these files by hand. An agent is refused either way.
type updateOperators struct{ auth *auth.Service }

func (u updateOperators) MayInstall(ctx context.Context) (bool, error) {
	who := identity.From(ctx)
	if who.AgentID != "" {
		return false, nil
	}
	if who.UserID == "" {
		return true, nil
	}
	user, err := u.auth.Get(ctx, who.UserID)
	if err != nil {
		return false, err
	}
	return user.Role == auth.Super, nil
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
