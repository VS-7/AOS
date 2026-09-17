package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/relsig"
	"github.com/OWNER/aos/internal/domain/update"
)

const version = "v0.16.0-fase10"

// release lays out what the publish job has by the time the feed is built:
// raw binaries, an archive the updater ignores, and checksums.txt over all
// of them.
func release(t *testing.T) (dir, pubPath, priv string) {
	t.Helper()
	dir = t.TempDir()
	files := map[string]string{
		"aosd_" + version + "_linux_amd64":              "aosd linux",
		"aos_" + version + "_linux_amd64":               "aos linux",
		"aosd_" + version + "_windows_amd64.exe":        "aosd windows",
		"aos-desktop_" + version + "_linux_amd64":       "desktop linux",
		"AOS-" + version + "-darwin-arm64.zip":          "a bundle",
		"AOS-server-" + version + "-linux-amd64.tar.gz": "a tarball",
	}
	var checksums strings.Builder
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(body))
		checksums.WriteString(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, checksumsName), []byte(checksums.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	pub, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	pubPath = filepath.Join(t.TempDir(), "release-pubkey.pub")
	if err := os.WriteFile(pubPath, []byte(pub+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, pubPath, priv
}

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestAFeedBuiltBySignManifestAndVerifyInstalls(t *testing.T) {
	dir, pub, priv := release(t)
	getenv := env(map[string]string{defaultKeyEnv: priv, "SOURCE_DATE_EPOCH": "1788000000"})
	base := "https://github.com/VS-7/AOS/releases/download/" + version

	for _, args := range [][]string{
		{"sign", "-dir", dir, "-pubkey", pub},
		{"manifest", "-dir", dir, "-version", version, "-base-url", base, "-page-url", "https://github.com/VS-7/AOS/releases/tag/" + version},
		{"verify", "-dir", dir, "-pubkey", pub},
	} {
		if err := run(args, getenv, io.Discard); err != nil {
			t.Fatalf("%s: %v", args[0], err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r update.Release
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if r.Version != version || r.SignatureURL != base+"/checksums.txt.sig" || r.PageURL == "" {
		t.Fatalf("manifest = %+v", r)
	}
	// Raw binaries only: the zip and the tarball are for people.
	if len(r.Assets) != 4 {
		t.Fatalf("assets = %+v", r.Assets)
	}
	for _, a := range r.Assets {
		if a.Filename == "aosd_"+version+"_windows_amd64.exe" && (a.Binary != "aosd" || a.Platform != "windows/amd64") {
			t.Fatalf("windows asset read as %+v", a)
		}
	}
	if r.PublishedAt.Unix() != 1788000000 {
		t.Fatalf("published at %v, want SOURCE_DATE_EPOCH", r.PublishedAt)
	}
}

// The failure worth stopping a release for: a secret that signs, but is not
// the pair of the public key every installation carries.
func TestSignRefusesAKeyThatInstallationsWouldReject(t *testing.T) {
	dir, pub, _ := release(t)
	_, other, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	err = run([]string{"sign", "-dir", dir, "-pubkey", pub}, env(map[string]string{defaultKeyEnv: other}), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected a key mismatch, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, signatureName)); !os.IsNotExist(err) {
		t.Fatal("no signature should be written for a key installations reject")
	}

	if err := run([]string{"sign", "-dir", dir, "-pubkey", pub}, env(nil), io.Discard); err == nil {
		t.Fatal("signing with no key must fail")
	}
}

// verify is the updater's own Download, so what it refuses is what every
// installation would have refused after the release was out.
func TestVerifyRefusesAReleaseAnInstallationWouldRefuse(t *testing.T) {
	cases := map[string]func(t *testing.T, dir string){
		"no signature": func(t *testing.T, dir string) {
			_ = os.Remove(filepath.Join(dir, signatureName))
		},
		"an asset changed after checksums": func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "aosd_"+version+"_linux_amd64"), []byte("tampered"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"no manifest": func(t *testing.T, dir string) {
			_ = os.Remove(filepath.Join(dir, "stable.json"))
		},
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			dir, pub, priv := release(t)
			getenv := env(map[string]string{defaultKeyEnv: priv})
			if err := run([]string{"sign", "-dir", dir, "-pubkey", pub}, getenv, io.Discard); err != nil {
				t.Fatal(err)
			}
			if err := run([]string{"manifest", "-dir", dir, "-version", version, "-base-url", "https://x.test/d"}, getenv, io.Discard); err != nil {
				t.Fatal(err)
			}
			breakIt(t, dir)
			if err := run([]string{"verify", "-dir", dir, "-pubkey", pub}, getenv, io.Discard); err == nil {
				t.Fatal("verify should refuse this release")
			}
		})
	}
}

func TestManifestRefusesWhatItCannotDescribe(t *testing.T) {
	dir, _, _ := release(t)
	if err := run([]string{"manifest", "-dir", dir, "-version", version}, env(nil), io.Discard); err == nil {
		t.Fatal("a manifest needs a base URL")
	}
	if err := run([]string{"manifest", "-dir", dir, "-version", "v9.9.9", "-base-url", "https://x.test"}, env(nil), io.Discard); err == nil {
		t.Fatal("a version with no assets is not a release")
	}
	if err := os.WriteFile(filepath.Join(dir, "aos_"+version+"_darwin_arm64"), []byte("unlisted"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"manifest", "-dir", dir, "-version", version, "-base-url", "https://x.test"}, env(nil), io.Discard); err == nil {
		t.Fatal("an asset missing from checksums.txt must not be published")
	}
	if err := run([]string{"bogus"}, env(nil), io.Discard); err == nil {
		t.Fatal("an unknown subcommand is an error")
	}
	if err := run(nil, env(nil), io.Discard); err == nil {
		t.Fatal("no subcommand is an error")
	}
}
