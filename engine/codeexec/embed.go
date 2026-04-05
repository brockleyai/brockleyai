// Package codeexec provides embedded Python assets for code execution
// and shared utilities used by both the coderunner and superagent handler.
package codeexec

import "embed"

// Assets contains the embedded Python runner files (run.py, brockley.py)
// and the guidelines markdown. Both the coderunner and superagent handler
// import this package:
//   - coderunner extracts run.py and brockley.py to a temp dir at execution time
//   - handler calls Guidelines() for _code_guidelines responses
//
//go:embed assets/*
var Assets embed.FS
