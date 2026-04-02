#!/bin/sh
# install.sh — Install meta-cli (Meta Marketing API CLI + MCP server)
# https://github.com/enriquefft/meta-cli
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/enriquefft/meta-cli/main/install.sh | sh
#
# Environment variables:
#   META_CLI_VERSION      Override version (default: latest from GitHub API)
#   META_CLI_INSTALL_DIR  Override install directory (default: ~/.local/bin)
#   NO_COLOR              Disable colored output when set

set -eu

# ── Constants ─────────────────────────────────────────────────────────────────

GITHUB_OWNER="enriquefft"
GITHUB_REPO="meta-cli"
BINARY_NAME="meta"
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"

# ── Color handling ────────────────────────────────────────────────────────────

setup_colors() {
    if [ -n "${NO_COLOR:-}" ] || [ ! -t 1 ]; then
        RED=""
        GREEN=""
        YELLOW=""
        BLUE=""
        BOLD=""
        DIM=""
        RESET=""
    else
        RED='\033[0;31m'
        GREEN='\033[0;32m'
        YELLOW='\033[0;33m'
        BLUE='\033[0;34m'
        BOLD='\033[1m'
        DIM='\033[2m'
        RESET='\033[0m'
    fi
}

# ── Output helpers ────────────────────────────────────────────────────────────

info() {
    printf "${BLUE}::${RESET} %s\n" "$1"
}

success() {
    printf "${GREEN}::${RESET} %s\n" "$1"
}

warn() {
    printf "${YELLOW}warning:${RESET} %s\n" "$1" >&2
}

error() {
    printf "${RED}error:${RESET} %s\n" "$1" >&2
}

fatal() {
    error "$1"
    exit 1
}

# ── Cleanup ───────────────────────────────────────────────────────────────────

TMPDIR_CREATED=""

cleanup() {
    if [ -n "${TMPDIR_CREATED}" ] && [ -d "${TMPDIR_CREATED}" ]; then
        rm -rf "${TMPDIR_CREATED}"
    fi
}

trap cleanup EXIT INT TERM

# ── Dependency checks ─────────────────────────────────────────────────────────

check_dependencies() {
    # Need either curl or wget for HTTP requests
    if command -v curl >/dev/null 2>&1; then
        HTTP_CLIENT="curl"
    elif command -v wget >/dev/null 2>&1; then
        HTTP_CLIENT="wget"
    else
        fatal "either 'curl' or 'wget' is required but neither was found"
    fi

    # Need tar for extracting archives
    if ! command -v tar >/dev/null 2>&1; then
        fatal "'tar' is required but was not found"
    fi

    # Need a sha256 tool for checksum verification
    if command -v sha256sum >/dev/null 2>&1; then
        SHA256_CMD="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
        SHA256_CMD="shasum -a 256"
    else
        fatal "either 'sha256sum' or 'shasum' is required for checksum verification but neither was found"
    fi
}

# ── HTTP helpers ──────────────────────────────────────────────────────────────

http_get() {
    url="$1"
    if [ "${HTTP_CLIENT}" = "curl" ]; then
        curl -fsSL "$url"
    else
        wget -qO- "$url"
    fi
}

http_download() {
    url="$1"
    output="$2"
    if [ "${HTTP_CLIENT}" = "curl" ]; then
        curl -fsSL -o "$output" "$url"
    else
        wget -q -O "$output" "$url"
    fi
}

# ── Platform detection ────────────────────────────────────────────────────────

detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        CYGWIN*|MINGW*|MSYS*|Windows_NT*)
            error "Windows is not supported by this installer."
            printf "\n"
            printf "  Download a release manually from:\n"
            printf "  ${BOLD}https://github.com/%s/%s/releases${RESET}\n" "${GITHUB_OWNER}" "${GITHUB_REPO}"
            printf "\n"
            exit 1
            ;;
        *)
            fatal "unsupported operating system: ${os}"
            ;;
    esac
}

detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)   echo "arm64" ;;
        *)
            fatal "unsupported architecture: ${arch}"
            ;;
    esac
}

# ── Version resolution ────────────────────────────────────────────────────────

get_latest_version() {
    api_url="https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest"
    response="$(http_get "$api_url" 2>/dev/null)" || fatal "failed to fetch latest release from GitHub API (${api_url}). Check your internet connection."

    # Extract tag_name from JSON without requiring jq.
    # The GitHub API returns "tag_name": "v1.2.3" — we extract the value.
    version="$(printf '%s' "$response" | tr ',' '\n' | grep '"tag_name"' | head -1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"

    if [ -z "$version" ]; then
        fatal "could not determine latest version from GitHub API response"
    fi

    # Strip leading 'v' if present — archive names use bare version numbers
    printf '%s' "$version" | sed 's/^v//'
}

# ── Checksum verification ────────────────────────────────────────────────────

verify_checksum() {
    archive_path="$1"
    checksums_path="$2"
    archive_name="$3"

    expected="$(grep "  ${archive_name}\$" "$checksums_path" | cut -d ' ' -f 1)"

    if [ -z "$expected" ]; then
        # Also try single-space separator (some sha256sum implementations)
        expected="$(grep " ${archive_name}\$" "$checksums_path" | cut -d ' ' -f 1)"
    fi

    if [ -z "$expected" ]; then
        fatal "archive '${archive_name}' not found in checksums.txt"
    fi

    actual="$(${SHA256_CMD} "$archive_path" | cut -d ' ' -f 1)"

    if [ "$expected" != "$actual" ]; then
        error "checksum verification failed for ${archive_name}"
        printf "  expected: %s\n" "$expected" >&2
        printf "  actual:   %s\n" "$actual" >&2
        fatal "the downloaded file may be corrupted or tampered with"
    fi
}

# ── PATH check ────────────────────────────────────────────────────────────────

check_path() {
    install_dir="$1"

    # Check if install_dir is in PATH
    case ":${PATH}:" in
        *":${install_dir}:"*)
            return 0
            ;;
    esac

    return 1
}

path_hint() {
    install_dir="$1"
    shell_name="$(basename "${SHELL:-sh}" 2>/dev/null || echo "sh")"

    warn "${install_dir} is not in your PATH"
    printf "\n"
    printf "  Add it by appending this to your shell profile:\n"
    printf "\n"

    case "$shell_name" in
        zsh)
            printf "    ${DIM}# ~/.zshrc${RESET}\n"
            printf "    export PATH=\"%s:\$PATH\"\n" "$install_dir"
            ;;
        bash)
            printf "    ${DIM}# ~/.bashrc${RESET}\n"
            printf "    export PATH=\"%s:\$PATH\"\n" "$install_dir"
            ;;
        fish)
            printf "    ${DIM}# ~/.config/fish/config.fish${RESET}\n"
            printf "    set -gx PATH %s \$PATH\n" "$install_dir"
            ;;
        *)
            printf "    export PATH=\"%s:\$PATH\"\n" "$install_dir"
            ;;
    esac

    printf "\n"
    printf "  Then reload your shell or run:\n"
    printf "    export PATH=\"%s:\$PATH\"\n" "$install_dir"
    printf "\n"
}

# ── Main ──────────────────────────────────────────────────────────────────────

