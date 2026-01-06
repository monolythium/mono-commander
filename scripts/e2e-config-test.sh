#!/bin/bash
# E2E Test: Canonical Network Configuration Flow
# Tests the single-writer model for network configuration
#
# Prerequisites:
# - monoctl binary in PATH
# - monod binary in PATH (for final validation)
# - Write access to $HOME/.monod (default) or custom --home
#
# Usage:
#   ./e2e-config-test.sh [--home <path>] [--network <network>]
#
# Default: Sprintnet network, ~/.monod home directory

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
HOME_DIR="${HOME}/.monod"
NETWORK="Sprintnet"
VERBOSE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --home)
            HOME_DIR="$2"
            shift 2
            ;;
        --network)
            NETWORK="$2"
            shift 2
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--home <path>] [--network <network>] [--verbose]"
            echo ""
            echo "Options:"
            echo "  --home      Node home directory (default: ~/.monod)"
            echo "  --network   Network to test (default: Sprintnet)"
            echo "  --verbose   Show detailed output"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Expected EVM chain IDs
declare -A EXPECTED_EVM_CHAIN_ID=(
    ["Localnet"]=262145
    ["Sprintnet"]=262146
    ["Testnet"]=262147
    ["Mainnet"]=262148
)

# Expected Cosmos chain IDs
declare -A EXPECTED_COSMOS_CHAIN_ID=(
    ["Localnet"]="mono-local-1"
    ["Sprintnet"]="mono-sprint-1"
    ["Testnet"]="mono-test-1"
    ["Mainnet"]="mono-1"
)

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_test() {
    echo -e "\n${GREEN}[TEST]${NC} $1"
}

pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    ((TESTS_PASSED++))
}

fail() {
    echo -e "  ${RED}✗${NC} $1"
    ((TESTS_FAILED++))
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v monoctl &> /dev/null; then
        log_error "monoctl not found in PATH"
        exit 1
    fi
    pass "monoctl found: $(which monoctl)"

    # Check network is valid
    if [[ -z "${EXPECTED_EVM_CHAIN_ID[$NETWORK]:-}" ]]; then
        log_error "Unknown network: $NETWORK"
        exit 1
    fi
    pass "Network $NETWORK is valid"
}

# Test 1: Fresh join
test_fresh_join() {
    log_test "Test 1: Fresh join to $NETWORK"

    # Clean existing config if present
    if [[ -d "$HOME_DIR" ]]; then
        log_warn "Removing existing home directory: $HOME_DIR"
        rm -rf "$HOME_DIR"
    fi

    # Join network
    log_info "Running: monoctl join --network $NETWORK --home $HOME_DIR"
    if monoctl join --network "$NETWORK" --home "$HOME_DIR"; then
        pass "monoctl join completed successfully"
    else
        fail "monoctl join failed"
        return 1
    fi

    # Verify config files exist
    for file in client.toml app.toml config.toml; do
        if [[ -f "$HOME_DIR/config/$file" ]]; then
            pass "$file exists"
        else
            fail "$file not found"
        fi
    done
}

# Test 2: Doctor with no drift
test_doctor_no_drift() {
    log_test "Test 2: Doctor with no drift"

    log_info "Running: monoctl doctor --network $NETWORK --home $HOME_DIR"
    output=$(monoctl doctor --network "$NETWORK" --home "$HOME_DIR" 2>&1) || true

    if echo "$output" | grep -qi "no drift"; then
        pass "No drift detected after fresh join"
    else
        fail "Drift detected after fresh join"
        if $VERBOSE; then
            echo "$output"
        fi
    fi
}

# Test 3: Verify correct values
test_verify_values() {
    log_test "Test 3: Verify configuration values"

    local expected_evm="${EXPECTED_EVM_CHAIN_ID[$NETWORK]}"
    local expected_cosmos="${EXPECTED_COSMOS_CHAIN_ID[$NETWORK]}"

    # Check EVM chain ID
    local actual_evm=$(grep -E "^evm-chain-id" "$HOME_DIR/config/app.toml" | grep -oE '[0-9]+' || echo "not found")
    if [[ "$actual_evm" == "$expected_evm" ]]; then
        pass "EVM chain ID is correct: $actual_evm"
    else
        fail "EVM chain ID mismatch: expected $expected_evm, got $actual_evm"
    fi

    # Check Cosmos chain ID
    local actual_cosmos=$(grep -E "^chain-id" "$HOME_DIR/config/client.toml" | sed 's/.*= *"\?\([^"]*\)"\?/\1/' || echo "not found")
    if [[ "$actual_cosmos" == "$expected_cosmos" ]]; then
        pass "Cosmos chain ID is correct: $actual_cosmos"
    else
        fail "Cosmos chain ID mismatch: expected $expected_cosmos, got $actual_cosmos"
    fi
}

# Test 4: Introduce drift
test_introduce_drift() {
    log_test "Test 4: Introduce configuration drift"

    # Backup original files
    cp "$HOME_DIR/config/app.toml" "$HOME_DIR/config/app.toml.backup"
    cp "$HOME_DIR/config/client.toml" "$HOME_DIR/config/client.toml.backup"

    # Introduce EVM chain ID drift (set to Localnet value)
    sed -i.bak "s/evm-chain-id = ${EXPECTED_EVM_CHAIN_ID[$NETWORK]}/evm-chain-id = 262145/" "$HOME_DIR/config/app.toml"
    pass "Introduced EVM chain ID drift: 262145 (Localnet leak)"

    # Introduce Cosmos chain ID drift
    sed -i.bak 's/chain-id = "mono-sprint-1"/chain-id = "mono-local-1"/' "$HOME_DIR/config/client.toml"
    pass "Introduced Cosmos chain ID drift: mono-local-1"
}

