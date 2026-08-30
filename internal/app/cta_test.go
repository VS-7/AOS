package app_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/runtime/toolexec/tools"
)

// selfCommands are the built-ins the terminal publishes outside the registry.
// They live under a namespace precisely so they cannot shadow a domain group.
var selfCommands = map[string]bool{
	"tools": true, "llms": true, "completions": true, "skill": true,
}

// TestEveryCallToActionNamesSomethingThatExists is the guard for defect #5.
//
// A call to action is not decoration: the system tells an agent to follow it,
// and a person reads it as the way out of a refusal. Three of them named
// `aos auth token issue` and one named `aos auth users list` — there is no
// `auth` group and there never has been, so the single most important refusal
// in the tunnel package answered a security question with a command that
// answers `unknown command "auth"`.
//
// The check is static because these strings are built at the moment of
// failure: only a test that reads them where they are written can see the ones
// no test ever triggers.
func TestEveryCallToActionNamesSomethingThatExists(t *testing.T) {
	a := newApp(t)

	groups := map[string]map[string]bool{}
	for _, g := range a.Registry.Groups() {
		names := map[string]bool{}
		for _, d := range g.Commands {
			names[d.Name()] = true
			for _, alias := range d.Aliases() {
				names[alias] = true
			}
		}
		groups[g.Name] = names
	}
	groups["self"] = selfCommands

	// The agent's own runtime tools are not commands: Read, Write, Edit, Glob,
	// Grep and Bash operate the sandbox rather than the domain, and an error
	// raised by one of them names another one of them. Taking the list from
	// the constructor keeps this from going stale the way the strings it
	// guards did.
	runtimeTools := map[string]bool{}
	for _, tool := range tools.FS(nil) {
		runtimeTools[tool.Name()] = true
	}

	for _, cta := range scanCallsToAction(t, repoRoot(t)) {
		if cta.command != "" {
			checkCommand(t, groups, cta)
		}
		if cta.tool != "" && !runtimeTools[cta.tool] {
			if _, _, known := a.Registry.Lookup(cta.tool); !known {
				t.Errorf("%s: the call to action names tool %q, which is not registered",
					cta.where, cta.tool)
			}
		}
	}
}

func checkCommand(t *testing.T, groups map[string]map[string]bool, cta callToAction) {
	t.Helper()
	fields := strings.Fields(cta.command)
	if len(fields) == 0 || fields[0] != build.Name {
		return // not an invocation of this binary
	}
	fields = fields[1:]
	if len(fields) == 0 {
		return // `aos` bare, which prints help
	}
	group := fields[0]
	names, exists := groups[group]
	if !exists {
		t.Errorf("%s: %q names the group %q, which this build does not publish",
			cta.where, cta.command, group)
		return
	}
	// A group with no action after it is a legitimate suggestion ("aos config"
	// prints the group's help), and so is one whose next token is a flag.
	if len(fields) < 2 || strings.HasPrefix(fields[1], "-") {
		return
	}
	if !names[fields[1]] {
		t.Errorf("%s: %q names %s %s, which is not a command of that group",
			cta.where, cta.command, group, fields[1])
	}
}

type callToAction struct {
	where   string
	command string
	tool    string
}

// scanCallsToAction reads every apperr.CallToAction literal in the tree.
//
// Only the literal part of a Command is read: `build.Name + " memories reflect
// " + id` yields "aos memories reflect", which is exactly the part that has to
// name a real command. Concatenation stops at the first expression this cannot
// evaluate, because everything after it is a runtime value.
func scanCallsToAction(t *testing.T, root string) []callToAction {
	t.Helper()
	var out []callToAction
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "frontend", "docs", "testdata", "build":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // a file this cannot parse is the compiler's problem, not this test's
		}
		rel, _ := filepath.Rel(root, path)
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isCallToAction(lit.Type) {
				return true
			}
			found := callToAction{where: rel + ":" + strconv.Itoa(fset.Position(lit.Pos()).Line)}
			for _, elt := range lit.Elts {
				kv, isKV := elt.(*ast.KeyValueExpr)
				if !isKV {
					continue
				}
				key, isIdent := kv.Key.(*ast.Ident)
				if !isIdent {
					continue
				}
				switch key.Name {
				case "Command":
					found.command = literalPrefix(kv.Value)
				case "Tool":
					found.tool = literalPrefix(kv.Value)
				}
			}
			out = append(out, found)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("no calls to action found — the scanner is looking in the wrong place")
	}
	return out
}

func isCallToAction(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "CallToAction" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "apperr"
}

// literalPrefix evaluates the leading, statically known part of a string
// expression: literals, build.Name, and `+` chains of them.
//
// It stops at the first operand it cannot evaluate, and everything after that
// one is dropped rather than concatenated across the gap — otherwise
// `build.Name + " " + name + " list"` reads as "aos  list", which would have
// this test complaining about a group called "list".
func literalPrefix(e ast.Expr) string {
	text, _ := literalOf(e)
	return text
}

// literalOf returns what it could evaluate and whether the whole expression
// was evaluable.
func literalOf(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		if err != nil {
			return "", false
		}
		return s, true
	case *ast.SelectorExpr:
		pkg, ok := v.X.(*ast.Ident)
		if ok && pkg.Name == "build" && v.Sel.Name == "Name" {
			return build.Name, true
		}
		return "", false
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		left, whole := literalOf(v.X)
		if !whole {
			return left, false
		}
		right, rightWhole := literalOf(v.Y)
		return left + right, rightWhole
	default:
		return "", false
	}
}

// repoRoot walks up from this package to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := filepath.Glob(filepath.Join(dir, "go.mod")); err == nil {
			if matches, _ := filepath.Glob(filepath.Join(dir, "go.mod")); len(matches) == 1 {
				return dir
			}
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("the module root is not above this package")
	return ""
}

var _ = command.IssueCommand // the metadata key this file's sibling tests pin
