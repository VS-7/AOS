package bot

import "regexp"

// envPlaceholder is the one interpolation syntax this package understands:
// ${env.VAR_NAME}. Mirrors internal/domain/toolset's own — see EnvResolver's
// doc comment on why this is a duplicate rather than an import.
var envPlaceholder = regexp.MustCompile(`\$\{env\.([A-Za-z_][A-Za-z0-9_]*)\}`)

// interpolate resolves a ${env.VAR} placeholder in text against src. A bare
// token with no placeholder syntax is returned unchanged and flagged
// literal=true, which is what triggers the boot-time warning the design doc
// asks for: a literal secret in an agent's versioned frontmatter is not
// itself an error, but it is a mistake worth surfacing.
func interpolate(text string, src EnvResolver) (resolved string, literal bool, err error) {
	if !envPlaceholder.MatchString(text) {
		return text, text != "", nil
	}
	var missing string
	out := envPlaceholder.ReplaceAllStringFunc(text, func(match string) string {
		if missing != "" {
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
		return "", false, errEnvMissing(missing)
	}
	return out, false, nil
}
