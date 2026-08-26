package gitcli

import (
	"context"
	"strings"

	"github.com/OWNER/aos/internal/domain/file"
)

// Changes lists every path the working tree differs from HEAD at.
//
// It is the list Status answers one path at a time. The Changes panel needs
// the whole set, and building it out of Status would mean walking the
// repository and shelling out once per file — which on any real repository is
// thousands of processes to draw one list.
//
// A directory that is not a repository comes back as an error from git, and
// the caller (file.Service.Changes) reads that as "nothing to report" rather
// than a failure: a workspace does not have to be a repository.
func (g *Git) Changes(ctx context.Context, dir string) ([]file.Change, error) {
	// -z, not the default: porcelain v1 quotes any path with a space, a
	// newline or a non-ASCII byte in it, in a C-string form that then has to
	// be unquoted correctly. The NUL-separated form never quotes, and paths
	// with spaces in them are ordinary on a Mac.
	//
	// --untracked-files=normal lists an untracked directory as the directory
	// rather than every file beneath it, which is what somebody scanning the
	// panel wants to see for a fresh node_modules.
	out, err := g.run(ctx, dir, "status", "--porcelain", "-z", "--untracked-files=normal")
	if err != nil {
		return nil, errGitFailed("status", dir, err)
	}
	return parseChanges(out), nil
}

// parseChanges reads `git status --porcelain -z`.
//
// Each entry is `XY<space><path>`, NUL-terminated. A rename or a copy is
// followed by a second NUL-terminated field holding the path it came from —
// which is why this consumes the fields with an index rather than ranging over
// a split: reading that second field as an entry of its own would put the old
// path in the list as a file, carrying whatever status came after it.
func parseChanges(out string) []file.Change {
	fields := strings.Split(out, "\x00")

	var changes []file.Change
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		// Split leaves a trailing empty field after the last NUL, and a
		// truncated read could leave a fragment too short to hold a status.
		if len(entry) < 4 {
			continue
		}

		code, path := entry[:2], entry[3:]
		change := file.Change{Path: path, Status: statusOf(code)}

		if change.Status == "renamed" && i+1 < len(fields) {
			i++
			change.OldPath = fields[i]
		}
		changes = append(changes, change)
	}
	return changes
}

// statusOf reduces the two porcelain letters — index state, then working-tree
// state — to the one word the panel shows.
//
// The order matters. A path can be both added to the index and deleted from
// the working tree, and "??" is the only code where both letters are the same
// character, so untracked is checked first and the rest in order of how much
// the reader needs to know: a rename is a move, a deletion is gone, an
// addition is new, and everything else that differs is a modification.
func statusOf(code string) string {
	switch {
	case strings.Contains(code, "?"):
		return "untracked"
	case strings.Contains(code, "R"):
		return "renamed"
	case strings.Contains(code, "D"):
		return "deleted"
	case strings.Contains(code, "A"), strings.Contains(code, "C"):
		return "added"
	default:
		return "modified"
	}
}
