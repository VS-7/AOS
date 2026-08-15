package gateway

import "github.com/OWNER/aos/internal/core/command"

// StatusInput asks what is going on. It takes nothing: there is one daemon.
type StatusInput struct {
	command.Reasoning
}

// StartInput launches the daemon.
type StartInput struct {
	command.Reasoning
}

// StopInput shuts the daemon down.
type StopInput struct {
	command.Reasoning
}

// StopOutput reports what stopping actually did.
type StopOutput struct {
	Stopped bool   `json:"stopped" jsonschema:"True when a running daemon was shut down."`
	Status  Status `json:"status" jsonschema:"The state afterwards."`
	PID     int    `json:"pid,omitempty" jsonschema:"The process that was stopped."`

	// Killed says the daemon ignored the shutdown request and had to be killed.
	// It is reported rather than hidden: a daemon that cannot shut down cleanly
	// is losing whatever it had not yet written.
	Killed bool `json:"killed,omitempty" jsonschema:"True when the daemon had to be killed after ignoring the request."`

	// Cleaned says there was a record of a daemon that was no longer running.
	Cleaned bool `json:"cleaned,omitempty" jsonschema:"True when a stale record was removed."`
}

// RestartInput stops and starts the daemon.
type RestartInput struct {
	command.Reasoning
}
