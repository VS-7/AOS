// Package file backs the UI file explorer.
//
// It is deliberately not exposed as commands or MCP tools: the agent's
// filesystem path is the sandbox ([[Sandbox (Go)]]), and having two doors to
// the same resource with different rules is how one of them gets forgotten.
package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"mime"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/pathx"
)

// maxReadBytes bounds how much of a file Read and Diff load into memory.
// Large enough for real source files, small enough that opening a 500 MB
// file in the UI does not take the browser down with it.
const maxReadBytes = 4 << 20 // 4 MiB

// ignoredDirs are never listed by Tree. These are the directories a
// workspace tree explodes into without ever being something a person wants
// to browse: version-control internals, dependency trees, and build output.
var ignoredDirs = map[string]bool{
	".git": true, ".aos": true, "node_modules": true,
	"dist": true, "build": true, "coverage": true, ".task": true,
}

// binaryExtensions are the extensions Read serves as Base64 rather than
// text, and Tree marks not editable. Everything else is treated as text:
// deny-binary, default-text — an unrecognised extension is far more often a
// config file with an odd name than it is binary.
var binaryExtensions = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "gif": true, "webp": true, "bmp": true, "ico": true, "avif": true, "heic": true,
	"mp4": true, "mov": true, "webm": true, "avi": true, "mkv": true,
	"mp3": true, "wav": true, "ogg": true, "flac": true, "m4a": true,
	"zip": true, "tar": true, "gz": true, "rar": true, "7z": true, "xz": true,
	"woff": true, "woff2": true, "ttf": true, "otf": true, "eot": true,
	"pdf": true, "doc": true, "docx": true, "xls": true, "xlsx": true, "ppt": true, "pptx": true,
	"exe": true, "dll": true, "so": true, "dylib": true, "bin": true, "wasm": true, "class": true, "sqlite": true, "db": true,
}

// ErrNotExist is what an FS implementation returns from Stat or ReadFile for
// a path that is not there. The domain checks for it with errors.Is; an
// adapter over os wraps os.ErrNotExist to satisfy that.
var ErrNotExist = errors.New("file: no such path")

// ErrOutside is what an FS implementation's Resolve returns for a path that
// does not confine to the root it was given. The domain checks for it with
// errors.Is; the real adapter wraps pathx.ErrOutside to satisfy that.
var ErrOutside = errors.New("file: path resolves outside the root")

// Deps is everything the service is built from.
type Deps struct {
	FS         FS
	Git        Git
	Workspaces Workspaces
}

// Service is the file domain aggregate.
type Service struct {
	fs         FS
	git        Git
	workspaces Workspaces
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	return &Service{fs: d.FS, git: d.Git, workspaces: d.Workspaces}
}

// Tree lists the directory at in.Path, non-recursively unless asked.
func (s *Service) Tree(ctx context.Context, in TreeInput) (Tree, error) {
	_, real, err := s.resolve(ctx, in.Path)
	if err != nil {
		return Tree{}, err
	}
	info, err := s.fs.Stat(ctx, real)
	if err != nil {
		return Tree{}, errFSFailed("stat", in.Path, err)
	}
	if !info.Dir {
		return Tree{}, errNotDirectory(in.Path)
	}

	nodes, err := s.walk(ctx, real, in.Path, in.Recursive)
	if err != nil {
		return Tree{}, err
	}
	return Tree{Path: in.Path, Nodes: nodes}, nil
}

// Read returns a file's content: Text for anything editable, Base64 for
// everything else, truncated at maxReadBytes either way.
func (s *Service) Read(ctx context.Context, in ReadInput) (Content, error) {
	_, real, err := s.resolve(ctx, in.Path)
	if err != nil {
		return Content{}, err
	}
	info, err := s.fs.Stat(ctx, real)
	if err != nil {
		return Content{}, errFSFailed("stat", in.Path, err)
	}
	if info.Dir {
		return Content{}, errIsDirectory("Read", in.Path)
	}

	data, truncated, err := s.fs.ReadFile(ctx, real, maxReadBytes)
	if err != nil {
		return Content{}, errFSFailed("read", in.Path, err)
	}

	content := Content{
		Path:      in.Path,
		MediaType: mediaTypeFor(in.Path),
		Size:      info.Size,
		Truncated: truncated,
	}
	// The extension is the fast path; content is the fallback for the files
	// it misses — a binary with no extension at all (a compiled tool
	// committed by mistake, say) would otherwise come back as "text" full of
	// invalid UTF-8.
	if isBinaryExt(in.Path) || isBinaryContent(data) {
		content.Base64 = base64.StdEncoding.EncodeToString(data)
	} else {
		content.Text = string(data)
	}
	return content, nil
}

