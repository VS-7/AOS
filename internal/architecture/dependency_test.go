package architecture_test

import (
	"context"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/architecture"
)

// moduleRoot walks up from the test's working directory to the go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

type importRef struct {
	file string
	line int
	path string
}

// scanImports returns every import of every non-test Go file under dir, keyed
// by the importing package path.
func scanImports(t *testing.T, root string) map[string][]importRef {
	t.Helper()
	out := map[string][]importRef{}
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "frontend", "docs", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		pkg := architecture.Module + "/" + filepath.ToSlash(filepath.Dir(rel))
		for _, spec := range f.Imports {
			p, uerr := strconv.Unquote(spec.Path.Value)
			if uerr != nil {
				continue
			}
			out[pkg] = append(out[pkg], importRef{
				file: rel,
				line: fset.Position(spec.Pos()).Line,
				path: p,
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestDependencyRule walks every package under the guarded prefixes and fails
// listing each violating import, so the message tells the developer exactly
// which file to fix.
func TestDependencyRule(t *testing.T) {
	root := moduleRoot(t)
	imports := scanImports(t, root)

	for pkg, refs := range imports {
		for guarded, banned := range architecture.Forbidden {
			if !underPrefix(pkg, guarded) {
				continue
			}
			for _, ref := range refs {
				for _, b := range banned {
					if strings.HasPrefix(ref.path, b) {
						t.Errorf("%s:%d: %s must not import %q (banned for %s)",
							ref.file, ref.line, pkg, ref.path, guarded)
					}
				}
			}
		}
	}
}

func underPrefix(pkg, prefix string) bool {
	return pkg == prefix || strings.HasPrefix(pkg, prefix+"/")
}

// TestNoDomainInClients asserts that the CLI and the desktop app do not link
// domain code. It uses the real dependency graph, not the source imports, so a
// transitive leak through a helper package is caught too.
func TestNoDomainInClients(t *testing.T) {
	root := moduleRoot(t)
	for _, bin := range architecture.ClientBinaries {
		pkgPath := "./" + strings.TrimPrefix(bin, architecture.Module+"/")
		if _, err := os.Stat(filepath.Join(root, strings.TrimPrefix(pkgPath, "./"))); os.IsNotExist(err) {
			t.Logf("skipping %s: binary not implemented yet", bin)
			continue
		}
		cmd := exec.CommandContext(context.Background(), "go", "list", "-deps", pkgPath)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list -deps %s: %v", pkgPath, err)
		}
		for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if !strings.HasPrefix(dep, architecture.Module+"/internal/domain") {
				continue
			}
			if allowedDomain(dep) {
				continue
			}
			t.Errorf("%s links domain package %s", bin, dep)
		}
	}
}

func allowedDomain(dep string) bool {
	for _, ok := range architecture.DomainExceptions {
		if underPrefix(dep, ok) {
			return true
		}
	}
	return false
}

// TestFeatureLayout asserts that every domain feature has the same skeleton.
// "Where does this go?" must have an answer, not a discussion.
func TestFeatureLayout(t *testing.T) {
	root := moduleRoot(t)
	domain := filepath.Join(root, "internal", "domain")
	entries, err := os.ReadDir(domain)
	if os.IsNotExist(err) {
		t.Skip("no domain packages yet")
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "testsuite" {
			continue
		}
		for _, required := range architecture.RequiredFeatureFiles {
			p := filepath.Join(domain, e.Name(), required)
			if _, err := os.Stat(p); err != nil {
				t.Errorf("internal/domain/%s is missing %s", e.Name(), required)
			}
		}
	}
}

// TestDomainTestsDoNotTouchIO enforces that domain tests run on fakes. A domain
// test that opens a file is testing something other than a rule.
func TestDomainTestsDoNotTouchIO(t *testing.T) {
	root := moduleRoot(t)
	domain := filepath.Join(root, "internal", "domain")
	banned := []string{"t.TempDir()", "os.Open(", "os.WriteFile(", "net/http", "os/exec"}

	err := filepath.WalkDir(domain, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, path)
		for _, b := range banned {
			if strings.Contains(string(raw), b) {
				t.Errorf("%s: domain tests must run on fakes, found %q", filepath.ToSlash(rel), b)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
