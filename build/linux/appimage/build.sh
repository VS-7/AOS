#!/usr/bin/env bash
# Copyright (c) 2018-Present Lea Anthony
# SPDX-License-Identifier: MIT

# Fail script on any error
set -euxo pipefail

# Define variables
APP_DIR="${APP_NAME}.AppDir"

# Create AppDir structure
mkdir -p "${APP_DIR}/usr/bin"
cp -r "${APP_BINARY}" "${APP_DIR}/usr/bin/"
# The daemon travels in the AppDir beside the application, for the reason
# ADR-0002 gives: aos-desktop supervises a separate aosd rather than embedding
# one, and supervise.Resolver finds it next to its own executable — which,
# inside a mounted AppImage, is this directory.
if [ -n "${APP_DAEMON:-}" ]; then
    cp "${APP_DAEMON}" "${APP_DIR}/usr/bin/"
fi
# Renamed on the way in: the .desktop file names its icon after the
# application, and appimagetool looks for exactly ${APP_NAME}.png at the
# AppDir root. Copying the source name through left it looking for an icon
# that was sitting right there under another name.
cp "${ICON_PATH}" "${APP_DIR}/${APP_NAME}.png"
cp "${DESKTOP_FILE}" "${APP_DIR}/"

if [[ $(uname -m) == *x86_64* ]]; then
    # Download linuxdeploy and make it executable
    wget -q -4 -N https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage
    chmod +x linuxdeploy-x86_64.AppImage

    # Run linuxdeploy to bundle the application
    ./linuxdeploy-x86_64.AppImage --appdir "${APP_DIR}" --output appimage
else
    # Download linuxdeploy and make it executable (arm64)
    wget -q -4 -N https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-aarch64.AppImage
    chmod +x linuxdeploy-aarch64.AppImage

    # Run linuxdeploy to bundle the application (arm64)
    ./linuxdeploy-aarch64.AppImage --appdir "${APP_DIR}" --output appimage
fi

# Rename the generated AppImage. The glob has to be outside the quotes: as
# written by the scaffolding it was inside them, so the shell passed the
# literal string through and the mv failed on every run. Nothing used this
# script, which is how that survived.
mv "${APP_NAME}"*.AppImage "${APP_NAME}.AppImage"

