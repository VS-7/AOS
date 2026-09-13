package update_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
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
	unconfigured bool
	release      *update.Release
	latestErr    error
	files        map[string][]byte
	fetchErr     error
	// beforeFetch runs as every fetch starts.
	beforeFetch func()
}

func (f *fakeSource) Configured() bool { return !f.unconfigured }

func (f *fakeSource) Latest(context.Context, update.Channel) (*update.Release, error) {
	if f.latestErr != nil {
		return nil, f.latestErr
	}
	return f.release, nil
}

func (f *fakeSource) Fetch(ctx context.Context, url string) ([]byte, error) {
	if f.beforeFetch != nil {
		f.beforeFetch()
	}
	// A real transfer stops when its context does.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	data, ok := f.files[url]
	if !ok {
		return nil, fmt.Errorf("%w: %s answered 404", update.ErrNotPublished, url)
	}
	return data, nil
}

// fakeMachine is one machine's binaries: the live ones, the staged copies
// and the backups a swap keeps. It is both the Stager and the Installer,
// because on a real machine those are one directory tree.
type fakeMachine struct {
	mu sync.Mutex
	// dir is where Target says the live binaries are.
	dir    string
	live   map[string]string
	staged map[string]string
	prev   map[string]string
	// reinstall is what Reinstall answers: why this machine's binaries
	// cannot be replaced one at a time, or nothing.
	reinstall update.ReinstallReason
	failSwap  string
	failUndo  string
	targetErr error
	commits   []string
	discarded int
	// commitErr keeps the backup a Commit was asked to remove, the way a
	// running program's file refuses to be removed on Windows.
	commitErr error
	tidied    int
}

func newFakeMachine(installed ...string) *fakeMachine {
	m := &fakeMachine{dir: "/opt/aos bin/", live: map[string]string{}, staged: map[string]string{}, prev: map[string]string{}}
	for _, b := range installed {
		m.live[b] = "old " + b
	}
	return m
}

func (m *fakeMachine) Stage(_ context.Context, binary string, data []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.staged[binary] = string(data)
	return "/state/update/staged/" + binary, nil
}

func (m *fakeMachine) Digest(_ context.Context, binary string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.staged[binary]
	if !ok {
		return "", errors.New("fakeMachine: nothing staged for " + binary)
	}
	return sha256Hex([]byte(data)), nil
}

func (m *fakeMachine) Discard(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.staged = map[string]string{}
	m.discarded++
	return nil
}

func (m *fakeMachine) Target(_ context.Context, binary string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.targetErr != nil {
		return "", false, m.targetErr
	}
	_, ok := m.live[binary]
	return m.dir + binary, ok, nil
}

// SwapIn refuses a staged copy whose digest is not the one it is handed,
// the way the filesystem Installer does with the bytes it copies.
func (m *fakeMachine) SwapIn(_ context.Context, binary, digest string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failSwap == binary {
		return errors.New("fakeMachine: swap refused for " + binary)
	}
	if sha256Hex([]byte(m.staged[binary])) != digest {
		return fmt.Errorf("%w: %s", update.ErrStagedChanged, binary)
	}
	m.prev[binary] = m.live[binary]
	m.live[binary] = m.staged[binary]
	return nil
}

func (m *fakeMachine) Rollback(_ context.Context, binary string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failUndo == binary {
		return errors.New("fakeMachine: rollback refused for " + binary)
	}
	if prev, ok := m.prev[binary]; ok {
		m.live[binary] = prev
		delete(m.prev, binary)
	}
	return nil
}

func (m *fakeMachine) Commit(_ context.Context, binary string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commits = append(m.commits, binary)
	if m.commitErr != nil {
		return m.commitErr
	}
	delete(m.prev, binary)
	return nil
}

func (m *fakeMachine) Tidy(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prev = map[string]string{}
	m.tidied++
	return nil
}

func (m *fakeMachine) backups() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.prev)
}

func (m *fakeMachine) Reinstall(context.Context) update.ReinstallReason { return m.reinstall }

func (m *fakeMachine) liveAt(binary string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.live[binary]
}

// fakeSupervisor is a daemon and the gateway that started it. Until told
// otherwise the daemon answering is the process the gateway started, and a
// restart brings up a new process running whatever version the live
// binaries on the machine carry.
type fakeSupervisor struct {
	mu      sync.Mutex
	machine *fakeMachine
	// oldVersion is what the daemon runs before anything is installed.
	oldVersion string
	// self: the process asking is the daemon itself.
	self bool
	// unsupervised: the daemon answering was started by hand, and the
	// gateway has no record of it.
	unsupervised bool
	// silent: the gateway's daemon is running and not answering.
	silent bool
	// idle: no daemon runs at all until a restart starts one.
	idle       bool
	observeErr error
	// elsewhere: a restart starts the daemon from another copy of the binary
	// than the one installed, which stays on oldVersion.
	elsewhere   bool
	restartErrs []error
	// newVersionSick keeps the daemon from answering after the first
	// restart — the one onto the new binaries — and not after any other.
	newVersionSick bool
	restarts       int
	// onRestart runs on every restart, while the swap is in place.
	onRestart func()
}

func (f *fakeSupervisor) Observe(context.Context) (update.Daemon, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.observeErr != nil {
		return update.Daemon{}, f.observeErr
	}
	pid := 1000 + f.restarts
	d := update.Daemon{Address: "127.0.0.1:5326", Self: f.self}
	switch {
	case f.self && f.unsupervised:
		d.Answering, d.PID, d.Version = true, 777, f.oldVersion
	case f.self:
		d.Answering, d.PID, d.RecordedPID, d.Version = true, pid, pid, f.oldVersion
	case f.unsupervised:
		d.Answering, d.PID, d.Version = true, 777, f.oldVersion
	case f.idle && f.restarts == 0:
	case f.silent, f.newVersionSick && f.restarts == 1:
		d.RecordedPID = pid
	default:
		d.Answering, d.PID, d.RecordedPID, d.Version = true, pid, pid, f.runningVersion()
	}
	return d, nil
}

// runningVersion is the version a daemon started now runs: the release the
// live binaries carry once one is swapped in.
func (f *fakeSupervisor) runningVersion() string {
	if f.restarts == 0 || f.elsewhere {
		return f.oldVersion
	}
	f.machine.mu.Lock()
	defer f.machine.mu.Unlock()
	for _, content := range f.machine.live {
		if strings.HasPrefix(content, "new ") {
			fields := strings.Fields(content)
			return fields[len(fields)-1]
		}
	}
	return f.oldVersion
}

func (f *fakeSupervisor) Restart(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restarts++
	if f.onRestart != nil {
		f.onRestart()
	}
	if len(f.restartErrs) >= f.restarts {
		return f.restartErrs[f.restarts-1]
	}
	return nil
}

type fakeActiveWork struct {
	mu     sync.Mutex
	counts []int
	calls  int
	err    error
	// during runs on every read: something happening while Apply waits.
	during func()
}

