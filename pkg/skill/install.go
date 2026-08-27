package skill

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Target is one place an agent harness reads skills from.
//
// Every harness that follows the Agent Skills convention scans
// `<something>/skills/<name>/SKILL.md`; what differs is the `<something>`.
// The list below is the set in use today, plus `.agents`, which is the
// cross-client directory the convention itself recommends so that a skill
// installed once is visible to every compliant client.
type Target struct {
	// ID is the name a person types: `aos self skill install --to codex`.
	ID string `json:"id"`
	// Label is the harness's display name.
	Label string `json:"label"`
	// Dir is the skills directory, absolute.
	Dir string `json:"dir"`
	// Present reports whether the harness appears to be installed on this
	// machine — its configuration directory (the parent of Dir) exists.
	// `install --all` writes only where this is true, so a machine without
	// Cursor does not grow a ~/.cursor of its own.
	Present bool `json:"present"`
	// Installed reports whether this skill is already there.
	Installed bool `json:"installed"`
}

// knownTargets lists the harness directories, relative to the home directory.
// Order is display order.
var knownTargets = []struct {
	id, label, dir string
}{
	{"agents", "Any agent (.agents convention)", ".agents/skills"},
	{"claude-code", "Claude Code", ".claude/skills"},
	{"codex", "Codex", ".codex/skills"},
	{"cursor", "Cursor", ".cursor/skills"},
	{"gemini", "Gemini CLI", ".gemini/skills"},
	{"opencode", "OpenCode", ".config/opencode/skills"},
}

// Targets lists every known harness location under home, saying which ones
// look installed and which already hold this skill. home is a parameter
// rather than read here so a test can point it at a temporary directory.
func Targets(home, name string) []Target {
	out := make([]Target, 0, len(knownTargets))
	for _, k := range knownTargets {
		dir := filepath.Join(home, filepath.FromSlash(k.dir))
		t := Target{ID: k.id, Label: k.label, Dir: dir}
		if info, err := os.Stat(filepath.Dir(dir)); err == nil && info.IsDir() {
			t.Present = true
		}
		if _, err := os.Stat(filepath.Join(dir, name, "SKILL.md")); err == nil {
			t.Installed = true
		}
		out = append(out, t)
	}
	return out
}

// TargetIDs is the list a flag's help text and a validation error both need.
func TargetIDs() []string {
	ids := make([]string, 0, len(knownTargets))
	for _, k := range knownTargets {
		ids = append(ids, k.id)
	}
	return ids
}

// LookupTarget resolves an id — "codex", "claude-code" — to its Target under
// home, or reports that there is no such harness in the list.
func LookupTarget(home, name, id string) (Target, bool) {
	for _, t := range Targets(home, name) {
		if t.ID == id {
			return t, true
		}
	}
	return Target{}, false
}

// InstallResult reports what Install did, per directory.
type InstallResult struct {
	// Installed lists the skill directories written: <target>/<name>.
	Installed []string `json:"installed"`
	// Skipped maps a target directory to why it was left alone.
	Skipped map[string]string `json:"skipped"`
}

// Install writes the skill held in src (SKILL.md + references/, the shape
// Generate produces and Files embeds) into <dir>/<name>/ for every dir in
// dirs, creating the directories that do not exist yet.
//
// Creating is the difference from Sync: Sync is the generator's own
// synchronisation and refuses to invent a harness directory, while Install
// is a person or the desktop asking for the skill in a place they named —
// an explicit ask is exactly the moment ~/.codex/skills may be created.
//
// What both keep is the one guard that matters: a directory that already
// holds a skill whose frontmatter `name` differs from name is somebody
// else's, living at the same path by coincidence, and is never overwritten.
func Install(src fs.FS, dirs []string, name string) (InstallResult, error) {
	if strings.TrimSpace(name) == "" {
		return InstallResult{}, fmt.Errorf("skill: Install needs a skill name")
	}
	result := InstallResult{Skipped: map[string]string{}}
	for _, dir := range dirs {
		dest := filepath.Join(dir, name)
		if existing, err := os.ReadFile(filepath.Join(dest, "SKILL.md")); err == nil {
			if existingName := frontmatterName(string(existing)); existingName != "" && existingName != name {
				result.Skipped[dest] = fmt.Sprintf("already holds a different skill (name: %q)", existingName)
				continue
			}
		}
		if err := copyFS(src, dest); err != nil {
			return result, fmt.Errorf("skill: installing into %s: %w", dest, err)
		}
		result.Installed = append(result.Installed, dest)
	}
	sort.Strings(result.Installed)
	return result, nil
}

// copyFS replaces dest with a fresh copy of every regular file under src,
// preserving the relative layout. Only regular files are copied: the embed
// holds nothing else, and a source directory with a symlink in it is not one
// this package produced.
func copyFS(src fs.FS, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(dest, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		content, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644) //nolint:gosec // skill text, not a secret
	})
}