main() {
    setup_colors

    printf "\n"
    printf "  ${BOLD}meta-cli installer${RESET}\n"
    printf "  ${DIM}https://github.com/%s/%s${RESET}\n" "${GITHUB_OWNER}" "${GITHUB_REPO}"
    printf "\n"

    check_dependencies

    # Detect platform
    os="$(detect_os)"
    arch="$(detect_arch)"
    info "Detected platform: ${BOLD}${os}/${arch}${RESET}"

    # Resolve version
    if [ -n "${META_CLI_VERSION:-}" ]; then
        version="$(printf '%s' "${META_CLI_VERSION}" | sed 's/^v//')"
        info "Using specified version: ${BOLD}${version}${RESET}"
    else
        info "Fetching latest version from GitHub..."
        version="$(get_latest_version)"
        info "Latest version: ${BOLD}${version}${RESET}"
    fi

    # Resolve install directory
    install_dir="${META_CLI_INSTALL_DIR:-${DEFAULT_INSTALL_DIR}}"

    # Build artifact names
    archive_name="${GITHUB_REPO}_${version}_${os}_${arch}.tar.gz"
    base_url="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download/v${version}"
    archive_url="${base_url}/${archive_name}"
    checksums_url="${base_url}/checksums.txt"

    # Create temp directory
    TMPDIR_CREATED="$(mktemp -d)" || fatal "failed to create temporary directory"

    # Download archive and checksums
    info "Downloading ${BOLD}${archive_name}${RESET}..."
    http_download "$archive_url" "${TMPDIR_CREATED}/${archive_name}" || fatal "failed to download archive from ${archive_url}"

    info "Downloading checksums..."
    http_download "$checksums_url" "${TMPDIR_CREATED}/checksums.txt" || fatal "failed to download checksums from ${checksums_url}"

    # Verify checksum
    info "Verifying checksum..."
    verify_checksum "${TMPDIR_CREATED}/${archive_name}" "${TMPDIR_CREATED}/checksums.txt" "$archive_name"
    success "Checksum verified"

    # Extract binary
    info "Extracting binary..."
    tar -xzf "${TMPDIR_CREATED}/${archive_name}" -C "${TMPDIR_CREATED}" "${BINARY_NAME}" || fatal "failed to extract '${BINARY_NAME}' from archive"

    if [ ! -f "${TMPDIR_CREATED}/${BINARY_NAME}" ]; then
        fatal "binary '${BINARY_NAME}' not found after extraction"
    fi

    # Install
    info "Installing to ${BOLD}${install_dir}/${BINARY_NAME}${RESET}..."

    if [ ! -d "$install_dir" ]; then
        mkdir -p "$install_dir" || fatal "failed to create directory: ${install_dir}"
    fi

    mv "${TMPDIR_CREATED}/${BINARY_NAME}" "${install_dir}/${BINARY_NAME}" || fatal "failed to move binary to ${install_dir}/${BINARY_NAME}. You may need to use sudo or change META_CLI_INSTALL_DIR."
    chmod +x "${install_dir}/${BINARY_NAME}" || fatal "failed to set executable permission"

    # Done
    printf "\n"
    printf "  ${GREEN}${BOLD}meta-cli %s installed successfully${RESET}\n" "$version"
    printf "\n"

    # PATH check
    if ! check_path "$install_dir"; then
        path_hint "$install_dir"
    fi

    # Usage instructions
    printf "  ${BOLD}Get started:${RESET}\n"
    printf "\n"
    printf "    ${DIM}# Set your Meta access token${RESET}\n"
    printf "    meta config set access_token <YOUR_TOKEN>\n"
    printf "\n"
    printf "    ${DIM}# Verify authentication${RESET}\n"
    printf "    meta auth status\n"
    printf "\n"
    printf "    ${DIM}# Create a campaign${RESET}\n"
    printf "    meta campaigns create --name \"My Campaign\" --objective OUTCOME_TRAFFIC\n"
    printf "\n"

    printf "  ${BOLD}MCP server configuration:${RESET}\n"
    printf "\n"
    printf "    Add to your MCP client config (e.g. Claude Desktop):\n"
    printf "\n"
    printf "    {\n"
    printf "      \"mcpServers\": {\n"
    printf "        \"meta\": {\n"
    printf "          \"command\": \"%s\",\n" "${install_dir}/${BINARY_NAME}"
    printf "          \"args\": [\"serve\"]\n"
    printf "        }\n"
    printf "      }\n"
    printf "    }\n"
    printf "\n"
}

main "$@"
