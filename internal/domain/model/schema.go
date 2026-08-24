package model

import "github.com/OWNER/aos/internal/core/command"

// ListInput selects whose catalogue to read.
type ListInput struct {
	Provider string `json:"provider,omitempty" cli:"arg" jsonschema:"One provider id. Empty asks every provider this installation has a credential for."`

	command.Reasoning
}

// ListOutput is what the providers answered.
type ListOutput struct {
	Providers []Provider `json:"providers" jsonschema:"One entry per provider asked, ordered by id."`
	Total     int        `json:"total" jsonschema:"How many models were found across all of them."`
}
