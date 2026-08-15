package format

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// EncodeTOON renders a normalised value as TOON — Token-Oriented Object
// Notation, the default output format of the original CLI.
//
// There is no mature Go implementation, so this is ours. The rules were read
// off the original binary running on this machine, comparing --format toon with
// --format json on the same command:
//
//	code: COMMAND_NOT_FOUND
//	message: 'themes' is not a command for 'fractal'.
//	cta:
//	  description: "Suggested command:"
//	  commands[1]{command,description}:
//	    fractal --help,see all available commands
//
// Four rules follow:
//
//  1. An object is YAML-like: "key: value", two spaces per level, in the
//     declaration order of the source.
//  2. An array declares its length: "key[N]:".
//  3. An array whose elements are objects with the same keys is tabular:
//     "key[N]{a,b}:" followed by comma-separated rows. That is where the format
//     earns its name — field names are written once instead of N times.
//  4. A non-uniform array falls back to a list of "- key: value" blocks.
//
// A scalar is quoted only when leaving it bare would be ambiguous.
func EncodeTOON(v any) string {
	if IsScalar(v) {
		return scalarTOON(v)
	}
	var b strings.Builder
	switch t := v.(type) {
	case *Object:
		encodeObject(&b, t, 0)
	case []any:
		encodeArray(&b, "", t, 0)
	}
	return strings.TrimRight(b.String(), "\n")
}

func encodeObject(b *strings.Builder, obj *Object, depth int) {
	for _, k := range obj.Keys() {
		val, _ := obj.Get(k)
		switch t := val.(type) {
		case *Object:
			fmt.Fprintf(b, "%s%s:\n", indent(depth), k)
			encodeObject(b, t, depth+1)
		case []any:
			encodeArray(b, k, t, depth)
		default:
			fmt.Fprintf(b, "%s%s: %s\n", indent(depth), k, scalarTOON(t))
		}
	}
}

func encodeArray(b *strings.Builder, key string, arr []any, depth int) {
	head := key + "[" + strconv.Itoa(len(arr)) + "]"
	if len(arr) == 0 {
		fmt.Fprintf(b, "%s%s:\n", indent(depth), head)
		return
	}

	if fields, ok := uniformFields(arr); ok {
		fmt.Fprintf(b, "%s%s{%s}:\n", indent(depth), head, strings.Join(fields, ","))
		for _, item := range arr {
			row := item.(*Object)
			cells := make([]string, 0, len(fields))
			for _, f := range fields {
				v, _ := row.Get(f)
				cells = append(cells, cellTOON(v))
			}
			fmt.Fprintf(b, "%s%s\n", indent(depth+1), strings.Join(cells, ","))
		}
		return
	}

	fmt.Fprintf(b, "%s%s:\n", indent(depth), head)
	for _, item := range arr {
		encodeListItem(b, item, depth+1)
	}
}

func encodeListItem(b *strings.Builder, item any, depth int) {
	obj, ok := item.(*Object)
	if !ok {
		if arr, isArr := item.([]any); isArr {
			encodeArray(b, "-", arr, depth)
			return
		}
		fmt.Fprintf(b, "%s- %s\n", indent(depth), scalarTOON(item))
		return
	}
	if obj.Len() == 0 {
		fmt.Fprintf(b, "%s-\n", indent(depth))
		return
	}
	for i, k := range obj.Keys() {
		lead := indent(depth) + "  "
		if i == 0 {
			lead = indent(depth) + "- "
		}
		val, _ := obj.Get(k)
		switch t := val.(type) {
		case *Object:
			fmt.Fprintf(b, "%s%s:\n", lead, k)
			encodeObject(b, t, depth+2)
		case []any:
			var nested strings.Builder
			encodeArray(&nested, k, t, 0)
			for j, line := range strings.Split(strings.TrimRight(nested.String(), "\n"), "\n") {
				pad := indent(depth) + "  "
				if j == 0 {
					pad = lead
				}
				fmt.Fprintf(b, "%s%s\n", pad, line)
			}
		default:
			fmt.Fprintf(b, "%s%s: %s\n", lead, k, scalarTOON(t))
		}
	}
}

// uniformFields reports the shared key order of an array of objects, when every
// element is an object with exactly the same keys in the same order, all of
// them scalar. That is the condition under which the original switches to the
// tabular form — a nested value could not fit in a row.
func uniformFields(arr []any) ([]string, bool) {
	var fields []string
	for i, item := range arr {
		obj, ok := item.(*Object)
		if !ok || obj.Len() == 0 {
			return nil, false
		}
		for _, k := range obj.Keys() {
			v, _ := obj.Get(k)
			if !IsScalar(v) {
				return nil, false
			}
		}
		if i == 0 {
			fields = obj.Keys()
			continue
		}
		keys := obj.Keys()
		if len(keys) != len(fields) {
			return nil, false
		}
		for j := range keys {
			if keys[j] != fields[j] {
				return nil, false
			}
		}
	}
	return fields, len(fields) > 0
}

func scalarTOON(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(t)
	case json.Number:
		return numberString(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case string:
		if needsQuote(t) {
			return strconv.Quote(t)
		}
		return t
	default:
		return fmt.Sprint(t)
	}
}

// cellTOON renders a value inside a tabular row, where a comma also separates
// fields and therefore forces a quote.
func cellTOON(v any) string {
	s, ok := v.(string)
	if !ok {
		return scalarTOON(v)
	}
	if strings.ContainsAny(s, ",\"\n") || needsQuote(s) {
		return strconv.Quote(s)
	}
	return s
}

func needsQuote(s string) bool {
	if s == "" || strings.TrimSpace(s) != s {
		return true
	}
	if strings.ContainsAny(s, ":\n\"") {
		return true
	}
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "#") {
		return true
	}
	// A string that would read back as another type must say it is a string.
	switch strings.ToLower(s) {
	case "true", "false", "null", "yes", "no":
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	return false
}

func indent(depth int) string { return strings.Repeat("  ", depth) }
