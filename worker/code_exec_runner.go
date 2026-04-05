package worker

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/engine/codeexec"
)

// codeExecRunner manages the subprocess lifecycle for a single code execution.
type codeExecRunner struct {
	rdb    *redis.Client
	logger *slog.Logger
}

// Run executes the code in a subprocess and relays tool calls through Redis.
func (r *codeExecRunner) Run(ctx context.Context, t CodeExecTask) (CodeExecResult, error) {
	// Create per-execution work directory.
	workDir, err := os.MkdirTemp("", "brockley-codeexec-*")
	if err != nil {
		return CodeExecResult{}, fmt.Errorf("creating work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Extract embedded Python assets.
	for _, name := range []string{"run.py", "brockley.py"} {
		data, err := codeexec.Assets.ReadFile("assets/" + name)
		if err != nil {
			return CodeExecResult{}, fmt.Errorf("reading embedded %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(workDir, name), data, 0600); err != nil {
			return CodeExecResult{}, fmt.Errorf("writing %s: %w", name, err)
		}
	}

	// Encode user code as base64.
	codeB64 := base64.StdEncoding.EncodeToString([]byte(t.Code))

	// Build command with context deadline.
	cmd := exec.CommandContext(ctx, "python3", filepath.Join(workDir, "run.py"))
	cmd.Dir = workDir
	cmd.Env = []string{
		"BROCKLEY_CODE_B64=" + codeB64,
		"BROCKLEY_WORK_DIR=" + workDir,
		"PYTHONPATH=" + workDir,
		"HOME=" + workDir,
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}

	// Apply platform-specific resource limits.
	applySysProcAttr(cmd, t.MaxMemoryMB)

	// Set up pipes for IPC.
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return CodeExecResult{}, fmt.Errorf("creating stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return CodeExecResult{}, fmt.Errorf("creating stdout pipe: %w", err)
	}
	// stderr is captured from the result.json file (run.py redirects print to stderr internally)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrWriter{w: &stderrBuf, limit: t.MaxOutputBytes}

	// Start subprocess.
	if err := cmd.Start(); err != nil {
		return CodeExecResult{}, fmt.Errorf("starting subprocess: %w", err)
	}

	// Process IPC signals.
	toolCalls := 0
	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "REQ ") {
			continue
		}

		seqStr := strings.TrimPrefix(line, "REQ ")
		seq, err := strconv.Atoi(seqStr)
		if err != nil {
			r.logger.Warn("invalid IPC sequence", "line", line)
			continue
		}

		// Read request file.
		reqPath := filepath.Join(workDir, fmt.Sprintf("request-%d.json", seq))
		reqData, err := os.ReadFile(reqPath)
		if err != nil {
			r.logger.Error("reading request file", "error", err, "path", reqPath)
			r.sendCancelToSubprocess(stdinPipe, seq, workDir)
			continue
		}

		var req CodeToolRequest
		if err := json.Unmarshal(reqData, &req); err != nil {
			r.logger.Error("unmarshaling request", "error", err)
			r.sendCancelToSubprocess(stdinPipe, seq, workDir)
			continue
		}

		toolCalls++

		// Check tool call limit.
		if t.MaxToolCalls > 0 && toolCalls > t.MaxToolCalls {
			r.sendErrorToSubprocess(stdinPipe, seq, workDir, "tool call limit exceeded")
			continue
		}

		// Relay to superagent handler via Redis.
		reqJSON, _ := json.Marshal(req)
		if err := r.rdb.LPush(ctx, t.CallbackKey, string(reqJSON)).Err(); err != nil {
			r.logger.Error("pushing tool request to Redis", "error", err)
			r.sendErrorToSubprocess(stdinPipe, seq, workDir, "internal relay error")
			continue
		}

		// Wait for response from handler.
		result, err := r.rdb.BRPop(ctx, 0, t.ResponseKey).Result()
		if err != nil {
			if ctx.Err() != nil {
				r.sendCancelToSubprocess(stdinPipe, seq, workDir)
				break
			}
			r.logger.Error("waiting for tool response", "error", err)
			r.sendErrorToSubprocess(stdinPipe, seq, workDir, "internal relay error")
			continue
		}

		// Write response file.
		var resp CodeToolResponse
		if err := json.Unmarshal([]byte(result[1]), &resp); err != nil {
			r.sendErrorToSubprocess(stdinPipe, seq, workDir, "invalid response from handler")
			continue
		}

		// Check for cancel from handler.
		if resp.Type == "cancel" {
			r.sendCancelToSubprocess(stdinPipe, seq, workDir)
			break
		}

		respPath := filepath.Join(workDir, fmt.Sprintf("response-%d.json", seq))
		respJSON, _ := json.Marshal(resp)
		if err := os.WriteFile(respPath, respJSON, 0600); err != nil {
			r.logger.Error("writing response file", "error", err)
			r.sendErrorToSubprocess(stdinPipe, seq, workDir, "internal file error")
			continue
		}

		// Signal subprocess.
		fmt.Fprintf(stdinPipe, "RESP %d\n", seq)
	}

	// Wait for subprocess to exit.
	stdinPipe.Close()
	cmdErr := cmd.Wait()

	// Read result.json from work directory.
	resultPath := filepath.Join(workDir, "result.json")
	resultData, err := os.ReadFile(resultPath)
	if err != nil {
		status := "error"
		errMsg := "no result.json produced"
		if ctx.Err() == context.DeadlineExceeded {
			status = "timeout"
			errMsg = "execution timed out"
		} else if ctx.Err() == context.Canceled {
			status = "cancelled"
			errMsg = "execution cancelled"
		} else if cmdErr != nil {
			errMsg = fmt.Sprintf("subprocess failed: %v", cmdErr)
		}
		return CodeExecResult{
			Status:    status,
			Error:     errMsg,
			Stderr:    stderrBuf.String(),
			ToolCalls: toolCalls,
		}, nil
	}

	var result CodeExecResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return CodeExecResult{
			Status:    "error",
			Error:     fmt.Sprintf("invalid result.json: %v", err),
			Stderr:    stderrBuf.String(),
			ToolCalls: toolCalls,
		}, nil
	}

	result.ToolCalls = toolCalls

	// Truncate output per MaxOutputBytes.
	if t.MaxOutputBytes > 0 {
		if len(result.Output) > t.MaxOutputBytes {
			result.Output = result.Output[:t.MaxOutputBytes]
		}
		if len(result.Stdout) > t.MaxOutputBytes {
			result.Stdout = result.Stdout[:t.MaxOutputBytes]
		}
		if len(result.Stderr) > t.MaxOutputBytes {
			result.Stderr = result.Stderr[:t.MaxOutputBytes]
		}
	}

	// Use captured stderr if result didn't produce any.
	if result.Stderr == "" && stderrBuf.Len() > 0 {
		result.Stderr = stderrBuf.String()
	}

	return result, nil
}

