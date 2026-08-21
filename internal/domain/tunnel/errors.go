package tunnel

import "github.com/OWNER/aos/internal/core/apperr"

// errInsecureExposure fires when Start is asked to publish the daemon while
// the API itself is not authenticated. The original checks neither of the
// two things this guards — see the design note's own account of how the
// reverse engineering found an unauthenticated API reachable through a
// tunnel. This is the single most important refusal in this package.
func errInsecureExposure() error {
	return apperr.New("TUNNEL_INSECURE_EXPOSURE").
		Causer("tunnel.Service.Start").
		Msgf("refusing to expose an API without authentication").
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label:   "enable authentication and issue an API token first",
			Command: "aos auth token issue --name tunnel",
		})
}

// errConfigIncomplete fires when Start is asked to run without both a
// hostname and a token configured — distinct from errInsecureExposure, which
// is about the local API's own guard rather than the tunnel's own setup.
func errConfigIncomplete() error {
	return apperr.New("TUNNEL_CONFIG_INCOMPLETE").
		Causer("tunnel.Service.Start").
		Msgf("tunnel hostname and token must both be configured").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "set tunnel.hostname and tunnel.token in the config before starting"})
}

// errBinaryMissing fires when cloudflared is not on PATH. It is not embedded
// — see the design doc — so this is the expected failure on a machine that
// never installed it, and the message says so.
func errBinaryMissing(cause error) error {
	return apperr.New("TUNNEL_BINARY_MISSING").
		Causer("tunnel.Service.Start").
		Msgf("cloudflared is not on PATH: %v", cause).
		Status(apperr.StatusServiceUnavailable).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "install cloudflared: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"})
}

// errReadinessTimeout fires when cloudflared was spawned but never reported
// itself connected within the bounded wait. The process this attempt started
// is killed before this returns — a caller retrying does not accumulate
// orphaned processes.
func errReadinessTimeout(cause error) error {
	e := apperr.New("TUNNEL_READINESS_TIMEOUT").
		Causer("tunnel.Service.Start").
		Msgf("cloudflared did not report a connected tunnel within the wait").
		Status(apperr.StatusGatewayTimeout).
		CTA(apperr.CallToAction{Label: "check the hostname and token are valid, then retry"})
	if cause != nil {
		e = e.Wrap(cause)
	}
	return e
}

// errSpawnFailed wraps any other failure to start the process.
func errSpawnFailed(cause error) error {
	return apperr.New("TUNNEL_SPAWN_FAILED").
		Causer("tunnel.Service.Start").
		Msgf("could not start cloudflared: %v", cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}
