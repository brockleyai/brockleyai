#!/usr/bin/env python3
"""
run.py — Brockley code execution runner.

Entry point for user code execution inside the coderunner subprocess.
Decodes user code from BROCKLEY_CODE_B64, executes it, and writes result.json.

IPC protocol:
    stdout (fd 1) is reserved for REQ/RESP protocol signals.
    User print() is redirected to stderr (fd 2).
"""

import base64
import io
import json
import os
import sys
import traceback

# Redirect user print() to stderr, keeping stdout clean for IPC.
_real_stdout = sys.stdout
sys.stdout = sys.stderr

# Set up work directory.
work_dir = os.environ.get("BROCKLEY_WORK_DIR", ".")
os.chdir(work_dir)

# Import brockley module (must be in the same directory or PYTHONPATH).
import brockley as _brockley_mod


def _restricted_import(name, *args, **kwargs):
    """UX guardrail: warn on non-stdlib imports.
    Not a security boundary — just helps users understand constraints."""
    _blocked = {
        "requests", "httpx", "urllib3", "aiohttp",
        "flask", "django", "fastapi",
        "numpy", "pandas", "scipy", "sklearn",
        "torch", "tensorflow",
        "subprocess", "socket",
    }
    if name in _blocked:
        raise ImportError(
            f"Module '{name}' is not available in the code execution environment. "
            f"Only Python stdlib modules are available. "
            f"Use brockley.tools.call() for external service access."
        )
    return _original_import(name, *args, **kwargs)


_original_import = __builtins__.__import__ if hasattr(__builtins__, '__import__') else __import__
try:
    if hasattr(__builtins__, '__import__'):
        __builtins__.__import__ = _restricted_import
    else:
        import builtins
        builtins.__import__ = _restricted_import
except Exception:
    pass  # Non-critical: guardrail only.


def main():
    result = {
        "status": "completed",
        "output": "",
        "stdout": "",
        "stderr": "",
        "error": "",
        "traceback": "",
    }

    # Decode user code.
    code_b64 = os.environ.get("BROCKLEY_CODE_B64", "")
    if not code_b64:
        result["status"] = "error"
        result["error"] = "BROCKLEY_CODE_B64 not set"
        _write_result(result)
        return

    try:
        code = base64.b64decode(code_b64).decode("utf-8")
    except Exception as e:
        result["status"] = "error"
        result["error"] = f"failed to decode code: {e}"
        _write_result(result)
        return

    # Capture stderr (user print output).
    stderr_capture = io.StringIO()
    old_stderr = sys.stdout  # Remember: sys.stdout is already redirected to stderr
    sys.stdout = stderr_capture

    try:
        exec_globals = {
            "__builtins__": __builtins__,
            "brockley": _brockley_mod,
        }
        exec(code, exec_globals)
        result["status"] = "completed"
    except KeyboardInterrupt:
        result["status"] = "cancelled"
    except Exception:
        result["status"] = "error"
        result["traceback"] = traceback.format_exc()
        result["error"] = str(sys.exc_info()[1])
    finally:
        sys.stdout = old_stderr

    # Capture outputs.
    result["stderr"] = stderr_capture.getvalue()

    output_val, output_set = _brockley_mod._get_output()
    if output_set:
        try:
            result["output"] = json.dumps(output_val)
        except (TypeError, ValueError) as e:
            result["output"] = str(output_val)
            if result["status"] == "completed":
                result["error"] = f"brockley.output() value not JSON-serializable: {e}"

    _write_result(result)


def _write_result(result):
    """Write result.json to the work directory."""
    result_path = os.path.join(work_dir, "result.json")
    with open(result_path, "w") as f:
        json.dump(result, f)


if __name__ == "__main__":
    main()
