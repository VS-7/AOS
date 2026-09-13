package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/OWNER/aos/internal/transport/daemonclient"
)

// The daemon paths the window serves through its own asset host rather than
// over the bridge: things an element puts in a `src`, which can carry no bearer
// and cannot read a JSON string.
const (
	contentRoute   = "/api/file/content"
	artifactsRoute = "/v/artifacts/"

	// artifactFrameRoute is where the window frames an artifact:
	// /v/frame/{grant}/{id}/{file}. See artifactFrames.
	artifactFrameRoute = "/v/frame/"

	// frameAddressRoute answers the page's question "where do I frame this
	// artifact?", and only when frameAddressHeader is on the request.
	frameAddressRoute  = "/v/frame-address"
	frameAddressHeader = "x-aos-frame"
)

// runtimeRoute is where @wailsio/runtime sends every bridge call.
const runtimeRoute = "/wails/runtime"

// bridgeDaemon is the asset host's middleware: it forwards the daemon paths an
// element loads by URL, with the window's credential attached, and refuses a
// bridge call the runtime did not make.
//
// The file content route exists because of what an `<img>` is. The Files
// panel's image, PDF and video viewers need a URL they can put in a `src`, and
// inside the desktop window neither of the two paths the interface normally
// uses can produce one: the Wails bridge answers a JSON string
// (`DomainService.Fetch`), which is not a video, and a plain URL to the daemon
// is cross-origin from `wails://localhost`, carries no bearer and no cookie,
// and is refused.
//
// The artifact route is the same problem in an `<iframe>`. Opening an artifact
// loaded its relative URL (`/v/artifacts/{id}/`, as the daemon publishes it) in
// the window's own origin, where nothing answered it: every artifact opened
// from Home or Surfaces was a blank 404. The daemon also sends the artifact's
// own security headers — its CSP above all — and they are passed back as they
// came.
//
// A relative URL now works in both modes. In a browser the daemon serves the
// page and the request is same-origin. In the window it reaches this
// middleware, which forwards it with the token the process already holds — the
// credential never enters the page — and passes Range and the conditional
// headers through in both directions, so a video still seeks and a cached image
// still 304s. An artifact's frame goes one step further than its relative URL
// — see artifactFrames.
//
// Only these paths, and only GET. The asset host is otherwise the interface's
// own bundle, and a proxy that forwarded anything would be a hole in the
// boundary `DomainService.Fetch`'s own allowlist exists to keep.
func bridgeDaemon(daemon *daemonclient.Client, log *slog.Logger) func(http.Handler) http.Handler {
	frames := newArtifactFrames(daemon.Workspace)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if refusedRuntimeCall(r) {
				http.Error(w, "not a call from this window's runtime", http.StatusForbidden)
				return
			}
			// Refused here rather than left to the asset handler, which in
			// development is a dev server that answers CORS preflights itself.
			if r.URL.Path == frameAddressRoute && r.Method != http.MethodGet {
				http.Error(w, "the frame address is only ever read", http.StatusForbidden)
				return
			}
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}
			path := r.URL.Path
			switch {
			case path == frameAddressRoute:
				frames.answerAddress(w, r)
			case strings.HasPrefix(path, artifactFrameRoute):
				daemonPath, frame, workspace, ok := frames.opened(r.URL.EscapedPath())
				if !ok {
					http.NotFound(w, r)
					return
				}
				forward(w, r, daemon, log, workspace, daemonPath, frames.connectTo(frame))
			case strings.HasPrefix(path, contentRoute), strings.HasPrefix(path, artifactsRoute):
				forward(w, r, daemon, log, daemon.Workspace(), r.URL.EscapedPath(), nil)
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}

