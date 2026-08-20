package file_test

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/file"
)

// maxReadBytes mirrors the unexported constant in service.go; Read and Diff
// truncate at this many bytes, and a test that wants to prove truncation has
// to know the boundary it is crossing.
const maxReadBytes = 4 << 20

const root = "/workspace"

// fakeFS is an in-memory FS: no disk, no symlinks, no temporary directory —
// domain tests run on fakes (see internal/architecture's
// TestDomainTestsDoNotTouchIO), and containment here is plain path
// arithmetic rather than pathx's disk-aware version, which is proven
// separately in internal/core/pathx.
type fakeFS struct {
	mu      sync.Mutex
	entries map[string]fakeEntry
}

type fakeEntry struct {
	dir  bool
	data []byte
}

func newFakeFS() *fakeFS {
	f := &fakeFS{entries: map[string]fakeEntry{}}
	f.mkdir(root)
	return f
}

func (f *fakeFS) put(p string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p = path.Clean(p)
	f.ensureDirsLocked(path.Dir(p))
	f.entries[p] = fakeEntry{data: data}
}

func (f *fakeFS) mkdir(p string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureDirsLocked(p)
}

func (f *fakeFS) ensureDirsLocked(p string) {
	p = path.Clean(p)
	f.entries["/"] = fakeEntry{dir: true}
	for d := p; d != "/" && d != "."; d = path.Dir(d) {
		f.entries[d] = fakeEntry{dir: true}
	}
}

func (f *fakeFS) Resolve(_ context.Context, root, p string) (string, error) {
	abs := p
	if !path.IsAbs(abs) {
		abs = path.Join(root, p)
	}
	abs = path.Clean(abs)
	if abs != root && !strings.HasPrefix(abs, root+"/") {
		return "", file.ErrOutside
	}
	return abs, nil
}

func (f *fakeFS) Stat(_ context.Context, p string) (file.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[p]
	if !ok {
		return file.Info{}, file.ErrNotExist
	}
	return file.Info{Name: path.Base(p), Dir: e.dir, Size: int64(len(e.data))}, nil
}