// Content opens in.Path for streaming, for the viewer route that serves a
// file's bytes as themselves.
//
// It goes through the same resolve every other method here does, so the
// workspace boundary holds: a path that climbs out of the root is refused
// before anything is opened, not after.
func (s *Service) Content(ctx context.Context, in ReadInput) (ContentStream, error) {
	_, real, err := s.resolve(ctx, in.Path)
	if err != nil {
		return ContentStream{}, err
	}
	info, err := s.fs.Stat(ctx, real)
	if err != nil {
		if errors.Is(err, ErrNotExist) {
			return ContentStream{}, errNoSuchFile(in.Path)
		}
		return ContentStream{}, errFSFailed("stat", in.Path, err)
	}
	if info.Dir {
		return ContentStream{}, errIsDirectory("Content", in.Path)
	}

	body, err := s.fs.Open(ctx, real)
	if err != nil {
		if errors.Is(err, ErrNotExist) {
			return ContentStream{}, errNoSuchFile(in.Path)
		}
		return ContentStream{}, errFSFailed("open", in.Path, err)
	}

	return ContentStream{
		Name:      info.Name,
		MediaType: mediaTypeFor(in.Path),
		ModTime:   info.ModTime,
		Size:      info.Size,
		Body:      body,
	}, nil
}

// Write persists content at in.Path, creating the file — and any missing
// parent directories — when nothing was there yet.
func (s *Service) Write(ctx context.Context, in WriteInput) error {
	_, real, err := s.resolve(ctx, in.Path)
	if err != nil {
		return err
	}
	if info, statErr := s.fs.Stat(ctx, real); statErr == nil && info.Dir {
		return errIsDirectory("Write", in.Path)
	}
	if err := s.fs.MkdirAll(ctx, filepath.Dir(real)); err != nil {
		return errFSFailed("mkdir", in.Path, err)
	}
	if err := s.fs.WriteFile(ctx, real, []byte(in.Content)); err != nil {
		return errFSFailed("write", in.Path, err)
	}
	return nil
}

// Move renames a path within the workspace. It refuses to overwrite an
// existing destination — silently replacing a file nobody named "overwrite"
// is how work goes missing.
func (s *Service) Move(ctx context.Context, from, to string) error {
	_, fromReal, err := s.resolve(ctx, from)
	if err != nil {
		return err
	}
	_, toReal, err := s.resolve(ctx, to)
	if err != nil {
		return err
	}

	exists, err := s.exists(ctx, toReal)
	if err != nil {
		return errFSFailed("stat", to, err)
	}
	if exists {
		return errAlreadyExists(to)
	}
	if err := s.fs.MkdirAll(ctx, filepath.Dir(toReal)); err != nil {
		return errFSFailed("mkdir", to, err)
	}
	if err := s.fs.Rename(ctx, fromReal, toReal); err != nil {
		return errFSFailed("move", from, err)
	}
	return nil
}

// Delete removes a path. The workspace root itself is refused, the same way
// the sandbox refuses to remove its own root.
func (s *Service) Delete(ctx context.Context, p string) error {
	trimmed := strings.Trim(strings.TrimSpace(p), "/")
	if trimmed == "" {
		return errRootRemoval()
	}
	root, real, err := s.resolve(ctx, p)
	if err != nil {
		return err
	}
	if real == root {
		return errRootRemoval()
	}
	if err := s.fs.Remove(ctx, real); err != nil {
		return errFSFailed("delete", p, err)
	}
	return nil
}

