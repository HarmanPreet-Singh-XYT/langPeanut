#!/usr/bin/env bash
# ==============================================================================
# langPeanut — Universal Multi-Agent Localization System
# First-Time Setup & CLI Installation Script
# ==============================================================================

set -e

# ANSI Colors
BOLD="\033[1m"
GREEN="\033[32m"
SKY="\033[38;5;75m"
AMBER="\033[33m"
RED="\033[31m"
MUTED="\033[38;5;244m"
NC="\033[0m"

echo -e "${SKY}${BOLD}"
cat << "BANNER"
  _                            _____                              _   
 | |                          |  __ \                            | |  
 | | __ _ _ __   __ _  _ __   | |__) |__  __ _ _ __  _   _  ___  | |_ 
 | |/ _` | '_ \ / _` || '_ \  |  ___/ _ \/ _` | '_ \| | | |/ _ \ | __|
 | | (_| | | | | (_| || |_) | | |  |  __/ (_| | | | | |_| |  __/ | |_ 
 |_|\__,_|_| |_|\__, || .__/  |_|   \___|\__,_|_| |_|\__,_|\___|  \__|
                 __/ || |                                             
                |___/ |_|    Universal Multi-Agent Localization Studio
BANNER
echo -e "${NC}"

echo -e "${BOLD}Starting langPeanut setup for first-time cloned workspace...${NC}\n"

# ── 1. Check Prerequisites ───────────────────────────────────────────────────
echo -e "${SKY}▶ Step 1/5: Checking system prerequisites...${NC}"

if ! command -v go &> /dev/null; then
    echo -e "${RED}✘ Error: Go is not installed or not in your PATH.${NC}"
    echo -e "  Please install Go (version 1.22+ recommended): https://go.dev/doc/install"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "  ${GREEN}✓${NC} Go detected: ${BOLD}${GO_VERSION}${NC}"

if ! command -v git &> /dev/null; then
    echo -e "${AMBER}! Warning: git command not found. Tree-sitter and AST operations will work, but git checkpoints require git.${NC}"
else
    echo -e "  ${GREEN}✓${NC} Git detected: ${BOLD}$(git --version)${NC}"
fi

# ── 2. Download Dependencies ─────────────────────────────────────────────────
echo -e "\n${SKY}▶ Step 2/5: Downloading Go module dependencies...${NC}"
cd "$(dirname "$0")"
go mod download
echo -e "  ${GREEN}✓${NC} All dependencies cached."

# ── 3. Compile Binary ────────────────────────────────────────────────────────
echo -e "\n${SKY}▶ Step 3/5: Compiling high-performance native binary...${NC}"
mkdir -p bin
go build -ldflags="-s -w" -o bin/langPeanut ./cmd/langPeanut
echo -e "  ${GREEN}✓${NC} Built static binary at: ${BOLD}$(pwd)/bin/langPeanut${NC}"

# Also place symlink in repository root for convenience
ln -sf bin/langPeanut langPeanut
chmod +x bin/langPeanut langPeanut

# ── 4. Install to System PATH ────────────────────────────────────────────────
echo -e "\n${SKY}▶ Step 4/5: Installing CLI binary to user PATH...${NC}"

INSTALL_DIR=""
if [ -n "$GOPATH" ] && [ -d "$GOPATH/bin" ]; then
    INSTALL_DIR="$GOPATH/bin"
elif [ -d "$HOME/.local/bin" ]; then
    INSTALL_DIR="$HOME/.local/bin"
elif [ -d "$HOME/go/bin" ]; then
    INSTALL_DIR="$HOME/go/bin"
else
    mkdir -p "$HOME/.local/bin"
    INSTALL_DIR="$HOME/.local/bin"
fi

cp bin/langPeanut "$INSTALL_DIR/langPeanut"
chmod +x "$INSTALL_DIR/langPeanut"
echo -e "  ${GREEN}✓${NC} Installed binary to: ${BOLD}$INSTALL_DIR/langPeanut${NC}"

# Check if INSTALL_DIR is in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "\n  ${AMBER}! Note: ${INSTALL_DIR} is not in your current \$PATH.${NC}"
    echo -e "  Add it to your shell profile (~/.zshrc or ~/.bashrc):"
    echo -e "    ${BOLD}export PATH=\"\$PATH:${INSTALL_DIR}\"${NC}"
fi

# ── 5. Environment & Configuration ───────────────────────────────────────────
echo -e "\n${SKY}▶ Step 5/5: Configuring environment settings...${NC}"
if [ ! -f .env ]; then
    if [ -f .env.example ]; then
        cp .env.example .env
        echo -e "  ${GREEN}✓${NC} Created ${BOLD}.env${NC} from template. Add your API keys (OPENAI_API_KEY, ANTHROPIC_API_KEY, or GEMINI_API_KEY) for frontier models."
    fi
else
    echo -e "  ${GREEN}✓${NC} Existing ${BOLD}.env${NC} file preserved."
fi

# ── 6. Verification & Complete ───────────────────────────────────────────────
echo -e "\n${GREEN}${BOLD}✔ langPeanut CLI installation completed successfully!${NC}\n"

echo -e "${BOLD}Quick Start Commands:${NC}"
echo -e "  ${SKY}langPeanut${NC}                     Launch interactive Bubble Tea TUI app"
echo -e "  ${SKY}langPeanut web${NC}                 Launch Web Studio with shadcn & Copilot (${MUTED}http://localhost:3000${NC})"
echo -e "  ${SKY}langPeanut run ./examples/nextjs-app${NC}  Execute 1-click autonomous localization"
echo -e "  ${SKY}langPeanut audit${NC}               Audit codebase for hardcoded AST strings"
echo -e "  ${SKY}langPeanut chat${NC}                Start Agentic Chat Copilot in terminal"
echo -e "  ${SKY}langPeanut benchmark${NC}           Run 10-Case Adversarial Benchmark"
echo -e "\n${MUTED}Documentation: file://$(pwd)/README.md${NC}\n"
