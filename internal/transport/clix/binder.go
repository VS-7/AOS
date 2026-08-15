package clix

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// binder maps an input struct to command-line flags and back.
//
// The mapping is reflection over the json tags, which is the same source the
// JSON schema and the HTTP body use. That is the point of the Command Layer: a
// field named once is named the same everywhere.
type binder struct {
	fields []boundField
	args   []boundField
}

type boundField struct {
	name  string // json name — the flag name
	help  string // jsonschema description — the flag help
	kind  bindKind
	typ   reflect.Type
	value any // pointer to the parsed value, filled by pflag
	arg   bool
}

type bindKind int

const (
	bindString bindKind = iota
	bindBool
	bindInt
	bindFloat
	bindStringSlice
	bindTime
	bindJSON // struct, map, slice of struct — accepted as a JSON string
)

// newBinder plans the flags of an input type.
func newBinder(t reflect.Type) *binder {
	b := &binder{}
	b.plan(t)
	return b
}

func (b *binder) plan(t reflect.Type) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if f.Anonymous {
			b.plan(f.Type)
			continue
		}
		name := jsonName(f)
		if name == "" || name == "-" {
			continue
		}
		// `_reasoning` is a tool-surface field. Asking a human at a terminal to
		// justify the command they just typed would be absurd, and the original
		// does not do it either: its CLI schema is args plus options, and the
		// tool schema is that plus `_reasoning`.
		if strings.HasPrefix(name, "_") {
			continue
		}
		bf := boundField{
			name: name,
			help: f.Tag.Get("jsonschema"),
			typ:  f.Type,
			kind: kindOf(f.Type),
			arg:  f.Tag.Get("cli") == "arg",
		}
		if bf.arg {
			b.args = append(b.args, bf)
			continue
		}
		b.fields = append(b.fields, bf)
	}
}

func kindOf(t reflect.Type) bindKind {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == reflect.TypeOf(time.Time{}) {
		return bindTime
	}
	switch t.Kind() {
	case reflect.String:
		return bindString
	case reflect.Bool:
		return bindBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return bindInt
	case reflect.Float32, reflect.Float64:
		return bindFloat
	case reflect.Slice:
		if t.Elem().Kind() == reflect.String {
			return bindStringSlice
		}
		return bindJSON
	default:
		return bindJSON
	}
}

// Bind declares the flags on a flag set.
func (b *binder) Bind(fs *pflag.FlagSet) {
	for i := range b.fields {
		f := &b.fields[i]
		help := f.help
		switch f.kind {
		case bindBool:
			f.value = fs.Bool(f.name, false, help)
		case bindInt:
			f.value = fs.Int64(f.name, 0, help)
		case bindFloat:
			f.value = fs.Float64(f.name, 0, help)
		case bindStringSlice:
			f.value = fs.StringSlice(f.name, nil, help)
		case bindJSON:
			// A []Rule does not exist on a command line. The original solves it
			// the same way: the field accepts a JSON string, decoded after the
			// parse. `--rules '[{"type":"always"}]'`.
			if help != "" {
				help += " "
			}
			f.value = fs.String(f.name, "", help+"(JSON)")
		case bindTime:
			f.value = fs.String(f.name, "", help+" (RFC3339)")
		default:
			f.value = fs.String(f.name, "", help)
		}
	}
}

// Collect turns the parsed flags and positional arguments into the JSON payload
// the descriptor expects. Only flags the user actually set are included, so an
// omitted optional field stays absent rather than becoming a zero value.
func (b *binder) Collect(fs *pflag.FlagSet, args []string) (json.RawMessage, error) {
	payload := map[string]any{}

	for i, f := range b.args {
		if i >= len(args) {
			break
		}
		v, err := parseArg(f, args[i])
		if err != nil {
			return nil, err
		}
		payload[f.name] = v
	}

	for _, f := range b.fields {
		if !fs.Changed(f.name) {
			continue
		}
		switch f.kind {
		case bindBool:
			payload[f.name] = *f.value.(*bool)
		case bindInt:
			payload[f.name] = *f.value.(*int64)
		case bindFloat:
			payload[f.name] = *f.value.(*float64)
		case bindStringSlice:
			payload[f.name] = *f.value.(*[]string)
		case bindJSON:
			raw := *f.value.(*string)
			var decoded any
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				return nil, fmt.Errorf("--%s expects JSON: %w", f.name, err)
			}
			payload[f.name] = decoded
		default:
			payload[f.name] = *f.value.(*string)
		}
	}

	// The reasoning of a human is the command they typed. The field is absent
	// from the CLI surface and the descriptor knows not to require it here.
	return json.Marshal(payload)
}

func parseArg(f boundField, raw string) (any, error) {
	switch f.kind {
	case bindJSON:
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return nil, fmt.Errorf("%s expects JSON: %w", f.name, err)
		}
		return decoded, nil
	default:
		return raw, nil
	}
}

// ArgNames returns the positional argument names, for the usage line.
func (b *binder) ArgNames() []string {
	out := make([]string, 0, len(b.args))
	for _, a := range b.args {
		out = append(out, a.name)
	}
	return out
}

func jsonName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return strings.ToLower(f.Name)
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}
