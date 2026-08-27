// Command covercheck enforces the per-package coverage floors.
//
// The gate is per package, not global: a high average hides one critical
// package with no tests at all.
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// floors maps an import path prefix to the minimum statement coverage. The
// longest matching prefix wins, so a package can be held to a higher bar than
// the tree it lives in.
var floors = map[string]float64{
	"github.com/OWNER/aos/internal/domain": 80,
	"github.com/OWNER/aos/internal/core":   80,
	// pkg/ is the one package meant to be useful outside this project (see
	// its own doc comment) — held to the same bar as internal/core, not
	// exempted as wiring.
	"github.com/OWNER/aos/pkg":                80,
	"github.com/OWNER/aos/internal/runtime":   75,
	"github.com/OWNER/aos/internal/transport": 60,
	// Adapters are held to their contract suite, not to a coverage number.
	"github.com/OWNER/aos/internal/adapters": 0,
	// The theme presets are files in a directory, exercised through the theme
	// aggregate and through the app. What would a percentage here measure?
	"github.com/OWNER/aos/internal/adapters/fsthemes": 0,
	// Model providers are adapters too, and the same reasoning applies: what
	// says an adapter is right is providertest running the same conversation
	// through it, not the fraction of its translation table a test touched.
	"github.com/OWNER/aos/internal/runtime/providers": 0,
	// Test infrastructure is proven by the tests that use it. A coverage
	// number on a fake or a fixture measures nothing: the contract suite is
	// what says whether the fake is right.
	"github.com/OWNER/aos/internal/domain/fakes":                    0,
	"github.com/OWNER/aos/internal/domain/testsuite":                0,
	"github.com/OWNER/aos/internal/domain/marketplace/registrytest": 0,
	"github.com/OWNER/aos/internal/testx":                           0,
	// The composition root is wiring: what it must prove is that the surfaces
	// agree, and that is the parity suite, not a percentage.
	"github.com/OWNER/aos/internal/app": 0,
	// The session runner is the same kind of thing one layer down: it holds
	// the pieces of a turn together and owns almost no behaviour of its own.
	// What it must prove is that a real message produces a real answer, and
	// that is TestTheDeliveryOfPhaseFive in internal/app. The parts of it that
	// are logic rather than wiring — the transcript translation and the policy
	// an agent's file declares — have their own tests beside them.
	"github.com/OWNER/aos/internal/runtime/session": 0,
	// Wiring and generators are exercised through the packages they serve.
	"github.com/OWNER/aos/cmd":                   0,
	"github.com/OWNER/aos/tools":                 0,
	"github.com/OWNER/aos/internal/architecture": 0,
}

// minimumPackages is how many packages a complete run reports. It is a floor
// on the *run*, not on any package: well under the real count (125 at the
// time of writing) so that adding or removing a package does not touch it,
// and far enough above zero that a truncated run cannot be mistaken for a
// clean one.
const minimumPackages = 100

func main() {
	type result struct {
		pkg      string
		coverage float64
		floor    float64
	}
	var failures []result
	var checked int

	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "coverage:") {
			continue
		}
		pkg, pct, ok := parse(line)
		if !ok {
			continue
		}
		floor, found := floorFor(pkg)
		if !found {
			fmt.Fprintf(os.Stderr, "covercheck: no floor declared for %s\n", pkg)
			os.Exit(1)
		}
		checked++
		if pct+1e-9 < floor {
			failures = append(failures, result{pkg, pct, floor})
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "covercheck:", err)
		os.Exit(1)
	}

	// A run that measured almost nothing must not pass for a run that
	// measured everything. `go test -cover ./...` can end early — a package
	// that fails to build, a full disk, an interrupted run — and every line
	// covercheck never saw is a floor it never enforced. It reported
	// "1 packages, all at or above their floor" and exited 0 while the tree
	// has more than a hundred, which is a gate saying yes to a question it
	// was not asked.
	if checked < minimumPackages {
		fmt.Fprintf(os.Stderr,
			"covercheck: only %d packages reported coverage, and this tree has at least %d — "+
				"the test run did not finish, so nothing here was actually checked\n",
			checked, minimumPackages)
		os.Exit(1)
	}

	if len(failures) == 0 {
		fmt.Printf("covercheck: %d packages, all at or above their floor\n", checked)
		return
	}
	sort.Slice(failures, func(i, j int) bool { return failures[i].pkg < failures[j].pkg })
	for _, f := range failures {
		fmt.Fprintf(os.Stderr, "covercheck: %s is at %.1f%%, floor is %.0f%%\n", f.pkg, f.coverage, f.floor)
	}
	os.Exit(1)
}

// parse reads a `go test -cover` line, in either the "ok" or the untested form.
func parse(line string) (pkg string, pct float64, ok bool) {
	fields := strings.Fields(line)
	for i, f := range fields {
		if strings.HasPrefix(f, "github.com/") {
			pkg = f
		}
		if f == "coverage:" && i+1 < len(fields) {
			v := strings.TrimSuffix(fields[i+1], "%")
			// "[no statements]" is not a number and not a failure.
			n, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return pkg, 0, false
			}
			pct = n
			ok = true
		}
	}
	return pkg, pct, ok && pkg != ""
}

func floorFor(pkg string) (float64, bool) {
	best, found := 0.0, false
	longest := -1
	for prefix, floor := range floors {
		if pkg != prefix && !strings.HasPrefix(pkg, prefix+"/") {
			continue
		}
		if len(prefix) > longest {
			best, longest, found = floor, len(prefix), true
		}
	}
	return best, found
}
