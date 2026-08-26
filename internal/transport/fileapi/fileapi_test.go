package fileapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/domain/file"
	"github.com/OWNER/aos/internal/transport/fileapi"
)

const root = "/workspace"

// fakeFS is an in-memory filesystem. The domain's own suite proves what the
// service does with it; what is under test here is the layer above — the
// routes, the query and body decoding, and the envelope every surface has to
// answer with — so the storage underneath only has to be honest, not real.
type fakeFS struct {
	mu      sync.Mutex
	entries map[string]entry
}

type entry struct {
	dir  bool
	data []byte
}

func newFakeFS() *fakeFS {
	f := &fakeFS{entries: map[string]entry{}}
	f.mkdirAll(root)
	return f
}

func (f *fakeFS) put(p string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p = path.Clean(p)
	f.mkdirAllLocked(path.Dir(p))
	f.entries[p] = entry{data: data}
}

func (f *fakeFS) mkdirAll(p string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mkdirAllLocked(p)
}

func (f *fakeFS) mkdirAllLocked(p string) {
	p = path.Clean(p)
	f.entries["/"] = entry{dir: true}
	for cur := p; cur != "/" && cur != "."; cur = path.Dir(cur) {
		f.entries[cur] = entry{dir: true}
	}
}

// Resolve is plain path arithmetic: this fake has no symlinks and no disk, so
// containment is a prefix check. The disk-aware version is pathx's, proven in
// its own package.
func (f *fakeFS) Resolve(_ context.Context, base, p string) (string, error) {
	full := path.Clean(path.Join(base, p))
	if full != base && !strings.HasPrefix(full, base+"/") {
		return "", file.ErrOutside
	}
	return full, nil
}

func (f *fakeFS) Stat(_ context.Context, p string) (file.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[path.Clean(p)]
	if !ok {
		return file.Info{}, file.ErrNotExist
	}
	return file.Info{
		Name:    path.Base(p),
		Dir:     e.dir,
		Size:    int64(len(e.data)),
		ModTime: time.Unix(0, 0).UTC(),
	}, nil
}

func (f *fakeFS) ReadDir(_ context.Context, p string) ([]file.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p = path.Clean(p)
	if e, ok := f.entries[p]; !ok || !e.dir {
		return nil, file.ErrNotExist
	}
	var out []file.Info
	for full, e := range f.entries {
		if full == p || path.Dir(full) != p {
			continue
		}
		out = append(out, file.Info{
			Name:    path.Base(full),
			Dir:     e.dir,
			Size:    int64(len(e.data)),
			ModTime: time.Unix(0, 0).UTC(),
		})
	}
	return out, nil
}

func (f *fakeFS) ReadFile(_ context.Context, p string, limit int64) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[path.Clean(p)]
	if !ok || e.dir {
		return nil, false, file.ErrNotExist
	}
	if int64(len(e.data)) > limit {
		return e.data[:limit], true, nil
	}
	return e.data, false, nil
}

func (f *fakeFS) WriteFile(_ context.Context, p string, data []byte) error {
	f.put(p, data)
	return nil
}

func (f *fakeFS) MkdirAll(_ context.Context, p string) error {
	f.mkdirAll(p)
	return nil
}

func (f *fakeFS) Rename(_ context.Context, from, to string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	from, to = path.Clean(from), path.Clean(to)
	e, ok := f.entries[from]
	if !ok {
		return file.ErrNotExist
	}
	delete(f.entries, from)
	f.mkdirAllLocked(path.Dir(to))
	f.entries[to] = e
	return nil
}

func (f *fakeFS) Remove(_ context.Context, p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p = path.Clean(p)
	if _, ok := f.entries[p]; !ok {
		return file.ErrNotExist
	}
	for full := range f.entries {
		if full == p || strings.HasPrefix(full, p+"/") {
			delete(f.entries, full)
		}
	}
	return nil
}

// fakeGit answers Diff's two questions from a table rather than a repository.
// The table is keyed by the workspace-relative path, which is what Diff hands
// to Status and Show — not the resolved absolute one.
type fakeGit struct {
	status  string
	head    map[string][]byte
	changed []file.Change
}

func (g fakeGit) Changes(context.Context, string) ([]file.Change, error) {
	return g.changed, nil
}

func (g fakeGit) Status(context.Context, string, string) (string, error) {
	return g.status, nil
}

func (g fakeGit) Show(_ context.Context, _, _, p string) ([]byte, bool, error) {
	data, ok := g.head[path.Clean(p)]
	return data, ok, nil
}

type fixedWorkspace struct {
	root string
	err  error
}

func (w fixedWorkspace) Root(context.Context) (string, error) { return w.root, w.err }

