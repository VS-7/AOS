package toolset

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
)

// errTypeUnknown fires when a wire value names a connection type the union
// does not have. Defaulting it to "custom" would connect to nothing and say
// nothing about why; refusing it says exactly what was wrong.
func errTypeUnknown(raw string) error {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return apperr.New("TOOLSET_TYPE_UNKNOWN").
		Causer("toolset.ParseType").
		Msgf("%q is not a toolset connection type", raw).
		Issue("type", raw).
		Issue("allowed", strings.Join(names, ", ")).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "use one of mcp-server::stdio, mcp-server::http, rest-api, cli, custom",
		})
}

// errTypeNotAvailable fires when a toolset's type decodes but this build has
// no adapter for it. It is deliberately a different code from
// errTypeUnknown: a caller reading "not available in this build" knows the
// configuration is fine and the binary is behind, where a decode error would
// send them chasing a typo that is not there.
func errTypeNotAvailable(t Type) error {
	return apperr.New("TOOLSET_TYPE_NOT_AVAILABLE").
		Causer("toolset.Service.Call").
		Msgf("connection type %q is not available in this build", t).
		Issue("type", string(t)).
		Status(apperr.StatusNotImplemented).
		CTA(apperr.CallToAction{
			Label: "mcp-server::stdio is the only connectable type in this build",
		})
}

// errEnvMissing fires when ${env.VAR} names a variable that is not set. It
// names the variable so the failure is diagnosable in one read, rather than
// surfacing later as an opaque 401 from whatever server the empty string was
// sent to.
func errEnvMissing(name string) error {
	return apperr.New("TOOLSET_ENV_MISSING").
		Causer("toolset.Interpolate").
		Msgf("the environment variable %q, referenced by ${env.%s}, is not set", name, name).
		Issue("variable", name).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "set the variable in the workspace environment before connecting this toolset",
		})
}

// errNotFound fires when no toolset has this ID.
func errNotFound(id string) error {
	return apperr.New("TOOLSET_NOT_FOUND").
		Causer("toolset.Service.Get").
		Msgf("no toolset %q", id).
		Issue("id", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label: "list configured toolsets",
			Tool:  "toolsets_list",
		})
}

// errDisabled fires when Call targets a toolset whose Status is
// StatusDisabled. Disabling keeps the configuration — deleting a misbehaving
// toolset would lose work someone spent time getting right — while refusing
// the calls that configuration would otherwise make.
func errDisabled(id string) error {
	return apperr.New("TOOLSET_DISABLED").
		Causer("toolset.Service.Call").
		Msgf("toolset %q is disabled", id).
		Issue("id", id).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "enable it before calling it",
			Tool:  "toolsets_update_config",
		})
}

// errCallIncomplete fires when Call is asked to run without naming both the
// toolset and the tool on it.
func errCallIncomplete(id, tool string) error {
	return apperr.New("TOOLSET_CALL_INCOMPLETE").
		Causer("toolset.Service.Call").
		Msgf("a call needs both a toolset id and a tool name").
		Issue("id", id).
		Issue("tool", tool).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "name the toolset and the tool to call on it",
			Tool:  "toolsets_call",
		})
}

// errConnectFailed wraps a failure to reach the external target. The
// underlying error comes from the adapter, never from this package echoing
// back the resolved configuration — a Toolset's Env and Headers may hold a
// secret that was just interpolated, and it must not travel in this message.
func errConnectFailed(id string, cause error) error {
	return apperr.New("TOOLSET_CONNECT_FAILED").
		Causer("toolset.Service.Call").
		Msgf("could not connect to toolset %q: %v", id, cause).
		Issue("id", id).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "check the toolset's configuration and that its target is reachable",
			Tool:  "toolsets_get",
		})
}

// errCallFailed wraps a failure of the tool call itself, once connected.
func errCallFailed(id, tool string, cause error) error {
	return apperr.New("TOOLSET_CALL_FAILED").
		Causer("toolset.Service.Call").
		Msgf("toolset %q reported an error calling %q: %v", id, tool, cause).
		Issue("id", id).
		Issue("tool", tool).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "inspect the tool's schema before retrying",
			Tool:  "toolsets_call",
		})
}

// errReadFailed wraps a repository failure that is not "not found" — the
// operation name is dynamic because it fires from several call sites.
func errReadFailed(op string, cause error) error {
	return apperr.New("TOOLSET_READ_FAILED").
		Causer("toolset.Service."+op).
		Msgf("could not read toolsets: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errWriteFailed wraps a repository failure while writing.
func errWriteFailed(op string, cause error) error {
	return apperr.New("TOOLSET_WRITE_FAILED").
		Causer("toolset.Service."+op).
		Msgf("could not save the toolset: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}
