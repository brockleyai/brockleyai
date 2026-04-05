#!/usr/bin/env bash
# stack.sh — Docker Compose lifecycle for E2E tests

COMPOSE_FILE="${E2E_DIR}/docker-compose.e2e.yml"
COMPOSE_PROJECT="brockley-e2e"

stack_up() {
    log_info "Starting E2E stack..."
    if [[ "${LLM_VERBOSE:-}" == "true" ]]; then
        docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" up -d --build > /dev/null 2>&1
    elif [[ "${VERBOSE:-}" == "true" ]]; then
        docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" up -d --build 2>&1
    else
        docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" up -d --build 2>&1 | tail -5
    fi
    if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
        log_error "Failed to start E2E stack"
        docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" logs 2>&1 | tail -30
        return 1
    fi
    log_info "E2E stack containers started"
}

stack_down() {
    log_info "Tearing down E2E stack..."
    if [[ "${VERBOSE:-}" == "true" ]]; then
        log_debug "Dumping service logs before teardown..."
        docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" logs --tail=100 2>&1
    fi
    if [[ "${LLM_VERBOSE:-}" == "true" ]]; then
        docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" down --volumes --remove-orphans > /dev/null 2>&1
    elif [[ "${VERBOSE:-}" == "true" ]]; then
        docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" down --volumes --remove-orphans 2>&1
    else
        docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" down --volumes --remove-orphans 2>&1 | tail -5
    fi
    log_info "E2E stack destroyed"
}

stack_wait_healthy() {
    local url="http://localhost:${E2E_PORT}/health"
    local max_wait=120
    local elapsed=0

    log_info "Waiting for server at ${url}..."
    while [[ $elapsed -lt $max_wait ]]; do
        if curl -sf "$url" > /dev/null 2>&1; then
            log_info "Server healthy after ${elapsed}s"
            if [[ "${VERBOSE:-}" == "true" ]]; then
                log_debug "Initial service logs:"
                docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" logs --tail=20 2>&1
            fi
            return 0
        fi
        log_debug "Health check attempt at ${elapsed}s — not ready"
        sleep 2
        elapsed=$((elapsed + 2))
    done

    log_error "Server not healthy after ${max_wait}s"
    docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" logs server 2>&1 | tail -30
    return 1
}
