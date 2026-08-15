package agentloop

import (
	"regexp"
	"strings"
)

// ModelRef is a provider and a model, resolved.
type ModelRef struct {
	Provider  string
	Model     string
	Reasoning ReasoningLevel
}

// AgentModel is what an agent's file asks for.
type AgentModel struct {
	Provider string
	Model    string

	// Reasoning is a divergence from the original, which always reads the
	// level from the global configuration. An agent that reviews code
	// critically and one that triages an inbox should not think equally hard,
	// and the global value stays the default.
	Reasoning string
}

// ConfigModel is what the installation's configuration says.
type ConfigModel struct {
	Provider  string
	Model     string
	Reasoning string
}

// LastResortModel is the fifth level of the cascade: a literal, so that an
// installation which configured nothing still starts and fails with a sentence
// about configuration rather than about a nil pointer.
const LastResortModel = "gpt-5"

// modelWithProvider parses the "{model} ({provider})" spelling the original
// introduced to work around front-matter validation in an editor — for example
// "Gemini 3 Flash (google)". It is carried over because agent files written for
// the original use it.
var modelWithProvider = regexp.MustCompile(`^(.+?)\s*\((.+)\)$`)

// Resolve walks the five levels of the cascade.
//
//  1. the agent's own provider and model;
//  2. the configured default provider, when the agent named only a model;
//  3. the configured default, whole;
//  4. a last-resort model under the configured provider;
//  5. nothing at all, which is an error that names what to configure.
//
// The reasoning level comes from the configuration unless the agent overrode
// it, and defaults to medium.
func Resolve(agent AgentModel, cfg ConfigModel) (ModelRef, error) {
	out := ModelRef{Reasoning: levelOf(firstNonEmpty(agent.Reasoning, cfg.Reasoning))}

	model, provider := split(agent.Model)
	if agent.Provider != "" {
		provider = agent.Provider
	}

	switch {
	case model != "" && provider != "":
		out.Provider, out.Model = provider, model

	case model != "":
		// The agent named a model and no provider: the configured default
		// provider is the only sensible owner of it.
		if cfg.Provider == "" {
			return ModelRef{}, errProviderNotEnabled(model)
		}
		out.Provider, out.Model = cfg.Provider, model

	case provider != "":
		// The reverse: a provider with no model. The configured model belongs
		// to a different provider, so it cannot be borrowed; the last resort is
		// the only thing left that is not a guess about somebody's catalogue.
		out.Provider = provider
		out.Model = LastResortModel
		if provider == cfg.Provider && cfg.Model != "" {
			out.Model = cfg.Model
		}

	case cfg.Provider != "" && cfg.Model != "":
		out.Provider, out.Model = cfg.Provider, cfg.Model

	case cfg.Provider != "":
		out.Provider, out.Model = cfg.Provider, LastResortModel

	default:
		return ModelRef{}, errProviderNotEnabled("")
	}
	return out, nil
}

// split reads the "{model} ({provider})" form, returning the bare model when
// the string is not in that form.
func split(model string) (string, string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", ""
	}
	if m := modelWithProvider.FindStringSubmatch(model); m != nil {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
	}
	return model, ""
}

func levelOf(s string) ReasoningLevel {
	switch ReasoningLevel(strings.ToLower(strings.TrimSpace(s))) {
	case ReasoningNone:
		return ReasoningNone
	case ReasoningLow:
		return ReasoningLow
	case ReasoningHigh:
		return ReasoningHigh
	case ReasoningMedium:
		return ReasoningMedium
	default:
		return DefaultReasoning
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
