package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/relsig"
)

// Timings of Apply's two bounded waits. Generous for the same reason
// gateway.Service's own timings are: reporting a turn as stuck while it is
// still finishing costs more than waiting for it to finish.
const (
	DefaultActiveWorkGrace = 5 * time.Minute
	DefaultHealthTimeout   = 30 * time.Second
	pollInterval           = 200 * time.Millisecond
)

// daemonBinary is the binary a terminal runs to install a staged release —
// see InstallFromTerminal.
const daemonBinary = "aosd"

type service struct {
	source     ReleaseSource
	stager     Stager
	installer  Installer
	store      Store
	supervisor DaemonSupervisor
	operators  Operators
	activeWork ActiveWork
	clock      Clock
	sleeper    Sleeper
	log        *slog.Logger

	// publicKey is the base64 Ed25519 public key (relsig.GenerateKey's own
	// format) every checksums file's signature is checked against —
	// go:embed'd from release-pubkey.pub by whoever constructs this
	// service (internal/app), not read here: this package has no opinion
	// on where the key file lives, only that one is required.
	publicKey string

	// platform is runtime.GOOS+"/"+runtime.GOARCH by default; overridable
	// so a test can exercise "this machine is windows/amd64" without
	// actually being one.
	platform string

	// version is this installation's own version, build.Current().Version by
	// default; overridable because every test binary is "dev".
	version string

	activeWorkGrace time.Duration
	healthTimeout   time.Duration
}

// Deps is what the service is built from.
type Deps struct {
	Source     ReleaseSource
	Stager     Stager
	Installer  Installer
	Store      Store // nil keeps the record in memory, for this process only
	Supervisor DaemonSupervisor
	// Operators is who may Download and Apply. Nil lets nobody: an update
	// service wired without an answer to that question refuses rather than
	// letting every account replace the binaries.
	Operators  Operators
	ActiveWork ActiveWork
	Clock      Clock
	Sleeper    Sleeper
	Log        *slog.Logger

	// PublicKey is required: a service built without one refuses every
	// Download rather than silently skipping verification. See NewService.
	PublicKey string

	Platform string
	Version  string

	ActiveWorkGrace time.Duration
	HealthTimeout   time.Duration
}

