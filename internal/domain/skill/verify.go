package skill

import (
	"net/url"
	"sort"
	"strings"
)

// ManifestVerifier is the default Verifier: content that exceeds what the
// manifest declares is refused, and the refusal names the excess. This is
// the check ADR-0015 exists for, and it is the installer's own verification,
// not the package author's word taken on trust — it runs before anything is
// written and before anyone is asked to approve.
type ManifestVerifier struct{}

// NewVerifier builds the default Verifier.
func NewVerifier() ManifestVerifier { return ManifestVerifier{} }

// VerifyManifest checks pkg's content against pkg.Manifest.Permissions.
//
// Four categories are checked because they are the ones a package can use to
// reach outside itself: a collection that was not declared, a toolset that
// runs a binary the manifest never named under exec, a toolset that reaches a
// host absent from network, and a hook that wants an event type the manifest
// never declared under hooks. Views carry no such check — a view renders
// declared data, it does not run anything — and neither does an agent beyond
// the identifier itself, which is checked against Permissions.Agents.
func (ManifestVerifier) VerifyManifest(pkg Package) (Diff, error) {
	perm := pkg.Manifest.Permissions
	var excess []string

	declaredCollections := toSet(perm.Collections)
	for _, c := range pkg.Collections {
		if !declaredCollections[c.ID] {
			excess = append(excess, "collection "+c.ID)
		}
	}

	declaredExec := toSet(perm.Exec)
	declaredHosts := toSet(perm.Network)
	for _, ts := range pkg.Toolsets {
		if ts.Command != "" && !declaredExec[ts.Command] {
			excess = append(excess, "exec "+ts.Command)
		}
		if ts.BaseURL != "" {
			if host := hostOf(ts.BaseURL); host != "" && !declaredHosts[host] {
				excess = append(excess, "network "+host)
			}
		}
	}

	declaredHooks := toSet(perm.Hooks)
	for _, h := range pkg.Hooks {
		for _, ev := range h.Events {
			if !declaredHooks[string(ev)] {
				excess = append(excess, "hook "+string(ev))
			}
		}
	}

	declaredAgents := toSet(perm.Agents)
	for _, id := range agentIDsIn(pkg.Files) {
		if !declaredAgents[id] {
			excess = append(excess, "agent "+id)
		}
	}

	if len(excess) > 0 {
		sort.Strings(excess)
		return Diff{}, errManifestExceeded(excess)
	}
	return Diff{Permissions: perm}, nil
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[i] = true
	}
	return m
}

// hostOf reads the host out of a toolset's base URL. An unparseable value is
// not this check's business — connecting to it is Adapter.Connect's problem —
// so it is skipped here rather than refused twice for the same reason.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// agentIDsIn scans a package's raw files for the ids of the agents it
// brings: the first path segment under "agents/". Deduplicated, because an
// agent with several files — AGENT.md plus its memories and routines — must
// not be counted, or named as excess, once per file.
func agentIDsIn(files []RawFile) []string {
	const prefix = "agents/"
	seen := map[string]bool{}
	var ids []string
	for _, f := range files {
		if !strings.HasPrefix(f.Path, prefix) {
			continue
		}
		id, _, ok := strings.Cut(strings.TrimPrefix(f.Path, prefix), "/")
		if !ok || id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}
