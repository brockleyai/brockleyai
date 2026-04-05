#!/usr/bin/env bash
set -euo pipefail

# run.sh — E2E test orchestrator for Brockley
#
# Usage:
#   ./tests/e2e/run.sh [OPTIONS]
#
# Options:
#   --env-file <path>   Load environment variables from file (default: .env if exists)
#   --no-llm            Skip graphs that require an LLM provider
#   --no-mcp            Skip graphs that require an MCP test server
#   --cli-only          Run only CLI tests (skip Terraform)
#   --tf-only           Run only Terraform tests (skip CLI)
#   --no-stack          Don't start/stop Docker stack (assume it's running)
#   -v, --verbose       Enable verbose output (debug logs, full command output)
#   --llm-verbose       Run only LLM manifests and print LLM request/response traces
#
# Environment:
#   ONLY=<graph>        Run only the specified graph (e.g., ONLY=comprehensive)
#   E2E_PORT=<port>     Server port (default: 18000)
#   VERBOSE=true        Same as --verbose

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
E2E_DIR="${SCRIPT_DIR}"

# Defaults
ENV_FILE=""
NO_LLM=false
NO_MCP=false
NO_API_SERVER=false
CLI_ONLY=false
TF_ONLY=false
NO_STACK=false
VERBOSE="${VERBOSE:-false}"
LLM_VERBOSE="${LLM_VERBOSE:-false}"
E2E_PORT="${E2E_PORT:-18000}"
RUN_ID="$(date +%s)"
CLI_BIN=""
TF_PLUGIN_DIR=""

# Parse args
while [[ $# -gt 0 ]]; do
    case "$1" in
        --env-file)
            ENV_FILE="$2"; shift 2 ;;
        --no-llm)
            NO_LLM=true; shift ;;
        --no-mcp)
            NO_MCP=true; shift ;;
        --no-api-server)
            NO_API_SERVER=true; shift ;;
        --cli-only)
            CLI_ONLY=true; shift ;;
        --tf-only)
            TF_ONLY=true; shift ;;
        --no-stack)
            NO_STACK=true; shift ;;
        --llm-verbose)
            LLM_VERBOSE=true; shift ;;
        -v|--verbose)
            VERBOSE=true; shift ;;
        *)
            echo "Unknown option: $1"; exit 1 ;;
    esac
done

# Export for lib scripts
export E2E_DIR E2E_PORT RUN_ID ENV_FILE VERBOSE LLM_VERBOSE PROJECT_ROOT

# Source library scripts
source "${E2E_DIR}/lib/common.sh"
source "${E2E_DIR}/lib/stack.sh"
source "${E2E_DIR}/lib/cli_test.sh"
source "${E2E_DIR}/lib/tf_test.sh"

# Auto-detect .env file
if [[ -z "$ENV_FILE" && -f "${PROJECT_ROOT}/.env" ]]; then
    ENV_FILE="${PROJECT_ROOT}/.env"
    log_info "Auto-detected env file: ${ENV_FILE}"
fi

# Load env file if specified
if [[ -n "$ENV_FILE" ]]; then
    if [[ ! -f "$ENV_FILE" ]]; then
        log_error "Env file not found: ${ENV_FILE}"
        exit 1
    fi
    log_info "Loading env file: ${ENV_FILE}"
    set -a
    source "$ENV_FILE"
    set +a
fi

# EXIT trap — always tear down stack
cleanup() {
    local exit_code=$?
    if [[ "$NO_STACK" != "true" ]]; then
        if [[ "$VERBOSE" == "true" ]]; then
            stack_down || true
        else
            stack_down 2>/dev/null || true
        fi
    fi
    exit $exit_code
}
trap cleanup EXIT INT TERM

log_header "Brockley E2E Tests"
log_info "Run ID: ${RUN_ID}"
log_info "Port: ${E2E_PORT}"
log_info "No LLM: ${NO_LLM}"
log_info "No MCP: ${NO_MCP}"
log_info "CLI only: ${CLI_ONLY}"
log_info "TF only: ${TF_ONLY}"
log_info "Verbose: ${VERBOSE}"
log_info "LLM verbose: ${LLM_VERBOSE}"

# When verbose, override service log levels to debug.
if [[ "$VERBOSE" == "true" ]]; then
    export BROCKLEY_LOG_LEVEL="debug"
fi

# 1. Build CLI binary
log_info "Building CLI binary..."
CLI_BIN="${PROJECT_ROOT}/bin/brockley-e2e"
(cd "$PROJECT_ROOT" && go build -o "$CLI_BIN" ./cmd/brockley)
export CLI_BIN
log_info "CLI binary: ${CLI_BIN}"

