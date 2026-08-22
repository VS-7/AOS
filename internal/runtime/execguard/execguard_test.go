package execguard_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/runtime/execguard"
	"github.com/OWNER/aos/internal/runtime/sandbox"
)

// fakeRunner is a minimal sandbox.CommandRunner — enough to prove With/From
// round-trip the value attached, without needing a real sandbox.
type fakeRunner struct{}

func (fakeRunner) Run(context.Context, sandbox.Command) (sandbox.Result, error) {
	return sandbox.Result{}, nil
}

func TestFromReadsWhatWithAttached(t *testing.T) {
	want := fakeRunner{}
	ctx := execguard.With(context.Background(), want)

	got, ok := execguard.From(ctx)
	if !ok {
		t.Fatal("From reported nothing attached, after With")
	}
	if got != sandbox.CommandRunner(want) {
		t.Fatalf("From returned a different runner than With attached")
	}
}

func TestFromReportsAbsenceOutsideATurn(t *testing.T) {
	_, ok := execguard.From(context.Background())
	if ok {
		t.Fatal("From reported a runner attached to a bare context")
	}
}

func TestFromReportsAbsenceAfterAnUnrelatedContextValue(t *testing.T) {
	type otherKey struct{}
	ctx := context.WithValue(context.Background(), otherKey{}, "irrelevant")
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	_, ok := execguard.From(ctx)
	if ok {
		t.Fatal("From reported a runner attached to a context that never had one")
	}
}
