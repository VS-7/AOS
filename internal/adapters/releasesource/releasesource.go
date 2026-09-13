// Package releasesource implements update.ReleaseSource over a hosted HTTP
// feed.
//
// The wire format is our own — GET {baseURL}/{channel}.json for a manifest,
// whose JSON fields are update.Release's own tags, so there is no separate
// DTO to keep in sync — matching internal/adapters/marketplacehttp's own
// choice for the same reason: "Distribuição por releases assinados,
// agnóstica de forja" (docs/08 - Entrega/Auto-Update.md) means the verifier
// only needs artifacts, a checksums file and a signature, served from
// wherever; there is no third-party release-feed spec to be compatible with.
package releasesource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/domain/update"
)

const (
	defaultTimeout = 30 * time.Second
	// defaultMaxSize is the most Fetch reads: an order of magnitude above the
	// largest release asset (aosd is about 50 MB), and a bound on what an
	// unsigned feed can make this process hold in memory before a checksum
	// is ever compared.
	defaultMaxSize = 512 << 20
)

// Source reaches one hosted release feed over HTTP.
type Source struct {
	// BaseURL is the feed's root, without a trailing slash. Empty disables
	// the source entirely — Configured reports false, which is what an
	// installation with no release infrastructure configured yet looks like.
	BaseURL string

	// Client replaces the one Source builds, whose timeouts are Timeout's.
	Client *http.Client
	// Timeout bounds every wait: connecting, the answer's headers, a whole
	// manifest, and any stretch of a download with no bytes arriving. It
	// does not bound a download that keeps arriving. It used to — as
	// http.Client.Timeout, which covers reading the body — and a 50 MB
	// release failed partway on any link slower than about 1.7 MB/s.
	Timeout time.Duration
	// MaxSize is the most Fetch reads before refusing the answer.
	MaxSize int64

	once  sync.Once
	built *http.Client
}

// New builds a Source over baseURL. An empty baseURL is valid — see BaseURL's
// own doc comment.
func New(baseURL string) *Source {
	return &Source{BaseURL: strings.TrimRight(baseURL, "/")}
}

var _ update.ReleaseSource = (*Source)(nil)

// Configured reports whether a feed address was given at all.
func (s *Source) Configured() bool { return s.BaseURL != "" }

func (s *Source) timeout() time.Duration {
	if s.Timeout <= 0 {
		return defaultTimeout
	}
	return s.Timeout
}

// client is Client, or one built once with Timeout on every wait but the
// body's — which Fetch bounds by progress instead.
func (s *Source) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	s.once.Do(func() {
		t := s.timeout()
		s.built = &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: t, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   t,
			ResponseHeaderTimeout: t,
			ForceAttemptHTTP2:     true,
			IdleConnTimeout:       90 * time.Second,
		}}
	})
	return s.built
}

