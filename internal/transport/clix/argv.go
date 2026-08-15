package clix

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/command"
)

// CommandLineFor renders the argv that runs a descriptor with a given payload.
//
// It exists because the same payload has to be expressible on every surface:
// the examples in --help and in the generated documentation must be lines a
// user can paste, and the parity suite uses it to drive the terminal surface
// with the very payload the other surfaces receive. Building the line by hand
// in two places is how the two drift apart.
func CommandLineFor(d command.Descriptor, payload json.RawMessage) ([]string, error) {
	var fields map[string]json.RawMessage
	if len(payload) > 0 && string(payload) != "null" {
		if err := json.Unmarshal(payload, &fields); err != nil {
			return nil, fmt.Errorf("%s: payload is not an object: %w", d.Key(), err)
		}
	}

	argv := append([]string{}, d.Path()...)
	b := newBinder(d.InputType())

	for _, arg := range b.args {
		raw, ok := fields[arg.name]
		if !ok {
			continue
		}
		text, err := literal(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", d.Key(), arg.name, err)
		}
		argv = append(argv, text)
		delete(fields, arg.name)
	}

	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		// A surface-private field has no flag: the reasoning of a human is the
		// command they typed.
		if strings.HasPrefix(name, "_") {
			continue
		}
		raw := fields[name]
		text, err := literal(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", d.Key(), name, err)
		}
		// A boolean flag is set by its presence, and pflag rejects a bare
		// "--flag value" for one.
		if text == "true" && isBool(raw) {
			argv = append(argv, "--"+name)
			continue
		}
		if text == "false" && isBool(raw) {
			continue
		}
		argv = append(argv, "--"+name, text)
	}
	return argv, nil
}

// literal renders one JSON value the way the flag parser expects to read it:
// scalars bare, anything structured as JSON.
func literal(raw json.RawMessage) (string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	switch t := v.(type) {
	case string:
		return t, nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	case float64:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%f", t), "0"), "."), nil
	case nil:
		return "", nil
	case []any:
		// A slice of strings is a repeatable flag; anything else is JSON.
		strs := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				return string(raw), nil
			}
			strs = append(strs, s)
		}
		return strings.Join(strs, ","), nil
	default:
		return string(raw), nil
	}
}

func isBool(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "true" || s == "false"
}