func (f *fakeActiveWork) Count(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.during != nil {
		f.during()
	}
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

type fakeOperators struct {
	deny bool
	err  error
}

func (f fakeOperators) MayInstall(context.Context) (bool, error) { return !f.deny, f.err }

// fakeLock is a Lock another install already holds, or one that cannot be
// taken at all.
type fakeLock struct {
	busy bool
	err  error
}

func (f fakeLock) TryLock(context.Context) (func(), bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	return func() {}, !f.busy, nil
}

type failingStore struct{}

func (failingStore) Load(context.Context) (update.Record, error) {
	return update.Record{}, errors.New("disk on fire")
}
func (failingStore) Update(context.Context, func(*update.Record) error) error {
	return errors.New("disk on fire")
}

// harness bundles a service with fakes cheap to assert on, mirroring
// internal/domain/gateway's own test-file shape.
type harness struct {
	svc        update.Service
	source     *fakeSource
	machine    *fakeMachine
	supervisor *fakeSupervisor
	activeWork *fakeActiveWork
	operators  fakeOperators
	store      update.Store
	lock       update.Lock
	clock      *steppingClock
	version    string
	platform   string
	customFeed bool
	flavour    string
	pub, priv  string
	// chosenInstall is set by installed(): the test decided what is on this
	// machine, and signedRelease leaves it alone. Otherwise a release
	// covers exactly what is installed, which is what a real one does.
	chosenInstall bool
}

type option func(*harness)

func running(version string) option { return func(h *harness) { h.version = version } }
func installed(binaries ...string) option {
	return func(h *harness) { h.machine, h.chosenInstall = newFakeMachine(binaries...), true }
}
func onPlatform(platform string) option    { return func(h *harness) { h.platform = platform } }
func withCustomFeed() option               { return func(h *harness) { h.customFeed = true } }
func withFlavour(f string) option          { return func(h *harness) { h.flavour = f } }
func withStore(store update.Store) option  { return func(h *harness) { h.store = store } }
func withOperators(o fakeOperators) option { return func(h *harness) { h.operators = o } }
func withLock(l update.Lock) option        { return func(h *harness) { h.lock = l } }

// sameMachineAs builds the service over another harness's machine, feed,
// daemon and key: a second binary on the same installation.
func sameMachineAs(o *harness) option {
	return func(h *harness) {
		h.source, h.machine, h.supervisor, h.activeWork = o.source, o.machine, o.supervisor, o.activeWork
		h.pub, h.priv, h.chosenInstall = o.pub, o.priv, true
	}
}

func newHarness(t *testing.T, opts ...option) *harness {
	t.Helper()
	pub, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		source:     &fakeSource{files: map[string][]byte{}},
		machine:    newFakeMachine("aos", "aosd"),
		supervisor: &fakeSupervisor{},
		activeWork: &fakeActiveWork{},
		clock:      &steppingClock{at: time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)},
		version:    "v0.9.0",
		platform:   "linux/amd64",
		flavour:    build.FlavourStandard,
		pub:        pub, priv: priv,
	}
	for _, o := range opts {
		o(h)
	}
	if h.supervisor.machine == nil {
		h.supervisor.machine, h.supervisor.oldVersion = h.machine, h.version
	}
	h.svc = update.NewService(update.Deps{
		Source: h.source, Stager: h.machine, Installer: h.machine, Store: h.store, Lock: h.lock,
		Supervisor: h.supervisor, Operators: h.operators, ActiveWork: h.activeWork,
		Clock: h.clock, Sleeper: h.clock,
		PublicKey: h.pub, Platform: h.platform, Version: h.version, CustomFeed: h.customFeed,
		Flavour: h.flavour,
	})
	return h
}

// signedRelease publishes a release whose checksums file is correctly signed
// with the harness's own key, one linux/amd64 asset per binary named after
// release.yml's own convention, so a test can mutate exactly the thing it
// wants to break from a known-good baseline.
func (h *harness) signedRelease(t *testing.T, version string, binaries ...string) *update.Release {
	t.Helper()
	release := &update.Release{
		Version:      version,
		Channel:      update.ChannelStable,
		ChecksumsURL: "https://example.test/checksums.txt",
		SignatureURL: "https://example.test/checksums.txt.sig",
	}
	if !h.chosenInstall {
		h.machine.mu.Lock()
		h.machine.live = map[string]string{}
		for _, b := range binaries {
			h.machine.live[b] = "old " + b
		}
		h.machine.mu.Unlock()
	}
	var checksums strings.Builder
	for _, b := range binaries {
		filename := b + "_" + version + "_linux_amd64"
		data := []byte("new " + b + " " + version)
		url := "https://example.test/" + filename
		h.source.files[url] = data
		checksums.WriteString(sha256Line(data, filename))
		release.Assets = append(release.Assets, update.Asset{
			Binary: b, Platform: "linux/amd64", URL: url, Filename: filename, Size: int64(len(data)),
		})
	}
	h.sign(t, checksums.String())
	h.source.release = release
	return release
}

func (h *harness) sign(t *testing.T, checksums string) {
	t.Helper()
	sig, err := relsig.Sign(h.priv, []byte(checksums))
	if err != nil {
		t.Fatal(err)
	}
	h.source.files["https://example.test/checksums.txt"] = []byte(checksums)
	h.source.files["https://example.test/checksums.txt.sig"] = []byte(sig)
}

func (h *harness) download(t *testing.T, release *update.Release) update.Staged {
	t.Helper()
	out, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if err != nil {
		t.Fatal(err)
	}
	return out.Staged
}

// wantCode asserts the refusal's code and hands the refusal back by value, so
// a test can go on to read its causer and calls to action.
func wantCode(t *testing.T, err error, code string) apperr.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got no error", code)
	}
	e, ok := apperr.As(err)
	if !ok || e.Code != "AOS_"+code {
		t.Fatalf("expected AOS_%s, got %v", code, err)
	}
	return *e
}

// --- Check ------------------------------------------------------------------

