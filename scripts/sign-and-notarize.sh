#!/bin/bash
# sign-and-notarize.sh — Full release pipeline: build → bundle deps → sign → DMG → notarize
# Usage: ./scripts/sign-and-notarize.sh [version]
# Example: ./scripts/sign-and-notarize.sh 1.0.0
#
# Prerequisites:
#   - Apple Developer ID certificate in Keychain
#   - Notarytool credentials stored: xcrun notarytool store-credentials "AC_PASSWORD" ...
#   - brew install create-dmg

set -euo pipefail

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "${SCRIPT_DIR}")"
CMD_GUI="${PROJECT_DIR}/cmd/aisupervisor-gui"
APP_PATH="${CMD_GUI}/build/bin/aisupervisor.app"
DMG_PATH="${PROJECT_DIR}/build/aisupervisor-${VERSION}.dmg"

echo "=== AI Supervisor Release v${VERSION} ==="

# Step 1: Build
echo ""
echo "--- Step 1: Building ---"
cd "${PROJECT_DIR}"
make build-gui VERSION="${VERSION}"

# Step 2: Bundle dependencies
echo ""
echo "--- Step 2: Bundling dependencies ---"
"${SCRIPT_DIR}/bundle-deps.sh" "${APP_PATH}"

# Step 3: Sign
echo ""
echo "--- Step 3: Code signing ---"
DEVELOPER_ID=$(security find-identity -v -p codesigning | grep "Developer ID Application" | head -1 | sed 's/.*"\(.*\)"/\1/')
if [ -z "${DEVELOPER_ID}" ]; then
    echo "ERROR: No Developer ID Application certificate found in Keychain"
    echo "To sign for local testing only, use: codesign --force --deep --sign - ${APP_PATH}"
    exit 1
fi
echo "Signing with: ${DEVELOPER_ID}"

# Sign bundled binaries first
if [ -d "${APP_PATH}/Contents/Resources/bin" ]; then
    for bin in "${APP_PATH}"/Contents/Resources/bin/*; do
        codesign --force --options runtime --sign "Developer ID Application: ${DEVELOPER_ID}" "${bin}"
    done
fi

# Sign the app bundle
codesign --deep --force --options runtime \
    --sign "Developer ID Application: ${DEVELOPER_ID}" \
    "${APP_PATH}"

# Verify signature
codesign --verify --verbose "${APP_PATH}"
echo "Signature verified"

# Step 4: Create DMG
echo ""
echo "--- Step 4: Creating DMG ---"
mkdir -p "${PROJECT_DIR}/build"
# Remove existing DMG if present (create-dmg fails otherwise)
rm -f "${DMG_PATH}"
create-dmg \
    --volname "AI Supervisor" \
    --window-pos 200 120 \
    --window-size 600 400 \
    --icon-size 100 \
    --app-drop-link 400 200 \
    --icon "aisupervisor-gui.app" 200 200 \
    "${DMG_PATH}" \
    "${APP_PATH}"

# Step 5: Notarize
echo ""
echo "--- Step 5: Notarizing ---"
xcrun notarytool submit "${DMG_PATH}" \
    --keychain-profile "AC_PASSWORD" --wait

# Step 6: Staple
echo ""
echo "--- Step 6: Stapling ---"
xcrun stapler staple "${DMG_PATH}"

echo ""
echo "=== Release complete ==="
echo "DMG: ${DMG_PATH}"
echo "Version: ${VERSION}"
echo ""
echo "To verify: spctl --assess --type open --context context:primary-signature ${DMG_PATH}"