// artifactFrames hands out and checks the addresses the window frames an
// artifact at.
//
// An artifact is framed without allow-same-origin (lib/wails.ts frameSandbox),
// so its origin is opaque and every file it loads of its own is a cross-origin
// request. Passed the daemon's same-origin resource policy, the browser
// cancelled every stylesheet, script and image, and an artifact rendered as
// unstyled, inert HTML; a fetch of its own data.json, a CORS request, was
// refused for want of Access-Control-Allow-Origin.
//
// Answering cross-origin for everything under /v/artifacts/ would have been
// the wrong fix here, though, because this asset host attaches the window's
// credential to what it forwards. Anything else the webview can be made to
// request — a page from another site open in a browser tab, another artifact —
// could then embed a private artifact's script or image by its id, and ids are
// caller-chosen names like "sales-dashboard" as often as they are random.
//
// So the window frames an artifact at /v/frame/{grant}/{id}/, where the grant
// is an HMAC of the workspace the window addresses and the id, under a key
// this process draws at start and never shares. The workspace is in it because
// an id is a name the caller chose — "sales-dashboard" in two workspaces is two
// artifacts — and a frame still mounted after the window switched workspace
// served the other one's artifact of that name: an address opens only while
// the window addresses the workspace it was handed out in, and is forwarded
// under that workspace. The artifact's own relative references resolve under that address,
// and only that address answers cross-origin (a resource policy of
// cross-origin, and Access-Control-Allow-Origin: * with no credentials — the
// grant is the credential); the plain /v/artifacts/ path, and file content,
// answer the window's own origin only. The page learns the address by asking
// frameAddressRoute, with a header no cross-origin request can carry without a
// preflight this host refuses. A redirect would have been simpler and does not
// work: Wails' asset server notes that an HTTP redirect is not followed by the
// WebKit webviews (asset_fileserver.go).
//
// One more thing an opaque page loses in WebKit: its own URL no longer counts
// as connect-src 'self', so an artifact's fetch("data.json") was refused
// before it was sent (script, style and image 'self' still match there, and
// in Chromium all of them do). The daemon's CSP is widened by exactly that
// artifact's own frame address — never /api, never the bridge. The address has
// to be absolute, and the asset server hides the scheme it was reached by, so
// the page states its own scheme and host when it asks for the address.
type artifactFrames struct {
	key []byte
	// workspace is the workspace the window addresses now.
	workspace func() string

	mu   sync.Mutex
	base string // the page's own scheme://host, as it stated it
}

func newArtifactFrames(workspace func() string) *artifactFrames {
	key := make([]byte, 32)
	// crypto/rand.Read does not fail; it aborts the process instead.
	_, _ = rand.Read(key)
	return &artifactFrames{key: key, workspace: workspace}
}

