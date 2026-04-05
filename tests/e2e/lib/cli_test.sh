#!/usr/bin/env bash
# cli_test.sh — CLI test flow for E2E tests

# cli_test_manifest <manifest_path>
# Runs a single test manifest via CLI: deploy, validate, invoke, assert, cleanup.
cli_test_manifest() {
    local manifest="$1"
    local test_name graph_dir graph_file input timeout_s
    test_name=$(jq -r '.name' "$manifest")
    local graph_name
    graph_name=$(jq -r '.graph' "$manifest")
    graph_dir="${PROJECT_ROOT}/examples/${graph_name}"
    graph_file="${graph_dir}/graph.json"
    input=$(jq -c '.input' "$manifest")
    timeout_s=$(jq -r '.timeout_seconds // 60' "$manifest")

    log_header "CLI: ${test_name}"

    if [[ ! -f "$graph_file" ]]; then
        log_fail "${test_name}: graph file not found: ${graph_file}"
        return 1
    fi

    local cli_flags="--server http://localhost:${E2E_PORT} -o json"
    local invoke_flags=""
    if [[ "${LLM_VERBOSE:-}" == "true" ]]; then
        invoke_flags="--debug"
    fi
    local run_id="${RUN_ID}"
    # Use a unique graph name to avoid collisions
    local deploy_name="e2e-cli-${test_name}-${run_id}"

    # Create a temp copy with the unique name
    local tmp_graph
    tmp_graph=$(mktemp)
    jq --arg name "$deploy_name" '.name = $name' "$graph_file" > "$tmp_graph"

    log_debug "Input: ${input}"

    # 1. Deploy
    log_info "Deploying graph: ${deploy_name}"
    local deploy_result
    local deploy_rc=0
    if [[ -n "${ENV_FILE:-}" ]]; then
        deploy_result=$($CLI_BIN deploy -f "$tmp_graph" --env-file "$ENV_FILE" $cli_flags 2>&1) || deploy_rc=$?
    else
        deploy_result=$($CLI_BIN deploy -f "$tmp_graph" $cli_flags 2>&1) || deploy_rc=$?
    fi
    rm -f "$tmp_graph"

    if [[ $deploy_rc -ne 0 ]]; then
        log_fail "${test_name}: deploy failed: ${deploy_result}"
        return 1
    fi
    log_info "Deploy output: ${deploy_result}"

    # Extract graph ID — deploy prints "Created: name (id: xxx)" or JSON
    local graph_id
    graph_id=$(echo "$deploy_result" | sed -n 's/.*id: \([^)]*\)).*/\1/p' | head -1)
    if [[ -z "$graph_id" ]]; then
        # Try JSON output
        graph_id=$(echo "$deploy_result" | jq -r '.id // empty' 2>/dev/null)
    fi
    if [[ -z "$graph_id" ]]; then
        log_fail "${test_name}: could not extract graph ID from deploy output"
        return 1
    fi
    log_info "Graph ID: ${graph_id}"

    # 2. Validate (remote)
    log_info "Validating graph..."
    local validate_result
    local validate_rc=0
    validate_result=$($CLI_BIN validate "$graph_id" $cli_flags 2>&1) || validate_rc=$?
    log_debug "Validate result: ${validate_result}"
    if [[ $validate_rc -ne 0 ]]; then
        log_fail "${test_name}: remote validation command failed: ${validate_result}"
        cleanup_graph "$graph_id"
        return 1
    fi
    if ! echo "$validate_result" | jq -e '.valid == true' > /dev/null 2>&1; then
        log_fail "${test_name}: remote validation failed: ${validate_result}"
        cleanup_graph "$graph_id"
        return 1
    fi

    # 3. List graphs — verify our graph appears
    log_info "Listing graphs..."
    local list_result
    list_result=$($CLI_BIN list graphs $cli_flags 2>&1)
    log_debug "List result: ${list_result}"

    # 4. Inspect graph
    log_info "Inspecting graph..."
    local inspect_result
    inspect_result=$($CLI_BIN inspect graph "$graph_id" $cli_flags 2>&1)
    log_verbose_json "Inspect result" "$inspect_result"

    # 5. Invoke (sync mode)
    log_info "Invoking graph (sync, timeout=${timeout_s}s)..."
    local invoke_result
    invoke_result=$($CLI_BIN invoke "$graph_id" --input "$input" --sync --timeout "$timeout_s" $invoke_flags $cli_flags 2>&1)
    if [[ $? -ne 0 ]]; then
        log_fail "${test_name}: invoke failed: ${invoke_result}"
        # Still cleanup
        cleanup_graph "$graph_id"
        return 1
    fi

    log_verbose_json "Invoke result" "$invoke_result"

    local execution_id
    execution_id=$(echo "$invoke_result" | jq -r '.id // empty' 2>/dev/null)

    # 6. Check status
    local status
    status=$(echo "$invoke_result" | jq -r '.status // empty')
    local expected_status
    expected_status=$(jq -r '.expected_status' "$manifest")
    assert_equals "$status" "$expected_status" "${test_name}: execution status"

    # 7. Run manifest assertions
    run_assertions "$manifest" "$invoke_result"

    # 8. Print LLM debug traces for debug executions.
    print_llm_debug_traces "$execution_id" || true

    # 9. Cleanup
    cleanup_graph "$graph_id"

    log_info "CLI test complete: ${test_name}"
}

# cleanup_graph <graph_id>
cleanup_graph() {
    local graph_id="$1"
    log_info "Cleaning up graph: ${graph_id}"
    curl -sf -X DELETE "http://localhost:${E2E_PORT}/api/v1/graphs/${graph_id}" > /dev/null 2>&1 || true
}
