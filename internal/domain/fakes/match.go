package fakes

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// matches and render mirror the filesystem adapter's filter semantics. They are
// duplicated rather than shared because the fake must not depend on the
// adapter: the contract suite is what keeps the two honest, and a shared helper
// would make the suite unable to catch a divergence in exactly this behaviour.
func matches[T any](v *T, filters map[string]any) bool {
	for name, want := range filters {
		got, ok := collections.FieldOf(v, name)
		if !ok {
			return false
		}
		if render(got) == render(want) {
			continue
		}
		rv := reflect.ValueOf(got)
		if rv.Kind() != reflect.Slice {
			return false
		}
		found := false
		for i := 0; i < rv.Len(); i++ {
			if render(rv.Index(i).Interface()) == render(want) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func render(v any) string {
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
		return render(float64(t))
	}
	return fmt.Sprint(v)
}
