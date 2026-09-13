package update

import (
	"context"
	"errors"
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

	// Apply swaps in the release Download staged and restarts the daemon at a
	// safe point: after in-flight turns finish (bounded by a grace period),
	// with the previous binaries kept until the new ones report healthy.
	Apply(ctx context.Context, in ApplyInput) (ApplyOutput, error)

	// Status reports the current version, the last check and what is staged,
	// without checking the network.
	Status(ctx context.Context, in StatusInput) (Status, error)
}

// The two ways a ReleaseSource fails that Check and Download report
// differently from any other failure. An adapter wraps them into the error it
// returns; everything else it returns is "the channel answered, badly".
//
// They exist because the old contract had one error for all three, and it
// named the network: a release published without a signature read as
// "could not reach the release channel … check network access".
var (
	// ErrNotPublished: the address answered, and nothing is there — a
	// manifest, a signature or an asset that was never uploaded.
	ErrNotPublished = errors.New("update: nothing is published at that address")
	// ErrUnreachable: the address did not answer at all.
	ErrUnreachable = errors.New("update: the release channel could not be reached")
)

// ReleaseSource is the one network-facing port: read a channel's manifest,
// fetch bytes. "Distribuição por releases assinados, agnóstica de forja" —
// the design's own decision — is why this is generic operations and not
// anything that names GitHub.
type ReleaseSource interface {
	// Configured reports whether this installation has a release feed at
	// all. It is a question of its own rather than a nil release from
	// Latest, because "no feed" and "a feed with nothing on this channel"
	// were once the same answer, and both came out as "you are on the
	// newest release".
	Configured() bool
	// Latest returns the newest release published on channel. A channel
	// with no manifest is an error wrapping ErrNotPublished.
	Latest(ctx context.Context, channel Channel) (*Release, error)
	// Fetch downloads the bytes at url — an asset, a checksums file, or a
	// signature file; the caller knows which.
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// Stager is where Download leaves verified bytes for Apply. Every method
// takes a binary name rather than a path: where a staged file lives is the
// adapter's to decide, so nothing a caller or a release feed sends can name
// a path at all.
type Stager interface {
	// Stage writes data as binary's staged copy and returns where it went.
	Stage(ctx context.Context, binary string, data []byte) (path string, err error)
	// Digest is the hex SHA-256 of binary's staged copy, read now — what
	// Apply compares against the signed checksums just before swapping.
	Digest(ctx context.Context, binary string) (string, error)
	// Discard removes every staged copy. Nothing staged is not an error.
	Discard(ctx context.Context) error
}

// Installer is where Apply touches the live binaries. Kept as a port for the
// same reason every domain here keeps real I/O behind one: the tests prove
// the verify-then-refuse behavior in memory, with no binary actually at risk
// of being swapped mid-test.
type Installer interface {
	// Target resolves where binary lives on this machine, and whether it is
	// installed there. A binary that is not installed is not added by an
	// update: the layout an installer produced is the layout that stays.
	Target(ctx context.Context, binary string) (path string, installed bool, err error)
	// SwapIn puts binary's staged copy in place, keeping the binary it
	// replaces so Rollback can undo exactly this swap. The staged copy stays
	// where it is until Discard, so a rolled-back update can be retried.
	SwapIn(ctx context.Context, binary string) error
	// Rollback restores binary from the copy SwapIn kept. Calling it without a
	// prior SwapIn is a no-op, not an error.
	Rollback(ctx context.Context, binary string) error
	// Commit drops the copy SwapIn kept, once the new binary is proven.
	Commit(ctx context.Context, binary string) error
	// InPlace reports whether binaries here can be replaced one at a time. A
	// macOS application bundle cannot: its signature seals every file in it,
	// so replacing one breaks the seal of the whole application.
	InPlace(ctx context.Context) bool
}

// Store keeps the Record between calls, and between daemon restarts.
type Store interface {
	Load(ctx context.Context) (Record, error)
	Save(ctx context.Context, record Record) error
}

// DaemonSupervisor is the narrow slice of the gateway this domain needs:
// restart the daemon process the new binaries were just swapped into, and
// ask whether it is answering. internal/app wires the real gateway.Service
// behind this — update does not import another domain directly, the same
// discipline internal/domain/tunnel's own Config port documents.
type DaemonSupervisor interface {
	// CanRestart reports whether Restart can work from the process this
	// service runs in. Inside the daemon it cannot — the daemon does not
	// restart itself (AOS_GATEWAY_SELF_RESTART) — and Apply asks before it
	// swaps anything rather than finding out after.
	CanRestart(ctx context.Context) bool
	Restart(ctx context.Context) error
	Healthy(ctx context.Context) bool
}

// Operators says who may change this installation's binaries. Checking an
// update is anyone's question; installing one replaces the programs every
// account on the machine runs, which is an administrator's decision.
type Operators interface {
	MayInstall(ctx context.Context) (bool, error)
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