// The shipped state: no feed. It used to answer UpToDate:true, which the
// window rendered as a green "You are on the newest release." while newer
// releases were published.
func TestCheckWithNoFeedSaysSoInsteadOfUpToDate(t *testing.T) {
	h := newHarness(t)
	h.source.unconfigured = true

	out, err := h.svc.Check(context.Background(), update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.State != update.StateNotConfigured || out.UpToDate {
		t.Fatalf("an installation with no feed must not report up to date, got %+v", out)
	}

	st, err := h.svc.Status(context.Background(), update.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Configured {
		t.Fatal("status should say no feed is configured")
	}
	if st.CheckedAt == nil || st.LastState != update.StateNotConfigured {
		t.Fatalf("the check should be remembered, got %+v", st)
	}
}

func TestCheckReportsANewerRelease(t *testing.T) {
	h := newHarness(t)
	h.source.release = &update.Release{Version: "v0.10.0", Channel: update.ChannelStable}

	out, err := h.svc.Check(context.Background(), update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.State != update.StateAvailable || out.UpToDate {
		t.Fatalf("a newer version should be available, got %+v", out)
	}
	if out.Release == nil || out.Release.Version != "v0.10.0" {
		t.Fatalf("expected the found release attached, got %+v", out.Release)
	}
	if out.Install == nil || out.Install.Method != update.InstallHere {
		t.Fatalf("expected an in-place install, got %+v", out.Install)
	}
}

// Status used to return a constant {current, channel}, so the window said
// "not checked yet" right under the answer of the check it had just made.
func TestStatusRemembersTheLastCheck(t *testing.T) {
	h := newHarness(t)
	h.source.release = &update.Release{Version: "v0.10.0"}
	if _, err := h.svc.Check(context.Background(), update.CheckInput{}); err != nil {
		t.Fatal(err)
	}

	st, err := h.svc.Status(context.Background(), update.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if st.CheckedAt == nil || !st.CheckedAt.Equal(h.clock.Now()) {
		t.Fatalf("CheckedAt = %v, want the check's time", st.CheckedAt)
	}
	if st.LatestKnown != "v0.10.0" || st.LastState != update.StateAvailable || !st.Configured {
		t.Fatalf("status did not carry the check, got %+v", st)
	}
}

func TestStatusDoesNotOfferWhatThisInstallationAlreadyReached(t *testing.T) {
	store := &sharedStore{}
	old := newHarness(t, withStore(store))
	old.source.release = &update.Release{Version: "v0.10.0"}
	if _, err := old.svc.Check(context.Background(), update.CheckInput{}); err != nil {
		t.Fatal(err)
	}

	// The same state directory, now read by the version that was offered.
	updated := newHarness(t, withStore(store), running("v0.10.0"))
	st, err := updated.svc.Status(context.Background(), update.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if st.LastState != update.StateUpToDate {
		t.Fatalf("a release this installation now runs is not available, got %q", st.LastState)
	}
}

// The record outlives the binary that wrote it. What it says is re-read
// against the binary reading it: a development build is not "up to date"
// with a release, and a release build behind the newest release known is
// behind it, whoever made the check.
func TestStatusReadsTheLastCheckAgainstTheBinaryReadingIt(t *testing.T) {
	cases := []struct {
		wrote, reads string
		want         update.CheckState
	}{
		{"v0.9.0", "dev", update.StateDeveloperBuild},
		{"v0.9.0", "v0.10.0", update.StateUpToDate},
		{"dev", "v0.9.0", update.StateAvailable},
		{"v0.10.0", "v0.9.0", update.StateAvailable},
	}
	for _, c := range cases {
		store := &sharedStore{}
		writer := newHarness(t, withStore(store), running(c.wrote))
		writer.source.release = &update.Release{Version: "v0.10.0"}
		if _, err := writer.svc.Check(context.Background(), update.CheckInput{}); err != nil {
			t.Fatal(err)
		}

		reader := newHarness(t, withStore(store), running(c.reads))
		st, err := reader.svc.Status(context.Background(), update.StatusInput{})
		if err != nil {
			t.Fatal(err)
		}
		if st.LastState != c.want {
			t.Errorf("checked by %s, read by %s: lastState = %q, want %q", c.wrote, c.reads, st.LastState, c.want)
		}
	}
}

// Ordering, not equality: a lagging mirror's older release is not an
// update, and a locally packaged -dirty build of the same release is that
// release.
func TestCheckOffersOnlyAStrictlyNewerRelease(t *testing.T) {
	cases := []struct {
		current, published string
		want               update.CheckState
	}{
		{"v0.15.2-fase9", "v0.1.0", update.StateUpToDate},
		{"v0.15.2-fase9-dirty", "v0.15.2-fase9", update.StateUpToDate},
		{"v0.15.2-fase9", "v0.15.2-fase9-dirty", update.StateUpToDate},
		{"v0.15.0-fase9-dirty", "v0.15.2-fase9", update.StateAvailable},
		{"dev", "v0.15.2-fase9", update.StateDeveloperBuild},
	}
	for _, c := range cases {
		h := newHarness(t, running(c.current))
		h.source.release = &update.Release{Version: c.published}
		out, err := h.svc.Check(context.Background(), update.CheckInput{})
		if err != nil {
			t.Fatal(err)
		}
		if out.State != c.want {
			t.Errorf("running %s, published %s: state = %q, want %q", c.current, c.published, out.State, c.want)
		}
		if out.UpToDate != (c.want == update.StateUpToDate) {
			t.Errorf("running %s, published %s: UpToDate = %v", c.current, c.published, out.UpToDate)
		}
	}
}

// A configured feed with nothing at the channel's address is a wrong address
// or an unpublished channel — never "you are on the newest release".
func TestCheckOnAFeedWithNoManifestIsAnError(t *testing.T) {
	h := newHarness(t)
	h.source.latestErr = fmt.Errorf("%w: stable.json answered 404", update.ErrNotPublished)

	_, err := h.svc.Check(context.Background(), update.CheckInput{})
	wantCode(t, err, "UPDATE_CHANNEL_EMPTY")
}

func TestCheckTellsTheNetworkApartFromABadAnswer(t *testing.T) {
	h := newHarness(t)
	h.source.latestErr = fmt.Errorf("%w: dial tcp: no route to host", update.ErrUnreachable)
	_, err := h.svc.Check(context.Background(), update.CheckInput{})
	if e := wantCode(t, err, "UPDATE_SOURCE_UNREACHABLE"); e.CauserName != "update.Service.Check" {
		t.Fatalf("causer = %q", e.CauserName)
	}

	h.source.latestErr = errors.New("stable.json answered 500")
	_, err = h.svc.Check(context.Background(), update.CheckInput{})
	wantCode(t, err, "UPDATE_SOURCE_FAILED")
}

func TestCheckRefusesAManifestWhoseVersionIsNotAVersion(t *testing.T) {
	h := newHarness(t)
	h.source.release = &update.Release{Version: "latest"}
	_, err := h.svc.Check(context.Background(), update.CheckInput{})
	wantCode(t, err, "UPDATE_RELEASE_VERSION_INVALID")
}

// The daemon itself cannot restart onto a new version, so an offered release
// says how it can be installed: the command, from a terminal.
func TestCheckNamesTheTerminalCommandWhenThisProcessCannotRestart(t *testing.T) {
	h := newHarness(t)
	h.supervisor.self = true
	h.source.release = &update.Release{Version: "v0.10.0"}

	out, err := h.svc.Check(context.Background(), update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Install == nil || out.Install.Method != update.InstallFromTerminal {
		t.Fatalf("install = %+v", out.Install)
	}
	if want := "'/opt/aos bin/aosd' update apply --version v0.10.0"; out.Install.Command != want {
		t.Fatalf("command = %q, want %q", out.Install.Command, want)
	}
}

// The command is pasted into whatever terminal the platform has. On Windows
// that is PowerShell, where a quoted path is a string rather than a command
// until & runs it — and POSIX single quotes around C:\... run nothing at all.
func TestCheckNamesATerminalCommandThatRunsOnThisPlatform(t *testing.T) {
	cases := []struct {
		platform, dir, want string
	}{
		{"linux/amd64", "/opt/aos/", "/opt/aos/aosd update apply --version v0.10.0"},
		{"darwin/arm64", "/Users/me/My Apps/", "'/Users/me/My Apps/aosd' update apply --version v0.10.0"},
		{"windows/amd64", `C:\Users\me\AOS\`, `C:\Users\me\AOS\aosd update apply --version v0.10.0`},
		{"windows/amd64", `C:\Program Files\AOS\`, `& 'C:\Program Files\AOS\aosd' update apply --version v0.10.0`},
		{"windows/arm64", `C:\Users\O'Neil\AOS\`, `& 'C:\Users\O''Neil\AOS\aosd' update apply --version v0.10.0`},
	}
	for _, c := range cases {
		h := newHarness(t, onPlatform(c.platform))
		h.machine.dir = c.dir
		h.supervisor.self = true
		h.source.release = &update.Release{Version: "v0.10.0"}

		out, err := h.svc.Check(context.Background(), update.CheckInput{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Install == nil || out.Install.Command != c.want {
			t.Errorf("%s in %s: command = %+v, want %q", c.platform, c.dir, out.Install, c.want)
		}
	}
}

// The manifest is not signed, and its page address is opened in the
// person's browser. Only a web page is kept.
func TestCheckKeepsOnlyAWebPageAsTheReleasePage(t *testing.T) {
	cases := map[string]string{
		"https://github.com/VS-7/AOS/releases/tag/v0.10.0": "https://github.com/VS-7/AOS/releases/tag/v0.10.0",
		"http://127.0.0.1:7498/releases/v0.10.0":           "http://127.0.0.1:7498/releases/v0.10.0",
		"http://localhost/releases":                        "http://localhost/releases",
		"http://example.com/releases":                      "",
		"file:///etc/passwd":                               "",
		"javascript:alert(1)":                              "",
		"https:///no-host":                                 "",
		"  ":                                               "",
	}
	for page, want := range cases {
		h := newHarness(t)
		h.source.release = &update.Release{Version: "v0.10.0", PageURL: page}
		out, err := h.svc.Check(context.Background(), update.CheckInput{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Release == nil || out.Release.PageURL != want {
			t.Errorf("pageUrl %q: kept %+v, want %q", page, out.Release, want)
		}
	}
}

// The feed a build carries is not something the person set, so an empty one
// cannot be answered with "check AOS_UPDATE_BASE_URL".
func TestCheckOnAnEmptyFeedSaysWhoseFeedItIs(t *testing.T) {
	builtIn := newHarness(t)
	builtIn.source.latestErr = fmt.Errorf("%w: stable.json answered 404", update.ErrNotPublished)
	_, err := builtIn.svc.Check(context.Background(), update.CheckInput{})
	e := wantCode(t, err, "UPDATE_CHANNEL_EMPTY")
	if len(e.Actions) == 0 || strings.Contains(e.Actions[0].Label, "AOS_UPDATE_BASE_URL") ||
		!strings.Contains(e.Actions[0].Label, "installer") {
		t.Fatalf("a build's own feed should send the person to the installer, got %+v", e.Actions)
	}

	custom := newHarness(t, withCustomFeed())
	custom.source.latestErr = builtIn.source.latestErr
	_, err = custom.svc.Check(context.Background(), update.CheckInput{})
	e = wantCode(t, err, "UPDATE_CHANNEL_EMPTY")
	if len(e.Actions) == 0 || !strings.Contains(e.Actions[0].Label, "AOS_UPDATE_BASE_URL") {
		t.Fatalf("a feed set on this machine should be named, got %+v", e.Actions)
	}
}

func TestCheckSendsABundleToTheInstaller(t *testing.T) {
	h := newHarness(t)
	h.machine.reinstall = update.ReinstallBundle
	h.source.release = &update.Release{Version: "v0.10.0"}

	out, err := h.svc.Check(context.Background(), update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Install == nil || out.Install.Method != update.InstallReinstall || out.Install.Reason != update.ReinstallBundle {
		t.Fatalf("install = %+v", out.Install)
	}
}

// The server tarball's aosd is built with the web interface compiled in; the
// release feed publishes the plain aosd, the one aos-desktop supervises. A
// server installation that took it came back answering the API only, and the
// machine a person reached with a browser had no interface left. It is sent
// to the installer instead, at every step that could have installed it.
func TestAServerInstallationIsNotSwappedForTheDaemonWithoutItsInterface(t *testing.T) {
	store := &sharedStore{}
	// Staged by a standard daemon sharing the state directory, which is
	// what Apply would otherwise have installed.
	standard := newHarness(t, withStore(store))
	release := standard.signedRelease(t, "v0.10.0", "aos", "aosd")
	standard.download(t, release)

	h := newHarness(t, withStore(store), withFlavour(build.FlavourServer), sameMachineAs(standard))

	out, err := h.svc.Check(context.Background(), update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.State != update.StateAvailable {
		t.Fatalf("a server installation should still be told a release exists, got %+v", out)
	}
	if out.Install == nil || out.Install.Method != update.InstallReinstall || out.Install.Reason != update.ReinstallServer {
		t.Fatalf("install = %+v, want reinstall for the server flavour", out.Install)
	}

	_, err = h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	e := wantCode(t, err, "UPDATE_REINSTALL_REQUIRED")
	if !strings.Contains(e.Message, "web interface") || len(e.Actions) == 0 || !strings.Contains(e.Actions[0].Label, "AOS_SERVER=1") {
		t.Fatalf("the refusal should say why and how, got %q %+v", e.Message, e.Actions)
	}

	_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_REINSTALL_REQUIRED")
	if h.machine.liveAt("aosd") != "old aosd" || h.supervisor.restarts != 0 {
		t.Fatal("the server's daemon must not be replaced")
	}

	st, err := h.svc.Status(context.Background(), update.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Install.Method != update.InstallReinstall || st.Install.Reason != update.ReinstallServer {
		t.Fatalf("status install = %+v", st.Install)
	}
}

// A directory this account cannot write — an AppImage's read-only mount, an
// install for every account under Program Files or /usr — was offered a
// terminal command that could only fail at the swap.
func TestAnInstallationThisAccountCannotChangeIsReinstalled(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos", "aosd")
	h.machine.reinstall = update.ReinstallReadOnly

	out, err := h.svc.Check(context.Background(), update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Install == nil || out.Install.Method != update.InstallReinstall || out.Install.Reason != update.ReinstallReadOnly || out.Install.Command != "" {
		t.Fatalf("install = %+v", out.Install)
	}
	_, err = h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	e := wantCode(t, err, "UPDATE_REINSTALL_REQUIRED")
	if !strings.Contains(e.Message, "cannot be changed by this account") {
		t.Fatalf("message = %q", e.Message)
	}
	if len(h.machine.staged) != 0 {
		t.Fatal("nothing should be downloaded for an installation that cannot take it")
	}
}

// An update installed from a terminal replaces the window's binary too, and
// the window already open keeps running the previous release until it is
// reopened. The command says so where there is a window to reopen.
func TestTheTerminalInstallSaysToReopenTheWindowWhenThereIsOne(t *testing.T) {
	for _, c := range []struct {
		installed []string
		reopen    bool
	}{
		{[]string{"aos", "aos-desktop", "aosd"}, true},
		{[]string{"aos", "aosd"}, false},
	} {
		h := newHarness(t, installed(c.installed...))
		h.supervisor.self = true
		h.download(t, h.signedRelease(t, "v0.10.0", c.installed...))

		out, err := h.svc.Check(context.Background(), update.CheckInput{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Install == nil || out.Install.Reopen != c.reopen {
			t.Errorf("installed %v: install = %+v, want reopen %v", c.installed, out.Install, c.reopen)
		}

		_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
		e := wantCode(t, err, "UPDATE_RESTART_UNAVAILABLE")
		said := false
		for _, a := range e.Actions {
			said = said || strings.Contains(a.Label, "reopen AOS")
		}
		if said != c.reopen {
			t.Errorf("installed %v: calls to action %+v, want reopen %v", c.installed, e.Actions, c.reopen)
		}
	}
}

// A check writes the record back after asking the network. A download that
// finished in between used to be overwritten with the record the check read
// before it: the staged release vanished from the record, and Apply answered
// UPDATE_NOTHING_STAGED over verified files still sitting in the stage.
func TestACheckDoesNotLoseWhatADownloadStagedMeanwhile(t *testing.T) {
	store := &racingStore{}
	h := newHarness(t, withStore(store))
	release := h.signedRelease(t, "v0.10.0", "aos", "aosd")

	downloaded := make(chan error, 1)
	store.overtake = func() {
		go func() {
			_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
			downloaded <- err
		}()
		// Long enough for a download that is not kept waiting to finish.
		select {
		case err := <-downloaded:
			downloaded <- err
		case <-time.After(300 * time.Millisecond):
		}
	}

	if _, err := h.svc.Check(context.Background(), update.CheckInput{}); err != nil {
		t.Fatal(err)
	}
	if err := <-downloaded; err != nil {
		t.Fatal(err)
	}

	st, err := h.svc.Status(context.Background(), update.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Staged == nil || st.Staged.Version != "v0.10.0" {
		t.Fatalf("the download's staged release was lost, status = %+v", st)
	}
	if st.CheckedAt == nil {
		t.Fatalf("the check was lost, status = %+v", st)
	}
	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"}); err != nil {
		t.Fatalf("what was staged should install, got %v", err)
	}
}

// Two installs at once swap over each other: the second one's SwapIn keeps
// the first one's new binary as its "previous" one, and a rollback restores
// that. A download beside an install discards the files it is swapping in.
// One at a time, across every process that can run one.
func TestOneDownloadOrApplyAtATime(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos", "aosd")
	h.download(t, release)

	busy := newHarness(t, sameMachineAs(h), withLock(fakeLock{busy: true}))
	_, err := busy.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_IN_PROGRESS")
	_, err = busy.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_IN_PROGRESS")
	if h.machine.liveAt("aosd") != "old aosd" || h.supervisor.restarts != 0 || h.machine.discarded != 1 {
		t.Fatal("a refused call must not touch the installation")
	}

	broken := newHarness(t, sameMachineAs(h), withLock(fakeLock{err: errors.New("read-only state directory")}))
	_, err = broken.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_LOCK_FAILED")

	// Without a Lock wired, one service still keeps its own calls apart.
	var second error
	var asked atomic.Bool
	h.activeWork.during = func() {
		if asked.CompareAndSwap(false, true) {
			_, second = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
		}
	}
	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"}); err != nil {
		t.Fatal(err)
	}
	wantCode(t, second, "UPDATE_IN_PROGRESS")
	h.activeWork.during = nil
	_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_NOTHING_STAGED")
}

// lockFreeAfter is a lock something else holds for its first n attempts.
type lockFreeAfter struct {
	mu    sync.Mutex
	n     int
	tries int
}

func (l *lockFreeAfter) TryLock(context.Context) (func(), bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tries++
	return func() {}, l.tries > l.n, nil
}

// A download whose answer was lost — the window's bridge gave up waiting, or
// sent it again and was told UPDATE_IN_PROGRESS — is followed through Status
// until it ends.
func TestStatusSaysWhileADownloadOrInstallIsRunning(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aosd")
	st, err := h.svc.Status(context.Background(), update.StatusInput{})
	if err != nil || st.Busy {
		t.Fatalf("nothing is running: %+v, %v", st, err)
	}

	var during update.Status
	h.source.beforeFetch = func() { during, _ = h.svc.Status(context.Background(), update.StatusInput{}) }
	h.download(t, release)
	if !during.Busy {
		t.Fatalf("while the download ran, status = %+v", during)
	}
	h.source.beforeFetch = nil
	if st, _ := h.svc.Status(context.Background(), update.StatusInput{}); st.Busy || st.Staged == nil {
		t.Fatalf("after it, status = %+v", st)
	}

	busy := newHarness(t, sameMachineAs(h), withLock(fakeLock{busy: true}))
	if st, _ := busy.svc.Status(context.Background(), update.StatusInput{}); !st.Busy {
		t.Fatalf("with the lock held elsewhere, status = %+v", st)
	}
}

// Status and Check take the lock for an instant to look. A download that
// arrives in that instant waits it out rather than being told another one is
// running.
func TestADownloadIsNotRefusedForALookAtTheLock(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aosd")
	glance := newHarness(t, sameMachineAs(h), withLock(&lockFreeAfter{n: 2}))
	if _, err := glance.svc.Download(context.Background(), update.DownloadInput{Release: release}); err != nil {
		t.Fatalf("a lock held for a glance should be waited out, got %v", err)
	}
}

// On Windows a program's own file can be renamed while it runs and not
// removed. `aosd.exe update apply` runs from the aosd.exe it renames to
// aosd.exe.prev, so its Commit cannot remove that backup, and 50 MB stayed
// beside the new daemon with only a log line to say so. The next check — by
// then that process has ended — removes what a finished install left.
func TestABackupAFinishedInstallCouldNotRemoveIsTidiedByTheNextCheck(t *testing.T) {
	store := &sharedStore{}
	h := newHarness(t, withStore(store))
	release := h.signedRelease(t, "v0.10.0", "aos", "aosd")
	h.download(t, release)
	h.machine.commitErr = errors.New("the process cannot access the file because it is being used by another process")

	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"}); err != nil {
		t.Fatal(err)
	}
	if h.machine.backups() == 0 {
		t.Fatal("the fake should have kept the backups its commit could not remove")
	}

	updated := newHarness(t, withStore(store), sameMachineAs(h), running("v0.10.0"))
	before := h.machine.tidied
	if _, err := updated.svc.Check(context.Background(), update.CheckInput{}); err != nil {
		t.Fatal(err)
	}
	if h.machine.backups() != 0 || h.machine.tidied != before+1 {
		t.Fatalf("the next check should remove the leftover backups, %d left", h.machine.backups())
	}
}

// A backup is the only copy of the previous binary while an install is
// under way, and after one that stopped partway — its process killed between
// the swap and the end. Tidying leaves both alone.
func TestTidyingLeavesTheBackupsOfAnInstallThatHasNotFinished(t *testing.T) {
	store := &sharedStore{}
	h := newHarness(t, withStore(store))
	h.source.release = &update.Release{Version: "v0.10.0"}

	// What an Apply killed between its swap and its end leaves: the record
	// still says it is installing, and the backup is the previous binary.
	store.record.Installing = "v0.10.0"
	h.machine.prev["aosd"] = "old aosd"

	if _, err := h.svc.Check(context.Background(), update.CheckInput{}); err != nil {
		t.Fatal(err)
	}
	if h.machine.tidied != 0 || h.machine.backups() != 1 {
		t.Fatal("the backup of an install that did not finish must be kept")
	}

	// Under way in another process: the lock is held.
	store.record.Installing = ""
	other := newHarness(t, withStore(store), sameMachineAs(h), withLock(fakeLock{busy: true}))
	if _, err := other.svc.Check(context.Background(), update.CheckInput{}); err != nil {
		t.Fatal(err)
	}
	if h.machine.tidied != 0 {
		t.Fatal("a check must not tidy while an install holds the lock")
	}
}

// The record says an install is under way from before its first swap until
// it ends, whichever way it ends — except when restoring the previous
// binaries failed, where the backups are what is left to recover by hand.
func TestApplyRecordsThatItIsInstallingUntilItEnds(t *testing.T) {
	cases := []struct {
		name     string
		sabotage func(h *harness)
		code     string
		after    string
	}{
		{"installed", func(*harness) {}, "", ""},
		{"rolled back", func(h *harness) { h.supervisor.newVersionSick = true }, "UPDATE_ROLLED_BACK", ""},
		{"rollback failed", func(h *harness) {
			h.supervisor.newVersionSick = true
			h.machine.failUndo = "aosd"
		}, "UPDATE_ROLLBACK_FAILED", "v0.10.0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &sharedStore{}
			h := newHarness(t, withStore(store))
			h.download(t, h.signedRelease(t, "v0.10.0", "aos", "aosd"))
			c.sabotage(h)
			var during string
			h.supervisor.onRestart = func() {
				if during == "" {
					during = store.record.Installing
				}
			}

			_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
			if c.code == "" && err != nil {
				t.Fatal(err)
			}
			if c.code != "" {
				wantCode(t, err, c.code)
			}
			if during != "v0.10.0" {
				t.Fatalf("while swapped in, the record said %q", during)
			}
			if store.record.Installing != c.after {
				t.Fatalf("afterwards the record says %q, want %q", store.record.Installing, c.after)
			}
		})
	}
}

// The window reaches the daemon over a bridge that gives up on a call after
// 30 seconds, and so does `aos update download`. A 70 MB release takes longer
// than that on a slow link. The request was cancelled with the caller, and the
// daemon abandoned the download partway. Once started, a download finishes and
// is staged, ready for the next look at Settings › Updates, and the lock keeps
// a second click from starting another one beside it.
func TestADownloadFinishesAfterTheCallerStopsWaiting(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos", "aosd")
	ctx, cancel := context.WithCancel(context.Background())
	fetches := 0
	h.source.beforeFetch = func() {
		fetches++
		if fetches == 2 {
			cancel()
		}
	}

	_, _ = h.svc.Download(ctx, update.DownloadInput{Release: release})

	st, err := h.svc.Status(context.Background(), update.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Staged == nil || st.Staged.Version != "v0.10.0" || len(h.machine.staged) != 2 {
		t.Fatalf("the download should have finished and staged the release, status %+v", st.Staged)
	}
}

// --- Download ---------------------------------------------------------------

func TestDownloadStagesAVerifiedRelease(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos", "aosd")

	staged := h.download(t, release)
	if staged.Version != "v0.10.0" || len(staged.Binaries) != 2 {
		t.Fatalf("staged = %+v", staged)
	}
	if staged.Dir != "/state/update/staged" {
		t.Fatalf("staged dir = %q", staged.Dir)
	}

	st, err := h.svc.Status(context.Background(), update.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Staged == nil || st.Staged.Version != "v0.10.0" {
		t.Fatalf("status should show the staged release, got %+v", st.Staged)
	}
}

// The property Download exists to protect: a checksums file whose signature
// does not verify is refused before a single asset is fetched, or trusted
// enough to compare an asset's checksum against.
func TestDownloadRefusesOnInvalidSignature(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos")
	// Tamper with the checksums file after it was signed — the classic
	// "swap the manifest, keep the old signature" attack this gate exists
	// to catch.
	h.source.files["https://example.test/checksums.txt"] = []byte(sha256Line([]byte("a DIFFERENT binary"), "aos_v0.10.0_linux_amd64"))

	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_SIGNATURE_INVALID")
	if len(h.machine.staged) != 0 {
		t.Fatal("nothing should be staged when the signature does not verify")
	}
}

// A release published without a signature is not a network problem, and it
// is not Check's: it used to read "could not reach the release channel …
// check network access", attributed to update.Service.Check.
func TestDownloadOfAnUnsignedReleaseSaysTheSignatureIsMissing(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos")
	delete(h.source.files, "https://example.test/checksums.txt.sig")

	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	e := wantCode(t, err, "UPDATE_SIGNATURE_MISSING")
	if e.CauserName != "update.Service.Download" {
		t.Fatalf("causer = %q", e.CauserName)
	}
}

func TestDownloadNamesWhatIsMissingOrUnreachable(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos")

	delete(h.source.files, "https://example.test/aos_v0.10.0_linux_amd64")
	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_ASSET_UNAVAILABLE")

	delete(h.source.files, "https://example.test/checksums.txt")
	_, err = h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_CHECKSUMS_MISSING")

	h.source.fetchErr = fmt.Errorf("%w: timeout", update.ErrUnreachable)
	_, err = h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	if e := wantCode(t, err, "UPDATE_SOURCE_UNREACHABLE"); e.CauserName != "update.Service.Download" {
		t.Fatalf("causer = %q", e.CauserName)
	}

	h.source.fetchErr = errors.New("answered 500")
	_, err = h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_SOURCE_FAILED")
}

func TestDownloadRefusesOnChecksumMismatch(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos")
	// The source now serves different bytes than what was signed for —
	// a corrupted or substituted download, not a manifest attack.
	h.source.files["https://example.test/aos_v0.10.0_linux_amd64"] = []byte("something else entirely")

	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_CHECKSUM_MISMATCH")
	if len(h.machine.staged) != 0 {
		t.Fatal("nothing should be staged on a checksum mismatch")
	}
}

func TestDownloadRefusesWhenNoAssetForThisPlatform(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos")
	release.Assets[0].Platform = "windows/amd64" // this harness is "linux/amd64"

	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_NO_ASSET_FOR_PLATFORM")
}

// The manifest is not signed. What it says about an asset — which binary,
// which platform, which version — is checked against the signed file name,
// and a binary name that is not one of the three is refused outright: it is
// a file name on disk.
func TestDownloadRefusesAManifestThatDisagreesWithTheSignedNames(t *testing.T) {
	cases := map[string]func(r *update.Release){
		"a traversal name":    func(r *update.Release) { r.Assets[0].Binary = "../planted" },
		"another binary":      func(r *update.Release) { r.Assets[0].Binary = "aosd" },
		"another version":     func(r *update.Release) { r.Version = "v0.11.0" },
		"an unsigned file":    func(r *update.Release) { r.Assets[0].Filename = "aos_v0.10.0_linux_amd64.extra" },
		"a malformed name":    func(r *update.Release) { r.Assets[0].Filename = "aos-linux" },
		"a duplicated binary": func(r *update.Release) { r.Assets = append(r.Assets, r.Assets[0]) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			release := h.signedRelease(t, "v0.10.0", "aos")
			if name == "a malformed name" {
				h.sign(t, sha256Line([]byte("x"), "aos-linux"))
			}
			mutate(release)

			_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
			wantCode(t, err, "UPDATE_ASSET_REJECTED")
			if len(h.machine.staged) != 0 {
				t.Fatal("nothing may be staged from a manifest that disagrees with its signature")
			}
		})
	}
}

// An update replaces what an installer put here, and adds nothing: a
// macOS bundle carries aosd and no aos, and an update that created one
// inside it broke the bundle's seal.
func TestDownloadStagesOnlyBinariesInstalledHere(t *testing.T) {
	h := newHarness(t, installed("aosd"))
	release := h.signedRelease(t, "v0.10.0", "aos", "aosd", "aos-desktop")

	staged := h.download(t, release)
	if len(staged.Binaries) != 1 || staged.Binaries["aosd"] == "" {
		t.Fatalf("only aosd is installed here, staged %+v", staged.Binaries)
	}

	none := newHarness(t, installed())
	release = none.signedRelease(t, "v0.10.0", "aos")
	_, err := none.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_NO_ASSET_FOR_PLATFORM")
}

// A release that leaves out one of the binaries installed here would update
// the rest and leave that one behind: a new aos beside an old aosd, or a new
// daemon under an old window. It is refused whole, naming what is missing —
// whether the manifest omits the binary or lists it for another platform.
func TestDownloadRefusesAReleaseThatLeavesAnInstalledBinaryBehind(t *testing.T) {
	cases := map[string]func(r *update.Release){
		"not listed":             func(r *update.Release) { r.Assets = r.Assets[:1] },
		"listed for another one": func(r *update.Release) { r.Assets[1].Platform = "windows/amd64" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, installed("aos", "aosd"))
			release := h.signedRelease(t, "v0.10.0", "aos", "aosd")
			mutate(release)

			_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
			e := wantCode(t, err, "UPDATE_NO_ASSET_FOR_PLATFORM")
			if !strings.Contains(e.Message, "aosd") {
				t.Fatalf("the refusal should name the binary left behind, got %q", e.Message)
			}
			if len(h.machine.staged) != 0 {
				t.Fatalf("nothing may be staged from an incomplete release, staged %v", h.machine.staged)
			}
		})
	}
}

func TestDownloadRefusesWhatIsNotAnUpdate(t *testing.T) {
	h := newHarness(t, running("v0.10.0"))
	release := h.signedRelease(t, "v0.10.0", "aos")
	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_NOT_NEWER")

	dev := newHarness(t, running("dev"))
	release = dev.signedRelease(t, "v0.10.0", "aos")
	_, err = dev.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_DEVELOPER_BUILD")

	_, err = h.svc.Download(context.Background(), update.DownloadInput{})
	wantCode(t, err, "UPDATE_NOTHING_STAGED")
}

func TestDownloadRefusesABundle(t *testing.T) {
	h := newHarness(t)
	h.machine.reinstall = update.ReinstallBundle
	release := h.signedRelease(t, "v0.10.0", "aosd")

	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_REINSTALL_REQUIRED")
}

func TestDownloadAndApplyAreForOperatorsOnly(t *testing.T) {
	h := newHarness(t, withOperators(fakeOperators{deny: true}))
	release := h.signedRelease(t, "v0.10.0", "aos")

	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_FORBIDDEN")
	_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_FORBIDDEN")

	agent := newHarness(t, withOperators(fakeOperators{err: update.ErrNotAPerson}))
	_, err = agent.svc.Download(context.Background(), update.DownloadInput{Release: release})
	e := wantCode(t, err, "UPDATE_NOT_FOR_AGENTS")
	if e.HTTPStatus != 403 || !strings.Contains(e.Message, "agent or an MCP client") {
		t.Fatalf("an agent's refusal should say whose decision it is, got %d %q", e.HTTPStatus, e.Message)
	}

	broken := newHarness(t, withOperators(fakeOperators{err: errors.New("no accounts file")}))
	_, err = broken.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_AUTHORIZATION_FAILED")

	// A service wired with no answer to "who may install" refuses everyone.
	pub, _, _ := relsig.GenerateKey()
	unguarded := update.NewService(update.Deps{
		Source: &fakeSource{}, Stager: newFakeMachine(), Installer: newFakeMachine(),
		Supervisor: &fakeSupervisor{}, ActiveWork: &fakeActiveWork{},
		Clock: &steppingClock{}, Sleeper: &steppingClock{}, PublicKey: pub, Version: "v0.9.0",
	})
	_, err = unguarded.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_FORBIDDEN")
}

func TestDownloadThatCannotRecordWhatItStagedKeepsNothing(t *testing.T) {
	h := newHarness(t, withStore(failingStore{}))
	release := h.signedRelease(t, "v0.10.0", "aos")

	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_STAGE_FAILED")
	if len(h.machine.staged) != 0 {
		t.Fatal("staged files with no record of them must not be left behind")
	}
}

// --- Apply ------------------------------------------------------------------

// Apply takes a version, and nothing about paths: a caller used to send
// staged.binaries and have any file renamed over any name.
func TestApplyInstallsOnlyWhatDownloadStagedUnderThatVersion(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_NOTHING_STAGED")

	h.download(t, h.signedRelease(t, "v0.10.0", "aos"))
	_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.11.0"})
	wantCode(t, err, "UPDATE_NOTHING_STAGED")
	if h.supervisor.restarts != 0 || h.machine.liveAt("aos") != "old aos" {
		t.Fatal("nothing may be touched for a version that was not staged")
	}
}

func TestApplySucceedsSwapsTheBinaryInAndCleansUp(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aos", "aosd"))

	out, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	if err != nil {
		t.Fatal(err)
	}
	if out.RolledBack || out.Version != "v0.10.0" {
		t.Fatalf("out = %+v", out)
	}
	if got := h.machine.liveAt("aos"); got != "new aos v0.10.0" {
		t.Fatalf("the live binary was not swapped: %q", got)
	}
	if h.supervisor.restarts != 1 {
		t.Fatalf("expected exactly one restart, got %d", h.supervisor.restarts)
	}
	// The previous binaries are dropped once the new ones are proven, and
	// the staged copies with them: a .prev left beside every binary forever
	// is litter, and inside a bundle it is a broken seal.
	if len(h.machine.prev) != 0 || len(h.machine.commits) != 2 {
		t.Fatalf("backups left behind: %v (commits %v)", h.machine.prev, h.machine.commits)
	}
	if len(h.machine.staged) != 0 {
		t.Fatal("the staged copies should be discarded after a successful apply")
	}
	st, _ := h.svc.Status(context.Background(), update.StatusInput{})
	if st.Staged != nil {
		t.Fatalf("nothing should remain staged, got %+v", st.Staged)
	}
}

// The daemon cannot restart itself, and Apply used to learn that only after
// swapping: then it reported "rollback ALSO failed — the daemon may be down"
// from a daemon that was answering, with the staged file consumed.
func TestApplyThatCannotRestartRefusesBeforeTouchingAnything(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))
	h.supervisor.self = true

	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	e := wantCode(t, err, "UPDATE_RESTART_UNAVAILABLE")
	if len(e.Actions) == 0 || !strings.Contains(e.Actions[0].Command, "update apply --version v0.10.0") {
		t.Fatalf("the refusal should name the command that works, got %+v", e.Actions)
	}
	if h.machine.liveAt("aosd") != "old aosd" || h.supervisor.restarts != 0 {
		t.Fatal("nothing may be swapped or restarted when a restart is impossible")
	}
	if _, ok := h.machine.staged["aosd"]; !ok {
		t.Fatal("the staged release must survive the refusal, so it can be installed")
	}
}

// A daemon started by hand — `aosd serve`, a service manager — is not one a
// restart stops. The install used to go ahead beside it: the previous release
// went on answering, the install was reported done and the backup deleted.
// It refuses before touching anything now, and says what to stop.
func TestApplyRefusesADaemonItsSupervisorDidNotStart(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aos", "aosd"))
	h.supervisor.unsupervised = true

	out, err := h.svc.Check(context.Background(), update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Install == nil || out.Install.Method != update.InstallFromTerminal || !out.Install.Unsupervised || out.Install.Command == "" {
		t.Fatalf("beside a daemon started by hand, the offer says to stop it first: %+v", out.Install)
	}

	_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	e := wantCode(t, err, "UPDATE_DAEMON_NOT_SUPERVISED")
	if !strings.Contains(e.Message, "127.0.0.1:5326") || !strings.Contains(e.Message, "process 777") {
		t.Fatalf("the refusal should name the daemon in the way, got %q", e.Message)
	}
	if len(e.Actions) != 2 || !strings.Contains(e.Actions[0].Label, "stop it where it was started") ||
		!strings.Contains(e.Actions[1].Command, "update apply --version v0.10.0") {
		t.Fatalf("the refusal should say what to stop and what to run then, got %+v", e.Actions)
	}
	if h.machine.liveAt("aosd") != "old aosd" || h.supervisor.restarts != 0 || h.machine.backups() != 0 {
		t.Fatal("nothing may be swapped or restarted beside a daemon the supervisor did not start")
	}
	if _, ok := h.machine.staged["aosd"]; !ok {
		t.Fatal("the staged release must survive the refusal")
	}
}

// Inside a daemon started by hand, the terminal command is not enough on its
// own either, and the refusal and the offer both say so.
func TestTheDaemonStartedByHandSaysToStopItBeforeTheTerminalInstall(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))
	h.supervisor.self, h.supervisor.unsupervised = true, true

	st, err := h.svc.Status(context.Background(), update.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Install.Method != update.InstallFromTerminal || !st.Install.Unsupervised {
		t.Fatalf("install = %+v", st.Install)
	}
	_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	e := wantCode(t, err, "UPDATE_RESTART_UNAVAILABLE")
	if len(e.Actions) < 2 || !strings.Contains(e.Actions[0].Label, "stop it where it was started") ||
		!strings.Contains(e.Actions[1].Command, "update apply --version v0.10.0") {
		t.Fatalf("calls to action = %+v", e.Actions)
	}

	// Started by the supervisor, the command alone installs it.
	h.supervisor.unsupervised = false
	st, _ = h.svc.Status(context.Background(), update.StatusInput{})
	if st.Install.Unsupervised {
		t.Fatalf("a daemon the supervisor started is not unsupervised: %+v", st.Install)
	}
}

// The supervisor's daemon is running and not answering: whether a restart
// would bring the new version up cannot be seen from there.
func TestApplyRefusesASupervisedDaemonThatDoesNotAnswer(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))
	h.supervisor.silent = true

	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	e := wantCode(t, err, "UPDATE_DAEMON_NOT_ANSWERING")
	if len(e.Actions) == 0 || e.Actions[0].Command != "aos gateway restart" {
		t.Fatalf("calls to action = %+v", e.Actions)
	}
	if h.machine.liveAt("aosd") != "old aosd" || h.supervisor.restarts != 0 {
		t.Fatal("nothing may be swapped or restarted")
	}

	h.supervisor.silent, h.supervisor.observeErr = false, errors.New("gateway.json unreadable")
	_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_DAEMON_UNKNOWN")
	if h.machine.liveAt("aosd") != "old aosd" || h.supervisor.restarts != 0 {
		t.Fatal("nothing may be swapped or restarted")
	}
}

// With no daemon running at all there is nothing to restart: the install
// starts the new version, and sees it answer as itself.
func TestApplyWithNoDaemonRunningStartsTheNewVersion(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))
	h.supervisor.idle = true

	st, _ := h.svc.Status(context.Background(), update.StatusInput{})
	if st.Install.Method != update.InstallHere {
		t.Fatalf("install = %+v", st.Install)
	}
	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"}); err != nil {
		t.Fatal(err)
	}
	if h.machine.liveAt("aosd") != "new aosd v0.10.0" || h.supervisor.restarts != 1 || h.machine.backups() != 0 {
		t.Fatalf("live %q, restarts %d, backups %d", h.machine.liveAt("aosd"), h.supervisor.restarts, h.machine.backups())
	}
}

// Minutes can pass waiting for turns to finish, and the daemon looked at
// before them is not necessarily the one there after.
func TestApplyLooksAtTheDaemonAgainAfterWaitingForWork(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))
	h.activeWork.counts = []int{1, 0}
	h.activeWork.during = func() {
		h.supervisor.mu.Lock()
		h.supervisor.unsupervised = true
		h.supervisor.mu.Unlock()
	}

	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_DAEMON_NOT_SUPERVISED")
	if h.machine.liveAt("aosd") != "old aosd" || h.supervisor.restarts != 0 {
		t.Fatal("nothing may be swapped or restarted")
	}
}

// The daemon came back up, as the previous release: the supervisor starts it
// from another copy of the binary than the one the install replaced. That is
// not an install, and the backups are what bring the previous state back.
func TestApplyIsDoneOnlyWhenTheNewVersionAnswers(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aos", "aosd"))
	h.supervisor.elsewhere = true

	out, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	e := wantCode(t, err, "UPDATE_ROLLED_BACK")
	if !out.RolledBack || out.Version != "" {
		t.Fatalf("out = %+v", out)
	}
	if !strings.Contains(e.Message, "is v0.9.0, not v0.10.0") {
		t.Fatalf("the refusal should say which version came back up, got %q", e.Message)
	}
	if len(e.Actions) == 0 || !strings.Contains(e.Actions[0].Label, "AOS_DAEMON_PATH") {
		t.Fatalf("the refusal should point at where the daemon is started from, got %+v", e.Actions)
	}
	if h.machine.liveAt("aosd") != "old aosd" || h.machine.liveAt("aos") != "old aos" || len(h.machine.commits) != 0 {
		t.Fatalf("the previous binaries should be back, and no backup dropped: live %v, commits %v", h.machine.live, h.machine.commits)
	}
	if _, ok := h.machine.staged["aosd"]; !ok {
		t.Fatal("a rolled-back release should stay staged")
	}
}

func TestApplyRefusesABundle(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))
	h.machine.reinstall = update.ReinstallBundle

	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_REINSTALL_REQUIRED")
}

// What is on disk between Download and Apply is proven again. A staged file
// that changed, or a record whose checksums no longer carry the signature,
// is discarded rather than installed.
func TestApplyReverifiesTheStagedFiles(t *testing.T) {
	t.Run("a changed file", func(t *testing.T) {
		h := newHarness(t)
		h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))
		h.machine.staged["aosd"] = "unsigned payload"

		_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
		wantCode(t, err, "UPDATE_STAGED_TAMPERED")
		if h.machine.liveAt("aosd") != "old aosd" {
			t.Fatal("a tampered file must not be swapped in")
		}
		if h.machine.discarded == 0 {
			t.Fatal("a tampered staged release should be discarded")
		}
	})

	t.Run("a rewritten record", func(t *testing.T) {
		store := &sharedStore{}
		h := newHarness(t, withStore(store))
		h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))
		store.record.Staged.Checksums = sha256Line([]byte("unsigned payload"), "aosd_v0.10.0_linux_amd64")
		h.machine.staged["aosd"] = "unsigned payload"

		_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
		wantCode(t, err, "UPDATE_STAGED_TAMPERED")
		if h.machine.liveAt("aosd") != "old aosd" {
			t.Fatal("a file proven only by a rewritten record must not be swapped in")
		}
	})

	t.Run("a record naming another file", func(t *testing.T) {
		store := &sharedStore{}
		h := newHarness(t, withStore(store))
		h.download(t, h.signedRelease(t, "v0.10.0", "aos", "aosd"))
		store.record.Staged.Files["aosd"] = "aos_v0.10.0_linux_amd64"

		_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
		wantCode(t, err, "UPDATE_STAGED_TAMPERED")
	})
}

