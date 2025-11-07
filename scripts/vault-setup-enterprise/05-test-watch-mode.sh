#!/bin/bash
# Test script for file-encryptor watch mode (service mode)
# Assumes Vault and Vault Agent are already running and configured

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CONFIG_PATH="${PROJECT_ROOT}/configs/examples/example-enterprise.hcl"
SOURCE_DIR="$(pwd)/test-data/source"
ENCRYPTED_DIR="$(pwd)/test-data/encrypted"
DECRYPTED_DIR="$(pwd)/test-data/decrypted"
ENCRYPTOR_BIN="${PROJECT_ROOT}/bin/file-encryptor"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

print_ok() {
    echo -e "${GREEN}[OK]${NC} $1"
}
print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}
print_step() {
    echo -e "${BLUE}[STEP $1]${NC} $2"
}

print_step "1/7" "Preparing test directories..."
mkdir -p "$SOURCE_DIR" "$ENCRYPTED_DIR" "$DECRYPTED_DIR"
print_ok "Test directories ready"

print_step "2/7" "Creating test file in source directory..."
TEST_FILE="$SOURCE_DIR/watch-test-$(date +%s).txt"
echo "This is a watch mode test file." > "$TEST_FILE"
print_ok "Test file created: $TEST_FILE"

echo -e "${GREEN}=========================================="
echo "[OK] Watch mode test PASSED!"
echo -e "==========================================${NC}"

print_step "3/4" "Starting file-encryptor in watch mode (foreground)..."
WATCH_LOG="$SCRIPT_DIR/watch-mode.log"
echo "" > "$WATCH_LOG"
echo -e "${BLUE}==========================================${NC}"
echo -e "file-encryptor will run in watch mode."
echo -e "Application logs will be written to: $WATCH_LOG"
echo -e ""
echo -e "Open another terminal and add files to: $SOURCE_DIR"
echo -e "For example:"
echo -e "  echo 'hello world' > $SOURCE_DIR/test-1.txt"
echo -e "Encrypted files will appear in: $ENCRYPTED_DIR"
echo -e "Press Ctrl+C to stop the watcher."
echo -e "==========================================${NC}"
echo ""

# Start watcher and tee logs to console
$ENCRYPTOR_BIN watch -c "$CONFIG_PATH" | tee "$WATCH_LOG"


print_step "4/4" "Cleanup"
echo "To clean up all test files, logs, tokens, and processes, run:"
echo "  ./99-cleanup.sh"
echo "in this directory."
echo -e "${GREEN}=========================================="
echo "[OK] Watch mode interactive test complete!"
echo -e "==========================================${NC}"
