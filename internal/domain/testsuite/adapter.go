package testsuite

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/domain/toolset"
)

// AdapterContract is what every toolset connection type must satisfy. It
// exists now, with one implementer (mcp-server::stdio, in
// internal/adapters/mcpclient), so the other four types this slice does not
// implement are added later by passing this rather than by being reviewed by
// eye — exactly the role RunRepositoryContract plays for collections.Repository.
type AdapterContract struct {
	// Name identifies the implementation in the subtests this suite creates.
	Name string

	// New returns a fresh, unconnected Adapter.
	New func(t *testing.T) toolset.Adapter

	// Toolset is a configuration New's adapter can actually connect to —
	// pointed at a real, reachable target for the duration of the test.
	Toolset toolset.Toolset

	// Invalid is a configuration of the same Type that Connect cannot reach:
	// a command that does not exist, a host that refuses the connection.
	// RunAdapterContract uses it to prove Connect fails fast instead of
	// hanging forever — the one failure mode a caller of Service.Call has no
	// way to bound from outside.
	Invalid toolset.Toolset

	// ExpectTool is a tool the connected target is known to publish.
	ExpectTool string
}

// connectTimeout bounds every Connect call this suite makes. It exists so
// that an implementation which blocks instead of erroring fails the test
// instead of hanging the suite — the exact defect the last subtest targets.
const connectTimeout = 15 * time.Second

// RunAdapterContract exercises the behavioural contract every toolset.Adapter
// implementation must satisfy.
func RunAdapterContract(t *testing.T, c AdapterContract) {
	t.Helper()

	t.Run(c.Name+"/connects to a valid target", func(t *testing.T) {
		a := c.New(t)
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()
		if err := a.Connect(ctx, c.Toolset); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		if err := a.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	t.Run(c.Name+"/ListTools returns at least the expected tool", func(t *testing.T) {
		a := c.New(t)
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()
		if err := a.Connect(ctx, c.Toolset); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		defer func() { _ = a.Close() }()

		tools, err := a.ListTools(ctx)
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}
		found := false
		names := make([]string, len(tools))
		for i, spec := range tools {
			names[i] = spec.Name
			if spec.Name == c.ExpectTool {
				found = true
			}
		}
		if !found {
			t.Fatalf("ListTools did not publish %q, got %v", c.ExpectTool, names)
		}
	})

	t.Run(c.Name+"/calling a tool that does not exist fails without crashing", func(t *testing.T) {
		a := c.New(t)
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()
		if err := a.Connect(ctx, c.Toolset); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		defer func() { _ = a.Close() }()

		if _, err := a.Call(ctx, "this-tool-does-not-exist-anywhere", nil); err == nil {
			t.Fatal("calling an unknown tool reported success")
		}

		// An error return alone cannot distinguish "errored gracefully" from
		// "errored and took the connection down with it" — both look like a
		// non-nil error from the line above. Proving the connection still
		// answers afterwards is the only way to tell those apart, and that
		// distinction is the whole reason this suite exists instead of a
		// reading of the adapter's code.
		tools, err := a.ListTools(ctx)
		if err != nil {
			t.Fatalf("ListTools after a failed call: %v — the connection did not survive it", err)
		}
		found := false
		for _, spec := range tools {
			if spec.Name == c.ExpectTool {
				found = true
			}
		}
		if !found {
			t.Fatalf("ListTools after a failed call no longer reports %q — the connection did not survive it", c.ExpectTool)
		}
	})

	t.Run(c.Name+"/Close is idempotent", func(t *testing.T) {
		a := c.New(t)
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()
		if err := a.Connect(ctx, c.Toolset); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		if err := a.Close(); err != nil {
			t.Fatalf("first Close: %v", err)
		}
		if err := a.Close(); err != nil {
			t.Fatalf("second Close must not error: %v", err)
		}
	})

	t.Run(c.Name+"/connecting to an invalid target fails instead of hanging forever", func(t *testing.T) {
		a := c.New(t)
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()

		err := a.Connect(ctx, c.Invalid)
		_ = a.Close()
		if ctx.Err() != nil {
			t.Fatal("Connect did not return before the test's own timeout — it hung")
		}
		if err == nil {
			t.Fatal("Connect on an invalid target reported success")
		}
	})
}