// What is installed can change between Download and Apply — the window
// installed beside the daemon, say. A staged release that no longer covers
// every installed binary is not installed.
func TestApplyRefusesAStagedReleaseThatNoLongerCoversWhatIsInstalled(t *testing.T) {
	h := newHarness(t, installed("aosd"))
	h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))
	h.machine.mu.Lock()
	h.machine.live["aos-desktop"] = "old aos-desktop"
	h.machine.mu.Unlock()

	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	e := wantCode(t, err, "UPDATE_STAGED_INCOMPLETE")
	if !strings.Contains(e.Message, "aos-desktop") {
		t.Fatalf("the refusal should name the binary left behind, got %q", e.Message)
	}
	if h.machine.liveAt("aosd") != "old aosd" || h.supervisor.restarts != 0 {
		t.Fatal("nothing may be swapped from an incomplete staged release")
	}
	if h.machine.discarded == 0 {
		t.Fatal("an incomplete staged release should be discarded, so the next download stages all of it")
	}
}

// The staged files are verified before Apply waits for work in flight, and
// the wait can be minutes. The digest travels with the swap, so a file that
// changes in between is refused by the Installer instead of installed.
func TestApplyRefusesAStagedFileThatChangesWhileItWaits(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aos", "aosd"))
	h.activeWork.counts = []int{1, 0}
	h.activeWork.during = func() {
		h.machine.mu.Lock()
		h.machine.staged["aosd"] = "swapped after it was verified"
		h.machine.mu.Unlock()
	}

	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_STAGED_TAMPERED")
	if h.machine.liveAt("aos") != "old aos" || h.machine.liveAt("aosd") != "old aosd" {
		t.Fatalf("every swap should be undone: aos=%q aosd=%q", h.machine.liveAt("aos"), h.machine.liveAt("aosd"))
	}
	if h.supervisor.restarts != 0 {
		t.Fatal("nothing should restart onto a refused swap")
	}
	if h.machine.discarded == 0 {
		t.Fatal("a staged release that changed should be discarded")
	}
}

