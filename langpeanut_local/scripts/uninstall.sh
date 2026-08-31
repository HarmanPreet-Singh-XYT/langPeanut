#!/usr/bin/env bash
# ==============================================================================
# langPeanut — Universal Multi-Agent Localization System
# CLI Uninstallation & Cleanup Script
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

echo -e "${BOLD}Uninstalling langPeanut CLI and cleaning system artifacts...${NC}\n"

REMOVED_COUNT=0

# ── 1. Remove System Binaries ────────────────────────────────────────────────
echo -e "${SKY}▶ Step 1/3: Removing system PATH binaries...${NC}"

CANDIDATE_PATHS=(
    "$GOPATH/bin/langPeanut"
    "$HOME/.local/bin/langPeanut"
    "$HOME/go/bin/langPeanut"
    "/usr/local/bin/langPeanut"
)

for p in "${CANDIDATE_PATHS[@]}"; do
    if [ -n "$p" ] && [ -f "$p" ]; then
        rm -f "$p"
        echo -e "  ${GREEN}✓${NC} Removed: ${BOLD}$p${NC}"
        REMOVED_COUNT=$((REMOVED_COUNT + 1))
    fi
done

# Check if command still resolves anywhere in PATH
if command -v langPeanut &> /dev/null; then
    RESOLVED_BIN="$(command -v langPeanut)"
    if [ -f "$RESOLVED_BIN" ]; then
        rm -f "$RESOLVED_BIN" 2>/dev/null || true
        echo -e "  ${GREEN}✓${NC} Removed PATH binary: ${BOLD}$RESOLVED_BIN${NC}"
        REMOVED_COUNT=$((REMOVED_COUNT + 1))
    fi
fi

if [ "$REMOVED_COUNT" -eq 0 ]; then
    echo -e "  ${MUTED}• No system PATH binary installations detected.${NC}"
fi

# ── 2. Remove Local Workspace Binaries ───────────────────────────────────────
echo -e "\n${SKY}▶ Step 2/3: Cleaning local build outputs...${NC}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/go.mod" ]; then
    PROJECT_DIR="$SCRIPT_DIR"
elif [ -f "$SCRIPT_DIR/../go.mod" ]; then
    PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
else
    PROJECT_DIR="$(pwd)"
fi

if [ -f "$PROJECT_DIR/bin/langPeanut" ]; then
    rm -f "$PROJECT_DIR/bin/langPeanut"
    echo -e "  ${GREEN}✓${NC} Removed: ${BOLD}$PROJECT_DIR/bin/langPeanut${NC}"
fi

if [ -L "$PROJECT_DIR/langPeanut" ] || [ -f "$PROJECT_DIR/langPeanut" ]; then
    rm -f "$PROJECT_DIR/langPeanut"
    echo -e "  ${GREEN}✓${NC} Removed symlink: ${BOLD}$PROJECT_DIR/langPeanut${NC}"
fi

# ── 3. Check / Purge Cache ───────────────────────────────────────────────────
echo -e "\n${SKY}▶ Step 3/3: Checking cache & temporary files...${NC}"

if [ -d "$HOME/.langpeanut" ]; then
    if [[ "$1" == "--purge" || "$1" == "--all" ]]; then
        rm -rf "$HOME/.langpeanut"
        echo -e "  ${GREEN}✓${NC} Purged configuration directory: ${BOLD}$HOME/.langpeanut${NC}"
    else
        echo -e "  ${MUTED}• Preserved user configuration directory ($HOME/.langpeanut).${NC}"
        echo -e "    ${MUTED}(To remove completely, run: ./uninstall.sh --purge)${NC}"
    fi
fi

# ── 4. Verification ──────────────────────────────────────────────────────────
echo ""
if command -v langPeanut &> /dev/null; then
    echo -e "${AMBER}! Note: 'langPeanut' command still resolves at $(command -v langPeanut).${NC}"
    echo -e "  You may need to restart your terminal shell or check your custom PATH."
else
    echo -e "${GREEN}${BOLD}✔ langPeanut CLI has been successfully uninstalled!${NC}\n"
fi
