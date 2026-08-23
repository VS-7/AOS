package skill

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SyncResult reports what Sync actually did, per target.
type SyncResult struct {
	// Synced lists target directories the skill was written (or
	// overwritten) into.
	Synced []string
	// Skipped maps a target directory to why Sync left it alone.
	Skipped map[string]string
}

// Sync copies the generated skill at srcDir (SKILL.md + references/, the
// output of Generate) into name's own subdirectory under each of
// targetDirs — creating <targetDir>/<name>/ when it does not exist yet.
//
// A target directory that does not exist is skipped, not created: an agent
// harness this machine does not have installed (no ~/.claude, say) is not
// this function's business to invent. A target that already holds a skill
// whose frontmatter `name` differs from name is also skipped — that is
// somebody else's skill living at the same path by coincidence, not a
// stale copy of this one, and overwriting it would be data loss with no
// way back.
func Sync(srcDir string, targetDirs []string, name string) (SyncResult, error) {
	result := SyncResult{Skipped: map[string]string{}}

	for _, targetDir := range targetDirs {
		info, err := os.Stat(targetDir)
		if err != nil || !info.IsDir() {
			result.Skipped[targetDir] = "target directory does not exist"
			continue
		}

		dest := filepath.Join(targetDir, name)
		if existing, err := os.ReadFile(filepath.Join(dest, "SKILL.md")); err == nil {
			if existingName := frontmatterName(string(existing)); existingName != "" && existingName != name {
				result.Skipped[dest] = fmt.Sprintf("already holds a different skill (name: %q)", existingName)
				continue
			}
		}

		if err := copyTree(srcDir, dest); err != nil {
			return result, fmt.Errorf("skill: syncing to %s: %w", dest, err)
		}
		result.Synced = append(result.Synced, dest)
	}
	return result, nil
}

// frontmatterName reads the `name:` field of a SKILL.md's YAML frontmatter
// — a single-field scan rather than a YAML dependency, which is all this
// ever needs to read.
func frontmatterName(skillMD string) string {
	lines := strings.Split(skillMD, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		if rest, ok := strings.CutPrefix(trimmed, "name:"); ok {
			return strings.Trim(strings.TrimSpace(rest), `"'`)
		}
	}
	return ""
}

// copyTree replaces dest with a fresh copy of every file under src,
// preserving the relative layout (SKILL.md, references/*.md).
func copyTree(src, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644) //nolint:gosec // generated skill text, not a secret
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
