package chat

import (
	"regexp"
	"strings"
)

// The three ways a target can be named, all inherited from the original because
// they come from three different places: a composer that writes markup, a
// person typing in a terminal, and an older convention still in use.
var (
	atMention     = regexp.MustCompile(`(^|[\s(])@(?:\[([^\]]+)\]|([a-zA-Z0-9_-]+))`)
	hashMention   = regexp.MustCompile(`(^|[\s(])#([a-zA-Z0-9_-]+)`)
	inlineMention = regexp.MustCompile(`<mention\b[^>]*\bid="([^"]*)"`)
)

// Mentions returns the identifiers named in a message, in the order they appear
// and without repeats.
//
// It does not decide which of them are agents. That is the caller's job,
// because the same syntax addresses people, and this function has no way to
// tell a person from an agent.
func Mentions(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}

	for _, m := range atMention.FindAllStringSubmatch(text, -1) {
		if m[2] != "" {
			add(m[2])
		} else {
			add(m[3])
		}
	}
	for _, m := range hashMention.FindAllStringSubmatch(text, -1) {
		add(m[2])
	}
	for _, m := range inlineMention.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	return out
}

// Target is who a message is addressed to, and how that was decided.
type Target struct {
	AgentID string `json:"agentId" jsonschema:"Agent that will answer."`
	Reason  string `json:"reason" jsonschema:"How the recipient was chosen: mention, participant or orchestrator."`
}

// The three ways a recipient gets chosen, reported so that a person surprised
// by who answered can see why.
const (
	ByMention      = "mention"
	ByParticipant  = "participant"
	ByOrchestrator = "orchestrator"
)

// resolveTarget decides which agent answers a message.
//
// The order is the original's, and each step is a different kind of evidence:
// an explicit mention is an instruction, a single agent in a direct message is
// an inference, and the orchestrator is the default. The first two are about
// what the person asked for; the third is what happens when they did not say.
func resolveTarget(c *Chat, text string, isAgent func(string) bool, orchestrator string) (Target, bool) {
	for _, id := range Mentions(text) {
		if isAgent(id) {
			return Target{AgentID: id, Reason: ByMention}, true
		}
	}
	if agents := c.AgentParticipants(); len(agents) == 1 {
		return Target{AgentID: agents[0], Reason: ByParticipant}, true
	}
	if orchestrator != "" {
		return Target{AgentID: orchestrator, Reason: ByOrchestrator}, true
	}
	return Target{}, false
}