// An installation whose binaries cannot even be listed cannot be kept on one
// version, so neither step guesses.
func TestAnInstallationThatCannotBeReadIsNotUpdated(t *testing.T) {
	h := newHarness(t)
	release := h.signedRelease(t, "v0.10.0", "aos")
	h.machine.targetErr = errors.New("permission denied")
	_, err := h.svc.Download(context.Background(), update.DownloadInput{Release: release})
	wantCode(t, err, "UPDATE_INSTALLATION_UNREADABLE")

	h.machine.targetErr = nil
	h.download(t, release)
	h.machine.targetErr = errors.New("permission denied")
	_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_INSTALLATION_UNREADABLE")
	if _, ok := h.machine.staged["aos"]; !ok {
		t.Fatal("a verified release should stay staged when only the installation could not be read")
	}
}

func TestApplyRefusesWhatIsNoLongerAnUpdate(t *testing.T) {
	store := &sharedStore{}
	h := newHarness(t, withStore(store))
	h.download(t, h.signedRelease(t, "v0.10.0", "aosd"))

	reinstalled := newHarness(t, withStore(store), running("v0.10.0"))
	_, err := reinstalled.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_NOT_NEWER")
}

// This is the property Apply's whole wait exists to protect: it must not
// restart the daemon while a turn most agents would consider a normal-length
// task is still running.
func TestApplyWaitsForActiveWorkToDrain(t *testing.T) {
	h := newHarness(t)
	h.activeWork.counts = []int{3, 2, 1, 0} // drains on the fourth read
	h.download(t, h.signedRelease(t, "v0.10.0", "aos"))

	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"}); err != nil {
		t.Fatal(err)
	}
	if h.activeWork.calls < 4 {
		t.Fatalf("expected Apply to poll until the count reached zero, got %d reads", h.activeWork.calls)
	}
}

