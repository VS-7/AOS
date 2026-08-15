package config

// GetInput is the payload of `config get`.
type GetInput struct {
	Reveal bool `json:"reveal,omitempty" jsonschema:"description=Show secret values in full. Only honoured for a human on an interactive terminal; an agent never receives unredacted secrets."`
}

// UpdateInput is the payload of `config update`. Fields are addressed by dotted
// path so a caller changes one setting without resending the whole file.
type UpdateInput struct {
	Set map[string]any `json:"set" jsonschema:"required,description=Dotted paths to set, for example {\"region.timezone\": \"America/Sao_Paulo\"}. An agent may only set fields on the agent-writable allowlist."`
}

// agentWritable lists the only paths an agent may change. Anything else fails
// with AOS_CONFIG_FIELD_FORBIDDEN and a CTA pointing the human at the UI.
var agentWritable = []string{
	"region.language", "region.city", "region.country", "region.timezone",
	"general.preventSleep",
	"notifications.enabled",
}

// AgentWritable reports whether a dotted path may be written by an agent.
func AgentWritable(path string) bool {
	for _, p := range agentWritable {
		if p == path {
			return true
		}
	}
	return false
}

// AgentWritablePaths returns the allowlist, for the error CTA and for the test
// that fails when a new field is neither allowed nor consciously denied.
func AgentWritablePaths() []string {
	out := make([]string, len(agentWritable))
	copy(out, agentWritable)
	return out
}
