package gitcli

import (
	"reflect"
	"testing"

	"github.com/OWNER/aos/internal/domain/file"
)

// TestParseChangesReadsPorcelainV1 pins the parsing of `git status
// --porcelain -z`, which is the one thing here worth testing without a real
// repository: the format is fixed, and every branch of it has a screen behind
// it in the Changes panel.
//
// -z rather than newline-separated, because a path with a newline or a space
// in it is not exotic on a Mac, and porcelain v1 quotes those in the default
// output in a form that then has to be unquoted. The NUL form never quotes.
func TestParseChangesReadsPorcelainV1(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  string
		want []file.Change
	}{
		{
			name: "a clean tree",
			out:  "",
			want: nil,
		},
		{
			name: "the four ordinary states",
			out: " M src/app.go\x00" +
				"A  README.md\x00" +
				" D old.txt\x00" +
				"?? scratch.log\x00",
			want: []file.Change{
				{Path: "src/app.go", Status: "modified"},
				{Path: "README.md", Status: "added"},
				{Path: "old.txt", Status: "deleted"},
				{Path: "scratch.log", Status: "untracked"},
			},
		},
		{
			// A rename is two NUL-separated fields: the new path, then the old
			// one. Reading it as one entry would put the old path in the list
			// as a file of its own, with whatever status the next entry had.
			name: "a rename carries where it came from",
			out:  "R  new/name.go\x00old/name.go\x00 M other.go\x00",
			want: []file.Change{
				{Path: "new/name.go", Status: "renamed", OldPath: "old/name.go"},
				{Path: "other.go", Status: "modified"},
			},
		},
		{
			name: "a path with a space in it",
			out:  " M docs/My Notes.md\x00",
			want: []file.Change{{Path: "docs/My Notes.md", Status: "modified"}},
		},
		{
			// Staged and then modified again. One path, one entry, and the
			// working-tree letter is the one the panel should show.
			name: "modified on both sides",
			out:  "MM src/app.go\x00",
			want: []file.Change{{Path: "src/app.go", Status: "modified"}},
		},
		{
			name: "a trailing separator does not invent an empty path",
			out:  " M a.go\x00\x00",
			want: []file.Change{{Path: "a.go", Status: "modified"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := parseChanges(tc.out)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
