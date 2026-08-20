package collections

// CloneJSON deep-copies a value shaped by encoding/json.Unmarshal or
// yaml.Unmarshal into an any: maps, slices, and scalars, recursively.
//
// A value read off a Record's Fields — or off any other JSON/YAML-decoded
// tree this engine hands a caller — nests: a "list" field is a []any, an
// object is a map[string]any, and either can contain the other arbitrarily
// deep. A one-level copy (make a new outer map, copy each value across) was
// tried first, here and in two other places in this codebase, and is not
// enough: the values it copies by reference are themselves maps and slices,
// so a caller mutating a nested object still reaches whatever this engine's
// in-memory index — or, for a caller that isn't this package, some other
// process-lifetime cache — is holding underneath it. There is no depth at
// which a shallow copy of a decoded document is safe to call finished; it has
// to be copied all the way down.
func CloneJSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = CloneJSON(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = CloneJSON(val)
		}
		return out
	default:
		// string, float64, bool, nil, time.Time, ...: copied by value already.
		return t
	}
}
