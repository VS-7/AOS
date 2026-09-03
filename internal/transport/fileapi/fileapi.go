// Package fileapi is the HTTP surface of the file domain: the explorer and
// editor the UI drives, mounted at /api/file inside the same authenticated
// group httpapi guards its command routes with.
//
// It is a router of its own, separate from httpapi's generic command
// dispatch, because this feature is deliberately outside the command
// registry — see File (Go)'s "não tem grupo de comando nem tools MCP". The
// agent reaches the filesystem through its sandbox; a human reaches it here.
package fileapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/file"
)

// maxBodyBytes bounds a write body. A file the UI edits is text a person is
// typing, not a bulk upload.
const maxBodyBytes = 8 << 20 // 8 MiB

// Config is what the router is built from.
type Config struct {
	Service *file.Service
	Log     *slog.Logger
}

// New builds the router. It is mounted by the caller — see httpapi's Files
// field — behind whatever authentication that mount point already applies.
func New(cfg Config) http.Handler {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	s := &server{svc: cfg.Service, log: cfg.Log}

	r := chi.NewRouter()
	r.Get("/tree", s.tree)
	r.Get("/read", s.read)
	r.Get("/content", s.content)
	r.Put("/write", s.write)
	r.Put("/move", s.move)
	r.Delete("/delete", s.delete)
	r.Get("/diff", s.diff)
	r.Get("/changes", s.changes)
	return r
}

type server struct {
	svc *file.Service
	log *slog.Logger
}

func (s *server) tree(w http.ResponseWriter, r *http.Request) {
	in := file.TreeInput{
		Path:      r.URL.Query().Get("path"),
		Recursive: r.URL.Query().Get("recursive") == "true",
	}
	out, err := s.svc.Tree(r.Context(), in)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, out)
}

func (s *server) read(w http.ResponseWriter, r *http.Request) {
	in := file.ReadInput{Path: r.URL.Query().Get("path")}
	out, err := s.svc.Read(r.Context(), in)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, out)
}

// content serves one file as its own bytes — the route the Files panel's
// image, PDF and video viewers put straight in a src attribute.
//
// The only handler here that does not answer the JSON envelope, on purpose:
// an <img> cannot decode one. http.ServeContent does the rest — Range (so a
// video can seek), 304 against If-Modified-Since, and the Content-Length the
// player needs to draw a scrub bar.
func (s *server) content(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.Content(r.Context(), file.ReadInput{Path: r.URL.Query().Get("path")})
	if err != nil {
		s.writeError(w, err)
		return
	}
	defer func() { _ = out.Body.Close() }()

	// Set before ServeContent, which only sniffs when the header is absent:
	// the domain already knows the type from the extension, and sniffing
	// reads the first 512 bytes back off the handle to guess it again.
	w.Header().Set("Content-Type", out.MediaType)
	http.ServeContent(w, r, out.Name, out.ModTime, out.Body)
}

func (s *server) write(w http.ResponseWriter, r *http.Request) {
	var in file.WriteInput
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.svc.Write(r.Context(), in); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, map[string]string{"path": in.Path})
}

func (s *server) move(w http.ResponseWriter, r *http.Request) {
	var in struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.svc.Move(r.Context(), in.From, in.To); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, map[string]string{"path": in.To})
}

func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if err := s.svc.Delete(r.Context(), p); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, map[string]string{"path": p})
}

func (s *server) diff(w http.ResponseWriter, r *http.Request) {
	in := file.DiffInput{Path: r.URL.Query().Get("path")}
	out, err := s.svc.Diff(r.Context(), in)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, out)
}

// changes lists every path the working tree differs from HEAD at — what the
// Changes panel draws before anybody opens a diff.
func (s *server) changes(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.Changes(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	// Under a key rather than as a bare array: an envelope whose data is a
	// list has nowhere to grow, and this one already wants a count beside it.
	s.writeJSON(w, map[string]any{"files": out, "total": len(out)})
}

func (s *server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		s.writeError(w, errBodyTooLarge(maxBodyBytes))
		return false
	}
	if err := json.Unmarshal(body, v); err != nil {
		s.writeError(w, errBadRequestBody(err))
		return false
	}
	return true
}

// writeJSON matches the envelope every other surface answers with — the
// frontend's unwrap() reads command.Envelope regardless of which transport
// produced it.
func (s *server) writeJSON(w http.ResponseWriter, out any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(command.Wrap(out, nil)); err != nil {
		s.log.Warn("a file response could not be written", "err", err)
	}
}

func (s *server) writeError(w http.ResponseWriter, err error) {
	e, ok := apperr.As(err)
	if !ok {
		e = apperr.New("FILE_HTTP_INTERNAL").
			Causer("fileapi").
			Msgf("the request could not be completed").
			Status(apperr.StatusInternalServerError).
			Wrap(err)
		s.log.Error("unclassified error escaped a file handler", "err", err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": e})
}
