package oauthfile_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/runtime/providers/oauthfile"
)

func codeOf(t *testing.T, err error) string {
	t.Helper()
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err is not *apperr.Error: %v", err)
	}
	return e.Code
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func geminiParse(raw []byte) (oauthfile.Credentials, error) {
	var doc struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiryDate   int64  `json:"expiry_date"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return oauthfile.Credentials{}, err
	}
	out := oauthfile.Credentials{AccessToken: doc.AccessToken, RefreshToken: doc.RefreshToken}
	if doc.ExpiryDate > 0 {
		out.ExpiresAt = time.UnixMilli(doc.ExpiryDate)
	}
	return out, nil
}

// --- reading -----------------------------------------------------------

func TestTokenFailsWithACTAWhenTheFileIsMissing(t *testing.T) {
	s := &oauthfile.Store{
		Path: filepath.Join(t.TempDir(), "does-not-exist.json"), Owner: "the Test CLI", Parse: geminiParse,
	}
	_, err := s.Token(context.Background())
	if code := codeOf(t, err); code != "AOS_OAUTH_FILE_MISSING" {
		t.Fatalf("code = %q", code)
	}
}

func TestTokenReturnsTheAccessTokenWhenNotExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	writeJSON(t, path, map[string]any{
		"access_token": "still-good", "expiry_date": time.Now().Add(time.Hour).UnixMilli(),
	})
	s := &oauthfile.Store{Path: path, Owner: "the Test CLI", Parse: geminiParse}

	got, err := s.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got != "still-good" {
		t.Fatalf("got %q", got)
	}
}

// TestACredentialWithNoExpiryIsNeverConsideredExpired documents the actual
// behaviour rather than an assumption about it: a file whose Parse never
// populates ExpiresAt — parseCodex's own shape, which this package does not
// export, so this test proves the general rule through a Parse that
// deliberately does the same thing — is read back as always fresh. A file
// that does not say when it expires cannot be assumed stale.
func TestACredentialWithNoExpiryIsNeverConsideredExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	writeJSON(t, path, map[string]any{"access_token": "no-expiry-here"})
	noExpiryParse := func(raw []byte) (oauthfile.Credentials, error) {
		var doc struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return oauthfile.Credentials{}, err
		}
		return oauthfile.Credentials{AccessToken: doc.AccessToken}, nil
	}
	s := &oauthfile.Store{Path: path, Owner: "the Test CLI", Parse: noExpiryParse}

	got, err := s.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got != "no-expiry-here" {
		t.Fatalf("got %q", got)
	}
}

func TestTokenFailsOnAFileItCannotParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &oauthfile.Store{Path: path, Owner: "the Test CLI", Parse: geminiParse}

	_, err := s.Token(context.Background())
	if code := codeOf(t, err); code != "AOS_OAUTH_FILE_UNREADABLE" {
		t.Fatalf("code = %q", code)
	}
}

// --- expiry with no Refresh configured ----------------------------------

func TestTokenWithNoRefreshConfiguredFailsOnExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	writeJSON(t, path, map[string]any{
		"access_token": "stale", "refresh_token": "rt", "expiry_date": time.Now().Add(-time.Hour).UnixMilli(),
	})
	s := &oauthfile.Store{Path: path, Owner: "the Test CLI", Parse: geminiParse} // Refresh left nil

	_, err := s.Token(context.Background())
	if code := codeOf(t, err); code != "AOS_OAUTH_TOKEN_EXPIRED" {
		t.Fatalf("code = %q, want AOS_OAUTH_TOKEN_EXPIRED", code)
	}
}

func TestTokenWithNoRefreshTokenInTheFileFailsOnExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	writeJSON(t, path, map[string]any{
		"access_token": "stale", "expiry_date": time.Now().Add(-time.Hour).UnixMilli(),
	})
	calls := 0
	s := &oauthfile.Store{
		Path: path, Owner: "the Test CLI", Parse: geminiParse,
		Refresh: func(context.Context, string) (oauthfile.Credentials, error) {
			calls++
			return oauthfile.Credentials{AccessToken: "should-not-be-reached"}, nil
		},
	}

	_, err := s.Token(context.Background())
	if code := codeOf(t, err); code != "AOS_OAUTH_TOKEN_EXPIRED" {
		t.Fatalf("code = %q, want AOS_OAUTH_TOKEN_EXPIRED", code)
	}
	if calls != 0 {
		t.Fatal("Refresh must not be called when the file carries no refresh token")
	}
}

// --- refresh, once implemented for a real provider, exercised here ------
// against a fake ------------------------------------------------------------

func TestTokenRenewsAnExpiredTokenAndPersistsIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	writeJSON(t, path, map[string]any{
		"access_token": "stale", "refresh_token": "old-rt",
		"expiry_date": time.Now().Add(-time.Hour).UnixMilli(),
		"id_token":    "untouched-by-refresh",
	})
	var gotRefreshToken string
	s := &oauthfile.Store{
		Path: path, Owner: "the Test CLI", Parse: geminiParse,
		Refresh: func(_ context.Context, refresh string) (oauthfile.Credentials, error) {
			gotRefreshToken = refresh
			return oauthfile.Credentials{AccessToken: "fresh-token", RefreshToken: "new-rt"}, nil
		},
	}

	got, err := s.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got != "fresh-token" {
		t.Fatalf("got %q, want fresh-token", got)
	}
	if gotRefreshToken != "old-rt" {
		t.Fatalf("Refresh was called with %q, want old-rt", gotRefreshToken)
	}

	// Persisted back to the file another tool owns, preserving what this
	// package never touches.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk["access_token"] != "fresh-token" {
		t.Fatalf("on-disk access_token = %v", onDisk["access_token"])
	}
	if onDisk["refresh_token"] != "new-rt" {
		t.Fatalf("on-disk refresh_token = %v", onDisk["refresh_token"])
	}
	if onDisk["id_token"] != "untouched-by-refresh" {
		t.Fatalf("a key this package never wrote to was disturbed: id_token = %v", onDisk["id_token"])
	}
}

// TestTokenKeepsTheOldRefreshTokenWhenTheResponseOmitsOne: some token
// exchanges only return a fresh access token, expecting the refresh token to
// still be valid — write must not blank it out.
func TestTokenKeepsTheOldRefreshTokenWhenTheResponseOmitsOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	writeJSON(t, path, map[string]any{
		"access_token": "stale", "refresh_token": "keep-me",
		"expiry_date": time.Now().Add(-time.Hour).UnixMilli(),
	})
	s := &oauthfile.Store{
		Path: path, Owner: "the Test CLI", Parse: geminiParse,
		Refresh: func(context.Context, string) (oauthfile.Credentials, error) {
			return oauthfile.Credentials{AccessToken: "fresh-token"}, nil // no RefreshToken
		},
	}

	if _, err := s.Token(context.Background()); err != nil {
		t.Fatalf("Token: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk["refresh_token"] != "keep-me" {
		t.Fatalf("refresh_token = %v, want the original preserved", onDisk["refresh_token"])
	}
}

func TestTokenPropagatesARefreshFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	writeJSON(t, path, map[string]any{
		"access_token": "stale", "refresh_token": "rt", "expiry_date": time.Now().Add(-time.Hour).UnixMilli(),
	})
	s := &oauthfile.Store{
		Path: path, Owner: "the Test CLI", Parse: geminiParse,
		Refresh: func(context.Context, string) (oauthfile.Credentials, error) {
			return oauthfile.Credentials{}, errors.New("the token endpoint said no")
		},
	}

	_, err := s.Token(context.Background())
	if code := codeOf(t, err); code != "AOS_OAUTH_REFRESH_FAILED" {
		t.Fatalf("code = %q, want AOS_OAUTH_REFRESH_FAILED", code)
	}

	// A failed refresh must not have touched the file at all.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk["access_token"] != "stale" {
		t.Fatalf("a failed refresh must not write anything: access_token = %v", onDisk["access_token"])
	}
}

// --- caching and cross-process locking ----------------------------------

func TestTokenCachesAndDoesNotReReadOrReRefreshOnASecondCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	writeJSON(t, path, map[string]any{
		"access_token": "stale", "refresh_token": "rt", "expiry_date": time.Now().Add(-time.Hour).UnixMilli(),
	})
	var refreshCalls int32
	s := &oauthfile.Store{
		Path: path, Owner: "the Test CLI", Parse: geminiParse,
		Refresh: func(context.Context, string) (oauthfile.Credentials, error) {
			atomic.AddInt32(&refreshCalls, 1)
			return oauthfile.Credentials{AccessToken: "fresh-token", RefreshToken: "new-rt"}, nil
		},
	}

	first, err := s.Token(context.Background())
	if err != nil {
		t.Fatalf("first Token: %v", err)
	}
	second, err := s.Token(context.Background())
	if err != nil {
		t.Fatalf("second Token: %v", err)
	}
	if first != second || second != "fresh-token" {
		t.Fatalf("first = %q, second = %q", first, second)
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("Refresh was called %d times, want 1", got)
	}
}

// TestConcurrentTokenCallsRenewOnlyOnce is the property the cross-process
// file lock exists for: two callers racing to renew the same expired
// credential must not both hit the token endpoint, which is the scenario
// that would corrupt a third party's file (the daemon and a CLI invocation
// refreshing at once).
func TestConcurrentTokenCallsRenewOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	writeJSON(t, path, map[string]any{
		"access_token": "stale", "refresh_token": "rt", "expiry_date": time.Now().Add(-time.Hour).UnixMilli(),
	})
	var refreshCalls int32
	s := &oauthfile.Store{
		Path: path, Owner: "the Test CLI", Parse: geminiParse,
		Refresh: func(context.Context, string) (oauthfile.Credentials, error) {
			atomic.AddInt32(&refreshCalls, 1)
			time.Sleep(20 * time.Millisecond) // wide enough for the race to matter
			return oauthfile.Credentials{AccessToken: "fresh-token", RefreshToken: "new-rt"}, nil
		},
	}

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = s.Token(context.Background())
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("Refresh was called %d times across %d concurrent callers, want 1", got, n)
	}
}

// --- Codex and GeminiCLI constructors: path and parse wiring ------------

func TestCodexReadsFromTheStandardPathAndParsesTheNestedTokensShape(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "auth.json"), map[string]any{
		"tokens":       map[string]any{"access_token": "nested-shape", "refresh_token": "rt", "id_token": "it"},
		"last_refresh": "2026-01-01T00:00:00Z",
	})

	s := oauthfile.Codex(home)
	got, err := s.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got != "nested-shape" {
		t.Fatalf("got %q", got)
	}
}

func TestCodexParsesTheTopLevelShapeToo(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "auth.json"), map[string]any{
		"access_token": "top-level-shape", "refresh_token": "rt",
	})

	s := oauthfile.Codex(home)
	got, err := s.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got != "top-level-shape" {
		t.Fatalf("got %q", got)
	}
}

func TestGeminiCLIReadsFromTheStandardPathAndHonoursExpiryDate(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "oauth_creds.json"), map[string]any{
		"access_token": "gemini-token", "refresh_token": "rt",
		"expiry_date": time.Now().Add(-time.Hour).UnixMilli(),
	})

	s := oauthfile.GeminiCLI(home)
	_, err := s.Token(context.Background())
	if code := codeOf(t, err); code != "AOS_OAUTH_TOKEN_EXPIRED" {
		t.Fatalf("code = %q, want AOS_OAUTH_TOKEN_EXPIRED (no Refresh wired by Codex/GeminiCLI yet)", code)
	}
}