func (f *fakeFS) ReadDir(_ context.Context, dir string) ([]file.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e, ok := f.entries[dir]; !ok || !e.dir {
		return nil, file.ErrNotExist
	}
	var out []file.Info
	for p, e := range f.entries {
		if p == dir || path.Dir(p) != dir {
			continue
		}
		out = append(out, file.Info{Name: path.Base(p), Dir: e.dir, Size: int64(len(e.data))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *fakeFS) ReadFile(_ context.Context, p string, limit int64) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[p]
	if !ok {
		return nil, false, file.ErrNotExist
	}
	if int64(len(e.data)) > limit {
		return e.data[:limit], true, nil
	}
	return e.data, false, nil
}

func (f *fakeFS) WriteFile(_ context.Context, p string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p = path.Clean(p)
	f.ensureDirsLocked(path.Dir(p))
	f.entries[p] = fakeEntry{data: data}
	return nil
}

func (f *fakeFS) MkdirAll(_ context.Context, p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureDirsLocked(p)
	return nil
}

func (f *fakeFS) Rename(_ context.Context, from, to string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[from]
	if !ok {
		return file.ErrNotExist
	}
	f.ensureDirsLocked(path.Dir(to))
	f.entries[to] = e
	delete(f.entries, from)
	return nil
}

func (f *fakeFS) Remove(_ context.Context, p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.entries[p]; !ok {
		return nil
	}
	prefix := p + "/"
	for k := range f.entries {
		if strings.HasPrefix(k, prefix) {
			delete(f.entries, k)
		}
	}
	delete(f.entries, p)
	return nil
}

type fakeGit struct {
	status map[string]string
	head   map[string][]byte
}

func newFakeGit() *fakeGit {
	return &fakeGit{status: map[string]string{}, head: map[string][]byte{}}
}

func (g *fakeGit) Status(_ context.Context, _, path string) (string, error) {
	return g.status[path], nil
}

func (g *fakeGit) Show(_ context.Context, _, ref, path string) ([]byte, bool, error) {
	if ref != "HEAD" {
		return nil, false, nil
	}
	data, ok := g.head[path]
	return data, ok, nil
}

type fakeWorkspaces struct{}

func (fakeWorkspaces) Root(context.Context) (string, error) { return root, nil }

func newService(fs *fakeFS, git file.Git) *file.Service {
	return file.NewService(file.Deps{FS: fs, Git: git, Workspaces: fakeWorkspaces{}})
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not an apperr: %v", err, err)
	}
	return app.Code
}

func rel(p string) string { return path.Join(root, p) }

func TestTreeListsADirectoryNonRecursively(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("README.md"), []byte("hello"))
	fs.put(rel("src/main.go"), []byte("package main"))
	s := newService(fs, newFakeGit())

	tree, err := s.Tree(context.Background(), file.TreeInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 2 {
		t.Fatalf("got %d nodes, want 2: %+v", len(tree.Nodes), tree.Nodes)
	}
	// directories sort before files
	if !tree.Nodes[0].Dir || tree.Nodes[0].Name != "src" {
		t.Errorf("first node = %+v, want the src directory", tree.Nodes[0])
	}
	if tree.Nodes[1].Dir || tree.Nodes[1].Name != "README.md" {
		t.Errorf("second node = %+v, want README.md", tree.Nodes[1])
	}
	for _, n := range tree.Nodes {
		if n.Name == "main.go" {
			t.Fatalf("non-recursive Tree returned a nested file: %+v", tree.Nodes)
		}
	}
}

func TestTreeIsRecursiveWhenAsked(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("src/main.go"), []byte("package main"))
	s := newService(fs, newFakeGit())

	tree, err := s.Tree(context.Background(), file.TreeInput{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range tree.Nodes {
		if n.Path == "src/main.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("recursive Tree missed src/main.go: %+v", tree.Nodes)
	}
}

func TestTreeIgnoresVersionControlAndDependencyDirectories(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel(".git/HEAD"), []byte("ref: refs/heads/main"))
	fs.put(rel("node_modules/pkg/index.js"), []byte("module.exports = {}"))
	fs.put(rel("README.md"), []byte("hello"))
	s := newService(fs, newFakeGit())

	tree, err := s.Tree(context.Background(), file.TreeInput{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 1 || tree.Nodes[0].Name != "README.md" {
		t.Fatalf("got %+v, want only README.md", tree.Nodes)
	}
}

func TestReadReturnsTextForATextFile(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("README.md"), []byte("# hello"))
	s := newService(fs, newFakeGit())

	got, err := s.Read(context.Background(), file.ReadInput{Path: "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "# hello" || got.Base64 != "" {
		t.Fatalf("got %+v", got)
	}
	if got.Truncated {
		t.Error("a small file must not be reported as truncated")
	}
}

func TestReadReturnsBase64ForABinaryExtension(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("logo.png"), []byte{0x89, 0x50, 0x4e, 0x47})
	s := newService(fs, newFakeGit())

	got, err := s.Read(context.Background(), file.ReadInput{Path: "logo.png"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "" || got.Base64 == "" {
		t.Fatalf("got %+v, want Base64 populated and Text empty", got)
	}
}

func TestReadDetectsBinaryContentEvenWithNoExtension(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("compiled-tool"), []byte{0x7f, 'E', 'L', 'F', 0x00, 0x01, 0x02})
	s := newService(fs, newFakeGit())

	got, err := s.Read(context.Background(), file.ReadInput{Path: "compiled-tool"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "" || got.Base64 == "" {
		t.Fatalf("got %+v, want a NUL-bearing extensionless file treated as binary", got)
	}
}

func TestReadTruncatesALargeFileWithTheFlagSet(t *testing.T) {
	fs := newFakeFS()
	big := strings.Repeat("a", maxReadBytes+10)
	fs.put(rel("big.txt"), []byte(big))
	s := newService(fs, newFakeGit())

	got, err := s.Read(context.Background(), file.ReadInput{Path: "big.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated {
		t.Fatal("a file over the ceiling must come back Truncated")
	}
	if len(got.Text) != maxReadBytes {
		t.Fatalf("got %d bytes of text, want exactly %d", len(got.Text), maxReadBytes)
	}
}

func TestWriteCreatesAFileAndItsParentDirectories(t *testing.T) {
	fs := newFakeFS()
	s := newService(fs, newFakeGit())

	err := s.Write(context.Background(), file.WriteInput{Path: "src/new/file.go", Content: "package new"})
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := fs.ReadFile(context.Background(), rel("src/new/file.go"), maxReadBytes)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package new" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteOverwritesAnExistingFile(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("README.md"), []byte("old"))
	s := newService(fs, newFakeGit())

	if err := s.Write(context.Background(), file.WriteInput{Path: "README.md", Content: "new"}); err != nil {
		t.Fatal(err)
	}
	got, _, err := fs.ReadFile(context.Background(), rel("README.md"), maxReadBytes)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("got %q, want the file overwritten", got)
	}
}

func TestMoveRenamesAPath(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("old.txt"), []byte("hi"))
	s := newService(fs, newFakeGit())

	if err := s.Move(context.Background(), "old.txt", "new.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(context.Background(), rel("old.txt")); !errors.Is(err, file.ErrNotExist) {
		t.Error("the source must be gone after a move")
	}
	if _, err := fs.Stat(context.Background(), rel("new.txt")); err != nil {
		t.Error("the destination must exist after a move")
	}
}

func TestMoveRefusesToOverwriteAnExistingDestination(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("old.txt"), []byte("hi"))
	fs.put(rel("new.txt"), []byte("already here"))
	s := newService(fs, newFakeGit())

	err := s.Move(context.Background(), "old.txt", "new.txt")
	if code := codeOf(t, err); code != "AOS_FILE_ALREADY_EXISTS" {
		t.Fatalf("code = %q, want AOS_FILE_ALREADY_EXISTS", code)
	}
}

func TestDeleteRemovesAPath(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("temp.txt"), []byte("scratch"))
	s := newService(fs, newFakeGit())

	if err := s.Delete(context.Background(), "temp.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(context.Background(), rel("temp.txt")); !errors.Is(err, file.ErrNotExist) {
		t.Error("the path must be gone after delete")
	}
}

func TestDeleteRefusesToRemoveTheWorkspaceRoot(t *testing.T) {
	fs := newFakeFS()
	s := newService(fs, newFakeGit())

	cases := []string{"", ".", "/", "//"}
	for _, p := range cases {
		err := s.Delete(context.Background(), p)
		if code := codeOf(t, err); code != "AOS_FILE_ROOT_REMOVAL" {
			t.Errorf("Delete(%q): code = %q, want AOS_FILE_ROOT_REMOVAL", p, code)
		}
	}
}

func TestPathTraversalIsRefusedAcrossEveryOperation(t *testing.T) {
	fs := newFakeFS()
	s := newService(fs, newFakeGit())
	ctx := context.Background()
	const outside = "../../etc/passwd"

	assertOutside := func(t *testing.T, err error) {
		t.Helper()
		if code := codeOf(t, err); code != "AOS_FILE_OUTSIDE_WORKSPACE" {
			t.Errorf("code = %q, want AOS_FILE_OUTSIDE_WORKSPACE", code)
		}
	}

	_, err := s.Tree(ctx, file.TreeInput{Path: outside})
	assertOutside(t, err)

	_, err = s.Read(ctx, file.ReadInput{Path: outside})
	assertOutside(t, err)

	err = s.Write(ctx, file.WriteInput{Path: outside, Content: "x"})
	assertOutside(t, err)

	err = s.Move(ctx, outside, "inside.txt")
	assertOutside(t, err)

	err = s.Delete(ctx, outside)
	assertOutside(t, err)

	_, err = s.Diff(ctx, file.DiffInput{Path: outside})
	assertOutside(t, err)
}

func TestDiffReportsAnAddedFileWithNoOldSide(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("new.txt"), []byte("brand new"))
	git := newFakeGit()
	git.status["new.txt"] = "untracked"
	s := newService(fs, git)

	d, err := s.Diff(context.Background(), file.DiffInput{Path: "new.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "untracked" || d.OldText != nil {
		t.Fatalf("got %+v", d)
	}
	if d.NewText == nil || *d.NewText != "brand new" {
		t.Fatalf("got NewText = %v, want %q", d.NewText, "brand new")
	}
}

func TestDiffReportsAModifiedFileWithBothSides(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("README.md"), []byte("new content"))
	git := newFakeGit()
	git.status["README.md"] = "modified"
	git.head["README.md"] = []byte("old content")
	s := newService(fs, git)

	d, err := s.Diff(context.Background(), file.DiffInput{Path: "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if d.OldText == nil || *d.OldText != "old content" {
		t.Fatalf("OldText = %v", d.OldText)
	}
	if d.NewText == nil || *d.NewText != "new content" {
		t.Fatalf("NewText = %v", d.NewText)
	}
}

func TestDiffReportsADeletedFileWithNoNewSide(t *testing.T) {
	fs := newFakeFS()
	git := newFakeGit()
	git.status["gone.txt"] = "deleted"
	git.head["gone.txt"] = []byte("it used to say this")
	s := newService(fs, git)

	d, err := s.Diff(context.Background(), file.DiffInput{Path: "gone.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if d.NewText != nil {
		t.Fatalf("NewText = %v, want nil for a deleted file", d.NewText)
	}
	if d.OldText == nil || *d.OldText != "it used to say this" {
		t.Fatalf("OldText = %v", d.OldText)
	}
}

func TestDiffMarksABinaryPairWithoutText(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("logo.png"), []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x01})
	git := newFakeGit()
	git.status["logo.png"] = "modified"
	git.head["logo.png"] = []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x02}
	s := newService(fs, git)

	d, err := s.Diff(context.Background(), file.DiffInput{Path: "logo.png"})
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsBinary || d.OldText != nil || d.NewText != nil {
		t.Fatalf("got %+v, want IsBinary with neither side of text set", d)
	}
}

