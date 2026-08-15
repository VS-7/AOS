package safe_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/safe"
)

func TestDoConvertsPanicIntoError(t *testing.T) {
	err := safe.Do(context.Background(), "test.boom", func(context.Context) error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("panic was swallowed")
	}
	e, ok := apperr.As(err)
	if !ok || e.CauserName != "test.boom" {
		t.Fatalf("unexpected error: %v", err)
	}
	var pe *safe.PanicError
	if !errors.As(err, &pe) {
		t.Fatal("the panic value is not reachable")
	}
	if pe.Value != "boom" || len(pe.Stack) == 0 {
		t.Fatalf("panic error = %+v", pe)
	}
}

func TestDoPassesErrorsThroughUnchanged(t *testing.T) {
	sentinel := errors.New("ordinary failure")
	err := safe.Do(context.Background(), "test.ok", func(context.Context) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
}

func TestGoReportsPanicOnTheChannel(t *testing.T) {
	ch := safe.Go(context.Background(), "test.worker", func(context.Context) error {
		panic(errors.New("worker died"))
	})
	err, ok := <-ch
	if !ok || err == nil {
		t.Fatal("expected the panic on the channel")
	}
	if _, ok := apperr.As(err); !ok {
		t.Fatalf("error = %v", err)
	}
	if _, open := <-ch; open {
		t.Fatal("channel should be closed after the single result")
	}
}

// TestGoDoesNotLeakWhenTheCallerIgnoresTheChannel: the channel is buffered, so
// a caller that never reads cannot pin the goroutine forever.
func TestGoDoesNotLeakWhenTheCallerIgnoresTheChannel(t *testing.T) {
	for i := 0; i < 100; i++ {
		_ = safe.Go(context.Background(), "test.fire-and-forget", func(context.Context) error {
			return errors.New("ignored")
		})
	}
}
