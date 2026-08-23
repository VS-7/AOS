package update_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/relsig"
	"github.com/OWNER/aos/internal/domain/update"
)

// steppingClock advances by the sleep duration instead of actually
// sleeping, the same pattern internal/domain/gateway's own tests use — a
// bounded polling loop's real deadline logic runs, and a 5-minute grace
// period takes nothing to test.
type steppingClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *steppingClock) Sleep(_ context.Context, d time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
	return nil
}

type fakeSource struct {
	release *update.Release
	files   map[string][]byte
	err     error
}

func (f *fakeSource) Latest(context.Context, update.Channel) (*update.Release, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.release, nil
}

func (f *fakeSource) Fetch(_ context.Context, url string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	data, ok := f.files[url]
	if !ok {
		return nil, errors.New("fakeSource: no file at " + url)
	}
	return data, nil
}

type fakeInstaller struct {
	mu            sync.Mutex
	stagedContent map[string][]byte
	live          map[string]string
	backup        map[string]string
	failStageFor  string
	failSwapFor   string
}

func newFakeInstaller() *fakeInstaller {
	return &fakeInstaller{
		stagedContent: map[string][]byte{},
		live:          map[string]string{},
		backup:        map[string]string{},
	}
}

func (f *fakeInstaller) Stage(_ context.Context, name string, data []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failStageFor == name {
		return "", errors.New("fakeInstaller: stage refused for " + name)
	}
	path := "staged/" + name
	f.stagedContent[path] = data
	return path, nil
}

func (f *fakeInstaller) PathOf(_ context.Context, binary string) (string, error) {
	return "target/" + binary, nil
}

func (f *fakeInstaller) SwapIn(_ context.Context, staged, target string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSwapFor != "" && staged == "staged/"+f.failSwapFor {
		return errors.New("fakeInstaller: swap refused for " + staged)
	}
	f.backup[target] = f.live[target]
	f.live[target] = string(f.stagedContent[staged])
	return nil
}

func (f *fakeInstaller) Rollback(_ context.Context, target string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.live[target] = f.backup[target]
	return nil
}

func (f *fakeInstaller) liveAt(target string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.live[target]
}

type fakeSupervisor struct {
	mu         sync.Mutex
	restartErr error
	healthySeq []bool
	calls      int
	restarts   int
}

func (f *fakeSupervisor) Restart(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restarts++
	return f.restartErr
}

func (f *fakeSupervisor) Healthy(context.Context) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.healthySeq) == 0 {
		return true
	}
	idx := f.calls
	if idx >= len(f.healthySeq) {
		idx = len(f.healthySeq) - 1
	}
	f.calls++
	return f.healthySeq[idx]
}

type fakeActiveWork struct {
	mu     sync.Mutex
	counts []int
	calls  int
	err    error
}

func (f *fakeActiveWork) Count(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return 0, f.err
	}
	if len(f.counts) == 0 {
		return 0, nil
	}
	idx := f.calls
	if idx >= len(f.counts) {
		idx = len(f.counts) - 1
	}
	f.calls++
	return f.counts[idx], nil
}

// harness bundles a service with fakes cheap to assert on, mirroring
// internal/domain/gateway's own test-file shape.
type harness struct {
	svc        update.Service
	source     *fakeSource
	installer  *fakeInstaller
	supervisor *fakeSupervisor
	activeWork *fakeActiveWork
	clock      *steppingClock
	pub, priv  string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pub, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		source:     &fakeSource{files: map[string][]byte{}},
		installer:  newFakeInstaller(),
		supervisor: &fakeSupervisor{},
		activeWork: &fakeActiveWork{},
		clock:      &steppingClock{at: time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)},
		pub:        pub, priv: priv,
	}
	h.svc = update.NewService(update.Deps{
		Source: h.source, Installer: h.installer, Supervisor: h.supervisor,
		ActiveWork: h.activeWork, Clock: h.clock, Sleeper: h.clock,
		PublicKey: pub, Platform: "linux/amd64",
	})
	return h
}

