#!/usr/bin/env bash
# Assembles the Linux AppImage.
#
# What this deliberately does NOT do is bundle GTK4 and WebKitGTK, which is
# what the scaffolded version tried to do and why the published AppImage
# exited the moment it was launched.
#
# The reasoning, because it will look like a regression to anyone who expects
# an AppImage to be self-contained:
#
#   * `linuxdeploy` bundles what `ldd` reports and nothing else. WebKitGTK is
#     not one library: it spawns WebKitWebProcess and WebKitNetworkProcess as
#     separate executables from a path compiled into libwebkitgtk, and dlopens
#     an injected bundle from another. None of those are link-time
#     dependencies, so none of them were ever copied in — while the library
#     that needs them was. The result is a bundled WebKit that cannot start,
#     next to a host WebKit it refuses to match versions with.
#
#   * The usual answer, `linuxdeploy-plugin-gtk`, does not apply: it is a
#     GTK+2/3 plugin, and this application links GTK4 and webkitgtk-6.0.
#     Nothing bundles the GLib schemas and GIO modules GTK4 aborts without,
#     which is the immediate exit.
#
#   * Making that work is an open problem for this stack, not an oversight —
#     wails#4313 is exactly this and is still open with no accepted fix.
#
# So the AppImage carries the application and its daemon, and asks the system
# for GTK4 and WebKitGTK — the same contract the tarball already ships under,
# and the same one the .deb and .rpm declare in nfpm.yaml. What it adds over
# the tarball is a single executable file, a desktop entry and an icon.
#
# A machine without those libraries gets a sentence naming them, rather than a
# window that never appears: see the AppRun written below.

set -euxo pipefail

APP_DIR="${APP_NAME}.AppDir"

# From scratch every time. A stale AppDir from an earlier layout is a file
# that ships without anything referring to it.
rm -rf "${APP_DIR}"
mkdir -p "${APP_DIR}/usr/bin" "${APP_DIR}/usr/share/applications"

install -m 0755 "${APP_BINARY}" "${APP_DIR}/usr/bin/${APP_NAME}"

# The daemon travels beside the application, for the reason ADR-0002 gives:
# aos-desktop supervises a separate aosd rather than embedding one, and
# supervise.Resolver looks for it next to its own executable — which, inside a
# mounted AppImage, is this directory.
if [ -n "${APP_DAEMON:-}" ]; then
    install -m 0755 "${APP_DAEMON}" "${APP_DIR}/usr/bin/"
fi

# The CLI, when there is one to carry. Not required for the window to run;
# included so a person who installed the AppImage has the same three binaries
# the tarball gives them.
if [ -n "${APP_CLI:-}" ] && [ -f "${APP_CLI}" ]; then
    install -m 0755 "${APP_CLI}" "${APP_DIR}/usr/bin/"
fi

# The icon, twice under two names, and no hicolor tree.
#
# ${APP_NAME}.png at the root is what appimagetool looks for and what the
# .desktop file's `Icon=` names; .DirIcon is what the format reads and what
# integration tools fall back to. A usr/share/icons/hicolor/<size>/apps copy
# would be the third, and it would have to name a size — this icon is 1024
# square, so the 256x256 directory the scaffolding used was a claim about the
# file that was not true, and would stop being true again the moment the icon
# is redrawn.
cp "${ICON_PATH}" "${APP_DIR}/${APP_NAME}.png"
cp "${ICON_PATH}" "${APP_DIR}/.DirIcon"

# Written here rather than taken from `wails3 generate .desktop`. This is the
# file appimagetool validates and the one a desktop environment reads after
# installation, and it has to say `Exec=${APP_NAME}` — a bare name resolved
# through the PATH that AppRun sets. The generated file is still what the .deb
# and .rpm use, where an absolute path is right; the two are not the same file
# and pointing both at one was how the AppImage ended up naming a path that
# only exists on a machine that installed the .deb.
cat > "${APP_DIR}/${APP_NAME}.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Version=1.0
Name=${APP_DISPLAY_NAME:-${APP_NAME}}
Comment=${APP_COMMENT:-An operating system for AI agents.}
Exec=${APP_NAME} %u
Icon=${APP_NAME}
Terminal=false
Categories=Development;Utility;
StartupWMClass=${APP_NAME}
DESKTOP
cp "${APP_DIR}/${APP_NAME}.desktop" "${APP_DIR}/usr/share/applications/"

# AppRun. Two lines of it are generated so the name is baked in; the rest is a
# quoted heredoc so nothing in it is expanded now.
{
    printf '#!/bin/sh\n'
    printf 'APP_NAME=%s\n' "${APP_NAME}"
    cat <<'APPRUN'
# The AppImage entry point.
#
# Everything this application is made of travels with it. GTK4 and WebKitGTK
# do not — see build.sh for why. This checks for them before handing over, so
# a system that has not got them is told which packages to install instead of
# watching a window fail to appear.
set -eu

HERE=$(dirname "$(readlink -f "$0")")
BIN="$HERE/usr/bin/$APP_NAME"

# `ldd` prints "\tlibfoo.so.1 => not found" for each one it cannot resolve.
# If ldd itself is missing this comes back empty and the launch proceeds:
# refusing to start over a check that could not run would be worse than the
# linker error it was trying to explain.
missing=$(ldd "$BIN" 2>/dev/null | sed -n 's/^[[:space:]]*\(.*\) => not found$/  \1/p' || true)

if [ -n "$missing" ]; then
    message="$APP_NAME needs libraries this system does not have:

$missing

Install them and run it again:

  Ubuntu 24.04+ / Debian 13+   sudo apt install libgtk-4-1 libwebkitgtk-6.0-4
  Fedora 39+                   sudo dnf install gtk4 webkitgtk6.0
  Arch                         sudo pacman -S gtk4 webkitgtk-6.0"

    printf '%s\n' "$message" >&2

    # Launched from a menu there is nowhere for stderr to go, which is exactly
    # the case where this reads as "it does nothing at all".
    if [ ! -t 2 ]; then
        if command -v zenity >/dev/null 2>&1; then
            zenity --error --no-wrap --title="$APP_NAME" --text="$message" >/dev/null 2>&1 || true
        elif command -v kdialog >/dev/null 2>&1; then
            kdialog --error "$message" >/dev/null 2>&1 || true
        fi
    fi
    exit 1
fi

# So `aos` is on the PATH for a person who opens a terminal from the app, and
# so the .desktop file's bare `Exec=` name resolves.
PATH="$HERE/usr/bin:$PATH"
export PATH

exec "$BIN" "$@"
APPRUN
} > "${APP_DIR}/AppRun"
chmod +x "${APP_DIR}/AppRun"

# appimagetool, not linuxdeploy: there is nothing left to deploy, only an
# AppDir to pack. It also means the build no longer downloads a tool whose job
# was to make the decision this script now makes deliberately.
case "$(uname -m)" in
    x86_64)  TOOL_ARCH=x86_64  ;;
    aarch64) TOOL_ARCH=aarch64 ;;
    *) echo "no appimagetool for $(uname -m)" >&2; exit 1 ;;
esac

TOOL="appimagetool-${TOOL_ARCH}.AppImage"
wget -q -4 -N "https://github.com/AppImage/appimagetool/releases/download/continuous/${TOOL}"
chmod +x "${TOOL}"

# ARCH is stated rather than guessed: appimagetool infers it from the binaries
# it finds and refuses to build when it cannot, which on a cross-built AppDir
# is a failure with nothing wrong.
ARCH="${TOOL_ARCH}" "./${TOOL}" "${APP_DIR}" "${APP_NAME}.AppImage"