// The suite above proves what the service does when its ports work. What
// follows proves what it does when they do not — every failure the ports can
// report has to arrive as a classified error, because the transport above
// turns the code into an HTTP status and the UI turns it into a message.

// brokenFS wraps a working fake and makes exactly one method fail, so a test
// names the failure it is about instead of building a whole broken world.
type brokenFS struct {
	*fakeFS
	fail error

	on string // "resolve", "stat", "readdir", "read", "write", "mkdir", "rename", "remove"
}

func (b brokenFS) Resolve(ctx context.Context, root, p string) (string, error) {
	if b.on == "resolve" {
		return "", b.fail
	}
	return b.fakeFS.Resolve(ctx, root, p)
}

func (b brokenFS) Stat(ctx context.Context, p string) (file.Info, error) {
	if b.on == "stat" {
		return file.Info{}, b.fail
	}
	return b.fakeFS.Stat(ctx, p)
}

func (b brokenFS) ReadDir(ctx context.Context, p string) ([]file.Info, error) {
	if b.on == "readdir" {
		return nil, b.fail
	}
	return b.fakeFS.ReadDir(ctx, p)
}

func (b brokenFS) ReadFile(ctx context.Context, p string, limit int64) ([]byte, bool, error) {
	if b.on == "read" {
		return nil, false, b.fail
	}
	return b.fakeFS.ReadFile(ctx, p, limit)
}