// signedRelease builds a release whose checksums file is correctly signed
// with the harness's own key, one asset for "linux/amd64", so a test can
// mutate exactly the thing it wants to break (a byte of the asset, the
// signature, the platform) from a known-good baseline.
func (h *harness) signedRelease(t *testing.T, version string, assetData []byte) *update.Release {
	t.Helper()
	checksums := sha256Line(assetData, "aos_linux_amd64")
	sig, err := relsig.Sign(h.priv, []byte(checksums))
	if err != nil {
		t.Fatal(err)
	}
	h.source.files["https://example.test/checksums.txt"] = []byte(checksums)
	h.source.files["https://example.test/checksums.txt.sig"] = []byte(sig)
	h.source.files["https://example.test/aos"] = assetData
	return &update.Release{
		Version:      version,
		Channel:      update.ChannelStable,
		ChecksumsURL: "https://example.test/checksums.txt",
		SignatureURL: "https://example.test/checksums.txt.sig",
		Assets: []update.Asset{
			{Binary: "aos", Platform: "linux/amd64", URL: "https://example.test/aos", Filename: "aos_linux_amd64", Size: int64(len(assetData))},
		},
	}
}

func TestCheckWithNoNewerReleaseReportsUpToDate(t *testing.T) {
	h := newHarness(t)
	h.source.release = nil

	out, err := h.svc.Check(context.Background(), update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !out.UpToDate {
		t.Fatal("an empty channel should report up to date")
	}
	if out.Release != nil {
		t.Fatal("no release should be attached when up to date")
	}
}

func TestCheckReportsANewerRelease(t *testing.T) {
	h := newHarness(t)
	h.source.release = &update.Release{Version: "v9.9.9", Channel: update.ChannelStable}

	out, err := h.svc.Check(context.Background(), update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.UpToDate {
		t.Fatal("a different version should not report up to date")
	}
	if out.Release == nil || out.Release.Version != "v9.9.9" {
		t.Fatalf("expected the found release attached, got %+v", out.Release)
	}
}

func TestCheckWrapsASourceFailure(t *testing.T) {
	h := newHarness(t)
	h.source.err = errors.New("network is down")

	if _, err := h.svc.Check(context.Background(), update.CheckInput{}); err == nil {
		t.Fatal("expected an error when the release source fails")
	}
}

func TestDownloadStagesAVerifiedRelease(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", []byte("a fake aos binary"))

	out, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if err != nil {
		t.Fatal(err)
	}
	if out.Staged.Version != "v0.10.0" {
		t.Fatalf("staged version = %q", out.Staged.Version)
	}
	if _, ok := out.Staged.Binaries["aos"]; !ok {
		t.Fatal("expected aos to be staged")
	}
}

// The property Download exists to protect: a checksums file whose signature
// does not verify is refused before a single asset is fetched, or trusted
// enough to compare an asset's checksum against.
func TestDownloadRefusesOnInvalidSignature(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", []byte("a fake aos binary"))
	// Tamper with the checksums file after it was signed — the classic
	// "swap the manifest, keep the old signature" attack this gate exists
	// to catch.
	h.source.files["https://example.test/checksums.txt"] = []byte(sha256Line([]byte("a DIFFERENT binary"), "aos_linux_amd64"))

	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if err == nil {
		t.Fatal("expected a signature verification failure")
	}
	if len(h.installer.stagedContent) != 0 {
		t.Fatal("nothing should be staged when the signature does not verify")
	}
}

func TestDownloadRefusesOnChecksumMismatch(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", []byte("a fake aos binary"))
	// The source now serves different bytes than what was signed for —
	// a corrupted or substituted download, not a manifest attack.
	h.source.files["https://example.test/aos"] = []byte("something else entirely")

	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if err == nil {
		t.Fatal("expected a checksum mismatch failure")
	}
	if len(h.installer.stagedContent) != 0 {
		t.Fatal("nothing should be staged on a checksum mismatch")
	}
}

func TestDownloadRefusesWhenNoAssetForThisPlatform(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", []byte("data"))
	release.Assets[0].Platform = "windows/amd64" // this harness is "linux/amd64"

	if _, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release}); err == nil {
		t.Fatal("expected a refusal when no asset targets this platform")
	}
}

func TestApplyWithNothingStagedRefuses(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{}); err == nil {
		t.Fatal("expected a refusal to apply an empty Staged")
	}
}

func TestApplySucceedsAndSwapsTheBinaryIn(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", []byte("new aos contents"))
	downloaded, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.Apply(context.Background(), update.ApplyInput{Staged: downloaded.Staged})
	if err != nil {
		t.Fatal(err)
	}
	if out.RolledBack {
		t.Fatal("a healthy apply should not report a rollback")
	}
	if out.Version != "v0.10.0" {
		t.Fatalf("applied version = %q", out.Version)
	}
	if got := h.installer.liveAt("target/aos"); got != "new aos contents" {
		t.Fatalf("the live binary was not swapped: %q", got)
	}
	if h.supervisor.restarts != 1 {
		t.Fatalf("expected exactly one restart, got %d", h.supervisor.restarts)
	}
}

