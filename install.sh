#!/bin/sh
# Installs AOS from a published release.
#
# The reason this exists rather than a link to the releases page: macOS marks
# anything a browser downloads with com.apple.quarantine, and an unsigned
# application carrying that mark does not open — since macOS 15 not even
# through right-click → Open, only through a trip into System Settings. curl
# sets no such mark, so what this script installs simply runs. That is the
# whole trick, and it is why the beta does not need a Developer ID yet.
#
#   curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh | sh
#
# Environment:
#   AOS_VERSION   a tag to install instead of the latest release
#   AOS_PREFIX    where the aos/aosd commands go (default ~/.local/bin)
#   AOS_NO_CLI=1  install only the application, no terminal commands
set -eu

REPO="VS-7/AOS"
PREFIX="${AOS_PREFIX:-$HOME/.local/bin}"

die() { printf 'install: %s\n' "$1" >&2; exit 1; }
say() { printf '%s\n' "$1"; }

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v ditto >/dev/null 2>&1 || die "ditto is required"

# --- what to install -------------------------------------------------------

os=$(uname -s)
arch=$(uname -m)
case "$arch" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64)  arch=amd64 ;;
  *) die "unsupported architecture: $arch" ;;
esac
case "$os" in
  Darwin) os=darwin ;;
  # Linux packaging (AppImage) lands next; until then this script would
  # install an application that does not exist for the platform.
  *) die "only macOS is supported by this installer today (got $os)" ;;
esac

version="${AOS_VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
    sed -n 's/.*"tag_name" *: *"\([^"]*\)".*/\1/p' | head -1)
  [ -n "$version" ] || die "could not resolve the latest release; set AOS_VERSION"
fi
say "AOS $version — $os/$arch"

base="https://github.com/$REPO/releases/download/$version"
app_zip="AOS-${version}-${os}-${arch}.zip"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
cd "$tmp"

# --- download and verify ---------------------------------------------------

fetch() {
  curl -fsSL -o "$1" "$base/$1" || die "could not download $1 from $version"
}

say "downloading..."
fetch checksums.txt
fetch "$app_zip"
cli="aos_${version}_${os}_${arch} aosd_${version}_${os}_${arch}"
if [ -z "${AOS_NO_CLI:-}" ]; then
  for f in $cli; do fetch "$f"; done
fi

# A release publishes one checksums file covering every asset, so verifying is
# a single command over whatever this run happened to download.
if command -v shasum >/dev/null 2>&1; then
  shasum -a 256 -c --ignore-missing checksums.txt >/dev/null || die "checksum mismatch — refusing to install"
elif command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c --ignore-missing checksums.txt >/dev/null || die "checksum mismatch — refusing to install"
else
  die "no sha256 tool found; refusing to install unverified binaries"
fi
say "checksums verified"

# --- install ---------------------------------------------------------------

# ditto, not unzip. The archive was written by ditto and carries the extended
# attributes a signed bundle is sealed over; unzip drops them, and the bundle
# it leaves behind fails codesign --verify with "a sealed resource is missing
# or invalid" — at launch, long after the install said it worked. Finder's own
# double-click uses the same machinery ditto does, so a hand-downloaded zip is
# fine; it is only the terminal tool that has to be the right one.
ditto -x -k "$app_zip" .
[ -d "AOS.app" ] || die "$app_zip did not contain AOS.app"

# The ad-hoc signature is not a trust statement — it is what lets the thing
# run on Apple Silicon at all. Checking it here turns a corrupted download
# into a message now instead of a bundle that installs and will not open.
if command -v codesign >/dev/null 2>&1; then
  codesign --verify --deep --strict "AOS.app" 2>/dev/null ||
    die "the downloaded bundle failed signature verification — not installing"
fi

apps="/Applications"
[ -w "$apps" ] || apps="$HOME/Applications"
mkdir -p "$apps"
# Replaced whole rather than merged: a bundle that is half one version and
# half another is worse than either, and the daemon inside it has to match
# the application that supervises it (build.Compatible).
rm -rf "$apps/AOS.app"
cp -R "AOS.app" "$apps/AOS.app"
say "installed $apps/AOS.app"

if [ -z "${AOS_NO_CLI:-}" ]; then
  mkdir -p "$PREFIX"
  # aosd travels beside aos for the same reason it travels inside the bundle:
  # supervise.Resolver looks next to the executable that is asking.
  for f in $cli; do
    name=${f%%_*}
    chmod +x "$f"
    mv "$f" "$PREFIX/$name"
  done
  say "installed $PREFIX/aos and $PREFIX/aosd"
fi

# --- what now --------------------------------------------------------------

say ""
say "Open it:  open -a AOS"
if [ -z "${AOS_NO_CLI:-}" ]; then
  case ":$PATH:" in
    *":$PREFIX:"*) say "The aos command is ready." ;;
    *) say "Add the commands to your PATH:  export PATH=\"\$PATH:$PREFIX\"" ;;
  esac
fi
