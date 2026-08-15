package apperr

import "sort"

// Entry is one row of the error catalog: a code that exists somewhere in the
// tree, with the properties the invariants are checked against.
//
// The catalog is generated from source (task gen-catalog) rather than
// hand-curated. The original's catalog was hand-collected and ended up with
// environment variable names in it (FRACTAL_BASE_URL, FRACTAL_TOKEN); a
// generated catalog cannot drift from the code that way.
type Entry struct {
	Code    string   `json:"code"`
	Package string   `json:"package"`
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Status  int      `json:"status"`
	Causer  string   `json:"causer"`
	CTA     bool     `json:"cta"`
	Issues  []string `json:"issues,omitempty"`
}

// Codes returns every catalogued code, sorted.
func Codes() []string {
	out := make([]string, 0, len(Catalog))
	for _, e := range Catalog {
		out = append(out, e.Code)
	}
	sort.Strings(out)
	return out
}

// Lookup returns the catalog entry for a code, if it exists.
func Lookup(code string) (Entry, bool) {
	want := qualify(code)
	for _, e := range Catalog {
		if e.Code == want {
			return e, true
		}
	}
	return Entry{}, false
}