func newServer(t *testing.T, fs *fakeFS, git file.Git, ws file.Workspaces) *httptest.Server {
	t.Helper()
	svc := file.NewService(file.Deps{FS: fs, Git: git, Workspaces: ws})
	srv := httptest.NewServer(fileapi.New(fileapi.Config{Service: svc}))
	t.Cleanup(srv.Close)
	return srv
}

// envelope is the success shape every surface answers with. The error shape is
// the other one — see errorOf.
type envelope struct {
	Data json.RawMessage `json:"data"`
}

type errorBody struct {
	Error struct {
		Code       string `json:"code"`
		HTTPStatus int    `json:"httpStatus"`
	} `json:"error"`
}

func do(t *testing.T, srv *httptest.Server, method, target string, body any) (*http.Response, []byte) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+target, reader)
	if err != nil {
		t.Fatal(err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := readAll(res)
	if err != nil {
		t.Fatal(err)
	}
	return res, raw
}

func readAll(res *http.Response) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(res.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// dataOf asserts a 200 and returns the envelope's data, so a test that cares
// about the payload does not also have to spell out the wrapper.
func dataOf(t *testing.T, res *http.Response, raw []byte, v any) {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", res.StatusCode, raw)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("the response is not an envelope: %v; body: %s", err, raw)
	}
	if env.Data == nil {
		t.Fatalf("the envelope has no data field; body: %s", raw)
	}
	if err := json.Unmarshal(env.Data, v); err != nil {
		t.Fatalf("the envelope's data does not decode: %v; data: %s", err, env.Data)
	}
}

func errorOf(t *testing.T, raw []byte) errorBody {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("the response is not an error body: %v; body: %s", err, raw)
	}
	if body.Error.Code == "" {
		t.Fatalf("the error body carries no code; body: %s", raw)
	}
	return body
}

func TestTreeListsTheWorkspaceRoot(t *testing.T) {
	fs := newFakeFS()
	fs.put(root+"/README.md", []byte("hello"))
	fs.mkdirAll(root + "/internal")
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	res, raw := do(t, srv, http.MethodGet, "/tree?path=", nil)
	var tree file.Tree
	dataOf(t, res, raw, &tree)

	names := map[string]bool{}
	for _, n := range tree.Nodes {
		names[n.Name] = n.Dir
	}
	if dir, ok := names["README.md"]; !ok || dir {
		t.Fatalf("README.md is missing or listed as a directory: %+v", tree.Nodes)
	}
	if dir, ok := names["internal"]; !ok || !dir {
		t.Fatalf("internal is missing or not listed as a directory: %+v", tree.Nodes)
	}
}

// The recursive flag is a string in the query and a bool in the input, and
// this handler is the only place that conversion happens.
func TestTreeIsRecursiveOnlyWhenTheQuerySaysTrue(t *testing.T) {
	fs := newFakeFS()
	fs.put(root+"/internal/deep/file.go", []byte("package deep"))
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"recursive=true", true},
		{"recursive=false", false},
		{"", false},
	} {
		res, raw := do(t, srv, http.MethodGet, "/tree?path=&"+tc.query, nil)
		var tree file.Tree
		dataOf(t, res, raw, &tree)

		found := false
		for _, n := range tree.Nodes {
			if n.Name == "file.go" {
				found = true
			}
		}
		if found != tc.want {
			t.Fatalf("query %q: reached the nested file = %v, want %v", tc.query, found, tc.want)
		}
	}
}

func TestReadReturnsTheFileAsText(t *testing.T) {
	fs := newFakeFS()
	fs.put(root+"/notes.md", []byte("# título"))
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	res, raw := do(t, srv, http.MethodGet, "/read?path=notes.md", nil)
	var content file.Content
	dataOf(t, res, raw, &content)

	if content.Text != "# título" {
		t.Fatalf("text = %q, want %q", content.Text, "# título")
	}
	if content.Base64 != "" {
		t.Fatalf("a text file came back with base64 set: %q", content.Base64)
	}
}

// SetEscapeHTML(false) is deliberate in writeJSON: the UI edits source files,
// and a file holding `<div>` must come back holding `<div>`.
func TestReadDoesNotEscapeHTMLInTheBody(t *testing.T) {
	fs := newFakeFS()
	fs.put(root+"/page.html", []byte(`<div class="a">&</div>`))
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	_, raw := do(t, srv, http.MethodGet, "/read?path=page.html", nil)
	if !bytes.Contains(raw, []byte(`<div class=\"a\">&</div>`)) {
		t.Fatalf("the HTML came back escaped: %s", raw)
	}
}

