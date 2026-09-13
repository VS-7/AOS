// Command releasefeed turns a directory of release assets into the update
// feed internal/domain/update reads: a signature over checksums.txt, a
// channel manifest, and a check that the two actually install.
//
//	releasefeed sign     -dir release
//	releasefeed manifest -dir release -version v0.16.0 -base-url https://…/download/v0.16.0
//	releasefeed verify   -dir release
//
// It exists because the release pipeline published checksums.txt and nothing
// else: no checksums.txt.sig, which Download requires, and no stable.json,
// which Check requests. Pointing an installation at the real releases
// answered "up to date" (the manifest 404'd) and, with a manifest written by
// hand, refused every download for want of a signature. There was no signer
// at all — relsig.Sign had no caller outside its own tests.
//
// The private key is read from the environment (AOS_RELEASE_SIGNING_KEY by
// default), never from a flag: a flag is in the process listing and in every
// log line that echoes a command.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/adapters/updateinstall"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/relsig"
	"github.com/OWNER/aos/internal/domain/update"
)

const (
	checksumsName = "checksums.txt"
	signatureName = checksumsName + ".sig"
	defaultPubKey = "internal/app/release-pubkey.pub"
	defaultKeyEnv = "AOS_RELEASE_SIGNING_KEY"
)

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "releasefeed:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: releasefeed sign|manifest|verify [flags]")
	}
	fs := flag.NewFlagSet("releasefeed "+args[0], flag.ContinueOnError)
	dir := fs.String("dir", "release", "the directory holding the release assets and checksums.txt")
	pubKey := fs.String("pubkey", defaultPubKey, "the public key installations verify against")
	keyEnv := fs.String("key-env", defaultKeyEnv, "the environment variable holding the base64 private key")
	version := fs.String("version", "", "the release version, exactly as the assets are named")
	baseURL := fs.String("base-url", "", "where this release's assets are downloaded from")
	pageURL := fs.String("page-url", "", "the release's page, for installations that update by reinstalling")
	channel := fs.String("channel", string(update.ChannelStable), "the channel the manifest publishes")
	published := fs.String("published", "", "the publication time, RFC 3339 (default: SOURCE_DATE_EPOCH, else now)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	switch args[0] {
	case "sign":
		return sign(*dir, *pubKey, getenv(*keyEnv), *keyEnv, out)
	case "manifest":
		at, err := publishedAt(*published, getenv("SOURCE_DATE_EPOCH"))
		if err != nil {
			return err
		}
		return manifest(*dir, *version, *baseURL, *pageURL, update.Channel(*channel), at, out)
	case "verify":
		return verify(*dir, *pubKey, update.Channel(*channel), out)
	default:
		return fmt.Errorf("unknown subcommand %q: want sign, manifest or verify", args[0])
	}
}

// sign writes checksums.txt.sig, then checks it against the public key the
// installations carry. A key that signs but does not match is the failure
// worth stopping a release for: every installation would refuse the update
// with UPDATE_SIGNATURE_INVALID, after it was published.
func sign(dir, pubKeyPath, privateKey, keyEnv string, out io.Writer) error {
	if strings.TrimSpace(privateKey) == "" {
		return fmt.Errorf("%s is empty: there is no key to sign the release with", keyEnv)
	}
	checksums, err := os.ReadFile(filepath.Join(dir, checksumsName))
	if err != nil {
		return err
	}
	sig, err := relsig.Sign(privateKey, checksums)
	if err != nil {
		return fmt.Errorf("the key in %s is not a release signing key: %w", keyEnv, err)
	}
	pub, err := readKey(pubKeyPath)
	if err != nil {
		return err
	}
	if err := relsig.Verify(pub, checksums, sig); err != nil {
		return fmt.Errorf("the key in %s does not match %s — installations would refuse this release; rotate the public key or the secret so they are one pair", keyEnv, pubKeyPath)
	}
	if err := os.WriteFile(filepath.Join(dir, signatureName), []byte(sig+"\n"), 0o644); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "signed %s\n", filepath.Join(dir, checksumsName))
	return nil
}

// assetPattern is release.yml's name for a raw binary. Archives, installers
// and bundles are published beside them for people; the updater installs raw
// binaries only, so only those go in the manifest.
var assetPattern = regexp.MustCompile(`^(aos|aosd|aos-desktop)_(.+)_([a-z0-9]+)_([a-z0-9]+?)(\.exe)?$`)

