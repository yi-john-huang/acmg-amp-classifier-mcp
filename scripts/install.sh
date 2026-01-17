#!/bin/bash
# ACMG-AMP MCP Server - One-liner Install Script
# Usage: curl -fsSL https://raw.githubusercontent.com/acmg-amp-mcp-server/main/scripts/install.sh | bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="yi-john-huang/acmg-amp-classifier-mcp"
BINARY_NAME="mcp-server-lite"
INSTALL_DIR="${HOME}/.local/bin"

echo -e "${BLUE}"
echo "╔══════════════════════════════════════════════════════════╗"
echo "║        ACMG-AMP MCP Server - Installation Script         ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin)
            OS="darwin"
            ;;
        linux)
            OS="linux"
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            echo -e "${RED}Error: Unsupported operating system: $OS${NC}"
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac

    echo -e "${GREEN}Detected platform: ${OS}/${ARCH}${NC}"
}

# Get latest release version
get_latest_version() {
    echo -e "${BLUE}Fetching latest version...${NC}"

    # Try to get the latest release
    if command -v curl &> /dev/null; then
        LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/' || echo "")
    elif command -v wget &> /dev/null; then
        LATEST_VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/' || echo "")
    fi

    if [ -z "$LATEST_VERSION" ]; then
        echo -e "${YELLOW}Warning: Could not determine latest version, using 'main' branch${NC}"
        LATEST_VERSION="main"
    else
        echo -e "${GREEN}Latest version: ${LATEST_VERSION}${NC}"
    fi
}

# Download and install binary
download_binary() {
    echo -e "${BLUE}Creating install directory: ${INSTALL_DIR}${NC}"
    mkdir -p "$INSTALL_DIR"

    local BINARY_URL
    local DOWNLOAD_FILE="$INSTALL_DIR/$BINARY_NAME"

    if [ "$LATEST_VERSION" = "main" ]; then
        # Build from source if no releases available
        echo -e "${YELLOW}No releases found. Building from source...${NC}"
        build_from_source
        return
    fi

    # Construct download URL for release binary
    BINARY_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/${BINARY_NAME}-${OS}-${ARCH}"

    if [ "$OS" = "windows" ]; then
        BINARY_URL="${BINARY_URL}.exe"
        DOWNLOAD_FILE="${DOWNLOAD_FILE}.exe"
    fi

    echo -e "${BLUE}Downloading from: ${BINARY_URL}${NC}"

    if command -v curl &> /dev/null; then
        if ! curl -fsSL "$BINARY_URL" -o "$DOWNLOAD_FILE" 2>/dev/null; then
            echo -e "${YELLOW}Binary download failed. Building from source...${NC}"
            build_from_source
            return
        fi
    elif command -v wget &> /dev/null; then
        if ! wget -q "$BINARY_URL" -O "$DOWNLOAD_FILE" 2>/dev/null; then
            echo -e "${YELLOW}Binary download failed. Building from source...${NC}"
            build_from_source
            return
        fi
    else
        echo -e "${RED}Error: curl or wget is required${NC}"
        exit 1
    fi

    chmod +x "$DOWNLOAD_FILE"
    echo -e "${GREEN}Binary installed to: ${DOWNLOAD_FILE}${NC}"
}

# Build from source as fallback
build_from_source() {
    echo -e "${BLUE}Building from source...${NC}"

    # Check for Go
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is required to build from source${NC}"
        echo -e "${YELLOW}Please install Go from https://golang.org/dl/${NC}"
        exit 1
    fi

    # Create temp directory
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    echo -e "${BLUE}Cloning repository...${NC}"
    git clone --depth 1 "https://github.com/${REPO}.git" "$TEMP_DIR/acmg-amp-mcp"

    cd "$TEMP_DIR/acmg-amp-mcp"

    echo -e "${BLUE}Building binary...${NC}"
    go build -o "$INSTALL_DIR/$BINARY_NAME" ./cmd/mcp-server-lite/

    echo -e "${GREEN}Binary built and installed to: ${INSTALL_DIR}/${BINARY_NAME}${NC}"
}

# Add to PATH if needed
setup_path() {
    local SHELL_CONFIG=""
    local PATH_LINE="export PATH=\"\$PATH:$INSTALL_DIR\""

    # Detect shell configuration file
    if [ -n "$ZSH_VERSION" ] || [ "$SHELL" = "/bin/zsh" ]; then
        SHELL_CONFIG="$HOME/.zshrc"
    elif [ -n "$BASH_VERSION" ] || [ "$SHELL" = "/bin/bash" ]; then
        if [ -f "$HOME/.bashrc" ]; then
            SHELL_CONFIG="$HOME/.bashrc"
        elif [ -f "$HOME/.bash_profile" ]; then
            SHELL_CONFIG="$HOME/.bash_profile"
        fi
    fi

    # Check if already in PATH
    if echo "$PATH" | grep -q "$INSTALL_DIR"; then
        echo -e "${GREEN}$INSTALL_DIR is already in PATH${NC}"
        return
    fi

    if [ -n "$SHELL_CONFIG" ]; then
        if ! grep -q "$INSTALL_DIR" "$SHELL_CONFIG" 2>/dev/null; then
            echo -e "${BLUE}Adding $INSTALL_DIR to PATH in $SHELL_CONFIG${NC}"
            echo "" >> "$SHELL_CONFIG"
            echo "# ACMG-AMP MCP Server" >> "$SHELL_CONFIG"
            echo "$PATH_LINE" >> "$SHELL_CONFIG"
            echo -e "${YELLOW}Please run: source $SHELL_CONFIG${NC}"
        fi
    else
        echo -e "${YELLOW}Add this to your shell configuration:${NC}"
        echo -e "${BLUE}$PATH_LINE${NC}"
    fi
}

# Run setup wizard
run_setup() {
    local BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"

    if [ "$OS" = "windows" ]; then
        BINARY_PATH="${BINARY_PATH}.exe"
    fi

    echo ""
    echo -e "${BLUE}Running setup wizard...${NC}"
    echo ""

    # Export PATH temporarily to find the binary
    export PATH="$PATH:$INSTALL_DIR"

    "$BINARY_PATH" setup wizard
}

# Main installation flow
main() {
    detect_platform
    get_latest_version
    download_binary
    setup_path

    echo ""
    echo -e "${GREEN}Installation complete!${NC}"
    echo ""

    # Ask if user wants to run setup
    read -p "Would you like to run the setup wizard now? [Y/n]: " response
    response=${response:-Y}

    case "$response" in
        [Yy]*)
            run_setup
            ;;
        *)
            echo ""
            echo -e "${BLUE}You can run the setup wizard later with:${NC}"
            echo -e "  ${BINARY_NAME} setup wizard"
            echo ""
            echo -e "${BLUE}Or manually configure Claude Desktop with:${NC}"
            echo -e "  ${BINARY_NAME} setup claude-desktop"
            echo ""
            ;;
    esac
}

# Run main function
main
