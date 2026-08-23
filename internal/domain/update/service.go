package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
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

type service struct {
	source     ReleaseSource
	installer  Installer
	supervisor DaemonSupervisor
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

	activeWorkGrace time.Duration
	healthTimeout   time.Duration
}

// Deps is what the service is built from.
type Deps struct {
	Source     ReleaseSource
	Installer  Installer
	Supervisor DaemonSupervisor
	ActiveWork ActiveWork
	Clock      Clock
	Sleeper    Sleeper
	Log        *slog.Logger

	// PublicKey is required: a service built without one refuses every
	// Download rather than silently skipping verification. See NewService.
	PublicKey string

	Platform string

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
		source: d.Source, installer: d.Installer, supervisor: d.Supervisor,
		activeWork: d.ActiveWork, clock: d.Clock, sleeper: d.Sleeper, log: d.Log,
		publicKey: d.PublicKey, platform: d.Platform,
		activeWorkGrace: d.ActiveWorkGrace, healthTimeout: d.HealthTimeout,
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.platform == "" {
		s.platform = runtime.GOOS + "/" + runtime.GOARCH
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
	current := build.Current().Version

	release, err := s.source.Latest(ctx, channel)
	if err != nil {
		return CheckOutput{}, errReleaseSourceUnavailable(err)
	}
	if release == nil || release.Version == current {
		return CheckOutput{UpToDate: true, Current: current, Channel: channel}, nil
	}
	return CheckOutput{
		UpToDate: false,
		Current:  current,
		Channel:  channel,
		Release:  release,
	}, nil
}

func (s *service) Status(ctx context.Context, in StatusInput) (Status, error) {
	return Status{Current: build.Current().Version, Channel: ChannelStable}, nil
}

// Download fetches every asset this platform needs, verifies the checksums
// file's signature and every asset's own checksum, and stages the result.
// Nothing is left staged on any failure — a partially-verified release is
// exactly as unsafe to apply as an unverified one.
func (s *service) Download(ctx context.Context, in DownloadInput) (DownloadOutput, error) {
	release := in.Release
	if release == nil {
		return DownloadOutput{}, errNothingStaged()
	}

	checksumsRaw, err := s.source.Fetch(ctx, release.ChecksumsURL)
	if err != nil {
		return DownloadOutput{}, errReleaseSourceUnavailable(err)
	}
	sigRaw, err := s.source.Fetch(ctx, release.SignatureURL)
	if err != nil {
		return DownloadOutput{}, errReleaseSourceUnavailable(err)
	}
	if err := relsig.Verify(s.publicKey, checksumsRaw, string(sigRaw)); err != nil {
		return DownloadOutput{}, errSignatureInvalid(err)
	}
	checksums := parseChecksums(string(checksumsRaw))

	assets := assetsForPlatform(release.Assets, s.platform)
	if len(assets) == 0 {
		return DownloadOutput{}, errNoAssetForPlatform("any", s.platform)
	}

	staged := Staged{Version: release.Version, Binaries: map[string]string{}}
	for _, asset := range assets {
		data, err := s.source.Fetch(ctx, asset.URL)
		if err != nil {
			return DownloadOutput{}, errReleaseSourceUnavailable(err)
		}
		want, known := checksums[asset.Filename]
		if !known || !strings.EqualFold(want, sha256Hex(data)) {
			return DownloadOutput{}, errChecksumMismatch(asset.Binary)
		}
		path, err := s.installer.Stage(ctx, asset.Binary, data)
		if err != nil {
			return DownloadOutput{}, errApplyFailed(err)
		}
		staged.Binaries[asset.Binary] = path
	}

	s.log.Info("staged a verified release", "version", staged.Version, "binaries", len(staged.Binaries))
	return DownloadOutput{Staged: staged}, nil
}

// Apply swaps in a Staged release at a point where nothing is lost:
//  1. wait for in-flight turns to finish, bounded by ActiveWorkGrace
//  2. swap each binary, keeping the previous one as a rollback target
//  3. restart the daemon
//  4. verify health within HealthTimeout; on failure, roll every swap back
//     and restart again on the previous binaries
func (s *service) Apply(ctx context.Context, in ApplyInput) (ApplyOutput, error) {
	if len(in.Staged.Binaries) == 0 {
		return ApplyOutput{}, errNothingStaged()
	}

	if err := s.waitForActiveWork(ctx); err != nil {
		return ApplyOutput{}, err
	}

	swapped := make([]string, 0, len(in.Staged.Binaries))
	for binary, stagedPath := range in.Staged.Binaries {
		target, err := s.installer.PathOf(ctx, binary)
		if err != nil {
			s.rollbackAll(ctx, swapped)
			return ApplyOutput{}, errApplyFailed(err)
		}
		if err := s.installer.SwapIn(ctx, stagedPath, target); err != nil {
			s.rollbackAll(ctx, swapped)
			return ApplyOutput{}, errApplyFailed(err)
		}
		swapped = append(swapped, target)
	}

	if err := s.supervisor.Restart(ctx); err != nil {
		return s.rollbackAndRestart(ctx, swapped, err)
	}
	if err := s.waitHealthy(ctx); err != nil {
		return s.rollbackAndRestart(ctx, swapped, err)
	}

	s.log.Info("applied a release", "version", in.Staged.Version)
	return ApplyOutput{Version: in.Staged.Version, RolledBack: false}, nil
}

func (s *service) rollbackAll(ctx context.Context, targets []string) {
	for _, target := range targets {
		if err := s.installer.Rollback(ctx, target); err != nil {
			s.log.Error("rollback failed for one binary", "target", target, "err", err)
		}
	}
}

func (s *service) rollbackAndRestart(ctx context.Context, targets []string, cause error) (ApplyOutput, error) {
	s.rollbackAll(ctx, targets)
	if err := s.supervisor.Restart(ctx); err != nil {
		return ApplyOutput{}, errRollbackFailed(fmt.Errorf("health check failed (%w), then restart-on-rollback also failed: %w", cause, err))
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

// assetsForPlatform narrows a release's assets to this machine's own
// GOOS/GOARCH — the platform field an asset is published under, not a path
// this function parses out of anything.
func assetsForPlatform(assets []Asset, platform string) []Asset {
	out := make([]Asset, 0, len(assets))
	for _, a := range assets {
		if a.Platform == platform {
			out = append(out, a)
		}
	}
	return out
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