func TestWriteCreatesTheFileAndAnswersWithItsPath(t *testing.T) {
	fs := newFakeFS()
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	res, raw := do(t, srv, http.MethodPut, "/write",
		file.WriteInput{Path: "docs/new.md", Content: "conteúdo"})
	var out map[string]string
	dataOf(t, res, raw, &out)

	if out["path"] != "docs/new.md" {
		t.Fatalf("path = %q, want %q", out["path"], "docs/new.md")
	}
	data, _, err := fs.ReadFile(t.Context(), root+"/docs/new.md", 1<<20)
	if err != nil {
		t.Fatalf("the file was not written: %v", err)
	}
	if string(data) != "conteúdo" {
		t.Fatalf("content = %q, want %q", data, "conteúdo")
	}
}

func TestMoveRenamesAndAnswersWithTheDestination(t *testing.T) {
	fs := newFakeFS()
	fs.put(root+"/old.md", []byte("x"))
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	res, raw := do(t, srv, http.MethodPut, "/move",
		map[string]string{"from": "old.md", "to": "new.md"})
	var out map[string]string
	dataOf(t, res, raw, &out)

	if out["path"] != "new.md" {
		t.Fatalf("path = %q, want the destination %q", out["path"], "new.md")
	}
	if _, err := fs.Stat(t.Context(), root+"/old.md"); !errors.Is(err, file.ErrNotExist) {
		t.Fatal("the source still exists after the move")
	}
	if _, err := fs.Stat(t.Context(), root+"/new.md"); err != nil {
		t.Fatalf("the destination does not exist after the move: %v", err)
	}
}

func TestDeleteRemovesThePath(t *testing.T) {
	fs := newFakeFS()
	fs.put(root+"/gone.md", []byte("x"))
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	res, raw := do(t, srv, http.MethodDelete, "/delete?path=gone.md", nil)
	var out map[string]string
	dataOf(t, res, raw, &out)

	if out["path"] != "gone.md" {
		t.Fatalf("path = %q, want %q", out["path"], "gone.md")
	}
	if _, err := fs.Stat(t.Context(), root+"/gone.md"); !errors.Is(err, file.ErrNotExist) {
		t.Fatal("the path still exists after the delete")
	}
}

func TestDiffReportsBothSidesOfAModifiedFile(t *testing.T) {
	fs := newFakeFS()
	fs.put(root+"/src.go", []byte("depois"))
	git := fakeGit{status: "modified", head: map[string][]byte{"src.go": []byte("antes")}}
	srv := newServer(t, fs, git, fixedWorkspace{root: root})

	res, raw := do(t, srv, http.MethodGet, "/diff?path=src.go", nil)
	var diff file.Diff
	dataOf(t, res, raw, &diff)

	if diff.Status != "modified" {
		t.Fatalf("status = %q, want %q", diff.Status, "modified")
	}
	if diff.OldText == nil || *diff.OldText != "antes" {
		t.Fatalf("oldText = %v, want %q", diff.OldText, "antes")
	}
	if diff.NewText == nil || *diff.NewText != "depois" {
		t.Fatalf("newText = %v, want %q", diff.NewText, "depois")
	}
}

// The domain's containment error has to survive the transport as a 403 with
// its code intact — a traversal that came back as a generic 500 would tell
// the UI nothing and tell an attacker just as little about why it failed.
func TestPathTraversalIsRefusedWithTheDomainCodeAndStatus(t *testing.T) {
	fs := newFakeFS()
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	for _, target := range []string{
		"/tree?path=../../etc",
		"/read?path=../../etc/passwd",
		"/diff?path=../outside.go",
		"/delete?path=../outside.go",
	} {
		res, raw := do(t, srv, methodFor(target), target, nil)
		body := errorOf(t, raw)
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s: status = %d, want 403; body: %s", target, res.StatusCode, raw)
		}
		if body.Error.Code != "AOS_FILE_OUTSIDE_WORKSPACE" {
			t.Fatalf("%s: code = %q, want AOS_FILE_OUTSIDE_WORKSPACE", target, body.Error.Code)
		}
	}
}

func methodFor(target string) string {
	if strings.HasPrefix(target, "/delete") {
		return http.MethodDelete
	}
	return http.MethodGet
}

func TestWriteRefusesTraversalToo(t *testing.T) {
	fs := newFakeFS()
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	res, raw := do(t, srv, http.MethodPut, "/write",
		file.WriteInput{Path: "../escaped.md", Content: "x"})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", res.StatusCode, raw)
	}
	if _, err := fs.Stat(t.Context(), "/escaped.md"); !errors.Is(err, file.ErrNotExist) {
		t.Fatal("a traversing write reached the filesystem")
	}
}

