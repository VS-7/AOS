// Package gateway supervises the daemon process: start it, stop it, and say
// truthfully whether it is running.
//
// It is the one command group that runs locally rather than through the daemon,
// for the obvious reason: starting a process by asking that process to start it
// does not work.
package gateway

import "time"

// Status is what the supervisor believes about the daemon.
type Status string

const (
	// Stopped means there is no record of a daemon.
	Stopped Status = "stopped"

	// Stale means there is a record and the process it names is gone. This is
	// the state that makes the whole design robust: a crashed daemon is
	// detected as an orphan record and cleaned up, rather than believed.
	Stale Status = "stale"

	// Running means the record names a live process.
	Running Status = "running"
)

// Meta is the record written when a daemon is started.
type Meta struct {
	PID       int       `json:"pid" jsonschema:"Process identifier of the daemon."`
	Port      int       `json:"port" jsonschema:"Port it listens on."`
	Host      string    `json:"host" jsonschema:"Address it is bound to."`
	StartedAt time.Time `json:"startedAt" jsonschema:"When it was started."`
	Version   string    `json:"version" jsonschema:"Version of the binary that was started."`
	Command   string    `json:"command" jsonschema:"Executable that was run."`
	Args      []string  `json:"args,omitempty" jsonschema:"Arguments it was run with."`
}

// State is the answer to "what is going on", with the evidence attached.
type State struct {
	Status Status `json:"status" jsonschema:"stopped, stale or running."`
	Meta   *Meta  `json:"meta,omitempty" jsonschema:"The record, when there is one."`

	// Healthy reports whether the daemon answered its health endpoint. A
	// process can be alive and not serving — it is the difference between the
	// port being bound and the process merely existing.
	Healthy bool `json:"healthy" jsonschema:"Whether the daemon answered its health check."`
}

// Command is what will be executed to start a daemon.
type Command struct {
	Path string
	Args []string
	Dir  string
	Env  []string

	// LogFile receives the daemon's output. A detached process with nowhere to
	// write is a process whose crash leaves no explanation.
	LogFile string
}