# 2. Build Terraform provider (if not --cli-only)
if [[ "$CLI_ONLY" != "true" ]]; then
    log_info "Building Terraform provider..."
    TF_PLUGIN_DIR="${PROJECT_ROOT}/bin/tf-plugins"
    local_arch_dir="${TF_PLUGIN_DIR}/registry.terraform.io/brockleyai/brockley/0.0.1/$(go env GOOS)_$(go env GOARCH)"
    mkdir -p "$local_arch_dir"
    (cd "$PROJECT_ROOT" && go build -o "${local_arch_dir}/terraform-provider-brockley" ./terraform-provider)
    export TF_PLUGIN_DIR
    log_info "TF provider: ${local_arch_dir}/terraform-provider-brockley"
fi

# 3. Start stack (unless --no-stack)
if [[ "$NO_STACK" != "true" ]]; then
    stack_up || exit 1
    stack_wait_healthy || exit 1
else
    log_info "Skipping stack startup (--no-stack)"
    # Verify server is healthy
    if ! curl -sf "http://localhost:${E2E_PORT}/health" > /dev/null 2>&1; then
        log_error "Server not healthy at http://localhost:${E2E_PORT}/health"
        exit 1
    fi
fi

# 3b. Deploy API tool definitions (if any exist)
API_TOOLS_DIR="${E2E_DIR}/api-tools"
if [[ -d "$API_TOOLS_DIR" ]]; then
    for api_tool_file in "${API_TOOLS_DIR}"/*.json; do
        [[ -f "$api_tool_file" ]] || continue
        local_name=$(basename "$api_tool_file" .json)
        log_info "Deploying API tool definition: ${local_name}"
        deploy_result=$($CLI_BIN deploy -f "$api_tool_file" --server "http://localhost:${E2E_PORT}" -o json 2>&1) || true
        log_info "API tool deploy: ${deploy_result}"
    done
fi

# 4. Collect manifests
manifests=()
for manifest_file in "${E2E_DIR}"/manifests/*.json; do
    [[ -f "$manifest_file" ]] || continue

    local_graph=$(jq -r '.graph' "$manifest_file")
    local_requires_llm=$(jq -r '.requires_llm // false' "$manifest_file")
    local_requires_mcp=$(jq -r '.requires_mcp // false' "$manifest_file")
    local_requires_api_server=$(jq -r '.requires_api_server // false' "$manifest_file")

    # Filter by ONLY
    if [[ -n "${ONLY:-}" && "$local_graph" != "$ONLY" ]]; then
        continue
    fi

    # Filter by llm-verbose mode
    if [[ "$LLM_VERBOSE" == "true" && "$local_requires_llm" != "true" ]]; then
        continue
    fi

    # Filter by --no-llm
    if [[ "$NO_LLM" == "true" && "$local_requires_llm" == "true" ]]; then
        test_name=$(jq -r '.name' "$manifest_file")
        log_skip "${test_name} (requires LLM)"
        continue
    fi

    # Filter by --no-mcp
    if [[ "$NO_MCP" == "true" && "$local_requires_mcp" == "true" ]]; then
        test_name=$(jq -r '.name' "$manifest_file")
        log_skip "${test_name} (requires MCP)"
        continue
    fi

    # Filter by --no-api-server
    if [[ "$NO_API_SERVER" == "true" && "$local_requires_api_server" == "true" ]]; then
        test_name=$(jq -r '.name' "$manifest_file")
        log_skip "${test_name} (requires API server)"
        continue
    fi

    # Check LLM API key availability
    if [[ "$local_requires_llm" == "true" && -z "${OPENROUTER_API_KEY:-}" ]]; then
        test_name=$(jq -r '.name' "$manifest_file")
        log_skip "${test_name} (OPENROUTER_API_KEY not set)"
        continue
    fi

    manifests+=("$manifest_file")
done

if [[ ${#manifests[@]} -eq 0 ]]; then
    log_info "No test manifests to run"
    print_summary
    exit 0
fi

log_info "Running ${#manifests[@]} manifest(s)"

# 5. Run CLI tests
if [[ "$TF_ONLY" != "true" ]]; then
    for manifest_file in "${manifests[@]}"; do
        run_with_retry cli_test_manifest "$manifest_file"
    done
fi

# 6. Run TF tests
if [[ "$CLI_ONLY" != "true" ]]; then
    # Brief pause between CLI and TF paths to avoid OpenRouter rate limiting
    # on LLM tests that share the same API key.
    if [[ "$TF_ONLY" != "true" ]]; then
        has_llm=false
        for manifest_file in "${manifests[@]}"; do
            if [[ "$(jq -r '.requires_llm // false' "$manifest_file")" == "true" ]]; then
                has_llm=true
                break
            fi
        done
        if [[ "$has_llm" == "true" ]]; then
            log_info "Pausing 10s between CLI and TF paths (LLM rate limit cooldown)..."
            sleep 10
        fi
    fi
    for manifest_file in "${manifests[@]}"; do
        run_with_retry tf_test_manifest "$manifest_file"
    done
fi

# 7. Summary
print_summary