// Diff compares a file against HEAD: the old side comes from git, the new
// side from disk. A binary pair is reported as such rather than diffed —
// nothing useful renders from a binary line-diff.
// Changes lists every path the working tree differs from HEAD at.
//
// A workspace that is not a Git repository answers an empty list rather than a
// failure. `workspace create` initialises one, but a directory somebody
// registered by hand need not be one, and the panel that shows this still has
// to open there — with nothing in it, which is the truth.
func (s *Service) Changes(ctx context.Context) ([]Change, error) {
	root, err := s.root(ctx)
	if err != nil {
		return nil, err
	}

	found, err := s.git.Changes(ctx, root)
	if err != nil {
		// Not a repository, or a git that would not run. Either way there is
		// nothing to report, and refusing to draw the file tree because of it
		// would be the wrong trade.
		return []Change{}, nil
	}

	// Ordered by path, so two reads of an unchanged repository are identical
	// and the list does not reshuffle under the pointer.
	out := append([]Change(nil), found...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	if out == nil {
		out = []Change{}
	}
	return out, nil
}

func (s *Service) Diff(ctx context.Context, in DiffInput) (Diff, error) {
	root, real, err := s.resolve(ctx, in.Path)
	if err != nil {
		return Diff{}, err
	}
	rel := gitRelative(root, real)

	status, err := s.git.Status(ctx, root, rel)
	if err != nil {
		return Diff{}, errGitFailed("Diff", in.Path, err)
	}
	if status == "" {
		status = "unchanged"
	}
	out := Diff{Path: in.Path, Status: status}

	if status != "added" && status != "untracked" {
		old, ok, err := s.git.Show(ctx, root, "HEAD", rel)
		if err != nil {
			return Diff{}, errGitFailed("Diff", in.Path, err)
		}
		if ok {
			if isBinaryContent(old) {
				out.IsBinary = true
			} else {
				text := string(old)
				out.OldText = &text
			}
		}
	}

	if status != "deleted" && !out.IsBinary {
		data, _, err := s.fs.ReadFile(ctx, real, maxReadBytes)
		switch {
		case errors.Is(err, ErrNotExist):
			// nothing on disk; the new side stays empty
		case err != nil:
			return Diff{}, errFSFailed("read", in.Path, err)
		case isBinaryContent(data):
			out.IsBinary = true
			out.OldText = nil
		default:
			text := string(data)
			out.NewText = &text
		}
	}

	return out, nil
}

func (s *Service) root(ctx context.Context) (string, error) {
	root, err := s.workspaces.Root(ctx)
	if err != nil {
		return "", errWorkspaceUnavailable(err)
	}
	real, err := pathx.Root(root)
	if err != nil {
		return "", errWorkspaceUnavailable(err)
	}
	return real, nil
}

// resolve confines p to the workspace root through the FS port, so a domain
// test can prove the confinement without touching a disk — the real adapter
// shares pathx with the sandbox precisely so the two cannot drift apart, see
// File (Go)'s "Contenção".
func (s *Service) resolve(ctx context.Context, p string) (root, real string, err error) {
	root, err = s.root(ctx)
	if err != nil {
		return "", "", err
	}
	real, err = s.fs.Resolve(ctx, root, p)
	if err != nil {
		if errors.Is(err, ErrOutside) {
			return "", "", errOutsideWorkspace(p)
		}
		return "", "", errUnreadable(p, err)
	}
	return root, real, nil
}

func (s *Service) exists(ctx context.Context, real string) (bool, error) {
	_, err := s.fs.Stat(ctx, real)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrNotExist) {
		return false, nil
	}
	return false, err
}

// walk lists absDir non-recursively, or the full subtree when recursive is
// set. relDir is the workspace-relative, POSIX-separated path each child's
// own Path is built from — the two path styles (OS-native for the disk,
// POSIX for the API) never mix.
func (s *Service) walk(ctx context.Context, absDir, relDir string, recursive bool) ([]Node, error) {
	entries, err := s.fs.ReadDir(ctx, absDir)
	if err != nil {
		return nil, errFSFailed("readdir", relDir, err)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return entries[i].Name < entries[j].Name
	})

	out := make([]Node, 0, len(entries))
	for _, e := range entries {
		if ignoredDirs[e.Name] {
			continue
		}
		relPath := path.Join(relDir, e.Name)
		out = append(out, toNode(relPath, e))

		if recursive && e.Dir {
			children, err := s.walk(ctx, filepath.Join(absDir, e.Name), relPath, true)
			if err != nil {
				return nil, err
			}
			out = append(out, children...)
		}
	}
	return out, nil
}

func toNode(relPath string, i Info) Node {
	n := Node{Path: relPath, Name: i.Name, Dir: i.Dir, Size: i.Size, ModifiedAt: i.ModTime}
	if !i.Dir {
		n.Extension = extensionOf(i.Name)
		n.MediaType = mediaTypeFor(i.Name)
		n.Editable = !isBinaryExt(i.Name)
	}
	return n
}

func extensionOf(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
}

func isBinaryExt(name string) bool {
	return binaryExtensions[extensionOf(name)]
}

func mediaTypeFor(name string) string {
	if mt := mime.TypeByExtension(filepath.Ext(name)); mt != "" {
		return mt
	}
	if isBinaryExt(name) {
		return "application/octet-stream"
	}
	return "text/plain; charset=utf-8"
}

// isBinaryContent uses the same heuristic git itself uses: a NUL byte in the
// first bytes of a file is not valid text.
func isBinaryContent(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0
}

// gitRelative turns a resolved absolute path back into the slash-separated,
// root-relative form git commands expect.
func gitRelative(root, real string) string {
	rel, err := filepath.Rel(root, real)
	if err != nil {
		return filepath.ToSlash(real)
	}
	return filepath.ToSlash(rel)
}
