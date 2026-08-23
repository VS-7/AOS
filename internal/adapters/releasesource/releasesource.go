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
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/update"
)

const defaultTimeout = 30 * time.Second

// Source reaches one hosted release feed over HTTP.
type Source struct {
	// BaseURL is the feed's root, without a trailing slash. Empty disables
	// the source entirely — Latest then reports "no release" rather than
	// erroring, which is what an installation with no release
	// infrastructure configured yet looks like.
	BaseURL string

	Client  *http.Client
	Timeout time.Duration
}

// New builds a Source over baseURL. An empty baseURL is valid — see BaseURL's
// own doc comment.
func New(baseURL string) *Source {
	return &Source{BaseURL: strings.TrimRight(baseURL, "/")}
}

var _ update.ReleaseSource = (*Source)(nil)

func (s *Source) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	t := s.Timeout
	if t <= 0 {
		t = defaultTimeout
	}
	return &http.Client{Timeout: t}
}

// Latest reads GET {baseURL}/{channel}.json and decodes an update.Release
// directly. A 404 (channel has no releases yet) and an unconfigured
// BaseURL both answer (nil, nil) — a channel with nothing published is not
// a failure to check it.
func (s *Source) Latest(ctx context.Context, channel update.Channel) (*update.Release, error) {
	if s.BaseURL == "" {
		return nil, nil
	}
	u := s.BaseURL + "/" + string(channel) + ".json"

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

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
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
func (s *Source) Fetch(ctx context.Context, url string) ([]byte, error) {
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

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errBadBody(url, err)
	}
	return data, nil
}

func errRequestFailed(u string, cause error) error {
	return apperr.New("RELEASESOURCE_REQUEST_FAILED").
		Causer("releasesource.Source").
		Msgf("could not build a request to %q: %v", u, cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errUnreachable(u string, cause error) error {
	return apperr.New("RELEASESOURCE_UNREACHABLE").
		Causer("releasesource.Source").
		Msgf("could not reach %q: %v", u, cause).
		Status(apperr.StatusServiceUnavailable).
		Wrap(cause)
}

func errBadStatus(u string, status int, body string) error {
	return apperr.New("RELEASESOURCE_BAD_STATUS").
		Causer("releasesource.Source").
		Msgf("%q answered %d", u, status).
		Issue("status", status).
		Issue("body", body).
		Status(apperr.StatusBadGateway)
}

func errBadBody(u string, cause error) error {
	return apperr.New("RELEASESOURCE_BAD_BODY").
		Causer("releasesource.Source").
		Msgf("could not decode the response from %q: %v", u, cause).
		Status(apperr.StatusBadGateway).
		Wrap(cause)
}
