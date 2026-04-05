#!/usr/bin/env bash
# common.sh — Shared assertion functions, logging, and manifest runner

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Counters
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

log_info()  { [[ "${LLM_VERBOSE:-}" == "true" ]] || echo -e "${BLUE}[INFO]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_pass()  { echo -e "${GREEN}[PASS]${NC}  $*"; PASS_COUNT=$((PASS_COUNT + 1)); }
log_fail()  { echo -e "${RED}[FAIL]${NC}  $*"; FAIL_COUNT=$((FAIL_COUNT + 1)); }
log_skip()  { echo -e "${YELLOW}[SKIP]${NC}  $*"; SKIP_COUNT=$((SKIP_COUNT + 1)); }
log_header(){ echo -e "\n${BOLD}═══ $* ═══${NC}\n"; }
log_debug() { [[ "${VERBOSE:-}" == "true" ]] && echo -e "${BLUE}[DEBUG]${NC} $*" || true; }
log_verbose_json() {
    local label="$1" json="$2"
    if [[ "${VERBOSE:-}" == "true" ]]; then
        echo -e "${BLUE}[DEBUG]${NC} ${label}:"
        echo "$json" | jq '.' 2>/dev/null || echo "$json"
    fi
}

# assert_equals <actual> <expected> <label>
assert_equals() {
    local actual="$1" expected="$2" label="$3"
    if [[ "$actual" == "$expected" ]]; then
        log_pass "$label"
        return 0
    else
        log_fail "$label: expected '$expected', got '$actual'"
        return 1
    fi
}

# assert_not_empty <value> <label>
assert_not_empty() {
    local value="$1" label="$2"
    if [[ -n "$value" && "$value" != "null" ]]; then
        log_pass "$label"
        return 0
    else
        log_fail "$label: value is empty or null"
        return 1
    fi
}

# assert_type <value> <expected_type> <label>
# expected_type: string, number, object, array, boolean
assert_type() {
    local value="$1" expected_type="$2" label="$3"
    local actual_type
    actual_type=$(echo "$value" | jq -r 'type' 2>/dev/null)
    if [[ "$actual_type" == "$expected_type" ]]; then
        log_pass "$label"
        return 0
    else
        log_fail "$label: expected type '$expected_type', got '$actual_type'"
        return 1
    fi
}

# assert_contains <haystack> <needle> <label>
assert_contains() {
    local haystack="$1" needle="$2" label="$3"
    if [[ "$haystack" == *"$needle"* ]]; then
        log_pass "$label"
        return 0
    else
        log_fail "$label: '$haystack' does not contain '$needle'"
        return 1
    fi
}

# assert_gte <actual> <min> <label>
assert_gte() {
    local actual="$1" min="$2" label="$3"
    if [[ "$actual" -ge "$min" ]]; then
        log_pass "$label"
        return 0
    else
        log_fail "$label: $actual < $min"
        return 1
    fi
}

# assert_lte <actual> <max> <label>
assert_lte() {
    local actual="$1" max="$2" label="$3"
    if [[ "$actual" -le "$max" ]]; then
        log_pass "$label"
        return 0
    else
        log_fail "$label: $actual > $max"
        return 1
    fi
}

# run_assertions <manifest_path> <execution_result_json>
run_assertions() {
    local manifest="$1"
    local result="$2"
    local test_name
    test_name=$(jq -r '.name' "$manifest")

    local assertion_count
    assertion_count=$(jq '.assertions | length' "$manifest")

    for ((i = 0; i < assertion_count; i++)); do
        local atype field value substring json_type min max
        atype=$(jq -r ".assertions[$i].type" "$manifest")

        case "$atype" in
            output_equals)
                field=$(jq -r ".assertions[$i].field" "$manifest")
                value=$(jq -r ".assertions[$i].value" "$manifest")
                local actual
                actual=$(echo "$result" | jq -r ".output.${field} // empty")
                assert_equals "$actual" "$value" "${test_name}: output.${field} == ${value}"
                ;;
            output_contains)
                field=$(jq -r ".assertions[$i].field" "$manifest")
                substring=$(jq -r ".assertions[$i].substring" "$manifest")
                local actual_val
                actual_val=$(echo "$result" | jq -r ".output.${field} // empty")
                assert_contains "$actual_val" "$substring" "${test_name}: output.${field} contains '${substring}'"
                ;;
            output_exists)
                field=$(jq -r ".assertions[$i].field" "$manifest")
                local exists_val
                exists_val=$(echo "$result" | jq -r ".output.${field} // empty")
                assert_not_empty "$exists_val" "${test_name}: output.${field} exists"
                ;;
            output_type)
                field=$(jq -r ".assertions[$i].field" "$manifest")
                json_type=$(jq -r ".assertions[$i].json_type" "$manifest")
                local type_val
                type_val=$(echo "$result" | jq ".output.${field}")
                assert_type "$type_val" "$json_type" "${test_name}: output.${field} is ${json_type}"
                ;;
            state_equals)
                field=$(jq -r ".assertions[$i].field" "$manifest")
                value=$(jq -r ".assertions[$i].value" "$manifest")
                local state_val
                state_val=$(echo "$result" | jq -r ".state.${field} // empty")
                assert_equals "$state_val" "$value" "${test_name}: state.${field} == ${value}"
                ;;
            state_exists)
                field=$(jq -r ".assertions[$i].field" "$manifest")
                local state_exists_val
                state_exists_val=$(echo "$result" | jq -r ".state.${field} // empty")
                assert_not_empty "$state_exists_val" "${test_name}: state.${field} exists"
                ;;
            step_count)
                min=$(jq -r ".assertions[$i].min // 0" "$manifest")
                max=$(jq -r ".assertions[$i].max // 999" "$manifest")
                local step_count
                step_count=$(echo "$result" | jq '.steps | length')
                assert_gte "$step_count" "$min" "${test_name}: step_count >= ${min}"
                assert_lte "$step_count" "$max" "${test_name}: step_count <= ${max}"
                ;;
            status_equals)
                value=$(jq -r ".assertions[$i].value" "$manifest")
                local status
                status=$(echo "$result" | jq -r '.status // empty')
                assert_equals "$status" "$value" "${test_name}: status == ${value}"
                ;;
            *)
                log_fail "${test_name}: unknown assertion type '${atype}'"
                ;;
        esac
    done
}