// manifest writes <channel>.json from the assets actually present and
// listed in checksums.txt, so it cannot name a file the checksums do not
// cover — Download would refuse that asset, and the release with it.
func manifest(dir, version, baseURL, pageURL string, channel update.Channel, at time.Time, out io.Writer) error {
	if version == "" || baseURL == "" {
		return errors.New("manifest needs -version and -base-url")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	checksums, err := os.ReadFile(filepath.Join(dir, checksumsName))
	if err != nil {
		return err
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(string(checksums), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 {
			listed[strings.TrimPrefix(fields[1], "*")] = true
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	release := update.Release{
		Version:      version,
		Channel:      channel,
		PublishedAt:  at.UTC(),
		ChecksumsURL: baseURL + "/" + checksumsName,
		SignatureURL: baseURL + "/" + signatureName,
		PageURL:      pageURL,
	}
	for _, e := range entries {
		m := assetPattern.FindStringSubmatch(e.Name())
		if e.IsDir() || m == nil || m[2] != version {
			continue
		}
		if !listed[e.Name()] {
			return fmt.Errorf("%s is not listed in %s", e.Name(), checksumsName)
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		release.Assets = append(release.Assets, update.Asset{
			Binary:   m[1],
			Platform: m[3] + "/" + m[4],
			URL:      baseURL + "/" + e.Name(),
			Size:     info.Size(),
			Filename: e.Name(),
		})
	}
	if len(release.Assets) == 0 {
		return fmt.Errorf("no raw binaries named <binary>_%s_<os>_<arch> in %s", version, dir)
	}
	sort.Slice(release.Assets, func(i, j int) bool { return release.Assets[i].Filename < release.Assets[j].Filename })

	raw, err := json.MarshalIndent(release, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, string(channel)+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "wrote %s with %d assets\n", path, len(release.Assets))
	return nil
}

// verify installs the release the way an installation would, from the
// directory instead of the network: the real update.Service reads the
// manifest, checks the signature against the committed public key, checks
// every asset's name against the signed checksums and its bytes against its
// checksum, and stages it — once per platform the manifest lists. A release
// that passes this is one Download accepts; the pipeline used to publish
// releases no installation could.
func verify(dir, pubKeyPath string, channel update.Channel, out io.Writer) error {
	pub, err := readKey(pubKeyPath)
	if err != nil {
		return err
	}
	source := dirSource{dir: dir}
	ctx := context.Background()
	release, err := source.Latest(ctx, channel)
	if err != nil {
		return err
	}
	source.base = strings.TrimSuffix(release.ChecksumsURL, "/"+checksumsName)

	platforms := map[string]bool{}
	for _, a := range release.Assets {
		platforms[a.Platform] = true
	}
	names := make([]string, 0, len(platforms))
	for p := range platforms {
		names = append(names, p)
	}
	sort.Strings(names)

	for _, platform := range names {
		scratch, err := os.MkdirTemp("", "releasefeed-verify-")
		if err != nil {
			return err
		}
		staged, err := stageFor(ctx, source, release, pub, platform, scratch)
		_ = os.RemoveAll(scratch)
		if err != nil {
			return fmt.Errorf("%s: %w", platform, err)
		}
		_, _ = fmt.Fprintf(out, "%s: %d binaries verified\n", platform, len(staged.Binaries))
	}
	return nil
}

func stageFor(ctx context.Context, source dirSource, release *update.Release, pub, platform, scratch string) (update.Staged, error) {
	bin := filepath.Join(scratch, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return update.Staged{}, err
	}
	// Every binary "installed", so every asset for the platform is staged —
	// under both spellings, because the installer names files for the
	// machine running this, not for the platform being verified.
	for _, b := range update.Binaries() {
		for _, name := range []string{b, b + ".exe"} {
			if err := os.WriteFile(filepath.Join(bin, name), nil, 0o755); err != nil {
				return update.Staged{}, err
			}
		}
	}
	installer := updateinstall.New(filepath.Join(scratch, "staged"), bin)
	svc := update.NewService(update.Deps{
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Source:     source,
		Stager:     installer,
		Installer:  installer,
		Supervisor: noSupervisor{},
		Operators:  anyone{},
		PublicKey:  pub,
		Platform:   platform,
		// Older than any release, so the version check passes for every
		// version the updater could ever install — and refuses one it could
		// not, like a tag that is not a version.
		Version: "v0.0.0",
	})
	out, err := svc.Download(ctx, update.DownloadInput{Release: release})
	return out.Staged, err
}

// dirSource is update.ReleaseSource over a local directory: a URL is the
// base URL plus a file name in dir.
type dirSource struct {
	dir, base string
}

func (d dirSource) Configured() bool { return true }

func (d dirSource) Latest(_ context.Context, channel update.Channel) (*update.Release, error) {
	raw, err := os.ReadFile(filepath.Join(d.dir, string(channel)+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: no %s.json in %s", update.ErrNotPublished, channel, d.dir)
	}
	if err != nil {
		return nil, err
	}
	var r update.Release
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (d dirSource) Fetch(_ context.Context, url string) ([]byte, error) {
	name, ok := strings.CutPrefix(url, d.base+"/")
	if !ok || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("%s is not under this release's base URL %s", url, d.base)
	}
	data, err := os.ReadFile(filepath.Join(d.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", update.ErrNotPublished, name)
	}
	return data, err
}

type noSupervisor struct{}

func (noSupervisor) CanRestart(context.Context) bool { return false }
func (noSupervisor) Restart(context.Context) error {
	return errors.New("verify never restarts anything")
}
func (noSupervisor) Healthy(context.Context) bool { return false }

type anyone struct{}

func (anyone) MayInstall(context.Context) (bool, error) { return true, nil }

func readKey(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading the public key: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}

// publishedAt pins the manifest's date to the tagged commit when the build
// is reproducible, the same source Taskfile's DATE reads.
func publishedAt(flagValue, epoch string) (time.Time, error) {
	if flagValue != "" {
		return time.Parse(time.RFC3339, flagValue)
	}
	if epoch != "" {
		var secs int64
		if _, err := fmt.Sscan(epoch, &secs); err != nil {
			return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH %q is not a number", epoch)
		}
		return time.Unix(secs, 0), nil
	}
	return clockx.System{}.Now(), nil
}