# Test 5: Detect drift
test_detect_drift() {
    log_test "Test 5: Detect configuration drift"

    log_info "Running: monoctl doctor --network $NETWORK --home $HOME_DIR"
    output=$(monoctl doctor --network "$NETWORK" --home "$HOME_DIR" 2>&1) || true

    if echo "$output" | grep -qi "CRITICAL"; then
        pass "Critical drift detected"
    else
        fail "Failed to detect critical drift"
    fi

    if echo "$output" | grep -qi "evm-chain-id"; then
        pass "EVM chain ID drift detected"
    else
        fail "Failed to detect EVM chain ID drift"
    fi

    if $VERBOSE; then
        echo "$output"
    fi
}

# Test 6: Repair drift (dry run)
test_repair_dry_run() {
    log_test "Test 6: Repair drift (dry run)"

    log_info "Running: monoctl repair --network $NETWORK --home $HOME_DIR --dry-run"
    output=$(monoctl repair --network "$NETWORK" --home "$HOME_DIR" --dry-run 2>&1) || true

    if echo "$output" | grep -qi "DRY RUN"; then
        pass "Dry run mode indicated"
    fi

    # Verify files weren't actually changed
    local actual_evm=$(grep -E "^evm-chain-id" "$HOME_DIR/config/app.toml" | grep -oE '[0-9]+' || echo "not found")
    if [[ "$actual_evm" == "262145" ]]; then
        pass "Files not modified during dry run"
    else
        fail "Files were modified during dry run"
    fi
}

# Test 7: Repair drift (actual)
test_repair_actual() {
    log_test "Test 7: Repair drift (actual)"

    log_info "Running: monoctl repair --network $NETWORK --home $HOME_DIR"
    if monoctl repair --network "$NETWORK" --home "$HOME_DIR"; then
        pass "Repair completed successfully"
    else
        fail "Repair command failed"
        return 1
    fi

    # Verify EVM chain ID was repaired
    local expected_evm="${EXPECTED_EVM_CHAIN_ID[$NETWORK]}"
    local actual_evm=$(grep -E "^evm-chain-id" "$HOME_DIR/config/app.toml" | grep -oE '[0-9]+' || echo "not found")
    if [[ "$actual_evm" == "$expected_evm" ]]; then
        pass "EVM chain ID repaired: $actual_evm"
    else
        fail "EVM chain ID not repaired: expected $expected_evm, got $actual_evm"
    fi
}

# Test 8: Doctor after repair
test_doctor_after_repair() {
    log_test "Test 8: Doctor after repair (no drift expected)"

    log_info "Running: monoctl doctor --network $NETWORK --home $HOME_DIR"
    output=$(monoctl doctor --network "$NETWORK" --home "$HOME_DIR" 2>&1) || true

    if echo "$output" | grep -qi "no drift"; then
        pass "No drift detected after repair"
    else
        fail "Drift still detected after repair"
        if $VERBOSE; then
            echo "$output"
        fi
    fi
}

# Test 9: Localnet leak prevention
test_localnet_leak_prevention() {
    log_test "Test 9: Localnet leak prevention"

    # This test only applies to non-Localnet networks
    if [[ "$NETWORK" == "Localnet" ]]; then
        log_info "Skipping Localnet leak test for Localnet network"
        return 0
    fi

    # Try to manually set EVM chain ID to Localnet value
    sed -i.bak "s/evm-chain-id = ${EXPECTED_EVM_CHAIN_ID[$NETWORK]}/evm-chain-id = 262145/" "$HOME_DIR/config/app.toml"

    # Doctor should detect this as FATAL
    output=$(monoctl doctor --network "$NETWORK" --home "$HOME_DIR" 2>&1) || true

    if echo "$output" | grep -qiE "(CRITICAL|FATAL|Localnet)"; then
        pass "Localnet EVM chain ID leak detected"
    else
        fail "Failed to detect Localnet EVM chain ID leak"
    fi

    # Repair it
    monoctl repair --network "$NETWORK" --home "$HOME_DIR" > /dev/null 2>&1 || true
}

# Print summary
print_summary() {
    echo ""
    echo "========================================"
    echo "           TEST SUMMARY"
    echo "========================================"
    echo ""
    echo -e "  ${GREEN}Passed:${NC} $TESTS_PASSED"
    echo -e "  ${RED}Failed:${NC} $TESTS_FAILED"
    echo ""

    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}All tests passed!${NC}"
        return 0
    else
        echo -e "${RED}Some tests failed.${NC}"
        return 1
    fi
}

# Cleanup
cleanup() {
    log_info "Cleaning up test artifacts..."
    rm -f "$HOME_DIR/config/"*.bak
    rm -f "$HOME_DIR/config/"*.backup
}

# Main
main() {
    echo "========================================"
    echo "  E2E Test: Canonical Network Config"
    echo "========================================"
    echo ""
    echo "Network: $NETWORK"
    echo "Home: $HOME_DIR"
    echo ""

    check_prerequisites

    test_fresh_join
    test_doctor_no_drift
    test_verify_values
    test_introduce_drift
    test_detect_drift
    test_repair_dry_run
    test_repair_actual
    test_doctor_after_repair
    test_localnet_leak_prevention

    cleanup
    print_summary
}

main "$@"
