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
	"github.com/OWNER/aos/internal/domain":    80,
	"github.com/OWNER/aos/internal/core":      80,
	"github.com/OWNER/aos/internal/runtime":   75,
	"github.com/OWNER/aos/internal/transport": 60,
	// Adapters are held to their contract suite, not to a coverage number.
	"github.com/OWNER/aos/internal/adapters": 0,
	// Test infrastructure is proven by the tests that use it. A coverage
	// number on a fake or a fixture measures nothing: the contract suite is
	// what says whether the fake is right.
	"github.com/OWNER/aos/internal/domain/fakes":     0,
	"github.com/OWNER/aos/internal/domain/testsuite": 0,
	"github.com/OWNER/aos/internal/testx":            0,
	// Wiring and generators are exercised through the packages they serve.
	"github.com/OWNER/aos/cmd":                   0,
	"github.com/OWNER/aos/tools":                 0,
	"github.com/OWNER/aos/internal/architecture": 0,
}

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
