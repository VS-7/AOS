package app

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// Opening a workspace is a whole New — scaffold, index, reconcile, a repository
// for a managed one — and forID did it holding the lock every routed request
// takes. The worker's first pass opens every registered workspace right after
// the daemon starts, so each request, for any workspace, waited behind each of
// those in turn. A workspace being opened now holds up only the callers that
// want that same workspace, and they get the one being built rather than a
// second.
func TestOpeningOneWorkspaceDoesNotHoldUpAnother(t *testing.T) {
	home, first, slow, quick := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	a, err := New(Options{
		Env:           env.New(env.Map(map[string]string{env.KeyHome: home})),
		WorkspaceRoot: first,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	ctx := context.Background()
	for name, dir := range map[string]string{"Slow": slow, "Quick": quick} {
		if _, err := a.Workspaces.Create(ctx, workspace.CreateInput{Name: name, Path: dir}); err != nil {
			t.Fatal(err)
		}
	}

	build := a.scopes.build
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	letGo := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(letGo) // before Close, which would otherwise wait on the build
	var slowBuilds atomic.Int32
	a.scopes.build = func(id, root string) (*App, error) {
		if filepath.Clean(root) == filepath.Clean(slow) {
			if slowBuilds.Add(1) == 1 {
				close(entered)
			}
			<-release
		}
		return build(id, root)
	}

	type opened struct {
		app *App
		err error
	}
	open := func(id string) <-chan opened {
		out := make(chan opened, 1)
		go func() {
			got, err := a.scopes.forID(ctx, a.Workspaces, id)
			out <- opened{got, err}
		}()
		return out
	}

	firstSlow := open("slow")
	<-entered
	secondSlow := open("slow")

	select {
	case got := <-open("quick"):
		if got.err != nil || got.app == nil {
			t.Fatalf("opening quick: %v", got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("opening one workspace waited for another one to be built")
	}

	letGo()
	one, two := <-firstSlow, <-secondSlow
	if one.err != nil || two.err != nil {
		t.Fatalf("opening slow: %v, %v", one.err, two.err)
	}
	if one.app != two.app {
		t.Error("two callers asking for the same workspace at once got two sets of services")
	}
	if n := slowBuilds.Load(); n != 1 {
		t.Errorf("slow was built %d times, want once", n)
	}
}

// A workspace whose opening finishes after the process has closed its scopes
// is closed with them rather than left open behind them.
func TestAWorkspaceOpenedAfterCloseIsNotKept(t *testing.T) {
	home, first, late := t.TempDir(), t.TempDir(), t.TempDir()
	a, err := New(Options{
		Env:           env.New(env.Map(map[string]string{env.KeyHome: home})),
		WorkspaceRoot: first,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	ctx := context.Background()
	if _, err := a.Workspaces.Create(ctx, workspace.CreateInput{Name: "Late", Path: late}); err != nil {
		t.Fatal(err)
	}

	build := a.scopes.build
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	letGo := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(letGo)
	a.scopes.build = func(id, root string) (*App, error) {
		close(entered)
		<-release
		return build(id, root)
	}
	done := make(chan error, 1)
	go func() {
		_, err := a.scopes.forID(ctx, a.Workspaces, "late")
		done <- err
	}()
	<-entered
	closed := make(chan error, 1)
	go func() { closed <- a.scopes.close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("closing the scopes waited for a workspace still being opened")
	}
	letGo()
	if err := <-done; err == nil {
		t.Error("a workspace opened after the scopes were closed was handed out")
	}
	a.scopes.mu.Lock()
	defer a.scopes.mu.Unlock()
	if len(a.scopes.byPath) != 0 {
		t.Errorf("kept %d workspaces after close", len(a.scopes.byPath))
	}
}
