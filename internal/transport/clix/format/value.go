// Package format renders a command result in the five output formats of the
// original CLI, filters it by path and pages it by token budget.
package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Object is a JSON object that remembers the order of its keys.
//
// Field order is part of the output: the original prints a gateway status as
// status, pid, port, startedAt, version — the declaration order of the struct,
// not the alphabetical order of a Go map. Rendering through map[string]any
// would shuffle it, and every golden file would depend on nothing.
type Object struct {
	keys []string
	vals map[string]any
}

// NewObject builds an empty ordered object.
func NewObject() *Object {
	return &Object{vals: map[string]any{}}
}

// Set adds or replaces a key, keeping first-insertion order.
func (o *Object) Set(key string, value any) {
	if _, exists := o.vals[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = value
}

// Get returns a value and whether it exists.
func (o *Object) Get(key string) (any, bool) {
	v, ok := o.vals[key]
	return v, ok
}

// Keys returns the keys in order.
func (o *Object) Keys() []string { return o.keys }

// Len reports how many keys the object has.
func (o *Object) Len() int { return len(o.keys) }

// MarshalJSON writes the object back in its original order.
func (o *Object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		val, err := json.Marshal(o.vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(val)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// Normalize turns any Go value into an ordered tree of Object, []any and
// scalars, by way of its JSON representation. Going through JSON is what makes
// every surface agree on field names: the json tags are the contract.
func Normalize(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return decodeOrdered(bytes.NewReader(raw))
}

func decodeOrdered(r io.Reader) (any, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return readValue(dec)
}

func readValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return readFrom(dec, tok)
}

func readFrom(dec *json.Decoder, tok json.Token) (any, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := NewObject()
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyTok.(string)
				if !ok {
					return nil, fmt.Errorf("object key is not a string: %v", keyTok)
				}
				val, err := readValue(dec)
				if err != nil {
					return nil, err
				}
				obj.Set(key, val)
			}
			if _, err := dec.Token(); err != nil { // consume '}'
				return nil, err
			}
			return obj, nil
		case '[':
			arr := []any{}
			for dec.More() {
				val, err := readValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			if _, err := dec.Token(); err != nil { // consume ']'
				return nil, err
			}
			return arr, nil
		}
		return nil, fmt.Errorf("unexpected delimiter %v", t)
	default:
		return tok, nil
	}
}

// IsScalar reports whether a normalised value is a leaf.
func IsScalar(v any) bool {
	switch v.(type) {
	case *Object, []any:
		return false
	default:
		return true
	}
}

// numberString renders a json.Number without losing precision.
func numberString(n json.Number) string {
	s := n.String()
	// Trim a trailing ".0" so an integer that travelled through a float field
	// prints as an integer, which is what the original shows.
	if strings.HasSuffix(s, ".0") {
		return strings.TrimSuffix(s, ".0")
	}
	return s
}
