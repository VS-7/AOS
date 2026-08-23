package update

import (
	"context"
	"time"
)

// Service is the update API of the domain.
type Service interface {
	// Check queries the release channel and reports what it found. It never
	// downloads anything — docs/08 - Entrega/Auto-Update.md's own line.
	Check(ctx context.Context, in CheckInput) (CheckOutput, error)

	// Download fetches every asset this platform needs for a release Check
	// already found, verifies the checksums file's signature and every
	// asset's checksum, and stages the result. A failed verification leaves
	// nothing installed — Apply is the only thing that touches the live
	// binaries, and Download never calls it.
	Download(ctx context.Context, in DownloadInput) (DownloadOutput, error)

	// Apply swaps in a Staged release and restarts the daemon at a safe
	// point: after in-flight turns finish (bounded by a grace period), with
	// the previous binaries kept until the new ones report healthy.
	Apply(ctx context.Context, in ApplyInput) (ApplyOutput, error)

	// Status reports the current version and channel without checking the
	// network.
	Status(ctx context.Context, in StatusInput) (Status, error)
}

// ReleaseSource is the one network-facing port: read a channel's manifest,
// fetch bytes. "Distribuição por releases assinados, agnóstica de forja" —
// the design's own decision — is why this is two generic operations and not
// anything that names GitHub.
type ReleaseSource interface {
	// Latest returns the newest release published on channel, or nil when
	// the channel has none. An empty channel is not an error: Check's own
	// contract is to report that state, not to fail on it.
	Latest(ctx context.Context, channel Channel) (*Release, error)
	// Fetch downloads the bytes at url — an asset, a checksums file, or a
	// signature file; the caller knows which.
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// Installer is where Download and Apply touch the filesystem. Kept as a
// port for the same reason every domain here keeps real I/O behind one:
// service_test.go proves the verify-then-refuse behavior in memory, with no
// disk and no binary actually at risk of being swapped mid-test.
type Installer interface {
	// Stage writes verified bytes to a scratch location under name, safe to
	// rename over a live binary later. Returns the path Apply will use.
	Stage(ctx context.Context, name string, data []byte) (path string, err error)
	// PathOf resolves where a named binary currently lives on this machine
	// — what Apply treats as the swap target.
	PathOf(ctx context.Context, binary string) (string, error)
	// SwapIn renames staged over target, first moving whatever is at target
	// to target+".prev" so Rollback can undo exactly this swap.
	SwapIn(ctx context.Context, staged, target string) error
	// Rollback restores target from the backup SwapIn made. Calling it
	// without a prior SwapIn for that target is a no-op, not an error —
	// Apply's own rollback path does not need to track which binaries it
	// actually got to before the failure that triggered it.
	Rollback(ctx context.Context, target string) error
}

// DaemonSupervisor is the narrow slice of the gateway this domain needs:
// restart the daemon process the new binaries were just swapped into, and
// ask whether it is answering. internal/app wires the real
// gateway.Service behind this — update does not import another domain
// directly, the same discipline internal/domain/tunnel's own Config port
// documents.
type DaemonSupervisor interface {
	Restart(ctx context.Context) error
	Healthy(ctx context.Context) bool
}

// ActiveWork reports how many turns are in flight right now, so Apply can
// wait for them before it restarts the daemon out from under them.
// internal/app wires this to the real job queue's claimed-job count.
type ActiveWork interface {
	Count(ctx context.Context) (int, error)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// Sleeper is how Apply waits without blocking a test — service_test.go
// fakes it to make a bounded wait finish instantly instead of for real.
type Sleeper interface {
	Sleep(ctx context.Context, d time.Duration) error
}
