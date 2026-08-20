// Package skillfetch fetches a skill package from a source.
//
// Local is the only Fetcher this build ships: a directory on disk becomes a
// skill.Package by reading its SKILL.md and everything under it. A registry
// fetcher — HTTP, git — is future work; nothing in skill.Installer assumes
// which one is wired in, which is the whole reason skill.Fetcher is a port.
package skillfetch

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/OWNER/aos/internal/core/pathx"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/skill"
	"github.com/OWNER/aos/internal/domain/view"
)

// Local reads a skill package from a directory.
//
// It never fetches a remote address — Fetch refuses a non-empty ref — which
// is why the Manifest it produces leaves Version and Source exactly as the
// front matter gave them, and why the Skill Install eventually builds from it
// leaves Commit blank: a directory on disk has no ref to check out and no
// commit to record, and inventing either would be worse than leaving it
// empty.
type Local struct{}

// New builds the local directory Fetcher.
func New() Local { return Local{} }

// Fetch reads source as a directory and returns its content as a
// skill.Package, not yet applied. Every path it reads — the standard
// collections/, views/, toolsets/ and the free-form directories a package
// brings — is confined to source: a SKILL.md that names a resource path
// which climbs out is refused rather than followed, the same way any other
// filesystem boundary in this system is (internal/core/pathx). A package
// that could read outside itself could read the workspace's .env.
func (Local) Fetch(_ context.Context, source, ref string) (skill.Package, error) {
	if strings.TrimSpace(ref) != "" {
		return skill.Package{}, errRefNotSupported(ref)
	}

	abs, err := filepath.Abs(source)
	if err != nil {
		return skill.Package{}, errRead(source, err)
	}
	root, err := pathx.Root(abs)
	if err != nil {
		return skill.Package{}, errRead(source, err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return skill.Package{}, errNotAPackage(source)
	}

	manifestPath := filepath.Join(root, "SKILL.md")
	manifestData, err := os.ReadFile(manifestPath) //nolint:gosec // root is resolved and stat-checked above; the filename is fixed
	if err != nil {
		return skill.Package{}, errNotAPackage(source)
	}
	front, body := splitFrontMatter(manifestData)

	var fm frontMatter
	if len(strings.TrimSpace(string(front))) > 0 {
		if err := yaml.Unmarshal(front, &fm); err != nil {
			return skill.Package{}, errManifest(source, err)
		}
	}

	pkg := skill.Package{
		Manifest: skill.Manifest{
			Name:        fm.Name,
			Description: fm.Description,
			Version:     fm.Version,
			Source:      fm.Source,
			Permissions: fm.Permissions,
		},
		Content: strings.TrimRight(string(body), "\n"),
	}

	if pkg.Collections, err = readCollections(root); err != nil {
		return skill.Package{}, err
	}
	if pkg.Views, err = readViews(root); err != nil {
		return skill.Package{}, err
	}
	if pkg.Toolsets, err = readToolsets(root); err != nil {
		return skill.Package{}, err
	}
	if pkg.Files, err = readFiles(root); err != nil {
		return skill.Package{}, err
	}
	resources, err := readResources(root, fm.Resources)
	if err != nil {
		return skill.Package{}, err
	}
	pkg.Files = append(pkg.Files, resources...)

	return pkg, nil
}

// frontMatter is the YAML front matter of SKILL.md, per ADR-0015.
type frontMatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Version     string            `yaml:"version,omitempty"`
	Source      string            `yaml:"source,omitempty"`
	Permissions skill.Permissions `yaml:"permissions,omitempty"`
	Resources   []skill.Resource  `yaml:"resources,omitempty"`
}

// toolsetFrontMatter is the subset of a toolset's own front matter this
// package reads to verify it against the manifest; the file itself is
// relocated unmodified by Apply, so nothing else about it needs decoding
// here.
type toolsetFrontMatter struct {
	Type    string `yaml:"type"`
	Command string `yaml:"command,omitempty"`
	BaseURL string `yaml:"baseUrl,omitempty"`
}

// readCollections decodes collections/{id}/schema.json for every declared
// collection. A package with no collections/ directory brings none — that is
// not an error, the same way an agent with no memories yet is not one.
func readCollections(root string) ([]collection.CreateInput, error) {
	dir := filepath.Join(root, "collections")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errRead(dir, err)
	}
	var out []collection.CreateInput
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		schemaPath := filepath.Join(dir, e.Name(), "schema.json")
		data, err := os.ReadFile(schemaPath) //nolint:gosec // schemaPath is built from a directory entry under root, not user input
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, errRead(schemaPath, err)
		}
		var in collection.CreateInput
		if err := json.Unmarshal(data, &in); err != nil {
			return nil, errDecode(schemaPath, err)
		}
		if in.ID == "" {
			in.ID = e.Name()
		}
		out = append(out, in)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// readViews decodes views/{id}.view.json for every declared view.
