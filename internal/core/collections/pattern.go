package collections

import (
	"regexp"
	"sort"
	"strings"
)

// Pattern compiles a path template into a bidirectional mapper:
//
//	".aos/agents/{agent}/memories/{id}.memory.md"
//
// Reading:  path → Key{"agent":"luara","id":"a1b2"}
// Writing:  Key  → path
//
// A literal "*" segment matches one path element and captures nothing; it is
// how the skill-scoped variants work:
//
//	".aos/skills/*/agents/{agent}/memories/{id}.memory.md"
type Pattern struct {
	raw    string
	re     *regexp.Regexp
	fields []string
	glob   string
	wild   bool
}

type tokenKind int

const (
	tokLiteral tokenKind = iota
	tokField
	tokStar
	tokDoubleStar
)

type token struct {
	kind tokenKind
	text string // literal text, or field name
}

// Compile parses a path template. It fails on an unterminated or empty
// placeholder and on a duplicated field name, both of which would produce a
// pattern that reads or writes the wrong path.
func Compile(raw string) (*Pattern, error) {
	tokens, err := tokenize(raw)
	if err != nil {
		return nil, err
	}

	var (
		re     strings.Builder
		glob   strings.Builder
		fields []string
		seen   = map[string]bool{}
		wild   bool
	)
	re.WriteString("^")
	for _, t := range tokens {
		switch t.kind {
		case tokLiteral:
			re.WriteString(regexp.QuoteMeta(t.text))
			glob.WriteString(t.text)
		case tokField:
			if seen[t.text] {
				return nil, errPatternDuplicateField(raw, t.text)
			}
			seen[t.text] = true
			fields = append(fields, t.text)
			re.WriteString("([^/]+)")
			glob.WriteString("*")
		case tokStar:
			wild = true
			re.WriteString("[^/]+")
			glob.WriteString("*")
		case tokDoubleStar:
			wild = true
			re.WriteString(".+")
			glob.WriteString("**")
		}
	}
	re.WriteString("$")

	compiled, err := regexp.Compile(re.String())
	if err != nil {
		return nil, errPatternInvalid(raw, err)
	}
	return &Pattern{raw: raw, re: compiled, fields: fields, glob: glob.String(), wild: wild}, nil
}

// MustCompile is Compile for the static registry, where a failure is a
// programming error that must surface at start-up rather than at first use.
func MustCompile(raw string) *Pattern {
	p, err := Compile(raw)
	if err != nil {
		panic(err)
	}
	return p
}

func tokenize(raw string) ([]token, error) {
	var out []token
	var lit strings.Builder

	flush := func() {
		if lit.Len() > 0 {
			out = append(out, token{kind: tokLiteral, text: lit.String()})
			lit.Reset()
		}
	}

	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			end := strings.IndexByte(raw[i:], '}')
			if end < 0 {
				return nil, errPatternUnterminated(raw, i)
			}
			name := raw[i+1 : i+end]
			if name == "" || strings.ContainsAny(name, "{}/*") {
				return nil, errPatternBadField(raw, name)
			}
			flush()
			out = append(out, token{kind: tokField, text: name})
			i += end
		case '*':
			flush()
			if i+1 < len(raw) && raw[i+1] == '*' {
				out = append(out, token{kind: tokDoubleStar})
				i++
				continue
			}
			out = append(out, token{kind: tokStar})
		default:
			lit.WriteByte(raw[i])
		}
	}
	flush()
	return out, nil
}

// Raw returns the template this pattern was compiled from.
func (p *Pattern) Raw() string { return p.raw }

// Fields returns the placeholder names, in the order they appear.
func (p *Pattern) Fields() []string {
	out := make([]string, len(p.fields))
	copy(out, p.fields)
	return out
}

// Writable reports whether Build can produce a path from this pattern. A
// pattern containing a wildcard cannot: the wildcard captures nothing, so there
// is no value to put back. The skill-scoped variants are read-only by design.
func (p *Pattern) Writable() bool { return !p.wild }

// Match extracts placeholder values, or returns ok=false when the path does not
// belong to this pattern. The path must be relative to the workspace root and
// use forward slashes.
func (p *Pattern) Match(rel string) (Key, bool) {
	m := p.re.FindStringSubmatch(rel)
	if m == nil {
		return nil, false
	}
	k := make(Key, len(p.fields))
	for i, name := range p.fields {
		k[name] = m[i+1]
	}
	return k, true
}

// Build renders a path from placeholder values, failing when one is missing — a
// missing placeholder must never silently produce a wrong path.
func (p *Pattern) Build(k Key) (string, error) {
	if !p.Writable() {
		return "", errPatternNotWritable(p.raw)
	}
	tokens, err := tokenize(p.raw)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, t := range tokens {
		switch t.kind {
		case tokLiteral:
			b.WriteString(t.text)
		case tokField:
			v, ok := k[t.text]
			if !ok || v == "" {
				return "", errPatternMissingField(p.raw, t.text, k)
			}
			if strings.Contains(v, "/") {
				return "", errPatternFieldHasSeparator(p.raw, t.text, v)
			}
			b.WriteString(v)
		}
	}
	return b.String(), nil
}

// Glob returns the walk pattern used by Refresh, so the engine never scans
// directories that cannot contain records of this collection.
func (p *Pattern) Glob() string { return p.glob }

// Prefix returns the longest leading directory of the pattern that contains no
// placeholder or wildcard. Walking can start there instead of at the workspace
// root, which is what keeps Refresh off node_modules.
func (p *Pattern) Prefix() string {
	idx := strings.IndexAny(p.raw, "{*")
	if idx < 0 {
		return dirOf(p.raw)
	}
	head := p.raw[:idx]
	return dirOf(head)
}

func dirOf(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		return s[:i]
	}
	return ""
}

// SortedFields returns the placeholder names in a stable order, for building
// deterministic cache keys.
func (p *Pattern) SortedFields() []string {
	out := p.Fields()
	sort.Strings(out)
	return out
}
