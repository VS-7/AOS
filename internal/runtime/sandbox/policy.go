package sandbox

import (
	"path/filepath"
	"runtime"
	"strings"
)

// Permissions is what an agent may do at all.
//
// The zero value is read-only, and that is the default an agent with no sandbox
// block in its file gets. It is stricter than the original, which also defaults
// to read but hands over unrestricted execution the moment the execute
// permission appears.
type Permissions struct {
	Read    bool `yaml:"-" json:"read"`
	Write   bool `yaml:"-" json:"write"`
	Delete  bool `yaml:"-" json:"delete"`
	Execute bool `yaml:"-" json:"execute"`
}

// DefaultPermissions is what an unconfigured agent gets.
func DefaultPermissions() Permissions { return Permissions{Read: true} }

// PermissionsFrom reads the list form used in the agent's front matter.
func PermissionsFrom(list []string) Permissions {
	var p Permissions
	for _, item := range list {
		switch Op(strings.ToLower(strings.TrimSpace(item))) {
		case OpRead:
			p.Read = true
		case OpWrite:
			p.Write = true
		case OpDelete:
			p.Delete = true
		case OpExecute:
			p.Execute = true
		}
	}
	return p
}

// List renders the permissions back to the front-matter form.
func (p Permissions) List() []string {
	var out []string
	for _, c := range []struct {
		on bool
		op Op
	}{{p.Read, OpRead}, {p.Write, OpWrite}, {p.Delete, OpDelete}, {p.Execute, OpExecute}} {
		if c.on {
			out = append(out, string(c.op))
		}
	}
	return out
}

func (p Permissions) allows(op Op) bool {
	switch op {
	case OpRead:
		return p.Read
	case OpWrite:
		return p.Write
	case OpDelete:
		return p.Delete
	case OpExecute:
		return p.Execute
	}
	return false
}

// Exec policies.
const (
	PolicyAllowlist = "allowlist"
	PolicyDenyAll   = "deny-all"
)

// ExecPolicy is what an agent may run.
//
// An allowlist rather than a blocklist, which is the whole of ADR-0006. A
// blocklist by basename is bypassed by `bash -c`, by `python -c`, by `find
// -delete` and by four other spellings the reverse engineering catalogued —
// and a blocklist that does not block is worse than none, because the person
// granting the execute permission believes they are protected.
type ExecPolicy struct {
	Policy     string   `yaml:"policy" json:"policy"`
	Allow      []string `yaml:"allow" json:"allow,omitempty"`
	DenyArgs   []string `yaml:"denyArgs" json:"denyArgs,omitempty"`
	AllowShell bool     `yaml:"allowShell" json:"allowShell,omitempty"`
}

// DefaultExecPolicy runs nothing.
func DefaultExecPolicy() ExecPolicy { return ExecPolicy{Policy: PolicyDenyAll} }

// allows reports whether a resolved binary is on the list.
//
// Two names are offered and either one satisfies the list: the file the
// controlled PATH found, and the canonical file that one points at. The
// comparison is still never against what the caller typed — `./rm` and
// `/bin/rm` are the same target and are treated as one, which is the second
// layer of ADR-0006 and is unchanged.
//
// The found name is accepted because a distribution's own symlinks are not an
// attack. On Debian and Ubuntu /usr/bin/sh points at dash, so matching the
// canonical name alone made `allow: [sh]` unsatisfiable there while the same
// frontmatter worked on macOS — one agent, two behaviours, and no diagnostic
// saying so. vi, python and awk are shipped the same way. Nothing is given up
// by it: lookPath only ever produces a file the controlled PATH resolved (or
// one an absolute allowlist entry names — see lookExplicitPath), so a link
// that lies about where it goes had to be written into /usr/bin by root, who
// never needed a symlink to replace the binary outright. What this comment
// used to claim — that lookPath "searches minimalPath and nowhere else" — was
// not true of a name written as a path, and that gap is the bypass
// lookExplicitPath now closes.
func (p ExecPolicy) allows(found, canonical string) bool {
	if p.Policy != PolicyAllowlist {
		return false
	}
	for _, entry := range p.Allow {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if filepath.IsAbs(entry) {
			if sameFile(entry, canonical) || sameFile(entry, found) {
				return true
			}
			continue
		}
		entry = strings.TrimSuffix(entry, ".exe")
		if strings.EqualFold(entry, execBase(canonical)) || strings.EqualFold(entry, execBase(found)) {
			return true
		}
	}
	return false
}

// execBase is the name a binary is known by: its last path element, without
// the extension Windows carries, lowercased so the list is not case bait.
func execBase(path string) string {
	return strings.ToLower(strings.TrimSuffix(filepath.Base(path), ".exe"))
}

func sameFile(a, b string) bool {
	if a == b {
		return true
	}
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false
	}
	return ra == b
}

// shells is the list of names that are an interpreter for arbitrary commands.
// Reaching one is what turns an allowlist into a suggestion, so it takes its
// own opt-in on top of being allowed at all.
var shells = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true,
	"csh": true, "tcsh": true, "ash": true,
	"cmd": true, "powershell": true, "pwsh": true,
}

// isShell reports whether either name names a shell — either, not both. A
// binary reached under a shell's name is a shell here even when the file it
// resolves to is called something else, and the reverse too. AllowShell is an
// opt-in on top of being allowed at all, and a spelling must not be able to
// step around it.
func isShell(found, canonical string) bool {
	return shells[execBase(canonical)] || shells[execBase(found)]
}

// minimalPath is the PATH a resolved binary is looked up in and a child process
// runs with.
//
// It keeps the agent away from binaries somebody installed into their home
// directory, which is where a supply-chain problem lands first. Allowlisted
// binaries outside it are still reachable: the policy accepts an absolute path.
func minimalPath() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32;C:\Windows;C:\Windows\System32\WindowsPowerShell\v1.0`
	}
	return "/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin"
}
