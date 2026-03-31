#!/bin/bash
#
# HZUUL Installer
# https://github.com/Valkyrie00/hzuul
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Valkyrie00/hzuul/main/install.sh | bash
#

set -e

REPO="Valkyrie00/hzuul"
BINARY="hzuul"
INSTALL_DIR="/usr/local/bin"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info()    { echo -e "${BLUE}==>${NC} ${BOLD}$1${NC}"; }
success() { echo -e "${GREEN}==>${NC} ${BOLD}$1${NC}"; }
warn()    { echo -e "${YELLOW}Warning:${NC} $1"; }
error()   { echo -e "${RED}Error:${NC} $1" >&2; exit 1; }

detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Darwin) PLATFORM="darwin" ;;
        Linux)  PLATFORM="linux" ;;
        *)      error "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *)             error "Unsupported architecture: $ARCH" ;;
    esac

    info "Detected: ${OS} ${ARCH}"
}

get_latest_version() {
    info "Fetching latest release..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"v?([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        error "Could not determine latest version. Check https://github.com/${REPO}/releases"
    fi

    info "Latest version: v${VERSION}"
}

download_and_install() {
    FILENAME="${BINARY}_${VERSION}_${PLATFORM}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Downloading ${FILENAME}..."
    if ! curl -fsSL -o "${TMPDIR}/${FILENAME}" "$URL"; then
        error "Download failed. Check if the release exists: ${URL}"
    fi

    info "Verifying checksum..."
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"
    if curl -fsSL -o "${TMPDIR}/checksums.txt" "$CHECKSUM_URL" 2>/dev/null; then
        cd "$TMPDIR"
        EXPECTED=$(grep "$FILENAME" checksums.txt | awk '{print $1}')
        if [ -n "$EXPECTED" ]; then
            if command -v sha256sum &>/dev/null; then
                ACTUAL=$(sha256sum "$FILENAME" | awk '{print $1}')
            else
                ACTUAL=$(shasum -a 256 "$FILENAME" | awk '{print $1}')
            fi
            if [ "$EXPECTED" != "$ACTUAL" ]; then
                error "Checksum mismatch!\n  expected: ${EXPECTED}\n  actual:   ${ACTUAL}"
            fi
            success "Checksum verified"
        fi
        cd - >/dev/null
    else
        warn "Could not verify checksum (checksums.txt not found)"
    fi

    info "Extracting..."
    tar -xzf "${TMPDIR}/${FILENAME}" -C "$TMPDIR"

    info "Installing to ${INSTALL_DIR}/${BINARY}..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi
    chmod +x "${INSTALL_DIR}/${BINARY}"

    success "Installed ${BINARY} v${VERSION} to ${INSTALL_DIR}/${BINARY}"
}

main() {
    echo ""
    echo -e "${BLUE}${BOLD}  HZUUL Installer${NC}"
    echo -e "  Terminal UI for Zuul CI/CD"
    echo ""

    if ! command -v curl &>/dev/null; then
        error "curl is required but not installed"
    fi

    detect_platform
    get_latest_version
    download_and_install

    echo ""
    echo -e "  Run ${BLUE}${BOLD}hzuul${NC} to get started."
    echo -e "  Config: ${BOLD}~/.hzuul/config.yaml${NC}"
    echo -e "  Docs:   ${BLUE}https://github.com/${REPO}${NC}"
    echo ""
}

main "$@"
