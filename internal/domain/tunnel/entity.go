// Package tunnel exposes the local daemon on the public internet via
// Cloudflare Tunnel, through the system's own cloudflared binary.
//
// It has no collection-backed entity: State is process state, not a record a
// person edits, so there is nothing here that fscollections persists — see
// docs/04 - Domínio/Tunnel (Go).md.
package tunnel

import "time"

// Status is where the tunnel is in its lifecycle.
type Status string

const (
	Stopped  Status = "stopped"
	Starting Status = "starting"
	Running  Status = "running"
	Failed   Status = "failed"
)

// State is what Start, Stop and Status report.
type State struct {
	Status    Status     `json:"status"`
	URL       string     `json:"url,omitempty"`
	PID       int        `json:"pid,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	Error     string     `json:"error,omitempty"`

	// Authenticated is the exposure guard's verdict at the time of the call:
	// authentication is on and at least one account holds an active API
	// token. Start refuses while it is false. It is reported so a screen can
	// explain a switch nobody can turn on yet from the rule itself, instead
	// of keeping a second copy of it that drifts.
	Authenticated bool `json:"authenticated"`
}
