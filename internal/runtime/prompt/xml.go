package prompt

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The dialect's two constants, both taken from the original: two spaces per
// level, and the length above which a string is emitted on its own lines.
const (
	indentUnit    = "  "
	longTextLimit = 80
)

// Value is one node of the context document.
//
// The type exists because Go maps do not keep insertion order and this document
// is compared against a golden file. An ordered tree makes the output a
// function of the code that built it rather than of the runtime's hash seed.
type Value interface{ node() }

// Text is a leaf.
type Text string

// List is a sequence of values under one tag.
type List []Value

// Object is an ordered set of fields. A key beginning with "@" becomes an
// attribute and the key "#" becomes the element's text, which is the dialect
// the original's XMLParser defines.
type Object []Field

// Field is one entry of an Object.
type Field struct {
	Key   string
	Value Value
}

func (Text) node()   {}
func (List) node()   {}
func (Object) node() {}

// Attr is shorthand for an attribute field.
func Attr(name, value string) Field { return Field{Key: "@" + name, Value: Text(value)} }

// Body is shorthand for the text-content field.
func Body(text string) Field { return Field{Key: "#", Value: Text(text)} }

// Strings lifts a slice of identifiers into a List, which is how the inventory
// carries names.
func Strings(items []string) List {
	out := make(List, 0, len(items))
	for _, s := range items {
		out = append(out, Text(s))
	}
	return out
}

// Encode serializes a value into the context XML dialect.
//
// Every text node is escaped. That is the second line of defence of the whole
// prompt: a memory whose body closes a tag and opens a trusted one must come
// out as characters, not as structure, and there is a test that says so.
func Encode(v any, tag string, indent int) (string, error) {
	node, err := coerce(v)
	if err != nil {
		return "", err
	}
	return encode(node, tag, indent), nil
}