# print_summary
print_summary() {
    echo ""
    log_header "Test Summary"
    echo -e "  ${GREEN}Passed:${NC}  ${PASS_COUNT}"
    echo -e "  ${RED}Failed:${NC}  ${FAIL_COUNT}"
    echo -e "  ${YELLOW}Skipped:${NC} ${SKIP_COUNT}"
    echo ""
    if [[ $FAIL_COUNT -gt 0 ]]; then
        echo -e "  ${RED}${BOLD}RESULT: FAILED${NC}"
        return 1
    else
        echo -e "  ${GREEN}${BOLD}RESULT: PASSED${NC}"
        return 0
    fi
}

# run_with_retry <test_func> <manifest_path>
# Runs a test function and retries up to 2 additional times if assertions fail.
# Counters are reset to pre-test values before each retry so only the final
# attempt's results are counted.
run_with_retry() {
    local test_func="$1"
    local manifest_path="$2"
    local max_retries=2

    local saved_pass=$PASS_COUNT
    local saved_fail=$FAIL_COUNT

    $test_func "$manifest_path" || true

    local attempt=1
    while [[ $FAIL_COUNT -gt $saved_fail && $attempt -le $max_retries ]]; do
        local test_name
        test_name=$(jq -r '.name' "$manifest_path")

        # Back off before retrying — gives LLM rate limits time to reset
        local delay=$((attempt * 15))
        log_info "Waiting ${delay}s before retry ${attempt}/${max_retries} for ${test_name}..."
        sleep "$delay"

        log_info "Retry ${attempt}/${max_retries} for ${test_name} (previous attempt had failures)..."

        # Reset counters to before the test
        PASS_COUNT=$saved_pass
        FAIL_COUNT=$saved_fail

        # Use a fresh RUN_ID so deploy names don't collide with previous attempts
        RUN_ID="$(date +%s)"
        export RUN_ID

        $test_func "$manifest_path" || true
        attempt=$((attempt + 1))
    done
}

fetch_execution_steps() {
    local execution_id="$1"
    curl -sf "http://localhost:${E2E_PORT}/api/v1/executions/${execution_id}/steps"
}

print_llm_debug_traces() {
    local execution_id="$1"
    local step_json

    if [[ "${LLM_VERBOSE:-}" != "true" || -z "$execution_id" ]]; then
        return 0
    fi

    if ! step_json=$(fetch_execution_steps "$execution_id" 2>/dev/null); then
        log_error "Failed to fetch execution steps for ${execution_id}"
        return 1
    fi

    if ! echo "$step_json" | jq -e '.items[]? | select(.llm_debug != null and (.llm_debug.calls | length) > 0)' > /dev/null 2>&1; then
        return 0
    fi

    echo ""
    echo "LLM traces for execution ${execution_id}:"
    echo "$step_json" | jq -r '
        .items[]
        | select(.llm_debug != null and (.llm_debug.calls | length) > 0)
        | . as $step
        | .llm_debug.calls[]
        | "----- " + $step.node_id + " (" + $step.node_type + ")" +
          (if .request_id != "" then " [" + .request_id + "]" else "" end) +
          " | " + .provider + " / " + .model + " -----\n" +
          "Request:\n" + (.request | tojson | fromjson | tostring) + "\n" +
          "Response:\n" + (.response | tojson | fromjson | tostring) + "\n"
    ' | while IFS= read -r line; do
        if [[ "$line" == "Request:" || "$line" == "Response:" || "$line" == -----* || -z "$line" ]]; then
            echo "$line"
            continue
        fi
        echo "$line" | jq '.' 2>/dev/null || echo "$line"
    done
}