// This is the property Apply's whole wait exists to protect: it must not
// restart the daemon while a turn most agents would consider a normal-length
// task is still running.
func TestApplyWaitsForActiveWorkToDrain(t *testing.T) {
	h := newHarness(t)
	h.activeWork.counts = []int{3, 2, 1, 0} // drains on the fourth read
	release := h.signedRelease(t, "v0.10.0", []byte("data"))
	downloaded, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{Staged: downloaded.Staged}); err != nil {
		t.Fatal(err)
	}
	if h.activeWork.calls < 4 {
		t.Fatalf("expected Apply to poll until the count reached zero, got %d reads", h.activeWork.calls)
	}
}

func TestApplyRefusesWhenActiveWorkNeverDrains(t *testing.T) {
	h := newHarness(t)
	h.activeWork.counts = []int{5} // never reaches zero — repeats forever
	release := h.signedRelease(t, "v0.10.0", []byte("data"))
	downloaded, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{Staged: downloaded.Staged}); err == nil {
		t.Fatal("expected a timeout when active work never drains")
	}
	if h.supervisor.restarts != 0 {
		t.Fatal("should never have restarted the daemon while work was still active")
	}
}

// The single most important property in this whole package: a new version
// that does not become healthy is rolled back, and the daemon is left
// running again — not left down.
func TestApplyRollsBackOnUnhealthyRestartAndTheDaemonComesBackUp(t *testing.T) {
	h := newHarness(t)
	h.supervisor.healthySeq = []bool{false, false, false, false, false, false, false} // never healthy
	release := h.signedRelease(t, "v0.10.0", []byte("new contents"))
	downloaded, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.Apply(context.Background(), update.ApplyInput{Staged: downloaded.Staged})
	if err == nil {
		t.Fatal("expected an error when the new version never becomes healthy")
	}
	if !out.RolledBack {
		t.Fatal("expected RolledBack to be reported")
	}
	if got := h.installer.liveAt("target/aos"); got != "" {
		t.Fatalf("expected the previous (empty, in this test) binary restored, got %q", got)
	}
	if h.supervisor.restarts != 2 {
		t.Fatalf("expected a restart for the new version and a second restart after rollback, got %d", h.supervisor.restarts)
	}
}

func TestApplyRollsBackWhenASecondBinarysSwapFails(t *testing.T) {
	h := newHarness(t)
	// Two assets, one of which fails to swap — the already-swapped one must
	// come back too, not be left half-updated.
	release := h.signedRelease(t, "v0.10.0", []byte("aos contents"))
	release.Assets = append(release.Assets, update.Asset{
		Binary: "aosd", Platform: "linux/amd64", URL: "https://example.test/aosd", Filename: "aosd_linux_amd64",
	})
	aosd := []byte("aosd contents")
	h.source.files["https://example.test/aosd"] = aosd
	// Extend the checksums file the harness already signed with the second
	// asset's own line, re-signed as one file — matching Download's own
	// expectation of a single signed manifest covering every asset.
	combined := string(h.source.files["https://example.test/checksums.txt"]) + sha256Line(aosd, "aosd_linux_amd64")
	sig, err := relsig.Sign(h.priv, []byte(combined))
	if err != nil {
		t.Fatal(err)
	}
	h.source.files["https://example.test/checksums.txt"] = []byte(combined)
	h.source.files["https://example.test/checksums.txt.sig"] = []byte(sig)

	downloaded, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if err != nil {
		t.Fatal(err)
	}

	h.installer.failSwapFor = "aosd"
	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{Staged: downloaded.Staged}); err == nil {
		t.Fatal("expected the apply to fail when one binary's swap fails")
	}
	if got := h.installer.liveAt("target/aos"); got != "" {
		t.Fatalf("the successfully-swapped binary should have been rolled back too, got %q", got)
	}
	if h.supervisor.restarts != 0 {
		t.Fatal("should never have restarted the daemon on a swap failure")
	}
}

func TestNewServicePanicsWithoutAPublicKey(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected NewService to panic without a PublicKey")
		}
	}()
	update.NewService(update.Deps{
		Source: &fakeSource{}, Installer: newFakeInstaller(),
		Supervisor: &fakeSupervisor{}, ActiveWork: &fakeActiveWork{},
		Clock: &steppingClock{}, Sleeper: &steppingClock{},
	})
}

func sha256Line(data []byte, filename string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + filename + "\n"
}