func (b brokenFS) WriteFile(ctx context.Context, p string, data []byte) error {
	if b.on == "write" {
		return b.fail
	}
	return b.fakeFS.WriteFile(ctx, p, data)
}

func (b brokenFS) MkdirAll(ctx context.Context, p string) error {
	if b.on == "mkdir" {
		return b.fail
	}
	return b.fakeFS.MkdirAll(ctx, p)
}

func (b brokenFS) Rename(ctx context.Context, from, to string) error {
	if b.on == "rename" {
		return b.fail
	}
	return b.fakeFS.Rename(ctx, from, to)
}

func (b brokenFS) Remove(ctx context.Context, p string) error {
	if b.on == "remove" {
		return b.fail
	}
	return b.fakeFS.Remove(ctx, p)
}

type brokenGit struct {
	*fakeGit
	fail error
	on   string // "status" or "show"
}

func (g brokenGit) Status(ctx context.Context, root, p string) (string, error) {
	if g.on == "status" {
		return "", g.fail
	}
	return g.fakeGit.Status(ctx, root, p)
}

func (g brokenGit) Show(ctx context.Context, root, ref, p string) ([]byte, bool, error) {
	if g.on == "show" {
		return nil, false, g.fail
	}
	return g.fakeGit.Show(ctx, root, ref, p)
}

