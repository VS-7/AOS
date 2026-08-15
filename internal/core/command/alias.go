package command

import "github.com/OWNER/aos/internal/core/apperr"

// Alias keeps a published tool name working after a rename.
//
// The consumer of these names is a model that learned them in one session and
// uses them in the next. Renaming is expensive in a way that does not show up
// in any test — the original renamed memories list/get/create/forgot to
// recall/reflect/store/forget between two releases and broke every automation
// written against the old names (ADR-0011).
type Alias struct {
	From       string // "memories_create"
	To         string // "memories_store"
	Since      string // "0.4.0" — the version the migration started in
	RemoveAt   string // "1.0.0" — never before a major
	Deprecated bool
}

// DeprecationNotice travels in the envelope of a successful call made through
// an alias. The model reads it and learns the new name by itself.
type DeprecationNotice struct {
	Called   string `json:"called"`
	Use      string `json:"use"`
	Since    string `json:"since,omitempty"`
	RemoveAt string `json:"removeAt,omitempty"`
	Message  string `json:"message"`
}

// Notice renders the notice for this alias.
func (a Alias) Notice() *DeprecationNotice {
	if !a.Deprecated {
		return nil
	}
	msg := a.From + " is deprecated; use " + a.To
	if a.RemoveAt != "" {
		msg += " (it stops working in " + a.RemoveAt + ")"
	}
	return &DeprecationNotice{
		Called: a.From, Use: a.To, Since: a.Since, RemoveAt: a.RemoveAt, Message: msg,
	}
}

// Alias registers an alternative name for an existing command.
func (r *Registry) Alias(a Alias) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.byKey[a.From]; taken {
		return errAliasShadows(a.From)
	}
	r.aliases[a.From] = a
	return nil
}

// MustAlias is Alias for static wiring.
func (r *Registry) MustAlias(a Alias) {
	if err := r.Alias(a); err != nil {
		panic(err)
	}
}

// Aliases returns every registered alias, keyed by the old name.
func (r *Registry) Aliases() map[string]Alias {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]Alias, len(r.aliases))
	for k, v := range r.aliases {
		out[k] = v
	}
	return out
}

func errAliasShadows(name string) error {
	return apperr.New("COMMAND_ALIAS_SHADOWS").
		Causer("command.Registry.Alias").
		Msgf("%q is a real command and cannot be an alias", name).
		Issue("alias", name).
		Status(apperr.StatusInternalServerError)
}
