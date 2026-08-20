package toolset

import "regexp"

// envPlaceholder is the one interpolation syntax this package understands:
// ${env.VAR_NAME}. Nothing else in a Toolset's configuration is templated.
var envPlaceholder = regexp.MustCompile(`\$\{env\.([A-Za-z_][A-Za-z0-9_]*)\}`)

// Interpolate resolves every ${env.VAR} placeholder in text against src.
//
// A missing variable fails the whole call, naming the variable, rather than
// substituting an empty string. The alternative — silently leaving it blank —
// was rejected: it produces a working-looking request that a remote server
// then rejects with a 401 or a 400, with nothing in that failure pointing back
// at the unset variable that actually caused it. Refusing here, by name, turns
// a debugging session into one line of configuration.
func Interpolate(text string, src EnvResolver) (string, error) {
	var missing string
	out := envPlaceholder.ReplaceAllStringFunc(text, func(match string) string {
		if missing != "" {
			// A variable already failed earlier in this string; stop
			// resolving further occurrences; the whole call is failing.
			return match
		}
		name := envPlaceholder.FindStringSubmatch(match)[1]
		val := src.String(name, "")
		if val == "" {
			missing = name
			return match
		}
		return val
	})
	if missing != "" {
		return "", errEnvMissing(missing)
	}
	return out, nil
}

// interpolateToolset resolves every ${env.VAR} placeholder across a Toolset's
// connection fields, returning a copy with its own Args, Env and Headers — the
// original is never mutated, and nothing downstream can reach back into what
// the repository or the caller is still holding.
func interpolateToolset(ts Toolset, src EnvResolver) (Toolset, error) {
	out := ts.Clone()

	var err error
	if out.Command, err = Interpolate(ts.Command, src); err != nil {
		return Toolset{}, err
	}
	if out.BaseURL, err = Interpolate(ts.BaseURL, src); err != nil {
		return Toolset{}, err
	}
	for i, a := range ts.Args {
		if out.Args[i], err = Interpolate(a, src); err != nil {
			return Toolset{}, err
		}
	}
	for k, v := range ts.Env {
		if out.Env[k], err = Interpolate(v, src); err != nil {
			return Toolset{}, err
		}
	}
	for k, v := range ts.Headers {
		if out.Headers[k], err = Interpolate(v, src); err != nil {
			return Toolset{}, err
		}
	}
	return out, nil
}