// Latest reads GET {baseURL}/{channel}.json and decodes an update.Release
// directly.
//
// A 404 is an error wrapping update.ErrNotPublished. It used to be (nil, nil),
// "no release", which Check turned into "you are on the newest release": a
// feed pointed at the wrong address, or at a release that never uploaded its
// manifest, told everybody they were current.
func (s *Source) Latest(ctx context.Context, channel update.Channel) (*update.Release, error) {
	if s.BaseURL == "" {
		return nil, errNotConfigured()
	}
	u := s.BaseURL + "/" + string(channel) + ".json"

	// A manifest is a few kilobytes: the whole request is bounded.
	ctx, cancel := context.WithTimeout(ctx, s.timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, errRequestFailed(u, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client().Do(req)
	if err != nil {
		return nil, errUnreachable(u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, errBadStatus(u, resp.StatusCode, string(body))
	}

	var release update.Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, errBadBody(u, err)
	}
	release.Channel = channel
	return &release, nil
}

// Fetch downloads url's bytes whole — an asset, a checksums file, or a
// signature file, all small enough (docs/08 - Entrega/Build e
// Cross-Compile.md's own size ceilings are in the tens of MB) that
// buffering the whole response is simpler than streaming it, and streaming
// would only move the checksum comparison from Download to here.
//
// The transfer is given up on when no bytes arrive for Timeout, however long
// it has taken so far, and when it grows past MaxSize.
func (s *Source) Fetch(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errRequestFailed(url, err)
	}

	resp, err := s.client().Do(req)
	if err != nil {
		return nil, errUnreachable(url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, errBadStatus(url, resp.StatusCode, string(body))
	}

	limit := s.MaxSize
	if limit <= 0 {
		limit = defaultMaxSize
	}
	body := newProgressReader(resp.Body, s.timeout(), cancel)
	defer body.stop()
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	switch {
	case body.stalled.Load():
		return nil, errUnreachable(url, fmt.Errorf("no data arrived for %s", s.timeout()))
	case err != nil:
		return nil, errBadBody(url, err)
	case int64(len(data)) > limit:
		return nil, errBadBody(url, fmt.Errorf("the answer is larger than %d bytes, more than any release asset", limit))
	}
	return data, nil
}

// progressReader cancels a transfer that goes idle for longer than idle:
// every read that returns bytes pushes the deadline back.
type progressReader struct {
	r       io.Reader
	idle    time.Duration
	timer   *time.Timer
	stalled atomic.Bool
}

func newProgressReader(r io.Reader, idle time.Duration, cancel context.CancelFunc) *progressReader {
	p := &progressReader{r: r, idle: idle}
	p.timer = time.AfterFunc(idle, func() {
		p.stalled.Store(true)
		cancel()
	})
	return p
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 && !p.stalled.Load() {
		p.timer.Reset(p.idle)
	}
	return n, err
}

func (p *progressReader) stop() { p.timer.Stop() }

func errRequestFailed(u string, cause error) error {
	return apperr.New("RELEASESOURCE_REQUEST_FAILED").
		Causer("releasesource.Source").
		Msgf("could not build a request to %q: %v", u, cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

// errUnreachable wraps update.ErrUnreachable: the address did not answer,
// which is the one failure the domain attributes to the network.
func errUnreachable(u string, cause error) error {
	return apperr.New("RELEASESOURCE_UNREACHABLE").
		Causer("releasesource.Source").
		Msgf("could not reach %q: %v", u, cause).
		Status(apperr.StatusServiceUnavailable).
		Wrap(fmt.Errorf("%w: %w", update.ErrUnreachable, cause))
}

// errBadStatus wraps update.ErrNotPublished on a 404, so a missing manifest,
// signature or asset is told apart from a server that answered with an
// error — the difference between "this release has no signature" and
// "retry later".
func errBadStatus(u string, status int, body string) error {
	e := apperr.New("RELEASESOURCE_BAD_STATUS").
		Causer("releasesource.Source").
		Msgf("%q answered %d", u, status).
		Issue("status", status).
		Issue("body", body).
		Status(apperr.StatusBadGateway)
	if status == http.StatusNotFound || status == http.StatusGone {
		e = e.Wrap(update.ErrNotPublished)
	}
	return e
}

func errNotConfigured() error {
	return apperr.New("RELEASESOURCE_NOT_CONFIGURED").
		Causer("releasesource.Source.Latest").
		Msgf("no release feed is configured").
		Status(apperr.StatusServiceUnavailable).
		CTA(apperr.CallToAction{Label: "set " + env.Key(env.KeyUpdateBaseURL) + " to a release feed, or reinstall to update"})
}

func errBadBody(u string, cause error) error {
	return apperr.New("RELEASESOURCE_BAD_BODY").
		Causer("releasesource.Source").
		Msgf("could not decode the response from %q: %v", u, cause).
		Status(apperr.StatusBadGateway).
		Wrap(cause)
}
