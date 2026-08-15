package apperr_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func TestNewAppliesBrandPrefix(t *testing.T) {
	e := apperr.New("MEMORY_NOT_FOUND")
	want := build.ErrorPrefix + "_MEMORY_NOT_FOUND"
	if e.Code != want {
		t.Fatalf("code = %q, want %q", e.Code, want)
	}
	// A code that already carries the prefix round-trips unchanged, so an
	// error parsed back from the wire is not double-prefixed.
	if again := apperr.New(want); again.Code != want {
		t.Fatalf("re-qualified code = %q, want %q", again.Code, want)
	}
}

func TestFluentBuilderFillsEveryField(t *testing.T) {
	cause := errors.New("disk on fire")
	e := apperr.New("MEMORY_NOT_FOUND").
		Causer("memory.Service.Get").
		Msgf("memory %q does not exist", "m1").
		Issue("id", "m1").
		Status(apperr.StatusNotFound).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "recall", Tool: "memories_recall"})

	if e.CauserName != "memory.Service.Get" {
		t.Errorf("causer = %q", e.CauserName)
	}
	if e.Message != `memory "m1" does not exist` {
		t.Errorf("message = %q", e.Message)
	}
	if e.Issues["id"] != "m1" {
		t.Errorf("issue = %v", e.Issues)
	}
	if len(e.Actions) != 1 {
		t.Errorf("cta count = %d", len(e.Actions))
	}
	if e.Error() != e.Code+": "+e.Message {
		t.Errorf("Error() = %q", e.Error())
	}
	if !errors.Is(e, cause) {
		t.Error("wrapped cause is not reachable with errors.Is")
	}
	if !errors.Is(e, apperr.ErrNotFound) {
		t.Error("404 does not unwrap to ErrNotFound")
	}
}

func TestSentinelFollowsStatus(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusBadRequest, apperr.ErrInvalid},
		{http.StatusUnauthorized, apperr.ErrUnauthorized},
		{http.StatusForbidden, apperr.ErrForbidden},
		{http.StatusNotFound, apperr.ErrNotFound},
		{http.StatusConflict, apperr.ErrConflict},
		{http.StatusServiceUnavailable, apperr.ErrUnavailable},
		{http.StatusInternalServerError, apperr.ErrInternal},
	}
	for _, c := range cases {
		e := apperr.New("X").Causer("test").Status(c.status)
		if !errors.Is(e, c.want) {
			t.Errorf("status %d did not unwrap to %v", c.status, c.want)
		}
	}
}

func TestKindOverridesStatus(t *testing.T) {
	e := apperr.New("X").Causer("test").Status(http.StatusInternalServerError).Kind(apperr.ErrConflict)
	if !errors.Is(e, apperr.ErrConflict) {
		t.Error("explicit kind was ignored")
	}
	if errors.Is(e, apperr.ErrInternal) {
		t.Error("status sentinel should not apply once kind is set")
	}
}

func TestStatusOfDefaultsToInternal(t *testing.T) {
	if got := apperr.StatusOf(errors.New("plain")); got != http.StatusInternalServerError {
		t.Fatalf("StatusOf(plain) = %d", got)
	}
	e := apperr.New("X").Causer("test").Status(http.StatusConflict)
	if got := apperr.StatusOf(e); got != http.StatusConflict {
		t.Fatalf("StatusOf(conflict) = %d", got)
	}
}

func TestAsFindsErrorThroughWrapping(t *testing.T) {
	inner := apperr.New("INNER").Causer("test").Status(apperr.StatusNotFound)
	outer := errors.Join(errors.New("context"), inner)
	got, ok := apperr.As(outer)
	if !ok || got.Code != inner.Code {
		t.Fatalf("As did not find the wrapped error: %v %v", got, ok)
	}
}
