#!/bin/sh
# Installs AOS from a published release.
#
#   curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh | sh
#
# On macOS the reason this exists rather than a link to the releases page is
# specific: macOS marks anything a browser downloads with
# com.apple.quarantine, and an unsigned application carrying that mark does
# not open — since macOS 15 not even through right-click → Open, only through
# a trip into System Settings. curl sets no such mark, so what this script
# installs simply runs. That is the whole trick, and it is why the beta does
# not need a Developer ID yet.
#
# On Linux there is no quarantine to dodge; what this does instead is the
# tedious part of a tarball install. It verifies the download against the
# release's checksums, puts the three binaries where the resolver expects to
# find each other, writes a menu entry, and — the part that matters most —
# says which packages are missing when GTK4 and WebKitGTK are not installed,
# rather than leaving a window that never appears.
#
# On a server it installs a third thing: the daemon with the interface
# compiled in, so a VPS is one file and a browser rather than an API you have
# to write a client for:
#
#   curl -fsSL .../install.sh | AOS_SERVER=1 sh
#
# The variable goes on `sh`, not on `curl` — prefixing the fetch sets it for
# the download and not for the script the download produces.
#
# Environment:
#   AOS_VERSION    a tag to install instead of the latest release
#   AOS_PREFIX     where the binaries go (default ~/.local/bin)
#   AOS_NO_CLI=1   install only the application, no terminal commands
#   AOS_SERVER=1   install the headless daemon and a systemd unit (Linux)
#   AOS_WORKSPACE  the directory the server daemon serves (default ~/aos)
set -eu

REPO="VS-7/AOS"
PREFIX="${AOS_PREFIX:-$HOME/.local/bin}"

die() { printf 'install: %s\n' "$1" >&2; exit 1; }
say() { printf '%s\n' "$1"; }

command -v curl >/dev/null 2>&1 || die "curl is required"

# --- what to install -------------------------------------------------------

os=$(uname -s)
arch=$(uname -m)
case "$arch" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64)  arch=amd64 ;;
  *) die "unsupported architecture: $arch" ;;
esac
case "$os" in
  Darwin)
    os=darwin
    # ditto, not unzip — see the extraction step for why.
    command -v ditto >/dev/null 2>&1 || die "ditto is required"
    ;;
  Linux)
    os=linux
    command -v tar >/dev/null 2>&1 || die "tar is required"
    ;;
  *) die "no build for $os; macOS and Linux are what this installs" ;;
esac

# What this run installs. Three shapes, because there are three ways to use
# the system and they do not want the same files:
#
#   desktop  the window, plus the daemon it supervises and the commands
#   server   the daemon with the interface compiled in, headless, plus a
#            systemd unit — a VPS you reach with a browser
#   cli      the commands alone, which is all that exists for a platform with
#            no desktop build
#
# The desktop is built on one Linux runner and that runner is amd64, so a Linux
# arm64 machine has no window to install. The server is pure Go and
# cross-compiled, so it exists for both.
mode=desktop
if [ -n "${AOS_SERVER:-}" ]; then
  [ "$os" = linux ] || die "AOS_SERVER is a Linux install; this is $os"
  mode=server
elif [ "$os" = linux ] && [ "$arch" != amd64 ]; then
  mode=cli
  # Checked here rather than at the install step: there is nothing to do, and
  # finding that out after downloading is a worse way to be told.
  [ -z "${AOS_NO_CLI:-}" ] || die "AOS_NO_CLI leaves nothing to install on $os/$arch"
fi

version="${AOS_VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
    sed -n 's/.*"tag_name" *: *"\([^"]*\)".*/\1/p' | head -1)
  # The common causes, in the order they happen: the repository is private
  # (this script and everything it downloads are then invisible without
  # credentials), there is no published release yet, or the network is down.
  [ -n "$version" ] || die "could not resolve the latest release — the repository may be private, or have no release yet. Set AOS_VERSION to install a known tag."
fi

case "$mode" in
  server) say "AOS $version — $os/$arch (server: the daemon and its interface)" ;;
  cli)    say "AOS $version — $os/$arch (commands only; the desktop is built for amd64)" ;;
  *)      say "AOS $version — $os/$arch" ;;
esac

base="https://github.com/$REPO/releases/download/$version"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
cd "$tmp"

# --- download and verify ---------------------------------------------------

fetch() {
  curl -fsSL -o "$1" "$base/$1" || die "could not download $1 from $version"
}

