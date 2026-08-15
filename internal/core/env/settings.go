package env

import "time"

// The tunables of the system, in one place. Defaults come from the original
// where it had one (jobs concurrency, tick interval) and are stated here where
// it hardcoded the value instead of exposing it.
const (
	KeyEnv              = "ENV"               // production disables the API playground (ADR-0009)
	KeyHome             = "HOME_DIR"          // overrides ~/.aos; used by every test
	KeyWorkspacePath    = "WORKSPACE_PATH"    // the directory that holds .aos/; defaults to the working directory
	KeyWorkspaceID      = "WORKSPACE_ID"      // which workspace a command addresses; written into the repository's .env
	KeyAgentID          = "AGENT_ID"          // the acting agent, when the caller is one
	KeyServerHost       = "SERVER_HOSTNAME"   //
	KeyServerPort       = "SERVER_PORT"       //
	KeyToken            = "TOKEN"             //
	KeyLogLevel         = "LOG_LEVEL"         //
	KeyLogFormat        = "LOG_FORMAT"        // text | json | auto
	KeyFsync            = "FSYNC"             // off disables fsync in tests (ADR-0012)
	KeyJobsConcurrency  = "JOBS_CONCURRENCY"  //
	KeyJobsTick         = "JOBS_TICK"         //
	KeyTickConcurrency  = "TICK_CONCURRENCY"  //
	KeyToolConcurrency  = "TOOL_CONCURRENCY"  //
	KeyShutdownTimeout  = "SHUTDOWN_TIMEOUT"  //
	KeyApprovalDeadline = "APPROVAL_DEADLINE" // ADR-0007
	KeyNoColor          = "NO_COLOR"          //
)

// Defaults for the tunables above.
const (
	DefaultServerHost       = "127.0.0.1" // ADR-0009: loopback unless told otherwise
	DefaultLogLevel         = "info"
	DefaultLogFormat        = "auto"
	DefaultJobsConcurrency  = 20
	DefaultJobsTick         = 15 * time.Minute
	DefaultTickConcurrency  = 8
	DefaultToolConcurrency  = 4
	DefaultShutdownTimeout  = 15 * time.Second
	DefaultApprovalDeadline = 120 * time.Second
)

// IsProduction reports whether the process runs in production mode. The
// original shipped with development: true hardcoded (defect #12); here the
// mode is resolved and defaults to production.
func (r *Resolver) IsProduction() bool {
	return r.String(KeyEnv, "production") == "production"
}