// NewService wires the update service over its ports. Panics on a missing
// PublicKey — the same "fail at construction, not at the moment it would
// have mattered" stance apperr.MustRegister and command.MustRegister take
// elsewhere in this tree: an update service with no key to verify against
// is not a degraded update service, it is a way to install anything.
func NewService(d Deps) Service {
	if strings.TrimSpace(d.PublicKey) == "" {
		panic("update.NewService: PublicKey is required — an update service cannot verify releases without one")
	}
	s := &service{
		source: d.Source, stager: d.Stager, installer: d.Installer, store: d.Store,
		supervisor: d.Supervisor, operators: d.Operators,
		activeWork: d.ActiveWork, clock: d.Clock, sleeper: d.Sleeper, log: d.Log,
		publicKey: d.PublicKey, platform: d.Platform, version: d.Version,
		activeWorkGrace: d.ActiveWorkGrace, healthTimeout: d.HealthTimeout,
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.store == nil {
		s.store = &memoryStore{}
	}
	if s.platform == "" {
		s.platform = runtime.GOOS + "/" + runtime.GOARCH
	}
	if s.version == "" {
		s.version = build.Current().Version
	}
	if s.activeWorkGrace == 0 {
		s.activeWorkGrace = DefaultActiveWorkGrace
	}
	if s.healthTimeout == 0 {
		s.healthTimeout = DefaultHealthTimeout
	}
	return s
}

func (s *service) channelOf(in CheckInput) Channel {
	if in.Channel == "" {
		return ChannelStable
	}
	return in.Channel
}

// Check queries the channel and never downloads anything.
func (s *service) Check(ctx context.Context, in CheckInput) (CheckOutput, error) {
	channel := s.channelOf(in)
	now := s.clock.Now()
	out := CheckOutput{Current: s.version, Channel: channel, CheckedAt: now}

	if !s.source.Configured() {
		out.State = StateNotConfigured
		s.remember(ctx, LastCheck{At: now, Channel: channel, State: out.State})
		return out, nil
	}

	release, err := s.source.Latest(ctx, channel)
	switch {
	case errors.Is(err, ErrNotPublished):
		return CheckOutput{}, errChannelEmpty(channel, err)
	case errors.Is(err, ErrUnreachable):
		return CheckOutput{}, errSourceUnreachable("update.Service.Check", err)
	case err != nil:
		return CheckOutput{}, errSourceFailed("update.Service.Check", err)
	case release == nil:
		return CheckOutput{}, errChannelEmpty(channel, ErrNotPublished)
	}
	latest, ok := build.ParseVersion(release.Version)
	if !ok {
		return CheckOutput{}, errReleaseVersionInvalid(release.Version)
	}

	current, ok := build.ParseVersion(s.version)
	switch {
	case !ok:
		out.State = StateDeveloperBuild
		out.Release = release
	case latest.Compare(current) > 0:
		out.State = StateAvailable
		out.Release = release
		install := s.install(ctx, release.Version)
		out.Install = &install
	default:
		// Equal, or older: a lagging mirror, or a beta ahead of this
		// channel. A release is offered only when it is newer, which is also
		// what keeps a locally packaged v0.15.2-dirty from being offered
		// v0.15.2 forever.
		out.State = StateUpToDate
	}
	out.UpToDate = out.State == StateUpToDate
	s.remember(ctx, LastCheck{At: now, Channel: channel, State: out.State, Latest: release.Version})
	return out, nil
}

// remember records a check's outcome. A record that cannot be written costs
// the "Last checked" line, not the check the caller asked for.
func (s *service) remember(ctx context.Context, check LastCheck) {
	record, err := s.store.Load(ctx)
	if err != nil {
		s.log.Warn("could not read the update record; the check is not remembered", "err", err)
		return
	}
	record.LastCheck = &check
	if err := s.store.Save(ctx, record); err != nil {
		s.log.Warn("could not remember the update check", "err", err)
	}
}

// Status reports without touching the network: the version, whether a feed
// exists, what the last check found, and what is staged.
func (s *service) Status(ctx context.Context, _ StatusInput) (Status, error) {
	st := Status{Current: s.version, Channel: ChannelStable, Configured: s.source.Configured()}
	record, err := s.store.Load(ctx)
	if err != nil {
		s.log.Warn("could not read the update record", "err", err)
		st.Install = s.install(ctx, "")
		return st, nil
	}
	// A check made by the binary this one replaced, or before a feed was
	// configured, can be out of date without anybody checking again: an
	// "available" release this installation has since reached is not
	// available any more.
	if c := record.LastCheck; c != nil && (c.State != StateNotConfigured || !st.Configured) {
		at := c.At
		st.CheckedAt, st.LatestKnown, st.LastState = &at, c.Latest, c.State
		if c.State == StateAvailable && s.newer("update.Service.Status", c.Latest) != nil {
			st.LastState = StateUpToDate
		}
		if c.Channel != "" {
			st.Channel = c.Channel
		}
	}
	staged := ""
	// A staged release this installation has since caught up with — it was
	// reinstalled by hand, say — is nothing to install any more.
	if record.Staged != nil && s.newer("update.Service.Status", record.Staged.Version) == nil {
		st.Staged = record.Staged.view()
		staged = record.Staged.Version
	}
	st.Install = s.install(ctx, staged)
	return st, nil
}

// install says how a release reaches this installation. version is the
// release a terminal command would install, when there is one to name.
func (s *service) install(ctx context.Context, version string) Install {
	if !s.installer.InPlace(ctx) {
		return Install{Method: InstallReinstall}
	}
	if s.supervisor.CanRestart(ctx) {
		return Install{Method: InstallHere}
	}
	out := Install{Method: InstallFromTerminal}
	if version == "" {
		return out
	}
	path, installed, err := s.installer.Target(ctx, daemonBinary)
	if err != nil || !installed {
		path = daemonBinary
	}
	out.Command = shellQuote(path) + " update apply --version " + shellQuote(version)
	return out
}

// newer checks that version is a release this installation may move to:
// parseable, and strictly newer than what runs. The signed asset names carry
// the version, so this is also what refuses an old signed release replayed
// under a fresh manifest.
func (s *service) newer(causer, version string) error {
	current, ok := build.ParseVersion(s.version)
	if !ok {
		return errDeveloperBuild(causer, s.version)
	}
	target, ok := build.ParseVersion(version)
	if !ok || target.Compare(current) <= 0 {
		return errNotNewer(causer, version, s.version)
	}
	return nil
}

func (s *service) authorize(ctx context.Context, causer string) error {
	if s.operators == nil {
		return errForbidden(causer)
	}
	ok, err := s.operators.MayInstall(ctx)
	if err != nil {
		return errAuthorizationFailed(causer, err)
	}
	if !ok {
		return errForbidden(causer)
	}
	return nil
}

// Download fetches every asset this platform needs, verifies the checksums
// file's signature and every asset's own checksum, and stages the result.
// Nothing is left staged on any failure — a partially-verified release is
// exactly as unsafe to apply as an unverified one.
func (s *service) Download(ctx context.Context, in DownloadInput) (DownloadOutput, error) {
	if err := s.authorize(ctx, "update.Service.Download"); err != nil {
		return DownloadOutput{}, err
	}
	release := in.Release
	if release == nil {
		return DownloadOutput{}, errNothingStaged("")
	}
	if err := s.newer("update.Service.Download", release.Version); err != nil {
		return DownloadOutput{}, err
	}
	if !s.installer.InPlace(ctx) {
		return DownloadOutput{}, errReinstallRequired("update.Service.Download")
	}

	checksumsRaw, err := s.source.Fetch(ctx, release.ChecksumsURL)
	if err != nil {
		return DownloadOutput{}, fetchError(err, errChecksumsMissing)
	}
	sigRaw, err := s.source.Fetch(ctx, release.SignatureURL)
	if err != nil {
		return DownloadOutput{}, fetchError(err, errSignatureMissing)
	}
	if err := relsig.Verify(s.publicKey, checksumsRaw, string(sigRaw)); err != nil {
		return DownloadOutput{}, errSignatureInvalid(err)
	}
	checksums := parseChecksums(string(checksumsRaw))

	assets, err := s.plan(ctx, release, checksums)
	if err != nil {
		return DownloadOutput{}, err
	}

	// Whatever an earlier download staged is replaced, not merged: a staged
	// set is one release or nothing.
	if err := s.discard(ctx); err != nil {
		return DownloadOutput{}, errStageFailed(err)
	}
	staged := &StagedRelease{
		Version:   release.Version,
		Checksums: string(checksumsRaw),
		Signature: string(sigRaw),
		Files:     map[string]string{},
		Paths:     map[string]string{},
	}
	for _, asset := range assets {
		if err := s.stageOne(ctx, asset, checksums, staged); err != nil {
			_ = s.stager.Discard(ctx)
			return DownloadOutput{}, err
		}
	}

	record, err := s.store.Load(ctx)
	if err == nil {
		record.Staged = staged
		err = s.store.Save(ctx, record)
	}
	if err != nil {
		_ = s.stager.Discard(ctx)
		return DownloadOutput{}, errStageFailed(err)
	}

	s.log.Info("staged a verified release", "version", staged.Version, "binaries", len(staged.Files))
	return DownloadOutput{Staged: *staged.view()}, nil
}

// plan picks the assets to download: this platform's, for binaries installed
// here, each one's manifest entry agreeing with its signed file name.
func (s *service) plan(ctx context.Context, release *Release, checksums map[string]string) ([]Asset, error) {
	var out []Asset
	seen := map[string]bool{}
	for _, asset := range release.Assets {
		if asset.Platform != s.platform {
			continue
		}
		if !IsBinary(asset.Binary) {
			return nil, errAssetRejected(asset.Filename, fmt.Sprintf("%q is not a binary an update installs", asset.Binary))
		}
		if _, signed := checksums[asset.Filename]; !signed {
			return nil, errAssetRejected(asset.Filename, "the checksums file does not list it")
		}
		name, ok := parseAssetName(asset.Filename)
		switch {
		case !ok:
			return nil, errAssetRejected(asset.Filename, "the file name is not <binary>_<version>_<os>_<arch>")
		case name.binary != asset.Binary:
			return nil, errAssetRejected(asset.Filename, fmt.Sprintf("the manifest calls it %s", asset.Binary))
		case name.platform != asset.Platform:
			return nil, errAssetRejected(asset.Filename, fmt.Sprintf("the manifest says it is for %s", asset.Platform))
		case name.version != release.Version:
			return nil, errAssetRejected(asset.Filename, fmt.Sprintf("the manifest says it is %s", release.Version))
		case seen[asset.Binary]:
			return nil, errAssetRejected(asset.Filename, fmt.Sprintf("the manifest lists %s twice", asset.Binary))
		}
		seen[asset.Binary] = true

		_, installed, err := s.installer.Target(ctx, asset.Binary)
		if err != nil {
			return nil, errStageFailed(err)
		}
		if installed {
			out = append(out, asset)
		}
	}
	if len(out) == 0 {
		return nil, errNoAssetForPlatform("installed binary", s.platform)
	}
	return out, nil
}

func (s *service) stageOne(ctx context.Context, asset Asset, checksums map[string]string, staged *StagedRelease) error {
	data, err := s.source.Fetch(ctx, asset.URL)
	if err != nil {
		return fetchError(err, func(cause error) error { return errAssetUnavailable(asset.Binary, cause) })
	}
	if !strings.EqualFold(checksums[asset.Filename], sha256Hex(data)) {
		return errChecksumMismatch(asset.Binary)
	}
	path, err := s.stager.Stage(ctx, asset.Binary, data)
	if err != nil {
		return errStageFailed(err)
	}
	staged.Files[asset.Binary] = asset.Filename
	staged.Paths[asset.Binary] = path
	staged.Dir = filepath.Dir(path)
	return nil
}

// fetchError tells the three ways a Download fetch fails apart: nothing
// published there (missing names what), no answer at all, or an answer that
// is an error.
func fetchError(err error, missing func(error) error) error {
	switch {
	case errors.Is(err, ErrNotPublished):
		return missing(err)
	case errors.Is(err, ErrUnreachable):
		return errSourceUnreachable("update.Service.Download", err)
	default:
		return errSourceFailed("update.Service.Download", err)
	}
}

// discard drops the staged files and the record of them together.
func (s *service) discard(ctx context.Context) error {
	if err := s.stager.Discard(ctx); err != nil {
		return err
	}
	record, err := s.store.Load(ctx)
	if err != nil {
		return err
	}
	if record.Staged == nil {
		return nil
	}
	record.Staged = nil
	return s.store.Save(ctx, record)
}

// Apply swaps in the staged release at a point where nothing is lost:
//  0. refuse up front what cannot finish: nothing staged under this version,
//     a release not newer than this one, a bundle, a process that cannot
//     restart the daemon — before a single file moves
//  1. prove the staged files against the signed checksums again
//  2. wait for in-flight turns to finish, bounded by ActiveWorkGrace
//  3. swap each binary, keeping the previous one as a rollback target
//  4. restart the daemon, and verify health within HealthTimeout; on
//     failure, roll every swap back and restart again on the previous binaries
func (s *service) Apply(ctx context.Context, in ApplyInput) (ApplyOutput, error) {
	if err := s.authorize(ctx, "update.Service.Apply"); err != nil {
		return ApplyOutput{}, err
	}
	record, err := s.store.Load(ctx)
	if err != nil {
		return ApplyOutput{}, errStateUnreadable("update.Service.Apply", err)
	}
	staged := record.Staged
	if staged == nil || len(staged.Files) == 0 || staged.Version != in.Version {
		return ApplyOutput{}, errNothingStaged(in.Version)
	}
	if err := s.newer("update.Service.Apply", staged.Version); err != nil {
		return ApplyOutput{}, err
	}
	if !s.installer.InPlace(ctx) {
		return ApplyOutput{}, errReinstallRequired("update.Service.Apply")
	}
	if !s.supervisor.CanRestart(ctx) {
		return ApplyOutput{}, errRestartUnavailable(s.install(ctx, staged.Version).Command)
	}
	binaries, err := s.verifyStaged(ctx, staged)
	if err != nil {
		if derr := s.discard(ctx); derr != nil {
			s.log.Error("could not discard a staged release that failed verification", "err", derr)
		}
		return ApplyOutput{}, err
	}

	if err := s.waitForActiveWork(ctx); err != nil {
		return ApplyOutput{}, err
	}

	swapped := make([]string, 0, len(binaries))
	for _, binary := range binaries {
		if err := s.installer.SwapIn(ctx, binary); err != nil {
			if rerr := s.rollbackAll(ctx, swapped); rerr != nil {
				return ApplyOutput{}, errRollbackFailed(err, rerr)
			}
			return ApplyOutput{}, errApplyFailed(err)
		}
		swapped = append(swapped, binary)
	}

	if err := s.supervisor.Restart(ctx); err != nil {
		return s.rollbackAndRestart(ctx, swapped, err)
	}
	if err := s.waitHealthy(ctx); err != nil {
		return s.rollbackAndRestart(ctx, swapped, err)
	}

	for _, binary := range swapped {
		if err := s.installer.Commit(ctx, binary); err != nil {
			s.log.Warn("the update is in place, but the previous binary it replaced could not be removed", "binary", binary, "err", err)
		}
	}
	if err := s.discard(ctx); err != nil {
		s.log.Warn("the update is in place, but its staged copy could not be removed", "err", err)
	}
	s.log.Info("applied a release", "version", staged.Version)
	return ApplyOutput{Version: staged.Version, RolledBack: false}, nil
}

// verifyStaged proves, again, what Download proved: the checksums file's
// signature, every file name's binary, version and platform, and every
// staged file's digest. The staged record and files live on disk between the
// two calls, and nothing that happened to them in between is taken on trust.
func (s *service) verifyStaged(ctx context.Context, staged *StagedRelease) ([]string, error) {
	if err := relsig.Verify(s.publicKey, []byte(staged.Checksums), staged.Signature); err != nil {
		return nil, errStagedTampered("checksums", err)
	}
	checksums := parseChecksums(staged.Checksums)
	binaries := make([]string, 0, len(staged.Files))
	for binary := range staged.Files {
		binaries = append(binaries, binary)
	}
	sort.Strings(binaries)

	for _, binary := range binaries {
		filename := staged.Files[binary]
		name, ok := parseAssetName(filename)
		want, signed := checksums[filename]
		if !IsBinary(binary) || !ok || !signed || name.binary != binary ||
			name.version != staged.Version || name.platform != s.platform {
			return nil, errStagedTampered(binary, fmt.Errorf("%q is not this binary's signed file", filename))
		}
		got, err := s.stager.Digest(ctx, binary)
		if err != nil {
			return nil, errStagedTampered(binary, err)
		}
		if !strings.EqualFold(want, got) {
			return nil, errStagedTampered(binary, errors.New("its digest changed since it was downloaded"))
		}
	}
	return binaries, nil
}

func (s *service) rollbackAll(ctx context.Context, binaries []string) error {
	var failed []error
	for _, binary := range binaries {
		if err := s.installer.Rollback(ctx, binary); err != nil {
			s.log.Error("rollback failed for one binary", "binary", binary, "err", err)
			failed = append(failed, err)
		}
	}
	return errors.Join(failed...)
}

// rollbackAndRestart undoes a swap whose new version did not come up, and
// reports what is actually true afterwards — which of three different states
// the machine is in, rather than the worst one every time.
func (s *service) rollbackAndRestart(ctx context.Context, binaries []string, cause error) (ApplyOutput, error) {
	if err := s.rollbackAll(ctx, binaries); err != nil {
		return ApplyOutput{}, errRollbackFailed(cause, err)
	}
	if err := s.supervisor.Restart(ctx); err != nil {
		return ApplyOutput{RolledBack: true}, errRestartAfterRollback(cause, err)
	}
	if err := s.waitHealthy(ctx); err != nil {
		return ApplyOutput{RolledBack: true}, errRestartAfterRollback(cause, err)
	}
	return ApplyOutput{RolledBack: true}, errRolledBack(cause)
}

func (s *service) waitForActiveWork(ctx context.Context) error {
	deadline := s.clock.Now().Add(s.activeWorkGrace)
	for {
		n, err := s.activeWork.Count(ctx)
		if err != nil {
			// A queue this cannot even ask is a queue that cannot prove
			// anything is still running — proceeding is the same call
			// conversing(t)-style tests already make when the queue itself
			// failed to open (see internal/app/wire.go): degrade, do not
			// block the whole update on it.
			s.log.Warn("could not read active work count; proceeding without the wait", "err", err)
			return nil
		}
		if n == 0 {
			return nil
		}
		if !s.clock.Now().Before(deadline) {
			return errActiveWorkTimeout(s.activeWorkGrace.String())
		}
		if err := s.sleeper.Sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
}

func (s *service) waitHealthy(ctx context.Context) error {
	deadline := s.clock.Now().Add(s.healthTimeout)
	for {
		if s.supervisor.Healthy(ctx) {
			return nil
		}
		if !s.clock.Now().Before(deadline) {
			return fmt.Errorf("daemon did not become healthy within %s", s.healthTimeout)
		}
		if err := s.sleeper.Sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
}

// parseChecksums reads "<hex sha256>  <filename>" lines — sha256sum's own
// output format, which is what a release process actually produces without
// inventing a bespoke one.
func parseChecksums(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// sha256sum prefixes the filename with "*" for binary mode; either
		// way, the filename is whatever follows the hash and any markers.
		out[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}
	return out
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// shellQuote makes a path or version safe to paste into a POSIX shell. The
// command it builds is shown to a person to copy, so it has to survive a
// directory with a space in its name.
func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`!*?[](){}<>|&;#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// memoryStore is the Store a service gets when it is built without one: the
// record lives as long as the process.
type memoryStore struct {
	mu     sync.Mutex
	record Record
}

func (m *memoryStore) Load(context.Context) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.record, nil
}

func (m *memoryStore) Save(_ context.Context, record Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record = record
	return nil
}
