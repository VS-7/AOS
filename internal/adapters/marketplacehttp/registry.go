// Package marketplacehttp implements marketplace.Registry over a hosted
// HTTP index. The wire format is our own — GET {baseURL}/listings for a
// search, GET {baseURL}/packages/{source} for a fetch — not a compatibility
// copy of the original, whose remote registry the reverse engineering could
// not determine (see the design doc's "Escopo honesto").
package marketplacehttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/marketplace"
	"github.com/OWNER/aos/internal/domain/skill"
)

const defaultTimeout = 15 * time.Second

// Registry reaches one hosted index over HTTP.
type Registry struct {
	// BaseURL is the index's root, without a trailing slash — e.g.
	// "https://skills.empresa.com".
	BaseURL string

	// Client is the HTTP client. Nil builds one with Timeout.
	Client *http.Client

	// Timeout bounds a request when Client is nil. Zero means
	// defaultTimeout.
	Timeout time.Duration
}

// New builds a Registry over baseURL.
func New(baseURL string) *Registry {
	return &Registry{BaseURL: strings.TrimRight(baseURL, "/")}
}

var _ marketplace.Registry = (*Registry)(nil)

func (r *Registry) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	t := r.Timeout
	if t <= 0 {
		t = defaultTimeout
	}
	return &http.Client{Timeout: t}
}

// Search asks GET {baseURL}/listings?text=&tag=&owner= and decodes a JSON
// array of marketplace.Listing directly — its json tags are the wire
// format, so there is no separate DTO to keep in sync.
func (r *Registry) Search(ctx context.Context, q marketplace.SearchQuery) ([]marketplace.Listing, error) {
	u := r.BaseURL + "/listings"
	vals := url.Values{}
	if q.Text != "" {
		vals.Set("text", q.Text)
	}
	if q.Tag != "" {
		vals.Set("tag", q.Tag)
	}
	if q.Owner != "" {
		vals.Set("owner", q.Owner)
	}
	if enc := vals.Encode(); enc != "" {
		u += "?" + enc
	}

	var out []marketplace.Listing
	if err := r.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Fetch asks GET {baseURL}/packages/{source}?ref= and decodes a JSON
// skill.Package.
func (r *Registry) Fetch(ctx context.Context, source, ref string) (skill.Package, error) {
	u := r.BaseURL + "/packages/" + url.PathEscape(source)
	if strings.TrimSpace(ref) != "" {
		u += "?ref=" + url.QueryEscape(ref)
	}

	var pkg skill.Package
	if err := r.getJSON(ctx, u, &pkg); err != nil {
		return skill.Package{}, err
	}
	return pkg, nil
}

func (r *Registry) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return errRequestFailed(u, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.client().Do(req)
	if err != nil {
		return errUnreachable(u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errNotFound(u)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return errBadStatus(u, resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return errBadBody(u, err)
	}
	return nil
}

func errRequestFailed(u string, cause error) error {
	return apperr.New("MARKETPLACEHTTP_REQUEST_FAILED").
		Causer("marketplacehttp.Registry.getJSON").
		Msgf("could not build a request to %q: %v", u, cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errUnreachable(u string, cause error) error {
	return apperr.New("MARKETPLACEHTTP_UNREACHABLE").
		Causer("marketplacehttp.Registry.getJSON").
		Msgf("%q did not answer: %v", u, cause).
		Issue("url", u).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check the registry's base URL and that it is reachable"})
}

func errNotFound(u string) error {
	return apperr.New("MARKETPLACEHTTP_NOT_FOUND").
		Causer("marketplacehttp.Registry.getJSON").
		Msgf("%q returned 404", u).
		Issue("url", u).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "search the registry before fetching a specific listing"})
}

func errBadStatus(u string, status int, body string) error {
	return apperr.New("MARKETPLACEHTTP_BAD_STATUS").
		Causer("marketplacehttp.Registry.getJSON").
		Msgf("%q returned %d: %s", u, status, body).
		Issue("url", u).
		Issue("status", status).
		Status(apperr.StatusBadGateway).
		CTA(apperr.CallToAction{Label: "check the registry's own logs"})
}

func errBadBody(u string, cause error) error {
	return apperr.New("MARKETPLACEHTTP_BAD_BODY").
		Causer("marketplacehttp.Registry.getJSON").
		Msgf("%q returned a body that does not decode: %v", u, cause).
		Issue("url", u).
		Status(apperr.StatusBadGateway).
		Wrap(cause)
}
