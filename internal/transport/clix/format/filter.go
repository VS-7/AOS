package format

import (
	"strconv"
	"strings"
)

// A filter expression selects parts of a result: "foo,bar.baz,a[0,3]".
//
// The syntax is the original's, unchanged. That is deliberate: agents were
// taught this syntax by the tool descriptions of the original, and a model that
// already knows it should not have to learn a second one.

// Segment is one step of a filter path: a key, or a slice of an array.
type Segment struct {
	Key   string
	Slice bool
	Start int
	End   int
}

// ParseFilter turns an expression into paths. A malformed expression yields no
// paths, which renders the value unfiltered — a filter is a view, and a typo in
// a view should not hide the answer.
func ParseFilter(expr string) [][]Segment {
	var paths [][]Segment
	for _, token := range splitTop(expr) {
		if path := parsePath(token); len(path) > 0 {
			paths = append(paths, path)
		}
	}
	return paths
}

// splitTop splits on commas that are not inside brackets, because a comma
// inside "[0,3]" belongs to the slice.
func splitTop(expr string) []string {
	var out []string
	depth := 0
	current := strings.Builder{}
	for _, ch := range expr {
		switch ch {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, current.String())
				current.Reset()
				continue
			}
		}
		current.WriteRune(ch)
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}

func parsePath(token string) []Segment {
	var path []Segment
	remaining := strings.TrimSpace(token)
	for remaining != "" {
		open := strings.IndexByte(remaining, '[')
		if open < 0 {
			for _, part := range strings.Split(remaining, ".") {
				if part != "" {
					path = append(path, Segment{Key: part})
				}
			}
			break
		}
		for _, part := range strings.Split(remaining[:open], ".") {
			if part != "" {
				path = append(path, Segment{Key: part})
			}
		}
		closeIdx := strings.IndexByte(remaining[open:], ']')
		if closeIdx < 0 {
			break
		}
		closeIdx += open
		inner := remaining[open+1 : closeIdx]
		start, end := parseSlice(inner)
		path = append(path, Segment{Slice: true, Start: start, End: end})
		remaining = strings.TrimPrefix(remaining[closeIdx+1:], ".")
	}
	return path
}

func parseSlice(inner string) (int, int) {
	parts := strings.SplitN(inner, ",", 2)
	start := atoi(parts[0], 0)
	end := start + 1
	if len(parts) == 2 {
		end = atoi(parts[1], start+1)
	}
	return start, end
}

func atoi(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

// Filter applies the parsed paths to a normalised value.
func Filter(v any, paths [][]Segment) any {
	if len(paths) == 0 {
		return v
	}

	// One key selecting a scalar returns the scalar itself, so that
	// `--filter-output status` prints "running" and not "status: running".
	if len(paths) == 1 && len(paths[0]) == 1 && !paths[0][0].Slice {
		key := paths[0][0].Key
		switch t := v.(type) {
		case *Object:
			val, ok := t.Get(key)
			if !ok {
				return NewObject()
			}
			if IsScalar(val) {
				return val
			}
			out := NewObject()
			out.Set(key, val)
			return out
		case []any:
			out := make([]any, 0, len(t))
			for _, item := range t {
				out = append(out, Filter(item, paths))
			}
			return out
		default:
			return nil
		}
	}

	if arr, ok := v.([]any); ok {
		out := make([]any, 0, len(arr))
		for _, item := range arr {
			out = append(out, Filter(item, paths))
		}
		return out
	}

	result := NewObject()
	for _, path := range paths {
		mergePath(result, v, path, 0)
	}
	return result
}

func mergePath(target *Object, source any, path []Segment, index int) {
	if index >= len(path) {
		return
	}
	seg := path[index]
	if seg.Slice {
		// A leading slice has no key to store the result under; the caller
		// handles slices as part of the key that precedes them.
		return
	}

	obj, ok := source.(*Object)
	if !ok {
		return
	}
	val, exists := obj.Get(seg.Key)
	if !exists {
		return
	}

	// A slice immediately after this key applies to the value.
	if index+1 < len(path) && path[index+1].Slice {
		arr, isArr := val.([]any)
		if !isArr {
			return
		}
		cut := sliceOf(arr, path[index+1].Start, path[index+1].End)
		if index+2 >= len(path) {
			target.Set(seg.Key, cut)
			return
		}
		rest := make([]any, 0, len(cut))
		for _, item := range cut {
			child := NewObject()
			mergePath(child, item, path, index+2)
			rest = append(rest, child)
		}
		target.Set(seg.Key, rest)
		return
	}

	if index == len(path)-1 {
		target.Set(seg.Key, val)
		return
	}

	existing, _ := target.Get(seg.Key)
	child, ok := existing.(*Object)
	if !ok {
		child = NewObject()
		target.Set(seg.Key, child)
	}
	mergePath(child, val, path, index+1)
}

func sliceOf(arr []any, start, end int) []any {
	if start < 0 {
		start = 0
	}
	if start > len(arr) {
		start = len(arr)
	}
	if end > len(arr) {
		end = len(arr)
	}
	if end < start {
		end = start
	}
	out := make([]any, end-start)
	copy(out, arr[start:end])
	return out
}
