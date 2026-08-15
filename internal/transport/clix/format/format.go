package format

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/OWNER/aos/internal/core/tokens"
)

// Format is one of the five output formats of the original CLI.
type Format string

const (
	TOON  Format = "toon" // compact, token-efficient — the default on a TTY
	JSON  Format = "json"
	YAML  Format = "yaml"
	MD    Format = "md"
	JSONL Format = "jsonl"
)

// All lists the formats, for flag help and completion.
var All = []Format{TOON, JSON, YAML, MD, JSONL}

// Parse validates a format name.
func Parse(s string) (Format, error) {
	f := Format(strings.ToLower(strings.TrimSpace(s)))
	for _, known := range All {
		if f == known {
			return f, nil
		}
	}
	return "", fmt.Errorf("unknown format %q, expected one of: %s", s, Join(All))
}

// Join renders the format list for a help string.
func Join(fs []Format) string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = string(f)
	}
	return strings.Join(out, ", ")
}

// Options control one rendering.
type Options struct {
	Format Format

	// Filter selects paths out of the result: "foo,bar.baz,a[0,3]".
	Filter string

	// TokenLimit and TokenOffset page the rendered text by token budget. No
	// conventional CLI paginates by tokens; here the primary consumer is a
	// model with a finite context window.
	TokenLimit  int
	TokenOffset int
}

// Result is rendered output plus what the caller needs to ask for more.
type Result struct {
	Text   string
	Tokens int
	More   bool
}

// Render turns a command result into text.
func Render(v any, opts Options) (Result, error) {
	normalized, err := Normalize(v)
	if err != nil {
		return Result{}, err
	}
	if opts.Filter != "" {
		normalized = Filter(normalized, ParseFilter(opts.Filter))
	}

	var text string
	switch opts.Format {
	case JSON:
		raw, err := json.MarshalIndent(normalized, "", "  ")
		if err != nil {
			return Result{}, err
		}
		text = string(raw)
	case YAML:
		raw, err := yaml.Marshal(toPlain(normalized))
		if err != nil {
			return Result{}, err
		}
		text = strings.TrimRight(string(raw), "\n")
	case JSONL:
		text, err = encodeJSONL(normalized)
		if err != nil {
			return Result{}, err
		}
	case MD:
		text = EncodeMarkdown(normalized)
	default:
		text = EncodeTOON(normalized)
	}

	full := tokens.Estimate(text)
	if opts.TokenLimit > 0 || opts.TokenOffset > 0 {
		sliced, more := tokens.Slice(text, opts.TokenLimit, opts.TokenOffset)
		return Result{Text: sliced, Tokens: full, More: more}, nil
	}
	return Result{Text: text, Tokens: full}, nil
}

func encodeJSONL(v any) (string, error) {
	arr, ok := v.([]any)
	if !ok {
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	lines := make([]string, 0, len(arr))
	for _, item := range arr {
		raw, err := json.Marshal(item)
		if err != nil {
			return "", err
		}
		lines = append(lines, string(raw))
	}
	return strings.Join(lines, "\n"), nil
}

// toPlain converts the ordered tree into what yaml.Marshal understands, using
// yaml.MapSlice-like ordering so the YAML keeps the field order too.
func toPlain(v any) any {
	switch t := v.(type) {
	case *Object:
		node := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range t.Keys() {
			val, _ := t.Get(k)
			key := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
			child, err := yamlNode(val)
			if err != nil {
				continue
			}
			node.Content = append(node.Content, key, child)
		}
		return node
	case []any:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range t {
			child, err := yamlNode(item)
			if err != nil {
				continue
			}
			node.Content = append(node.Content, child)
		}
		return node
	default:
		return v
	}
}

func yamlNode(v any) (*yaml.Node, error) {
	if node, ok := toPlain(v).(*yaml.Node); ok {
		return node, nil
	}
	node := &yaml.Node{}
	if err := node.Encode(scalarGo(v)); err != nil {
		return nil, err
	}
	return node, nil
}

// scalarGo unwraps a json.Number into the narrowest Go type, so YAML prints 5
// rather than "5".
func scalarGo(v any) any {
	n, ok := v.(json.Number)
	if !ok {
		return v
	}
	if i, err := n.Int64(); err == nil {
		return i
	}
	if f, err := n.Float64(); err == nil {
		return f
	}
	return n.String()
}
