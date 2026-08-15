package gateway

import (
	"context"
	"time"
)

// Processes is the operating-system surface this aggregate needs. It is a port
// because internal/domain may not import os/exec, and because a test that
// spawned real daemons to check a state machine would be testing the operating
// system.
type Processes interface {
	// Start launches a detached process and returns its identifier. Detached
	// matters: the daemon must outlive the terminal that started it.
	Start(ctx context.Context, cmd Command) (pid int, err error)

	// Alive reports whether a process with this identifier exists.
	Alive(pid int) bool

	// Terminate asks a process to stop, politely.
	Terminate(pid int) error

	// Kill stops a process that did not listen.
	Kill(pid int) error
}

// Health probes the daemon's health endpoint.
//
// Liveness is not the question. A process that started and failed to bind its
// port is alive and useless, and the original's 1.5-second wait for liveness
// would call it running.
type Health interface {
	Probe(ctx context.Context, host string, port int) error
}

// Store holds the record of the running daemon.
type Store interface {
	Read(ctx context.Context) (*Meta, error)
	Write(ctx context.Context, m Meta) error
	Clear(ctx context.Context) error
}

// Locker serialises supervision across processes.
//
// This is defect #18. The original crosses a pid file with liveness and takes
// no lock, so two simultaneous starts can both observe "stopped" and both
// spawn — leaving one daemon holding the port and another one dead, with the
// record naming whichever wrote last.
type Locker interface {
	Lock(ctx context.Context) (unlock func() error, err error)
}

// Resolver finds the daemon binary.
type Resolver interface {
	Resolve(ctx context.Context) (Command, error)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// Sleeper waits. It is a port so the health-check loop does not make a test
// take as long as a real boot.
type Sleeper interface {
	Sleep(ctx context.Context, d time.Duration) error
}
