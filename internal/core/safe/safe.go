// Package safe contains the panic boundary of the system.
//
// A panic is a bug, but a bug in one feature must not take down a daemon that
// serves N workspaces. The original calls process.exit(1) on an unhandled
// rejection (defect #16); here a panic degrades one operation.
//
// Recovery happens at three boundaries and nowhere else: the HTTP handler, the
// job worker, and a tool call inside the agent loop.
package safe

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/logging"
)

// PanicError wraps a recovered panic together with the stack captured at the
// point of recovery. It never reaches an end user verbatim: the surface layer
// turns it into a 500, a failed job, or a tool error.
type PanicError struct {
	Name  string
	Value any
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic in %s: %v", e.Name, e.Value)
}

// Do runs fn on the current goroutine and converts a panic into an error.
func Do(ctx context.Context, name string, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = report(ctx, name, r)
		}
	}()
	return fn(ctx)
}

// Go runs fn in a goroutine that recovers from panics, logs the stack with the
// ambient request id, and reports the incident. It never swallows the failure
// silently: the returned channel receives the wrapped panic error.
//
// The channel is buffered, so an abandoned caller cannot leak the goroutine.
func Go(ctx context.Context, name string, fn func(context.Context) error) <-chan error {
	ch := make(chan error, 1)
	go func() {
		defer close(ch)
		if err := Do(ctx, name, fn); err != nil {
			ch <- err
		}
	}()
	return ch
}

func report(ctx context.Context, name string, r any) error {
	stack := debug.Stack()
	logging.FromContext(ctx).Error("recovered panic",
		"component", name,
		"panic", fmt.Sprint(r),
		"stack", string(stack),
	)
	pe := &PanicError{Name: name, Value: r, Stack: stack}
	return apperr.New("INTERNAL_PANIC").
		Causer(name).
		Msgf("internal error while running %s", name).
		Wrap(pe)
}