say "downloading..."
fetch checksums.txt

# What each platform ships, and why they are not the same shape.
#
# macOS publishes an app bundle and the two commands separately, because the
# bundle is a directory and the commands are not part of it. Linux publishes
# one tarball holding all three binaries already beside each other, which is
# exactly the layout supervise.Resolver wants — so there is nothing to fetch
# separately, and AOS_NO_CLI becomes a question of what to unpack rather than
# what to download.
cli=""
app_zip=""
tarball=""
case "$mode" in
  desktop)
    if [ "$os" = darwin ]; then
      app_zip="AOS-${version}-${os}-${arch}.zip"
      fetch "$app_zip"
      cli="aos_${version}_${os}_${arch} aosd_${version}_${os}_${arch}"
    else
      tarball="AOS-${version}-${os}-${arch}.tar.gz"
      fetch "$tarball"
    fi
    ;;
  server)
    # A separate artifact from the desktop tarball above, and not a bigger
    # one: the daemon in there carries no interface, because the window it is
    # supervised by already has a copy. This one is the interface.
    tarball="AOS-server-${version}-${os}-${arch}.tar.gz"
    fetch "$tarball"
    ;;
  cli)
    cli="aos_${version}_${os}_${arch} aosd_${version}_${os}_${arch}"
    ;;
esac
if [ -n "$cli" ] && [ -z "${AOS_NO_CLI:-}" ]; then
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

if [ "$os" = darwin ]; then
  # ditto, not unzip. The archive was written by ditto and carries the
  # extended attributes a signed bundle is sealed over; unzip drops them, and
  # the bundle it leaves behind fails codesign --verify with "a sealed
  # resource is missing or invalid" — at launch, long after the install said
  # it worked. Finder's own double-click uses the same machinery ditto does,
  # so a hand-downloaded zip is fine; it is only the terminal tool that has to
  # be the right one.
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

elif [ "$mode" = server ]; then
  tar -xzf "$tarball"
  [ -d "AOS" ] || die "$tarball did not contain AOS/"

  mkdir -p "$PREFIX"
  for f in aosd aos; do
    [ -f "AOS/$f" ] || die "$tarball did not contain $f"
    chmod +x "AOS/$f"
    mv -f "AOS/$f" "$PREFIX/$f"
  done
  say "installed $PREFIX/aosd and $PREFIX/aos"

  workspace="${AOS_WORKSPACE:-$HOME/aos}"
  mkdir -p "$workspace"

  # A user unit, not a system one. It needs no root, which is what lets this
  # whole script stay a `curl | sh` on a fresh VPS; `loginctl enable-linger`
  # is what keeps it running when nobody is logged in, and is printed below
  # rather than done here — enabling a service that survives logout is a
  # change to the machine, not a detail of unpacking an archive.
  units="$HOME/.config/systemd/user"
  mkdir -p "$units"
  cat > "$units/aos.service" <<UNIT
[Unit]
Description=AOS daemon
Documentation=https://github.com/$REPO
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$PREFIX/aosd serve
WorkingDirectory=$workspace
Environment=AOS_WORKSPACE_PATH=$workspace
# Loopback, and left that way: ADR-0009 refuses a wider bind while security is
# off, and the documented way to reach this from outside is a reverse proxy
# terminating TLS in front of it. Nothing here should be on a public port.
Environment=AOS_SERVER_HOSTNAME=127.0.0.1
Environment=AOS_SERVER_PORT=5326
Restart=on-failure
RestartSec=3
# The daemon owns the workspace and writes to it; everything else on the
# machine is none of its business.
PrivateTmp=true
NoNewPrivileges=true

[Install]
WantedBy=default.target
UNIT
  say "installed $units/aos.service"

