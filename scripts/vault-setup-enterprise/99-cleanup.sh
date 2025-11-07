#!/bin/bash
# Cleanup script for Vault Enterprise development environment
# Stops all processes, removes temporary files, and resets the environment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
VAULT_ADDR="https://127.0.0.1:8200"
VAULT_AGENT_PORT=8210

echo -e "${BLUE}=========================================="
echo "Vault Enterprise Cleanup"
echo -e "==========================================${NC}"
echo ""

# Function to print status messages
print_step() {
    echo -e "${BLUE}[STEP $1]${NC} $2"
}

print_ok() {
    echo -e "${GREEN}[OK]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to kill processes by pattern
kill_processes() {
    local pattern="$1"
    local description="$2"
    
    local pids=$(pgrep -f "$pattern" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        echo "  - Found PIDs: $pids"
        echo $pids | xargs kill -TERM 2>/dev/null || true
        sleep 2
        
        # Force kill if still running
        local remaining_pids=$(pgrep -f "$pattern" 2>/dev/null || true)
        if [ -n "$remaining_pids" ]; then
            echo "  - Force killing remaining PIDs: $remaining_pids"
            echo $remaining_pids | xargs kill -KILL 2>/dev/null || true
        fi
        print_ok "$description stopped"
    else
        print_ok "$description not running"
    fi
}

# Step 1: Stop Vault Agent
print_step "1/8" "Stopping Vault Agent processes..."
kill_processes "vault agent" "Vault Agent"
echo ""

# Step 2: Stop Vault Server
print_step "2/8" "Stopping Vault Server processes..."
kill_processes "vault server" "Vault Server"
echo ""

# Step 3: Check port availability
print_step "3/8" "Checking port availability..."
if lsof -Pi :8200 -sTCP:LISTEN -t >/dev/null 2>&1; then
    print_warning "Port 8200 still in use"
    echo "  - Process using port 8200:"
    lsof -Pi :8200 -sTCP:LISTEN
else
    print_ok "Port 8200 is free"
fi

if lsof -Pi :$VAULT_AGENT_PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
    print_warning "Port $VAULT_AGENT_PORT still in use"
    echo "  - Process using port $VAULT_AGENT_PORT:"
    lsof -Pi :$VAULT_AGENT_PORT -sTCP:LISTEN
else
    print_ok "Port $VAULT_AGENT_PORT is free"
fi
echo ""

# Step 4: Remove Vault tokens and credentials
print_step "4/8" "Removing tokens and credentials..."
files_to_remove=(
    "$SCRIPT_DIR/.vault-token"
    "/tmp/vault-token-enterprise"
    "/tmp/vault-agent-enterprise.pid"
)

for file in "${files_to_remove[@]}"; do
    if [ -f "$file" ]; then
        rm -f "$file"
        echo "  - Removed: $file"
    else
        echo "  - Not found: $file"
    fi
done
print_ok "Tokens and credentials cleaned up"
echo ""

# Step 5: Remove TLS certificates (dev mode only)
print_step "5/8" "Removing development TLS certificates..."
tls_dir="$SCRIPT_DIR/.vault-tls"
if [ -d "$tls_dir" ]; then
    rm -rf "$tls_dir"
    print_ok "TLS directory removed: $tls_dir"
else
    print_ok "TLS directory not found: $tls_dir"
fi
echo ""

# Step 6: Clean up test certificates (optional)
print_step "6/8" "Cleaning up test certificates..."
test_certs_dir="$PROJECT_ROOT/scripts/test-certs"
cert_files=(
    "$test_certs_dir/ca.crt"
    "$test_certs_dir/ca-key.pem"
    "$test_certs_dir/ca.srl"
    "$test_certs_dir/client.crt"
    "$test_certs_dir/client-key.pem"
)

read -p "Remove test certificates? This will require regenerating them. [y/N]: " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    for cert_file in "${cert_files[@]}"; do
        if [ -f "$cert_file" ]; then
            rm -f "$cert_file"
            echo "  - Removed: $cert_file"
        fi
    done
    print_ok "Test certificates removed"
else
    print_ok "Test certificates kept (use ./generate-certs.sh to regenerate if needed)"
fi
echo ""

# Step 7: Clean up test data
print_step "7/8" "Cleaning up test data..."
test_data_dirs=(
    "$PROJECT_ROOT/test-data/source"
    "$PROJECT_ROOT/test-data/encrypted"
    "$PROJECT_ROOT/test-data/decrypted"
)

read -p "Remove test data directories? This will delete all test files. [y/N]: " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    for test_dir in "${test_data_dirs[@]}"; do
        if [ -d "$test_dir" ]; then
            rm -rf "$test_dir"
            echo "  - Removed: $test_dir"
        fi
    done
    print_ok "Test data directories removed"
else
    print_ok "Test data directories kept"
fi
echo ""

# Step 8: Verify cleanup
print_step "8/8" "Verifying cleanup..."

# Check processes
vault_processes=$(pgrep -f "vault" 2>/dev/null || true)
if [ -n "$vault_processes" ]; then
    print_warning "Some Vault processes may still be running:"
    ps -p $vault_processes -o pid,ppid,cmd 2>/dev/null || true
else
    print_ok "No Vault processes running"
fi

# Check ports
open_ports=""
if lsof -Pi :8200 -sTCP:LISTEN -t >/dev/null 2>&1; then
    open_ports="$open_ports 8200"
fi
if lsof -Pi :$VAULT_AGENT_PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
    open_ports="$open_ports $VAULT_AGENT_PORT"
fi

if [ -n "$open_ports" ]; then
    print_warning "Some ports still in use:$open_ports"
else
    print_ok "All ports are free"
fi

# Check files
remaining_files=()
check_files=(
    "$SCRIPT_DIR/.vault-token"
    "/tmp/vault-token-enterprise" 
    "$SCRIPT_DIR/.vault-tls/vault-ca.pem"
)

for file in "${check_files[@]}"; do
    if [ -f "$file" ]; then
        remaining_files+=("$file")
    fi
done

if [ ${#remaining_files[@]} -gt 0 ]; then
    print_warning "Some files remain:"
    printf '  - %s\n' "${remaining_files[@]}"
else
    print_ok "All temporary files removed"
fi
echo ""

echo -e "${GREEN}=========================================="
echo "[OK] Cleanup complete!"
echo -e "==========================================${NC}"
echo ""
echo "Summary of what was cleaned up:"
echo "  ✓ Vault Server processes stopped"
echo "  ✓ Vault Agent processes stopped"
echo "  ✓ Token files removed"
echo "  ✓ TLS certificates removed"
echo "  ✓ Ports freed"
echo ""
echo "To restart the development environment:"
echo "  1. ./01-start-vault-dev.sh"
echo "  2. ./02-configure-vault.sh"
echo "  3. ./03-test-setup.sh (optional)"
echo ""
echo -e "${YELLOW}Note:${NC} In dev mode, all Vault data is stored in-memory"
echo "      and is automatically lost when Vault stops."