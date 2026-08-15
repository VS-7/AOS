// Command gencatalog regenerates internal/core/apperr/catalog.gen.go from the
// apperr.New calls in the tree. Run it with `task gen-catalog`; CI verifies
// that the committed file matches by running the generator and diffing.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OWNER/aos/internal/core/apperr/scan"
)

func main() {
	root := flag.String("root", ".", "module root to scan")
	out := flag.String("out", filepath.Join("internal", "core", "apperr", "catalog.gen.go"), "output file")
	flag.Parse()

	entries, err := scan.Scan(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gencatalog:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, scan.Render(entries), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gencatalog:", err)
		os.Exit(1)
	}
	fmt.Printf("gencatalog: %d codes → %s\n", len(entries), *out)
}