// coerce accepts the ordinary Go values a caller is likely to have, so that a
// section does not have to be written as a tree by hand.
//
// A map is accepted and emitted with its keys sorted. Sorting is not the
// original's order — JavaScript keeps insertion order and Go does not — but it
// is the only order that is the same on two runs, and a prompt that differs
// between runs cannot have a golden file.
func coerce(v any) (Value, error) {
	switch t := v.(type) {
	case nil:
		return Text(""), nil
	case Value:
		return t, nil
	case string:
		return Text(t), nil
	case bool:
		return Text(strconv.FormatBool(t)), nil
	case int:
		return Text(strconv.Itoa(t)), nil
	case int64:
		return Text(strconv.FormatInt(t, 10)), nil
	case float64:
		return Text(strconv.FormatFloat(t, 'f', -1, 64)), nil
	case []string:
		return Strings(t), nil
	case []any:
		out := make(List, 0, len(t))
		for _, item := range t {
			node, err := coerce(item)
			if err != nil {
				return nil, err
			}
			out = append(out, node)
		}
		return out, nil
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(Object, 0, len(keys))
		for _, k := range keys {
			node, err := coerce(t[k])
			if err != nil {
				return nil, err
			}
			out = append(out, Field{Key: k, Value: node})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("prompt: %T cannot be part of a context document", v)
	}
}

func encode(v Value, tag string, level int) string {
	indent := strings.Repeat(indentUnit, level)
	switch t := v.(type) {
	case Text:
		return encodeText(string(t), tag, indent)
	case List:
		return encodeList(t, tag, level, indent)
	case Object:
		return encodeObject(t, tag, level, indent)
	}
	return ""
}

func encodeText(raw, tag, indent string) string {
	if raw == "" {
		return ""
	}
	if tag == "" {
		return indent + escape(raw)
	}
	if len(raw) > longTextLimit || strings.Contains(raw, "\n") {
		trimmed := strings.TrimRight(raw, "\n")
		inner := indent + indentUnit
		lines := strings.Split(escape(trimmed), "\n")
		for i, line := range lines {
			lines[i] = inner + line
		}
		return indent + "<" + tag + ">\n" + strings.Join(lines, "\n") + "\n" + indent + "</" + tag + ">"
	}
	return indent + "<" + tag + ">" + escape(raw) + "</" + tag + ">"
}

// encodeList reproduces the original's optimisation, which is also a reduction
// of markup: a list of plain strings repeats its own tag instead of nesting an
// <item> level under it. On a workspace with two hundred skill names that is
// two hundred tags saved, and the model reads the same thing.
func encodeList(items List, tag string, level int, indent string) string {
	if len(items) == 0 {
		return ""
	}
	wrapper := tag
	if wrapper == "" {
		wrapper = "list"
	}

	if tag != "" && allText(items) {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			if s := encode(item, wrapper, level); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n")
	}

	item := singular(wrapper)
	parts := make([]string, 0, len(items))
	for _, v := range items {
		if s := encode(v, item, level+1); s != "" {
			parts = append(parts, s)
		}
	}
	inner := strings.Join(parts, "\n")
	if inner == "" {
		return ""
	}
	if wrapper == item || tag == "" {
		return inner
	}
	return indent + "<" + wrapper + ">\n" + inner + "\n" + indent + "</" + wrapper + ">"
}

func encodeObject(obj Object, tag string, level int, indent string) string {
	var attrs strings.Builder
	var text string
	children := make([]string, 0, len(obj))

	for _, f := range obj {
		switch {
		case strings.HasPrefix(f.Key, "@"):
			if s, ok := f.Value.(Text); ok && s != "" {
				attrs.WriteString(" " + f.Key[1:] + `="` + escape(string(s)) + `"`)
			}
		case f.Key == "#":
			if s, ok := f.Value.(Text); ok {
				text = escape(string(s))
			}
		case strings.HasPrefix(f.Key, "_"):
			// The original skips keys beginning with an underscore, which is
			// how it carries bookkeeping that must not reach the model.
		default:
			if s := encode(f.Value, f.Key, childLevel(tag, level)); s != "" {
				children = append(children, s)
			}
		}
	}

	if tag == "" {
		return strings.Join(children, "\n")
	}

	body := strings.Join(children, "\n")
	switch {
	case text == "" && body == "":
		return indent + "<" + tag + attrs.String() + "></" + tag + ">"
	case text != "" && body == "":
		if len(text) > longTextLimit || strings.Contains(text, "\n") {
			trimmed := strings.TrimRight(text, "\n")
			inner := indent + indentUnit
			lines := strings.Split(trimmed, "\n")
			for i, line := range lines {
				lines[i] = inner + line
			}
			return indent + "<" + tag + attrs.String() + ">\n" + strings.Join(lines, "\n") + "\n" + indent + "</" + tag + ">"
		}
		return indent + "<" + tag + attrs.String() + ">" + text + "</" + tag + ">"
	default:
		return indent + "<" + tag + attrs.String() + ">\n" + body + "\n" + indent + "</" + tag + ">"
	}
}

func childLevel(tag string, level int) int {
	if tag == "" {
		return level
	}
	return level + 1
}

func allText(items List) bool {
	for _, v := range items {
		if _, ok := v.(Text); !ok {
			return false
		}
	}
	return true
}

// singular derives the tag of a list item from the tag of the list, as the
// original does.
func singular(tag string) string {
	switch {
	case tag == "structure":
		return "node"
	case tag == "list":
		return "item"
	case strings.HasSuffix(tag, "ies"):
		return tag[:len(tag)-3] + "y"
	case strings.HasSuffix(tag, "s") && len(tag) > 1:
		return tag[:len(tag)-1]
	default:
		return "item"
	}
}

// escape covers the four characters the original escapes. The apostrophe is
// deliberately left alone: attribute values are written with double quotes, so
// escaping it would only add noise to every possessive in the prompt.
func escape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	).Replace(s)
}
