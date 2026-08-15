package fscollections

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// matchesFilters compares front-matter fields for equality by their JSON name.
//
// Comparison is by rendered value rather than by Go type, because a filter
// arrives from a CLI flag, a JSON body or a tool payload — where "0.9" and 0.9
// and a float64 all mean the same thing, and refusing one of them would make
// the same query behave differently on two surfaces.
func matchesFilters[T any](v *T, filters map[string]any) bool {
	for name, want := range filters {
		got, ok := collections.FieldOf(v, name)
		if !ok {
			return false
		}
		if !valueMatches(got, want) {
			return false
		}
	}
	return true
}

func valueMatches(got, want any) bool {
	if equalValues(got, want) {
		return true
	}
	// A slice field matches when it contains the wanted value: filtering
	// memories by one tag is the common case.
	rv := reflect.ValueOf(got)
	if rv.Kind() == reflect.Slice {
		for i := 0; i < rv.Len(); i++ {
			if equalValues(rv.Index(i).Interface(), want) {
				return true
			}
		}
	}
	return false
}

func equalValues(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	return renderValue(a) == renderValue(b)
}

func renderValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	case *time.Time:
		if t == nil {
			return ""
		}
		return t.UTC().Format(time.RFC3339Nano)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	case float32:
		return renderValue(float64(t))
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.String {
		return rv.String()
	}
	return fmt.Sprint(v)
}

// compareValues orders two front-matter values. It returns -1, 0 or 1 and
// falls back to the rendered form for anything it does not know, so ordering by
// an arbitrary field never panics and never returns an unstable order.
func compareValues(a, b any) int {
	if at, ok := asTime(a); ok {
		if bt, ok := asTime(b); ok {
			switch {
			case at.Before(bt):
				return -1
			case at.After(bt):
				return 1
			default:
				return 0
			}
		}
	}
	if af, ok := asFloat(a); ok {
		if bf, ok := asFloat(b); ok {
			switch {
			case af < bf:
				return -1
			case af > bf:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(renderValue(a), renderValue(b))
}

func asTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, true
	}
	return time.Time{}, false
}

func asFloat(v any) (float64, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), true
	}
	return 0, false
}