func TestApplyProceedsWhenActiveWorkCannotBeRead(t *testing.T) {
	h := newHarness(t)
	h.activeWork.err = errors.New("queue closed")
	h.download(t, h.signedRelease(t, "v0.10.0", "aos"))

	if _, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"}); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRefusesWhenActiveWorkNeverDrains(t *testing.T) {
	h := newHarness(t)
	h.activeWork.counts = []int{5} // never reaches zero — repeats forever
	h.download(t, h.signedRelease(t, "v0.10.0", "aos"))

	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_ACTIVE_WORK_TIMEOUT")
	if h.supervisor.restarts != 0 {
		t.Fatal("should never have restarted the daemon while work was still active")
	}
}

// The single most important property in this whole package: a new version
// that does not become healthy is rolled back, and the daemon is left
// running again — not left down. The staged copy survives, so the same
// release can be tried again without downloading it twice.
func TestApplyRollsBackOnUnhealthyRestartAndTheDaemonComesBackUp(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aos"))
	h.supervisor.newVersionSick = true

	out, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_ROLLED_BACK")
	if !out.RolledBack {
		t.Fatal("expected RolledBack to be reported")
	}
	if got := h.machine.liveAt("aos"); got != "old aos" {
		t.Fatalf("expected the previous binary restored, got %q", got)
	}
	if h.supervisor.restarts != 2 {
		t.Fatalf("expected a restart for the new version and a second restart after rollback, got %d", h.supervisor.restarts)
	}
	if _, ok := h.machine.staged["aos"]; !ok {
		t.Fatal("a rolled-back release should stay staged for a retry")
	}
}