// grant is the capability for one artifact in one workspace. The workspace
// goes in with its length first, so no pair of workspace and id can be read
// as another.
func (f *artifactFrames) grant(workspace, id string) string {
	mac := hmac.New(sha256.New, f.key)
	_, _ = mac.Write(binary.BigEndian.AppendUint64(nil, uint64(len(workspace))))
	_, _ = mac.Write([]byte(workspace))
	_, _ = mac.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// answerAddress tells the page where to frame the artifact URL it names.
func (f *artifactFrames) answerAddress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Header.Get(frameAddressHeader) == "" {
		http.Error(w, "not a request from this window's interface", http.StatusForbidden)
		return
	}
	target, err := url.Parse(r.URL.Query().Get("url"))
	if err != nil || target.Scheme != "" || target.Host != "" {
		http.NotFound(w, r)
		return
	}
	id, rest, ok := artifactSegment(target.EscapedPath())
	if !ok {
		http.NotFound(w, r)
		return
	}
	if base := r.URL.Query().Get("base"); pageBase.MatchString(base) {
		f.mu.Lock()
		f.base = base
		f.mu.Unlock()
	}
	address := artifactFrameRoute + f.grant(f.workspace(), id) + "/" + url.PathEscape(id) + "/" + rest
	if target.RawQuery != "" {
		address += "?" + target.RawQuery
	}
	if target.Fragment != "" {
		address += "#" + target.EscapedFragment()
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": address})
}

// pageBase is a scheme and a host with an optional port, and nothing a CSP
// would read as another source or directive.
var pageBase = regexp.MustCompile(`^[a-z][a-z0-9+.-]*://[A-Za-z0-9.-]+(:[0-9]{1,5})?$`)

// opened checks an escaped /v/frame/{grant}/{id}/{file} path against the
// workspace the window addresses now, and answers the daemon path it stands
// for, the artifact's own frame address, and the workspace to ask it in.
func (f *artifactFrames) opened(escaped string) (daemonPath, frame, workspace string, ok bool) {
	grant, tail, found := strings.Cut(strings.TrimPrefix(escaped, artifactFrameRoute), "/")
	if !found {
		return "", "", "", false
	}
	id, rest, ok := artifactSegment(artifactsRoute + tail)
	workspace = f.workspace()
	if !ok || !hmac.Equal([]byte(grant), []byte(f.grant(workspace, id))) {
		return "", "", "", false
	}
	frame = artifactFrameRoute + grant + "/" + url.PathEscape(id) + "/"
	return artifactsRoute + url.PathEscape(id) + "/" + rest, frame, workspace, true
}

// connectTo is how an answer served at frame rewrites the daemon's CSP: its
// connect-src also names the frame's absolute address, when the page has said
// what its scheme and host are.
func (f *artifactFrames) connectTo(frame string) func(csp string) string {
	return func(csp string) string {
		f.mu.Lock()
		base := f.base
		f.mu.Unlock()
		if base == "" {
			return csp
		}
		directives := strings.Split(csp, ";")
		for i, directive := range directives {
			if name, _, _ := strings.Cut(strings.TrimSpace(directive), " "); name == "connect-src" {
				directives[i] = strings.TrimRight(directive, " ") + " " + base + frame
			}
		}
		return strings.Join(directives, ";")
	}
}

// artifactSegment splits an escaped /v/artifacts/{id}/{rest} path. The id comes
// back unescaped, which is the form a grant is made over; "." and ".." are not
// ids.
func artifactSegment(escaped string) (id, rest string, ok bool) {
	tail, found := strings.CutPrefix(escaped, artifactsRoute)
	if !found {
		return "", "", false
	}
	escapedID, rest, found := strings.Cut(tail, "/")
	if !found {
		return "", "", false
	}
	id, err := url.PathUnescape(escapedID)
	if err != nil || id == "" || id == "." || id == ".." || strings.Contains(id, "/") {
		return "", "", false
	}
	return id, rest, true
}

// refusedRuntimeCall is a request to the bridge, of any method, without the
// header every call @wailsio/runtime makes carries.
//
// An artifact is HTML a model generated, and it is served from this window's
// own origin. Its document is sandboxed without allow-same-origin, so a script
// in it cannot reach the page or make a same-origin request — but plenty
// reaches the bridge without permission. A no-cors "simple" POST is sent
// anyway, and Wails processes its body without looking at the content type.
// A GET is enough, too: Wails' HTTP transport reads object, method and args
// from the query string when there is no body (its WebKitGTK fallback), so an
// `<img>`, a `<form method=get>` or a navigation runs DomainService.Invoke with
// the window's credential, without a line of script. Guarding POST alone left
// exactly that open.
//
// None of those can set a custom header: a no-cors request drops it, and a
// cross-origin request that asks for one is preflighted first — and the
// preflight, which carries no such header either, is refused here. The runtime
// sets this one on every call, chunked or not, so its absence is the tell.
func refusedRuntimeCall(r *http.Request) bool {
	return r.URL.Path == runtimeRoute &&
		strings.TrimSpace(r.Header.Get("x-wails-client-id")) == ""
}

// responseHeaders are the daemon's headers a forwarded answer keeps: what a
// player needs to seek and a cache needs to revalidate, and the security
// headers the artifact route sends with every file. The resource policy is
// not among them: the window sets its own (see artifactFrames).
var responseHeaders = []string{
	"content-type", "content-length", "content-range",
	"accept-ranges", "last-modified", "etag", "cache-control",
	"content-security-policy", "x-content-type-options", "referrer-policy",
	"content-disposition",
}

// forward sends a GET for daemonPath (escaped) to the daemon, in workspace,
// with the window's credential and relays the answer. An opened artifact's
// answer (see artifactFrames) goes to any origin, with its CSP rewritten by
// opened; every other answer goes to the window's own origin only.
func forward(w http.ResponseWriter, r *http.Request, daemon *daemonclient.Client, log *slog.Logger, workspace, daemonPath string, opened func(csp string) string) {
	if r.URL.RawQuery != "" {
		daemonPath += "?" + r.URL.RawQuery
	}
	res, err := daemon.StreamIn(r.Context(), workspace, daemonPath, r.Header)
	if err != nil {
		log.Warn("a file could not be read from the daemon", "path", r.URL.Path, "err", err)
		http.Error(w, "the daemon did not answer", http.StatusBadGateway)
		return
	}
	defer func() { _ = res.Body.Close() }()

	for _, name := range responseHeaders {
		if v := res.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	if opened != nil {
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if csp := w.Header().Get("Content-Security-Policy"); csp != "" {
			w.Header().Set("Content-Security-Policy", opened(csp))
		}
	} else {
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	}
	w.WriteHeader(res.StatusCode)
	if _, err := io.Copy(w, res.Body); err != nil {
		// The viewer navigated away mid-stream, which is ordinary.
		log.Debug("a file stream ended early", "err", err)
	}
}