elif [ "$mode" = desktop ]; then
  tar -xzf "$tarball"
  [ -d "AOS" ] || die "$tarball did not contain AOS/"

  mkdir -p "$PREFIX"
  # All three into one directory on purpose: supervise.Resolver looks for the
  # daemon next to whichever executable is asking, so aos-desktop and aos both
  # find it without anything being configured.
  # An `if`, not `[ ... ] && ...`: the short form leaves the statement with a
  # non-zero status when the test fails, which is a conversation with `set -e`
  # that nobody reading this should have to have.
  binaries="aos-desktop aosd"
  if [ -z "${AOS_NO_CLI:-}" ]; then
    binaries="$binaries aos"
  fi
  for f in $binaries; do
    [ -f "AOS/$f" ] || die "$tarball did not contain $f"
    chmod +x "AOS/$f"
    mv -f "AOS/$f" "$PREFIX/$f"
  done
  say "installed $PREFIX/aos-desktop"

  # The menu entry. Written here rather than shipped in the tarball because it
  # has to name the absolute path this run installed to: a desktop environment
  # launching from the menu does not inherit the PATH a shell would, so a bare
  # `Exec=aos-desktop` resolves for a terminal and not for the menu that needs
  # it most.
  share="$HOME/.local/share"
  mkdir -p "$share/applications" "$share/aos"

  # Best effort, and not checksummed: this is decoration, and refusing to
  # install over a missing icon would be the tail wagging the dog. The tag is
  # used rather than main so the icon matches the version being installed.
  icon="aos-desktop"
  if curl -fsSL -o "$share/aos/aos-desktop.png" \
      "https://raw.githubusercontent.com/$REPO/$version/build/appicon.png" 2>/dev/null; then
    # An absolute path is a valid Icon value and sidesteps the icon theme
    # entirely — no hicolor directory to name a size this file may not be.
    icon="$share/aos/aos-desktop.png"
  fi

  cat > "$share/applications/aos-desktop.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Version=1.0
Name=AOS
Comment=An operating system for AI agents.
Exec=$PREFIX/aos-desktop %u
Icon=$icon
Terminal=false
Categories=Development;Utility;
StartupWMClass=aos-desktop
DESKTOP
  say "installed $share/applications/aos-desktop.desktop"

  # Some desktop environments read the directory on change and some cache it.
  if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$share/applications" >/dev/null 2>&1 || true
  fi
fi

# The commands, when they were downloaded separately rather than unpacked.
if [ -n "$cli" ] && [ -z "${AOS_NO_CLI:-}" ]; then
  mkdir -p "$PREFIX"
  # aosd travels beside aos for the same reason it travels inside the bundle:
  # supervise.Resolver looks next to the executable that is asking.
  for f in $cli; do
    name=${f%%_*}
    chmod +x "$f"
    mv -f "$f" "$PREFIX/$name"
  done
  say "installed $PREFIX/aos and $PREFIX/aosd"
fi

# --- the one dependency ----------------------------------------------------

# GTK4 and WebKitGTK are not bundled — see build/linux/appimage/build.sh for
# the long version. A missing one of them is a window that never appears and
# says nothing, which is worth two seconds here to turn into a sentence.
#
# A warning rather than a failure: everything downloaded is installed and
# correct, and the person is one package manager command from a working
# application.
if [ "$os" = linux ] && [ "$mode" = desktop ] && command -v ldd >/dev/null 2>&1; then
  missing=$(ldd "$PREFIX/aos-desktop" 2>/dev/null | sed -n 's/^[[:space:]]*\(.*\) => not found$/  \1/p' || true)
  if [ -n "$missing" ]; then
    say ""
    say "AOS is installed, but these libraries are missing:"
    say ""
    say "$missing"
    say ""
    say "  Ubuntu 24.04+ / Debian 13+   sudo apt install libgtk-4-1 libwebkitgtk-6.0-4"
    say "  Fedora 39+                   sudo dnf install gtk4 webkitgtk6.0"
    say "  Arch                         sudo pacman -S gtk4 webkitgtk-6.0"
  fi
fi

# --- what now --------------------------------------------------------------

say ""
if [ "$mode" = server ]; then
  say "Start it, and keep it running after you log out:"
  say ""
  say "  systemctl --user enable --now aos"
  say "  loginctl enable-linger \"$USER\""
  say ""
  say "It listens on 127.0.0.1:5326 and stays there. To reach it from a"
  say "browser, put a reverse proxy in front — with Caddy that is one line:"
  say ""
  say "  aos.example.com {"
  say "      reverse_proxy 127.0.0.1:5326"
  say "  }"
  say ""
  say "Caddy gets the certificate itself. The first page asks you to create"
  say "the account, and until you have, nothing else answers."
elif [ "$os" = darwin ]; then
  say "Open it:  open -a AOS"
elif [ "$mode" = desktop ]; then
  say "Open it:  aos-desktop        (or find AOS in your applications menu)"
fi
if [ -z "${AOS_NO_CLI:-}" ]; then
  case ":$PATH:" in
    *":$PREFIX:"*) say "The aos command is ready." ;;
    *) say "Add the commands to your PATH:  export PATH=\"\$PATH:$PREFIX\"" ;;
  esac
fi
