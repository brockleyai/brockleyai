#!/usr/bin/env bash
# tf_test.sh — Terraform test flow for E2E tests
#
# Uses the static graph.tf files from each graph directory rather than
# generating Terraform configs on the fly.

# tf_test_manifest <manifest_path>
# Runs a single test manifest via Terraform: init, plan, apply, invoke via API, assert, destroy.
tf_test_manifest() {
    local manifest="$1"
    local test_name graph_name graph_tf input timeout_s
    test_name=$(jq -r '.name' "$manifest")
    graph_name=$(jq -r '.graph' "$manifest")
    graph_tf="${PROJECT_ROOT}/examples/${graph_name}/graph.tf"
    input=$(jq -c '.input' "$manifest")
    timeout_s=$(jq -r '.timeout_seconds // 60' "$manifest")

    log_header "TF: ${test_name}"

    if [[ ! -f "$graph_tf" ]]; then
        log_fail "${test_name}: graph.tf not found: ${graph_tf}"
        return 1
    fi

    local run_id="${RUN_ID}"
    local deploy_name="e2e-tf-${test_name}-${run_id}"
    local workdir
    workdir=$(mktemp -d)

    # Copy graph.tf and generate provider.tf with runtime config
    cp "$graph_tf" "${workdir}/graph.tf"
    setup_tf_provider "$workdir"

    # Build -var flags: override name and supply env-var–backed variables
    local var_flags="-var name=${deploy_name}"
    if grep -q 'variable "openrouter_api_key"' "$graph_tf" 2>/dev/null; then
        var_flags="${var_flags} -var openrouter_api_key=${OPENROUTER_API_KEY:-}"
    fi

    if [[ "${VERBOSE:-}" == "true" ]]; then
        log_debug "graph.tf source: ${graph_tf}"
        log_debug "var flags: ${var_flags}"
    fi

    local orig_dir
    orig_dir=$(pwd)
    cd "$workdir" || return 1

    # Use a clean TF CLI config to avoid dev_overrides from ~/.terraformrc
    export TF_CLI_CONFIG_FILE="${workdir}/.terraformrc"
    cat > "${workdir}/.terraformrc" <<RCEOF
provider_installation {
  filesystem_mirror {
    path = "${TF_PLUGIN_DIR}"
  }
  direct {
    exclude = ["brockleyai/*"]
  }
}
RCEOF

    # 1. Init
    log_info "Terraform init..."
    if [[ "${VERBOSE:-}" == "true" ]]; then
        if ! terraform init -plugin-dir="$TF_PLUGIN_DIR" 2>&1; then
            log_fail "${test_name}: terraform init failed"
            cd "$orig_dir"
            rm -rf "$workdir"
            return 1
        fi
    else
        if ! terraform init -plugin-dir="$TF_PLUGIN_DIR" > /dev/null 2>&1; then
            log_fail "${test_name}: terraform init failed"
            cd "$orig_dir"
            rm -rf "$workdir"
            return 1
        fi
    fi

    # 2. Plan
    log_info "Terraform plan..."
    if [[ "${VERBOSE:-}" == "true" ]]; then
        if ! terraform plan $var_flags -out=tfplan 2>&1; then
            log_fail "${test_name}: terraform plan failed"
            cd "$orig_dir"
            rm -rf "$workdir"
            return 1
        fi
    else
        if ! terraform plan $var_flags -out=tfplan > /dev/null 2>&1; then
            log_fail "${test_name}: terraform plan failed"
            cd "$orig_dir"
            rm -rf "$workdir"
            return 1
        fi
    fi

    # 3. Apply
    log_info "Terraform apply..."
    local apply_out
    apply_out=$(terraform apply -auto-approve tfplan 2>&1)
    if [[ $? -ne 0 ]]; then
        log_fail "${test_name}: terraform apply failed: ${apply_out}"
        cd "$orig_dir"
        rm -rf "$workdir"
        return 1
    fi
    log_debug "Apply output: ${apply_out}"

    # 4. Extract graph ID from state
    local graph_id
    graph_id=$(terraform show -json 2>/dev/null | jq -r '.values.root_module.resources[0].values.id // empty')
    if [[ -z "$graph_id" ]]; then
        log_fail "${test_name}: could not extract graph ID from terraform state"
        terraform destroy -auto-approve $var_flags > /dev/null 2>&1
        cd "$orig_dir"
        rm -rf "$workdir"
        return 1
    fi
    log_info "Graph ID: ${graph_id}"

    # 5. Validate via API
    log_info "Validating via API..."
    local validate_out
    local validate_rc=0
    validate_out=$(curl -sf -X POST "http://localhost:${E2E_PORT}/api/v1/graphs/${graph_id}/validate" \
        -H "Content-Type: application/json" 2>&1) || validate_rc=$?
    log_debug "Validate response: ${validate_out}"
    if [[ $validate_rc -ne 0 ]]; then
        log_fail "${test_name}: remote validation request failed: ${validate_out}"
        terraform destroy -auto-approve $var_flags > /dev/null 2>&1
        cd "$orig_dir"
        rm -rf "$workdir"
        return 1
    fi
    if ! echo "$validate_out" | jq -e '.valid == true' > /dev/null 2>&1; then
        log_fail "${test_name}: remote validation failed: ${validate_out}"
        terraform destroy -auto-approve $var_flags > /dev/null 2>&1
        cd "$orig_dir"
        rm -rf "$workdir"
        return 1
    fi

    # 6. Invoke via API (sync)
    log_info "Invoking via API (sync, timeout=${timeout_s}s)..."
    log_debug "Invoke input: ${input}"
    local exec_result invoke_rc=0
    local curl_max_time=$(( timeout_s + 30 ))
    exec_result=$(curl -sf --max-time "$curl_max_time" -X POST "http://localhost:${E2E_PORT}/api/v1/executions" \
        -H "Content-Type: application/json" \
        -d "{\"graph_id\": \"${graph_id}\", \"input\": ${input}, \"mode\": \"sync\", \"timeout_seconds\": ${timeout_s}, \"debug\": ${LLM_VERBOSE:-false}}" 2>&1) || invoke_rc=$?

    if [[ $invoke_rc -ne 0 || -z "$exec_result" ]]; then
        log_fail "${test_name}: API invoke failed (curl exit code: ${invoke_rc})"
        terraform destroy -auto-approve $var_flags > /dev/null 2>&1
        cd "$orig_dir"
        rm -rf "$workdir"
        return 1
    fi

    log_verbose_json "Execution result" "$exec_result"

    local execution_id
    execution_id=$(echo "$exec_result" | jq -r '.id // empty' 2>/dev/null)

    # 7. Assert status
    local status
    status=$(echo "$exec_result" | jq -r '.status // empty')
    local expected_status
    expected_status=$(jq -r '.expected_status' "$manifest")
    assert_equals "$status" "$expected_status" "${test_name} (TF): execution status"

    # 8. Run manifest assertions
    run_assertions "$manifest" "$exec_result"

    # 9. Print LLM debug traces for debug executions.
    print_llm_debug_traces "$execution_id" || true

    # 10. Destroy
    log_info "Terraform destroy..."
    if [[ "${VERBOSE:-}" == "true" ]]; then
        terraform destroy -auto-approve $var_flags 2>&1
    else
        terraform destroy -auto-approve $var_flags > /dev/null 2>&1
    fi

    # 11. Verify graph is gone (404 or 500 for soft-deleted)
    local http_code
    http_code=$(curl -s -o /dev/null -w "%{http_code}" \
        "http://localhost:${E2E_PORT}/api/v1/graphs/${graph_id}")
    if [[ "$http_code" == "404" || "$http_code" == "500" ]]; then
        log_pass "${test_name} (TF): graph deleted after destroy (HTTP ${http_code})"
    else
        log_fail "${test_name} (TF): graph not deleted after destroy (HTTP ${http_code})"
    fi

    cd "$orig_dir"
    rm -rf "$workdir"

    log_info "TF test complete: ${test_name}"
}

# setup_tf_provider <workdir>
# Generates provider.tf with the required_providers block and provider config.
setup_tf_provider() {
    local workdir="$1"

    cat > "${workdir}/provider.tf" <<TFEOF
terraform {
  required_providers {
    brockley = {
      source  = "brockleyai/brockley"
      version = "0.0.1"
    }
  }
}

provider "brockley" {
  server_url = "http://localhost:${E2E_PORT}"
  api_key    = ""
}
TFEOF
}
