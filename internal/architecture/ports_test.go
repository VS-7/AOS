package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maxPortMethods is the point at which an interface stops being a port and
// starts being a monolith. The original's Sandbox port had eleven methods; here
// it is split by consumer, so a tool that only reads receives an interface that
// cannot write.
const maxPortMethods = 6

// TestPortsAreSmall fails when a port.go declares an interface with more than
// six methods and no comment justifying it.
func TestPortsAreSmall(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()

	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "port.go" {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return perr
		}
		rel, _ := filepath.Rel(root, path)
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				iface, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				n := len(iface.Methods.List)
				if n <= maxPortMethods {
					continue
				}
				if hasJustification(gen.Doc, ts.Doc) {
					continue
				}
				t.Errorf("%s: interface %s has %d methods and no comment justifying more than %d",
					filepath.ToSlash(rel), ts.Name.Name, n, maxPortMethods)
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// hasJustification looks for a doc comment that explicitly argues for the size.
func hasJustification(groups ...*ast.CommentGroup) bool {
	for _, g := range groups {
		if g == nil {
			continue
		}
		if strings.Contains(strings.ToLower(g.Text()), "wide port:") {
			return true
		}
	}
	return false
}
