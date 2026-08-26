// Package scan extracts the error catalog from Go source.
//
// It is shared by the generator (tools/gencatalog) and by the catalog test, so
// the committed catalog and the assertion about it cannot disagree about what
// "the codes in the tree" means.
package scan

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

// builderMethods are the fluent methods whose arguments the scanner reads.
var builderMethods = map[string]bool{
	"Causer": true, "Status": true, "CTA": true, "Issue": true,
	"Msgf": true, "Wrap": true, "Kind": true,
}

// Scan walks root and returns every apperr.New(...) construction it finds,
// sorted by code then position.
//
// Errors built across statements (e := apperr.New(...); e.Status(...)) are
// recorded as the New call alone. The convention in this tree is a single
// fluent chain per error, which the catalog test relies on.
func Scan(root string) ([]apperr.Entry, error) {
	fset := token.NewFileSet()
	var out []apperr.Entry

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
		if strings.HasSuffix(path, ".gen.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		out = append(out, scanFile(fset, file, filepath.ToSlash(rel))...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

func scanFile(fset *token.FileSet, file *ast.File, rel string) []apperr.Entry {
	entries := map[*ast.CallExpr]*apperr.Entry{}
	var order []*ast.CallExpr

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// apperr.New("CODE")
		if id, isIdent := sel.X.(*ast.Ident); isIdent && id.Name == "apperr" && sel.Sel.Name == "New" {
			code, ok := stringArg(call, 0)
			if !ok {
				return true
			}
			if _, seen := entries[call]; !seen {
				e := newEntry(qualify(code), rel, fset.Position(call.Pos()).Line)
				entries[call] = e
				order = append(order, call)
			}
			return true
		}

		// A fluent method on a chain rooted at apperr.New.
		if !builderMethods[sel.Sel.Name] {
			return true
		}
		root := rootNew(sel.X)
		if root == nil {
			return true
		}
		e, ok := entries[root]
		if !ok {
			// The chain is visited outside-in, so the root may not be recorded
			// yet. Record it now with the data available.
			code, cok := stringArg(root, 0)
			if !cok {
				return true
			}
			e = newEntry(qualify(code), rel, fset.Position(root.Pos()).Line)
			entries[root] = e
			order = append(order, root)
		}
		applyMethod(e, sel.Sel.Name, call)
		return true
	})

	out := make([]apperr.Entry, 0, len(order))
	for _, call := range order {
		e := entries[call]
		sort.Strings(e.Issues)
		out = append(out, *e)
	}
	return out
}

// newEntry mirrors apperr.New: the default status of an error whose Status is
// never set is 500.
func newEntry(code, rel string, line int) *apperr.Entry {
	return &apperr.Entry{
		Code:    code,
		Package: filepath.ToSlash(filepath.Dir(rel)),
		File:    rel,
		Line:    line,
		Status:  http.StatusInternalServerError,
	}
}

func applyMethod(e *apperr.Entry, name string, call *ast.CallExpr) {
	switch name {
	case "Causer":
		if v, ok := stringArg(call, 0); ok {
			e.Causer = v
			return
		}
		// A causer computed at run time (safe.Do passes the component name)
		// still satisfies the invariant: the field is populated.
		e.Causer = "<dynamic>"
	case "Status":
		if v, ok := statusArg(call); ok {
			e.Status = v
		}
	case "CTA":
		e.CTA = true
	case "Issue":
		if v, ok := stringArg(call, 0); ok {
			e.Issues = append(e.Issues, v)
		}
	}
}

// rootNew unwinds a fluent chain to the apperr.New call at its root, or returns
// nil when the chain does not start there.
func rootNew(e ast.Expr) *ast.CallExpr {
	for {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return nil
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		if id, isIdent := sel.X.(*ast.Ident); isIdent {
			if id.Name == "apperr" && sel.Sel.Name == "New" {
				return call
			}
			return nil
		}
		e = sel.X
	}
}

func stringArg(call *ast.CallExpr, i int) (string, bool) {
	if len(call.Args) <= i {
		return "", false
	}
	lit, ok := call.Args[i].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// statusArg reads Status(http.StatusNotFound) and Status(404) alike.
func statusArg(call *ast.CallExpr) (int, bool) {
	if len(call.Args) == 0 {
		return 0, false
	}
	switch a := call.Args[0].(type) {
	case *ast.BasicLit:
		if a.Kind != token.INT {
			return 0, false
		}
		v, err := strconv.Atoi(a.Value)
		if err != nil {
			return 0, false
		}
		return v, true
	case *ast.SelectorExpr:
		// Both http.StatusNotFound and apperr.StatusNotFound appear: domain
		// code may not import net/http, so it uses the apperr re-exports.
		if id, ok := a.X.(*ast.Ident); ok && (id.Name == "http" || id.Name == "apperr") {
			if v, ok := httpStatusNames[a.Sel.Name]; ok {
				return v, true
			}
		}
	}
	return 0, false
}

var httpStatusNames = map[string]int{
	"StatusBadRequest":          http.StatusBadRequest,
	"StatusUnauthorized":        http.StatusUnauthorized,
	"StatusPaymentRequired":     http.StatusPaymentRequired,
	"StatusForbidden":           http.StatusForbidden,
	"StatusNotFound":            http.StatusNotFound,
	"StatusMethodNotAllowed":    http.StatusMethodNotAllowed,
	"StatusConflict":            http.StatusConflict,
	"StatusGone":                http.StatusGone,
	"StatusPreconditionFailed":  http.StatusPreconditionFailed,
	"StatusUnprocessableEntity": http.StatusUnprocessableEntity,
	// apperr spells these two differently from net/http, and the scanner reads
	// `apperr.StatusX` as well as `http.StatusX` — so both names have to be
	// here or the catalogue silently records 500. Four errors across authapi,
	// fileapi, httpapi and wailsvc were catalogued that way.
	"StatusPayloadTooLarge":       http.StatusRequestEntityTooLarge,
	"StatusRequestEntityTooLarge": http.StatusRequestEntityTooLarge,
	"StatusRequestTimeout":        http.StatusRequestTimeout,
	"StatusTooManyRequests":       http.StatusTooManyRequests,
	"StatusNotImplemented":        http.StatusNotImplemented,
	"StatusInternalServerError":   http.StatusInternalServerError,
	"StatusBadGateway":            http.StatusBadGateway,
	"StatusServiceUnavailable":    http.StatusServiceUnavailable,
	"StatusGatewayTimeout":        http.StatusGatewayTimeout,
}

func qualify(code string) string {
	prefix := build.ErrorPrefix + "_"
	if strings.HasPrefix(code, prefix) {
		return code
	}
	return prefix + code
}

// Render produces the source of internal/core/apperr/catalog.gen.go.
func Render(entries []apperr.Entry) []byte {
	var b bytes.Buffer
	b.WriteString("// Code generated by tools/gencatalog. DO NOT EDIT.\n\n")
	b.WriteString("package apperr\n\n")
	b.WriteString("// Catalog lists every error code constructed in this tree, with the\n")
	b.WriteString("// properties the catalog test asserts on. Regenerate with `task gen-catalog`.\n")
	b.WriteString("var Catalog = []Entry{\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "\t{Code: %q, Package: %q, File: %q, Line: %d, Status: %d, Causer: %q, CTA: %t",
			e.Code, e.Package, e.File, e.Line, e.Status, e.Causer, e.CTA)
		if len(e.Issues) > 0 {
			b.WriteString(", Issues: []string{")
			for i, is := range e.Issues {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%q", is)
			}
			b.WriteString("}")
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	return b.Bytes()
}