// sendCancelToSubprocess writes a cancel response to the subprocess.
func (r *codeExecRunner) sendCancelToSubprocess(stdin io.Writer, seq int, workDir string) {
	resp := CodeToolResponse{Type: "cancel", Seq: seq}
	respJSON, _ := json.Marshal(resp)
	respPath := filepath.Join(workDir, fmt.Sprintf("response-%d.json", seq))
	_ = os.WriteFile(respPath, respJSON, 0600)
	fmt.Fprintf(stdin, "RESP %d\n", seq)
}

// sendErrorToSubprocess writes an error response to the subprocess.
func (r *codeExecRunner) sendErrorToSubprocess(stdin io.Writer, seq int, workDir string, errMsg string) {
	resp := CodeToolResponse{Type: "error", Content: errMsg, IsError: true, Seq: seq}
	respJSON, _ := json.Marshal(resp)
	respPath := filepath.Join(workDir, fmt.Sprintf("response-%d.json", seq))
	_ = os.WriteFile(respPath, respJSON, 0600)
	fmt.Fprintf(stdin, "RESP %d\n", seq)
}

// stderrWriter is a limited writer for subprocess stderr.
type stderrWriter struct {
	w       *strings.Builder
	limit   int
	written int
}

func (w *stderrWriter) Write(p []byte) (int, error) {
	if w.limit > 0 && w.written >= w.limit {
		return len(p), nil // silently discard
	}
	remaining := len(p)
	if w.limit > 0 && w.written+remaining > w.limit {
		remaining = w.limit - w.written
	}
	n, err := w.w.Write(p[:remaining])
	w.written += n
	return len(p), err // always report full write to subprocess
}
