// Package marketplacegit implements marketplace.Registry over a plain Git
// repository: a registry is a repo with an index.json at its root naming
// each listing and the subdirectory its package lives in. No central
// service is required — a repository with an index is sufficient, which is
// what lets a private corporate registry exist without any infrastructure
// of our own.
package marketplacegit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/adapters/skillfetch"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/marketplace"
	"github.com/OWNER/aos/internal/domain/skill"
)

// defaultTimeout bounds one clone. A registry that has not answered in
// thirty seconds is not going to.
const defaultTimeout = 30 * time.Second

// Registry reaches one Git repository. Every Search and Fetch clones fresh,
// depth 1, into a temp directory removed before returning — this is not
// cached, deliberately: a registry's own git remote is already a cache in
// front of whatever wrote it, and a second, longer-lived one here would be
// a second place staleness could hide.
type Registry struct {
	// URL is anything `git clone` accepts: an https URL, an ssh remote, or
	// a local path — the last is what makes this adapter testable without a
	// network.
	URL string

	// Binary is the git executable. Empty means "git", resolved on PATH.
	Binary string

	// Timeout bounds one clone. Zero means defaultTimeout.
	Timeout time.Duration
}

// New builds a Registry over url.
func New(url string) *Registry { return &Registry{URL: url} }

var _ marketplace.Registry = (*Registry)(nil)

// Search clones the registry, reads its index, filters, and — because
// permissions have to be visible at discovery time, per ADR-0015 — fetches
// each surviving listing's own SKILL.md to read its Permissions. A listing
// whose package cannot be read is skipped rather than failing the whole
// search: an index entry pointing at a broken package is that package's
// problem, not the search's.
func (r *Registry) Search(ctx context.Context, q marketplace.SearchQuery) ([]marketplace.Listing, error) {
	dir, cleanup, err := r.clone(ctx, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	idx, err := readIndex(dir)
	if err != nil {
		return nil, err
	}

	fetcher := skillfetch.New()
	var out []marketplace.Listing
	for _, e := range idx.Listings {
		if !matches(e, q) {
			continue
		}
		var perms skill.Permissions
		if pkg, err := fetcher.Fetch(ctx, filepath.Join(dir, e.Path), ""); err == nil {
			perms = pkg.Manifest.Permissions
		}
		out = append(out, marketplace.Listing{
			Source:      e.Source,
			Name:        e.Name,
			Description: e.Description,
			Version:     e.Version,
			Tags:        e.Tags,
			Stars:       e.Stars,
			UpdatedAt:   e.UpdatedAt,
			Permissions: perms,
		})
	}
	return out, nil
}

// Fetch clones the registry at ref, resolves source in its index, and
// reads the package the same way skillfetch.Local reads any other local
// directory — a cloned package is, at that point, exactly that.
func (r *Registry) Fetch(ctx context.Context, source, ref string) (skill.Package, error) {
	dir, cleanup, err := r.clone(ctx, ref)
	if err != nil {
		return skill.Package{}, err
	}
	defer cleanup()

	idx, err := readIndex(dir)
	if err != nil {
		return skill.Package{}, err
	}
	for _, e := range idx.Listings {
		if e.Source == source {
			return skillfetch.New().Fetch(ctx, filepath.Join(dir, e.Path), "")
		}
	}
	return skill.Package{}, errSourceNotInIndex(r.URL, source)
}

func matches(e indexEntry, q marketplace.SearchQuery) bool {
	if q.Owner != "" && !strings.HasPrefix(e.Source, q.Owner+"/") {
		return false
	}
	if q.Tag != "" {
		found := false
		for _, t := range e.Tags {
			if t == q.Tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if q.Text != "" {
		text := strings.ToLower(q.Text)
		if !strings.Contains(strings.ToLower(e.Name), text) &&
			!strings.Contains(strings.ToLower(e.Description), text) &&
			!strings.Contains(strings.ToLower(e.Source), text) {
			return false
		}
	}
	return true
}

// indexFile is the registry's own index.json, at its root.
type indexFile struct {
	Listings []indexEntry `json:"listings"`
}

// indexEntry is one listing's index record: everything Search can answer
// without fetching the package, plus Path, where Fetch finds it.
type indexEntry struct {
	Source      string    `json:"source"` // "<owner>/<repo>"
	Path        string    `json:"path"`   // subdirectory the package lives in
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	Tags        []string  `json:"tags,omitempty"`
	Stars       int       `json:"stars,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func readIndex(dir string) (indexFile, error) {
	b, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return indexFile{}, errIndexMissing(dir, err)
	}
	var idx indexFile
	if err := json.Unmarshal(b, &idx); err != nil {
		return indexFile{}, errIndexInvalid(dir, err)
	}
	return idx, nil
}

func (r *Registry) binary() string {
	if r.Binary != "" {
		return r.Binary
	}
	return "git"
}

func (r *Registry) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return defaultTimeout
}

// clone checks out ref (HEAD when empty) into a fresh temp directory. The
// caller must call cleanup once it is done reading from dir.
func (r *Registry) clone(ctx context.Context, ref string) (dir string, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "aos-marketplacegit-*")
	if err != nil {
		return "", nil, errCloneFailed(r.URL, err)
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }

	args := []string{"clone", "--depth", "1"}
	if strings.TrimSpace(ref) != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, r.URL, tmp)

	cctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()
	cmd := exec.CommandContext(cctx, r.binary(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return "", nil, errGitMissing(err)
		}
		return "", nil, errCloneFailed(r.URL, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String())))
	}
	return tmp, cleanup, nil
}

func errGitMissing(cause error) error {
	return apperr.New("MARKETPLACEGIT_BINARY_MISSING").
		Causer("marketplacegit.Registry.clone").
		Msgf("git is not on PATH: %v", cause).
		Status(apperr.StatusServiceUnavailable).
		CTA(apperr.CallToAction{Label: "install git"})
}

func errCloneFailed(url string, cause error) error {
	return apperr.New("MARKETPLACEGIT_CLONE_FAILED").
		Causer("marketplacegit.Registry.clone").
		Msgf("could not clone %q: %v", url, cause).
		Issue("url", url).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check the registry URL and that it is reachable"})
}

func errIndexMissing(dir string, cause error) error {
	return apperr.New("MARKETPLACEGIT_INDEX_MISSING").
		Causer("marketplacegit.readIndex").
		Msgf("no index.json at the registry root: %v", cause).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the registry must publish an index.json at its root"})
}

func errIndexInvalid(dir string, cause error) error {
	return apperr.New("MARKETPLACEGIT_INDEX_INVALID").
		Causer("marketplacegit.readIndex").
		Msgf("index.json is not valid: %v", cause).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "fix the registry's index.json"})
}

func errSourceNotInIndex(url, source string) error {
	return apperr.New("MARKETPLACEGIT_SOURCE_NOT_FOUND").
		Causer("marketplacegit.Registry.Fetch").
		Msgf("registry %q has no listing %q", url, source).
		Issue("url", url).
		Issue("source", source).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "search the registry before fetching a specific listing"})
}