// The files are back and only the process is down: that is what the error
// says, and the way out is starting the daemon — not "the daemon may be down,
// restart it with the binaries in dist/".
func TestApplyWhoseRollbackRestartFailsSaysTheFilesAreBack(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aos"))
	h.supervisor.restartErrs = []error{errors.New("port busy"), errors.New("port still busy")}

	out, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	e := wantCode(t, err, "UPDATE_RESTART_AFTER_ROLLBACK_FAILED")
	if !out.RolledBack || h.machine.liveAt("aos") != "old aos" {
		t.Fatalf("the files should be back: out=%+v live=%q", out, h.machine.liveAt("aos"))
	}
	if len(e.Actions) == 0 || e.Actions[0].Command != "aos gateway start" {
		t.Fatalf("the way out is starting the daemon, got %+v", e.Actions)
	}
}

func TestApplyWhoseRollbackFailsSaysTheBinariesMayBeMixed(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aos"))
	h.supervisor.restartErrs = []error{errors.New("port busy")}
	h.machine.failUndo = "aos"

	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_ROLLBACK_FAILED")
}

func TestApplyRollsBackWhenASecondBinarysSwapFails(t *testing.T) {
	h := newHarness(t)
	h.download(t, h.signedRelease(t, "v0.10.0", "aos", "aosd"))
	h.machine.failSwap = "aosd" // swapped in name order: aos first, then aosd

	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_APPLY_FAILED")
	if got := h.machine.liveAt("aos"); got != "old aos" {
		t.Fatalf("the successfully-swapped binary should have been rolled back too, got %q", got)
	}
	if h.supervisor.restarts != 0 {
		t.Fatal("should never have restarted the daemon on a swap failure")
	}

	h.machine.failUndo = "aos"
	_, err = h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_ROLLBACK_FAILED")
}

