package fscollections

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/OWNER/aos/internal/core/collections"
)

// resolver builds the smallest repository that can answer resolve. The model
// is irrelevant here — containment is decided before any pattern is consulted.
func resolver(t *testing.T, root string) *Repo[struct{}] {
	t.Helper()
	return New(root, collections.Model[struct{}]{Name: "probe"})
}

// TestResolveAcceptsWhatIsInsideTheRoot covers the ordinary root, so a
// regression in the containment check cannot pass by only fixing the edge case
// below.
func TestResolveAcceptsWhatIsInsideTheRoot(t *testing.T) {
	root := t.TempDir()
	r := resolver(t, root)

	got, err := r.resolve(".aos/agents/atlas/AGENT.md")
	if err != nil {
		t.Fatalf("a path plainly inside the root was refused: %v", err)
	}
	if want := filepath.Join(root, ".aos", "agents", "atlas", "AGENT.md"); got != want {
		t.Fatalf("resolved to %q, want %q", got, want)
	}

	if _, err := r.resolve(""); err != nil {
		t.Errorf("the root itself was refused: %v", err)
	}
}

func TestResolveRefusesWhatEscapesTheRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	r := resolver(t, root)

	for _, rel := range []string{"../outside", "../../etc/passwd", "a/../../b"} {
		if _, err := r.resolve(rel); err == nil {
			t.Errorf("%q escaped the root and was accepted", rel)
		}
	}
}

// TestResolveRefusesASiblingWithTheRootAsAPrefix is why the separator is part
// of the comparison at all: "/tmp/ws-evil" starts with "/tmp/ws" as a string
// and is not inside it as a directory.
func TestResolveRefusesASiblingWithTheRootAsAPrefix(t *testing.T) {
	base := t.TempDir()
	r := resolver(t, filepath.Join(base, "ws"))

	if _, err := r.resolve("../ws-evil/secret"); err == nil {
		t.Fatal("a sibling directory sharing the root's name prefix was accepted")
	}
}

// TestResolveAtTheFilesystemRoot reproduces the defect that made every
// collection-backed command in the desktop application answer 403.
//
// A macOS application bundle launched from Finder is started with "/" as its
// working directory, and the daemon takes its working directory as the
// workspace root when nothing else names one. The containment check compared
// the resolved path against root + separator — "//" when the root is already
// "/" — so "/.aos/agents" did not match, and the repository reported that a
// path directly inside the root escaped it. The daemon logged
// AOS_COLLECTION_PATH_ESCAPES_ROOT for agents, skills and collections at every
// boot, and agents_list, tasks_list, goals_list and the rest answered 403 or
// 500 for the whole session.
//
// "/" being the root at all is a separate defect, fixed where the root is
// chosen (cmd/aos-desktop and internal/app). This is about the check: it has
// to be correct at every root it is given, and a root of "/" is a legitimate,
// if unusual, one.
func TestResolveAtTheFilesystemRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`"/" is not a filesystem root on Windows`)
	}
	r := resolver(t, "/")

	got, err := r.resolve(".aos/agents")
	if err != nil {
		t.Fatalf(`".aos/agents" is inside "/" and was refused: %v`, err)
	}
	if got != "/.aos/agents" {
		t.Fatalf("resolved to %q, want %q", got, "/.aos/agents")
	}

	// Nothing is above the filesystem root, so a traversal clamps back into
	// it rather than escaping. What matters is that the answer is still a
	// path inside the root, which every caller then treats as workspace data.
	clamped, err := r.resolve("../etc/passwd")
	if err != nil {
		t.Fatalf(`"../etc/passwd" clamps to "/etc/passwd", which is inside "/": %v`, err)
	}
	if clamped != "/etc/passwd" {
		t.Fatalf("resolved to %q, want %q", clamped, "/etc/passwd")
	}
}
