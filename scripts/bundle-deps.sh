#!/bin/bash
# bundle-deps.sh — Copy tmux and git binaries + their dylibs into the .app bundle
# Usage: ./scripts/bundle-deps.sh <path-to-app>
# Example: ./scripts/bundle-deps.sh cmd/aisupervisor-gui/build/bin/aisupervisor-gui.app

set -euo pipefail

APP_PATH="${1:?Usage: $0 <path-to.app>}"
RESOURCES_BIN="${APP_PATH}/Contents/Resources/bin"

mkdir -p "${RESOURCES_BIN}"

# Copy a binary and fix up its dylib references
bundle_binary() {
    local bin_name="$1"
    local bin_path
    bin_path=$(which "${bin_name}" 2>/dev/null || true)

    if [ -z "${bin_path}" ]; then
        echo "WARNING: ${bin_name} not found in PATH, skipping"
        return
    fi

    echo "Bundling ${bin_name} from ${bin_path}"
    cp "${bin_path}" "${RESOURCES_BIN}/${bin_name}"
    chmod 755 "${RESOURCES_BIN}/${bin_name}"

    # Find and copy non-system dylibs
    otool -L "${bin_path}" | tail -n +2 | awk '{print $1}' | while read -r dylib; do
        # Skip system libraries
        case "${dylib}" in
            /usr/lib/*|/System/*|@rpath/*|@executable_path/*|@loader_path/*)
                continue
                ;;
        esac

        if [ -f "${dylib}" ]; then
            local dylib_name
            dylib_name=$(basename "${dylib}")
            if [ ! -f "${RESOURCES_BIN}/${dylib_name}" ]; then
                echo "  Copying dylib: ${dylib}"
                cp "${dylib}" "${RESOURCES_BIN}/${dylib_name}"
                chmod 644 "${RESOURCES_BIN}/${dylib_name}"
            fi
            # Fix the reference in the binary
            install_name_tool -change "${dylib}" "@executable_path/../Resources/bin/${dylib_name}" "${RESOURCES_BIN}/${bin_name}" 2>/dev/null || true
        fi
    done
}

# Bundle required dependencies
bundle_binary "tmux"
bundle_binary "git"

# Also copy libevent if tmux needs it (common on Homebrew)
for lib in "${RESOURCES_BIN}"/*; do
    if [ -f "${lib}" ]; then
        otool -L "${lib}" 2>/dev/null | tail -n +2 | awk '{print $1}' | while read -r dylib; do
            case "${dylib}" in
                /usr/lib/*|/System/*|@rpath/*|@executable_path/*|@loader_path/*)
                    continue
                    ;;
            esac
            if [ -f "${dylib}" ]; then
                local_name=$(basename "${dylib}")
                if [ ! -f "${RESOURCES_BIN}/${local_name}" ]; then
                    echo "  Copying transitive dylib: ${dylib}"
                    cp "${dylib}" "${RESOURCES_BIN}/${local_name}"
                    chmod 644 "${RESOURCES_BIN}/${local_name}"
                fi
                install_name_tool -change "${dylib}" "@executable_path/../Resources/bin/${local_name}" "${lib}" 2>/dev/null || true
            fi
        done
    fi
done

echo "Done! Bundled binaries in ${RESOURCES_BIN}:"
ls -la "${RESOURCES_BIN}"
