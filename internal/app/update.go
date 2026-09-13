package app

import (
	"context"
	_ "embed"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/OWNER/aos/internal/adapters/supervise"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/auth"
	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/domain/job"
	"github.com/OWNER/aos/internal/domain/update"
)

// releasePubKey is the Ed25519 public key (relsig.GenerateKey's format)
// every release checksums file's signature is verified against — generated
// by tools/genreleasekey. The matching private key is never committed; see
// that tool's own doc comment.
//
//go:embed release-pubkey.pub
var releasePubKeyRaw string

func releasePublicKey() string { return strings.TrimSpace(releasePubKeyRaw) }

// updateFeed is the release feed this installation checks, and whether it was
// set on this machine (AOS_UPDATE_BASE_URL) rather than compiled into the
// build (build.UpdateBaseURL). The update service needs the difference only
// to say something useful when the feed is empty: a wrong address somebody
// set can be fixed, and a release published without a feed cannot be, from
// here.
func updateFeed(resolver *env.Resolver) (feed string, custom bool) {
	if feed := strings.TrimSpace(resolver.String(env.KeyUpdateBaseURL, "")); feed != "" {
		return feed, true
	}
	return build.UpdateBaseURL, false
}

// updateSupervisor adapts gateway.Service to update.DaemonSupervisor — the
// narrow slice update.Apply needs (which daemon answers, and restarting it),
// not the whole Start/Stop/Status surface. update does not import gateway
// directly, the same discipline internal/domain/tunnel's own Config port
// documents.
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
	// host and port are where this installation's daemon is configured to
	// answer: where to look when no record names a daemon.
	host string
	port int
	// identify reads which daemon answers at an address — supervise.Health.
	identify func(ctx context.Context, host string, port int) (supervise.Identity, error)
}

// Observe crosses the outside gateway's record — the process it started, if
// that is still alive — with what the daemon answering says about itself.
//
// "A gateway exists" was the whole test once, and a daemon started by hand
// passed it: `aosd update apply` restarted nothing, found the previous release
// answering the port, and reported the new one installed. Which process
// answers is what tells the two apart.
func (u updateSupervisor) Observe(ctx context.Context) (update.Daemon, error) {
	state, err := u.outside.Status(ctx, gateway.StatusInput{})
	if err != nil {
		return update.Daemon{}, err
	}
	host, port := u.host, u.port
	var daemon update.Daemon
	if state.Status == gateway.Running && state.Meta != nil {
		daemon.RecordedPID = state.Meta.PID
		// The record says where the daemon it started answers, which is
		// where it answers even when this process was configured otherwise.
		host, port = state.Meta.Host, state.Meta.Port
	}
	daemon.Address = net.JoinHostPort(host, strconv.Itoa(port))
	if u.serving() {
		// The daemon itself: no need to ask the port who it is.
		daemon.Self, daemon.Answering = true, true
		daemon.PID, daemon.Version = os.Getpid(), build.Version
		return daemon, nil
	}
	identify := u.identify
	if identify == nil {
		identify = supervise.NewHealth().Identify
	}
	if id, err := identify(ctx, host, port); err == nil {
		daemon.Answering, daemon.PID, daemon.Version = true, id.PID, id.Version
	}
	return daemon, nil
}

func (u updateSupervisor) Restart(ctx context.Context) error {
	svc := u.outside
	if u.serving() {
		svc = u.inside
	}
	_, err := svc.Restart(ctx, gateway.RestartInput{})
	return err
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
// these files by hand.
//
// An agent is refused either way, and so is every call through MCP. That is
// told by the surface, not the identity: `aosd --mcp` is started by an MCP
// client with no account and no agent on its calls, which is exactly what a
// terminal looks like, and it was let through to install and restart the
// daemon.
type updateOperators struct{ auth *auth.Service }

func (u updateOperators) MayInstall(ctx context.Context) (bool, error) {
	who := identity.From(ctx)
	if surface, ok := command.SurfaceOf(ctx); who.AgentID != "" ||
		(ok && (surface == command.SurfaceMCP || surface == command.SurfaceAgent)) {
		return false, update.ErrNotAPerson
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