func readViews(root string) ([]view.CreateInput, error) {
	dir := filepath.Join(root, "views")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errRead(dir, err)
	}
	var out []view.CreateInput
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".view.json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p) //nolint:gosec // p is built from a directory entry under root, not user input
		if err != nil {
			return nil, errRead(p, err)
		}
		var in view.CreateInput
		if err := json.Unmarshal(data, &in); err != nil {
			return nil, errDecode(p, err)
		}
		if in.ID == "" {
			in.ID = strings.TrimSuffix(e.Name(), ".view.json")
		}
		out = append(out, in)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// readToolsets decodes toolsets/{id}.toolset.md for every declared toolset:
// enough of its front matter (type, command, baseUrl) for VerifyManifest to
// check it, plus the file itself, unmodified, for Apply to write.
func readToolsets(root string) ([]skill.ToolsetDecl, error) {
	dir := filepath.Join(root, "toolsets")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errRead(dir, err)
	}
	var out []skill.ToolsetDecl
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toolset.md") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p) //nolint:gosec // p is built from a directory entry under root, not user input
		if err != nil {
			return nil, errRead(p, err)
		}
		front, _ := splitFrontMatter(data)
		var tm toolsetFrontMatter
		if len(strings.TrimSpace(string(front))) > 0 {
			if err := yaml.Unmarshal(front, &tm); err != nil {
				return nil, errDecode(p, err)
			}
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil, errRead(p, err)
		}
		out = append(out, skill.ToolsetDecl{
			ID:      strings.TrimSuffix(e.Name(), ".toolset.md"),
			Type:    tm.Type,
			Command: tm.Command,
			BaseURL: tm.BaseURL,
			RawFile: skill.RawFile{Path: filepath.ToSlash(rel), Content: data},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// readFiles walks everything under root except SKILL.md and the three
// directories with their own reader above, and returns it as raw files: an
// agent's directory (with its memories and routines nested inside), a
// template, an instruction, a goal, a reference. The walk never leaves root —
// filepath.WalkDir only visits real directory entries — so this is safe
// without pathx; readResources below is where a declared path, rather than an
// entry the walk found itself, needs confining.
func readFiles(root string) ([]skill.RawFile, error) {
	skipTop := map[string]bool{"SKILL.md": true, "collections": true, "views": true, "toolsets": true}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, errRead(root, err)
	}
	var out []skill.RawFile
	for _, e := range entries {
		if skipTop[e.Name()] || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		sub := filepath.Join(root, e.Name())
		walkErr := filepath.WalkDir(sub, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, rerr := os.ReadFile(p) //nolint:gosec // p comes from WalkDir over sub, itself under root
			if rerr != nil {
				return rerr
			}
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				return rerr
			}
			out = append(out, skill.RawFile{Path: filepath.ToSlash(rel), Content: data})
			return nil
		})
		if walkErr != nil {
			return nil, errRead(sub, walkErr)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// readResources reads every resource the manifest declares by a path
// relative to the package, confined with pathx.ResolveInside — this is the
// one read in this fetcher driven by a path the package itself names rather
// than one the directory walk found on its own, and it is exactly the read a
// SKILL.md with "../../../.env" in it is trying to get: refused, not
// followed. A resource whose uri looks like an absolute URL is left alone;
// this fetcher only ever reads its own package.
func readResources(root string, resources []skill.Resource) ([]skill.RawFile, error) {
	var out []skill.RawFile
	for _, r := range resources {
		uri := strings.TrimSpace(r.URI)
		if uri == "" || strings.Contains(uri, "://") {
			continue
		}
		p, err := pathx.ResolveInside(root, uri)
		if err != nil {
			return nil, errOutsidePackage(uri, err)
		}
		data, err := os.ReadFile(p) //nolint:gosec // p was just confined to root by pathx.ResolveInside
		if err != nil {
			return nil, errRead(p, err)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil, errRead(p, err)
		}
		out = append(out, skill.RawFile{Path: filepath.ToSlash(rel), Content: data})
	}
	return out, nil
}

// splitFrontMatter separates the YAML block of a Markdown file from its
// body. It is a smaller, adapter-local twin of the engine's own
// collections.splitFrontMatter — unexported there, and this package reads a
// package on disk before any collections.Model exists to decode it through.
func splitFrontMatter(data []byte) (front, body []byte) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return nil, data
	}
	rest := s[len("---\n"):]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return []byte(rest), nil
	}
	front = []byte(rest[:idx])
	after := strings.TrimPrefix(rest[idx+len("\n---"):], "\n")
	return front, []byte(after)
}