type brokenWorkspaces struct{ fail error }

func (w brokenWorkspaces) Root(context.Context) (string, error) { return "", w.fail }

// A workspace that cannot report its root is not a path problem, and saying
// so is the difference between "check your path" and "the daemon lost the
// workspace" in front of whoever is reading the message.
func TestEveryOperationReportsAnUnavailableWorkspace(t *testing.T) {
	svc := file.NewService(file.Deps{
		FS:         newFakeFS(),
		Git:        newFakeGit(),
		Workspaces: brokenWorkspaces{fail: errors.New("the workspace store is unreachable")},
	})
	ctx := t.Context()

	for name, call := range map[string]func() error{
		"Tree":   func() error { _, err := svc.Tree(ctx, file.TreeInput{}); return err },
		"Read":   func() error { _, err := svc.Read(ctx, file.ReadInput{Path: "a.md"}); return err },
		"Write":  func() error { return svc.Write(ctx, file.WriteInput{Path: "a.md"}) },
		"Move":   func() error { return svc.Move(ctx, "a.md", "b.md") },
		"Delete": func() error { return svc.Delete(ctx, "a.md") },
		"Diff":   func() error { _, err := svc.Diff(ctx, file.DiffInput{Path: "a.md"}); return err },
	} {
		if code := codeOf(t, call()); code != "AOS_FILE_WORKSPACE_UNAVAILABLE" {
			t.Fatalf("%s: code = %q, want FILE_WORKSPACE_UNAVAILABLE", name, code)
		}
	}
}

// A resolution failure that is not a containment breach must not be reported
// as one: FILE_OUTSIDE_WORKSPACE reads as an attempted escape, and sending
// whoever reads the audit log after an attack that did not happen is its own
// kind of failure.
func TestAResolutionFailureThatIsNotAnEscapeIsReportedAsUnreadable(t *testing.T) {
	fs := brokenFS{fakeFS: newFakeFS(), on: "resolve", fail: errors.New("the path could not be walked")}
	svc := newService2(fs, newFakeGit())

	_, err := svc.Read(t.Context(), file.ReadInput{Path: "a.md"})
	if code := codeOf(t, err); code != "AOS_FILE_PATH_UNREADABLE" {
		t.Fatalf("code = %q, want FILE_PATH_UNREADABLE", code)
	}
}

func TestTreeRefusesAPathThatIsNotADirectory(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("README.md"), []byte("hello"))
	svc := newService(fs, newFakeGit())

	_, err := svc.Tree(t.Context(), file.TreeInput{Path: "README.md"})
	if code := codeOf(t, err); code != "AOS_FILE_NOT_A_DIRECTORY" {
		t.Fatalf("code = %q, want FILE_NOT_A_DIRECTORY", code)
	}
}

