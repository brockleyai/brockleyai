"""
brockley — helper module for Brockley code execution.

Provides:
    brockley.output(value)       — set the structured result for the LLM
    brockley.tools.call(name, **kwargs) — synchronous tool call via IPC
    brockley.tools.batch(calls)  — batch multiple tool calls
    brockley.ToolError           — raised on tool call failure

IPC protocol (stdin/stdout signals + file-based data):
    Python writes request-<n>.json, prints "REQ <n>" to stdout (protocol channel)
    Coderunner reads signal, reads file, relays to Redis
    Coderunner writes response-<n>.json, sends "RESP <n>" to stdin
    Python reads signal, reads file
"""

import json
import os
import sys

_work_dir = os.environ.get("BROCKLEY_WORK_DIR", ".")
_seq = 0
_output_value = None
_output_set = False

# stdout is reserved for the IPC protocol. User print() goes to stderr.
_proto_stdout = sys.stdout


class ToolError(Exception):
    """Raised when a tool call fails."""
    pass


class _Tools:
    """Tool calling interface."""

    def call(self, name, **kwargs):
        """Call a tool synchronously. Returns the result string.
        Raises ToolError on failure."""
        global _seq
        _seq += 1
        seq = _seq

        request = {
            "type": "tool_call",
            "name": name,
            "arguments": kwargs,
            "seq": seq,
        }

        # Write request file.
        req_path = os.path.join(_work_dir, f"request-{seq}.json")
        with open(req_path, "w") as f:
            json.dump(request, f)

        # Signal coderunner via protocol stdout.
        _proto_stdout.write(f"REQ {seq}\n")
        _proto_stdout.flush()

        # Wait for response signal on stdin.
        line = sys.stdin.readline().strip()
        if not line.startswith("RESP "):
            raise ToolError(f"unexpected IPC signal: {line!r}")

        resp_seq = int(line.split(" ", 1)[1])
        if resp_seq != seq:
            raise ToolError(f"IPC sequence mismatch: expected {seq}, got {resp_seq}")

        # Read response file.
        resp_path = os.path.join(_work_dir, f"response-{seq}.json")
        with open(resp_path, "r") as f:
            response = json.load(f)

        if response.get("type") == "cancel":
            raise KeyboardInterrupt("execution cancelled")

        if response.get("is_error"):
            raise ToolError(response.get("content", "tool call failed"))

        return response.get("content", "")

    def batch(self, calls):
        """Call multiple tools and collect results.
        Each entry is (name, kwargs_dict).
        Returns list of {"result": ..., "error": ...} dicts."""
        results = []
        for name, kwargs in calls:
            try:
                result = self.call(name, **kwargs)
                results.append({"result": result, "error": None})
            except ToolError as e:
                results.append({"result": None, "error": str(e)})
        return results


tools = _Tools()


def output(value):
    """Set the structured output that will be returned to the LLM.
    Only the last call is kept. Value must be JSON-serializable."""
    global _output_value, _output_set
    _output_value = value
    _output_set = True


def _get_output():
    """Internal: return (value, was_set) for the runner."""
    return _output_value, _output_set