func TestABodyThatIsNotJSONIsRejectedAsBadRequest(t *testing.T) {
	fs := newFakeFS()
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/write", strings.NewReader("not json at all"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := readAll(res)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", res.StatusCode, raw)
	}
	if code := errorOf(t, raw).Error.Code; code != "AOS_FILE_HTTP_BAD_BODY" {
		t.Fatalf("code = %q, want AOS_FILE_HTTP_BAD_BODY", code)
	}
}

// maxBodyBytes exists so the editor endpoint cannot be used as a bulk upload.
// The reader trips at the limit, which decode reports as the oversize error.
func TestABodyOverTheLimitIsRejected(t *testing.T) {
	fs := newFakeFS()
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	huge := strings.Repeat("a", (8<<20)+1024)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/write", strings.NewReader(`{"path":"x.md","content":"`+huge+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := readAll(res)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body: %s", res.StatusCode, raw)
	}
	if code := errorOf(t, raw).Error.Code; code != "AOS_FILE_HTTP_BODY_TOO_LARGE" {
		t.Fatalf("code = %q, want AOS_FILE_HTTP_BODY_TOO_LARGE", code)
	}
}

// An error the domain did not classify must not leak out as a naked 200 or as
// a Go error string. writeError's fallback is what covers that, and this is
// the only way to reach it: a port failing with a plain error.
func TestAnUnclassifiedFailureBecomesAFiveHundredWithACode(t *testing.T) {
	fs := newFakeFS()
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{
		root: root,
		err:  errors.New("the workspace service is down"),
	})

	res, raw := do(t, srv, http.MethodGet, "/tree?path=", nil)
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body: %s", res.StatusCode, raw)
	}
	if code := errorOf(t, raw).Error.Code; code == "" {
		t.Fatalf("the failure came back without a code; body: %s", raw)
	}
	if bytes.Contains(raw, []byte("the workspace service is down")) {
		t.Fatalf("the underlying error string leaked to the client: %s", raw)
	}
}

// Every response, success or failure, has to be JSON: the frontend's unwrap()
// decodes before it looks at the status.
func TestEveryRouteAnswersJSON(t *testing.T) {
	fs := newFakeFS()
	fs.put(root+"/a.md", []byte("x"))
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	for _, tc := range []struct {
		method, target string
		body           any
	}{
		{http.MethodGet, "/tree?path=", nil},
		{http.MethodGet, "/read?path=a.md", nil},
		{http.MethodPut, "/write", file.WriteInput{Path: "b.md", Content: "y"}},
		{http.MethodPut, "/move", map[string]string{"from": "b.md", "to": "c.md"}},
		{http.MethodDelete, "/delete?path=c.md", nil},
		{http.MethodGet, "/diff?path=a.md", nil},
	} {
		res, raw := do(t, srv, tc.method, tc.target, tc.body)
		if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("%s %s: content-type = %q", tc.method, tc.target, ct)
		}
		if !json.Valid(raw) {
			t.Fatalf("%s %s: the body is not valid JSON: %s", tc.method, tc.target, raw)
		}
	}
}

// TestChangesListsTheWorkingTree is what the Changes panel reads. It rendered
// nothing before this route existed: the domain could report one path's status
// for a diff already open, and had no way to answer which paths had changed.
func TestChangesListsTheWorkingTree(t *testing.T) {
	fs := newFakeFS()
	fs.mkdirAll(root)
	srv := newServer(t, fs, fakeGit{changed: []file.Change{
		{Path: "src/app.go", Status: "modified"},
		{Path: "README.md", Status: "added"},
	}}, fixedWorkspace{root: root})

	res, raw := do(t, srv, http.MethodGet, "/changes", nil)
	var out struct {
		Files []file.Change `json:"files"`
		Total int           `json:"total"`
	}
	dataOf(t, res, raw, &out)

	if out.Total != 2 || len(out.Files) != 2 {
		t.Fatalf("out = %+v", out)
	}
	// Ordered by path, so the list does not reshuffle between reads.
	if out.Files[0].Path != "README.md" || out.Files[1].Path != "src/app.go" {
		t.Fatalf("files = %+v, want them ordered by path", out.Files)
	}
}

func TestChangesOnACleanTreeIsAnEmptyList(t *testing.T) {
	fs := newFakeFS()
	fs.mkdirAll(root)
	srv := newServer(t, fs, fakeGit{}, fixedWorkspace{root: root})

	res, raw := do(t, srv, http.MethodGet, "/changes", nil)
	var out struct {
		Files []file.Change `json:"files"`
		Total int           `json:"total"`
	}
	dataOf(t, res, raw, &out)

	if out.Total != 0 {
		t.Fatalf("total = %d", out.Total)
	}
	if out.Files == nil {
		t.Error("a clean tree answered null rather than an empty list")
	}
}