func TestReadRefusesADirectory(t *testing.T) {
	fs := newFakeFS()
	fs.mkdir(rel("internal"))
	svc := newService(fs, newFakeGit())

	_, err := svc.Read(t.Context(), file.ReadInput{Path: "internal"})
	if code := codeOf(t, err); code != "AOS_FILE_IS_A_DIRECTORY" {
		t.Fatalf("code = %q, want FILE_IS_A_DIRECTORY", code)
	}
}

// Write has no separate create, so a write onto a directory would otherwise
// be an attempt to replace it with a file.
func TestWriteRefusesADirectory(t *testing.T) {
	fs := newFakeFS()
	fs.mkdir(rel("internal"))
	svc := newService(fs, newFakeGit())

	err := svc.Write(t.Context(), file.WriteInput{Path: "internal", Content: "x"})
	if code := codeOf(t, err); code != "AOS_FILE_IS_A_DIRECTORY" {
		t.Fatalf("code = %q, want FILE_IS_A_DIRECTORY", code)
	}
}

// Every filesystem failure arrives as one code carrying which operation
// failed, so the message can name it without the transport having to know
// the difference between a failed rename and a failed mkdir.
func TestEveryFilesystemFailureArrivesAsIOFailed(t *testing.T) {
	boom := errors.New("the device is not ready")

	for name, tc := range map[string]struct {
		on   string
		call func(*file.Service) error
	}{
		"Tree/stat": {"stat", func(s *file.Service) error {
			_, err := s.Tree(t.Context(), file.TreeInput{})
			return err
		}},
		"Tree/readdir": {"readdir", func(s *file.Service) error {
			_, err := s.Tree(t.Context(), file.TreeInput{})
			return err
		}},
		"Read/read": {"read", func(s *file.Service) error {
			_, err := s.Read(t.Context(), file.ReadInput{Path: "a.md"})
			return err
		}},
		"Write/mkdir": {"mkdir", func(s *file.Service) error {
			return s.Write(t.Context(), file.WriteInput{Path: "new/a.md", Content: "x"})
		}},
		"Write/write": {"write", func(s *file.Service) error {
			return s.Write(t.Context(), file.WriteInput{Path: "a.md", Content: "x"})
		}},
		"Move/rename": {"rename", func(s *file.Service) error {
			return s.Move(t.Context(), "a.md", "b.md")
		}},
		"Delete/remove": {"remove", func(s *file.Service) error {
			return s.Delete(t.Context(), "a.md")
		}},
	} {
		fs := newFakeFS()
		fs.put(rel("a.md"), []byte("hello"))
		svc := newService2(brokenFS{fakeFS: fs, on: tc.on, fail: boom}, newFakeGit())

		if code := codeOf(t, tc.call(svc)); code != "AOS_FILE_IO_FAILED" {
			t.Fatalf("%s: code = %q, want FILE_IO_FAILED", name, code)
		}
	}
}

// Diff is the only place git is reached, and both of its calls have to be
// classified — a git failure is not a filesystem failure, and telling
// someone to check their path when the repository is what broke sends them
// looking in the wrong place.
func TestDiffReportsAGitFailureAsItsOwnCode(t *testing.T) {
	boom := errors.New("not a git repository")

	for _, on := range []string{"status", "show"} {
		fs := newFakeFS()
		fs.put(rel("src.go"), []byte("depois"))
		git := newFakeGit()
		git.status["src.go"] = "modified"
		git.head["src.go"] = []byte("antes")

		svc := newService(fs, brokenGit{fakeGit: git, on: on, fail: boom})
		_, err := svc.Diff(t.Context(), file.DiffInput{Path: "src.go"})
		if code := codeOf(t, err); code != "AOS_FILE_GIT_FAILED" {
			t.Fatalf("git %s: code = %q, want FILE_GIT_FAILED", on, code)
		}
	}
}

// newService2 takes an FS rather than the concrete fake, for the tests above
// that substitute a broken one.
func newService2(fs file.FS, git file.Git) *file.Service {
	return file.NewService(file.Deps{FS: fs, Git: git, Workspaces: fakeWorkspaces{}})
}
