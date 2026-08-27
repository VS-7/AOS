// Command genwebui prepares the frontend bundle for embedding in the daemon.
//
// It copies frontend/dist into cmd/aosd/dist, gzipping what gzip actually
// shrinks and dropping the source maps.
//
// Why compress at build time rather than per request: the bundle is 53 MB and
// gzips to 14. That is the difference between a server binary somebody will
// download onto a VPS and one they will not — and a page fetched over a
// network rather than off the local disk wants its assets pre-compressed
// anyway, which is the shape a static server should have had regardless.
//
// Why the maps are dropped: 133 MB of them against 53 of bundle. They are
// exactly as useful in frontend/dist, where symbolication can still reach
// them, and exactly as useless inside a binary every user downloads.
//
// Why Go rather than three lines of shell: go-task runs commands through its
// own shell on Windows, where neither `find -delete` nor a gzip that keeps the
// original is available. The desktop's copy step already carries a paragraph
// of comment about that; this does the same work in one place that behaves the
// same everywhere.
package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "genwebui:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	src, dst := "frontend/dist", filepath.Join("cmd", "aosd", "dist")
	if len(args) >= 1 && args[0] != "" {
		src = args[0]
	}
	if len(args) >= 2 && args[1] != "" {
		dst = args[1]
	}

	if _, err := os.Stat(filepath.Join(src, "index.html")); err != nil {
		return fmt.Errorf("%s holds no index.html — build the frontend first", src)
	}

	// Everything but .gitkeep, which is the one versioned file in there and
	// the reason `//go:embed all:dist` compiles at all on a clean checkout.
	if err := clean(dst); err != nil {
		return err
	}

	var files, compressed int
	var rawTotal, outTotal int64
	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".map") {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rawTotal += int64(len(raw))

		out := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}

		// Only when it wins. Storing a .gz that is larger than the file it
		// replaces costs the binary space and every request a decompression.
		if squeezed, ok := shrink(raw); ok {
			out += ".gz"
			raw = squeezed
			compressed++
		}
		if err := os.WriteFile(out, raw, 0o644); err != nil {
			return err
		}
		files++
		outTotal += int64(len(raw))
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("genwebui: %d files (%d compressed) — %.1f MB → %.1f MB\n",
		files, compressed, mb(rawTotal), mb(outTotal))
	return nil
}

// shrink gzips b, and reports whether the result is worth keeping.
func shrink(b []byte) ([]byte, bool) {
	var buf bytes.Buffer
	// BestCompression, not the default: this runs once per release and the
	// bytes are downloaded by every visitor of every deployment.
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, false
	}
	if _, err := w.Write(b); err != nil {
		return nil, false
	}
	if err := w.Close(); err != nil {
		return nil, false
	}
	if buf.Len() >= len(b) {
		return nil, false
	}
	return buf.Bytes(), true
}

// clean empties dst of everything generated, keeping .gitkeep.
func clean(dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dst)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func mb(n int64) float64 { return float64(n) / (1024 * 1024) }
