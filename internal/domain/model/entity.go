// Package model answers one question: which models can this installation
// actually reach right now.
//
// It exists because the answer was a list written by hand, and a list written
// by hand is wrong the day after it is written. This build shipped a Codex
// entry for a model that does not exist and was missing three that do; the
// Google entries had to be corrected twice against the live API before any
// turn could answer at all. Nothing in that sequence was avoidable by being
// more careful — the provider is the only authority on what the provider
// serves, so this asks it.
//
// The package owns no catalogue of its own. It knows how to ask, how to report
// one provider failing without losing the others, and nothing else.
package model

// Model is one model a provider serves.
type Model struct {
	ID   string `json:"id" jsonschema:"The identifier to configure, spelled the way the provider spells it."`
	Name string `json:"name" jsonschema:"What to show a person: the provider's own display name, or the id when it publishes none."`
}

// Provider is one provider's answer, including the answer "I could not ask".
//
// Error is a field rather than a returned error because the interesting case is
// partial: four providers connected, one key expired. Failing the whole call
// there would hide three working catalogues behind one broken credential, and
// silently dropping the broken one would leave a person staring at a provider
// that shows no models with nothing saying why — which is the failure this
// project has already paid for once, in the chat.
type Provider struct {
	ID     string  `json:"id" jsonschema:"The provider this catalogue belongs to."`
	Models []Model `json:"models" jsonschema:"What it serves, best first where the provider publishes a ranking."`
	Error  string  `json:"error,omitempty" jsonschema:"Why this provider's catalogue is empty, when it is empty because asking failed."`
}
