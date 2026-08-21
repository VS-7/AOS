// Package bot gives an agent presence outside the UI, over an external
// messaging channel — today, Telegram. See docs/04 - Domínio/Bot (Go).md.
//
// A conversation over the channel and the same conversation in the app are
// the same chat.Chat record, found by chat.Service.GetByChannel — this
// package never keeps its own copy of message history.
package bot

import "time"

// Status is where a channel registration sits in its own lifecycle.
type Status string

const (
	// Pending means the agent's frontmatter declares a channel but no tunnel
	// is up yet to give it a public URL — deferred, not failed.
	Pending Status = "pending"

	// Registered means the provider's webhook is pointed at this daemon.
	Registered Status = "registered"

	// Failed means registration was attempted and the provider refused it —
	// see Registration's own Error field for why.
	Failed Status = "failed"
)

// Registration is one agent's binding to one channel provider.
type Registration struct {
	Provider    string `json:"provider"`
	WorkspaceID string `json:"workspaceId"`
	AgentID     string `json:"agentId"`

	WebhookURL    string `json:"webhookUrl,omitempty"`
	WebhookSecret string `json:"-" secret:"true"`

	Status       Status    `json:"status"`
	Error        string    `json:"error,omitempty"`
	RegisteredAt time.Time `json:"registeredAt,omitempty"`
}

// key identifies one registration: an agent has at most one binding per
// provider.
func (r Registration) key() string { return r.Provider + ":" + r.AgentID }