func TestApplyWithAnUnreadableRecordRefuses(t *testing.T) {
	h := newHarness(t, withStore(failingStore{}))
	_, err := h.svc.Apply(context.Background(), update.ApplyInput{Version: "v0.10.0"})
	wantCode(t, err, "UPDATE_STATE_UNREADABLE")

	// Status degrades rather than failing: it is read on every visit to the
	// screen, and the record is bookkeeping.
	st, err := h.svc.Status(context.Background(), update.StatusInput{})
	if err != nil || st.Current != "v0.9.0" {
		t.Fatalf("status = %+v, %v", st, err)
	}
	// And a check that cannot be remembered is still answered.
	h.source.release = &update.Release{Version: "v0.10.0"}
	if _, err := h.svc.Check(context.Background(), update.CheckInput{}); err != nil {
		t.Fatal(err)
	}
}

func TestNewServicePanicsWithoutAPublicKey(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected NewService to panic without a PublicKey")
		}
	}()
	update.NewService(update.Deps{
		Source: &fakeSource{}, Stager: newFakeMachine(), Installer: newFakeMachine(),
		Supervisor: &fakeSupervisor{}, ActiveWork: &fakeActiveWork{},
		Clock: &steppingClock{}, Sleeper: &steppingClock{},
	})
}

func TestIsBinaryIsTheWholeList(t *testing.T) {
	for _, name := range []string{"aos", "aosd", "aos-desktop"} {
		if !update.IsBinary(name) {
			t.Errorf("%s should be a managed binary", name)
		}
	}
	for _, name := range []string{"", "../aosd", "aosd.exe", "sh", "aos/aosd"} {
		if update.IsBinary(name) {
			t.Errorf("%q must not be a managed binary", name)
		}
	}
}

// sharedStore is a Store two services can read, the way two daemon
// processes share one state directory — and one a test can reach into.
type sharedStore struct {
	mu     sync.Mutex
	record update.Record
}

func (s *sharedStore) Load(context.Context) (update.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.record, nil
}

func (s *sharedStore) Save(_ context.Context, r update.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record = r
	return nil
}

func (s *sharedStore) Update(_ context.Context, change func(*update.Record) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.record
	if err := change(&r); err != nil {
		return err
	}
	s.record = r
	return nil
}

func sha256Line(data []byte, filename string) string {
	return sha256Hex(data) + "  " + filename + "\n"
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// racingStore is a sharedStore whose first Update is overtaken:
// between reading the record and writing it back, overtake runs — another
// call on the same installation, from another workspace's service or a
// terminal. A store that keeps read-modify-writes apart makes overtake wait
// until the first one is written.
type racingStore struct {
	sharedStore
	writers  sync.Mutex
	raced    atomic.Bool
	overtake func()
}

// race runs overtake on the first read only. Not a sync.Once: Once holds
// every other caller until the first returns, which would keep the overtaking
// call out of the store exactly as a correct store would.
func (s *racingStore) race() {
	if s.overtake != nil && s.raced.CompareAndSwap(false, true) {
		s.overtake()
	}
}

func (s *racingStore) Update(ctx context.Context, change func(*update.Record) error) error {
	s.writers.Lock()
	defer s.writers.Unlock()
	r, _ := s.Load(ctx)
	s.race()
	if err := change(&r); err != nil {
		return err
	}
	return s.Save(ctx, r)
}
