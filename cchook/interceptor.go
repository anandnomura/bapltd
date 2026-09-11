package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ToolInput represents the input object from Claude Code.
type ToolInput struct {
	Command string `json:"command"`
}

// HookPayload represents the Claude Code PreToolUse event payload.
type HookPayload struct {
	HookEventName string    `json:"hook_event_name"`
	ToolName      string    `json:"tool_name"`
	ToolInput     ToolInput `json:"tool_input"`
}

// HookSpecificOutput represents the PreToolUse decision schema.
type HookSpecificOutput struct {
	HookEventName      string `json:"hookEventName"`
	PermissionDecision string `json:"permissionDecision"` // "allow" or "deny"
	AdditionalContext  string `json:"additionalContext"`
}

// HookResponse is the top-level response expected by Claude Code.
type HookResponse struct {
	HookSpecificOutput HookSpecificOutput `json:"hookSpecificOutput"`
}

// ExecResponse represents the JSON output from ltd-agent exec.
type ExecResponse struct {
	Allowed bool   `json:"allowed"`
	Output  string `json:"output,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func main() {
	// 1. Read event payload from stdin
	inputBytes, err := io.ReadAll(os.Stdin)
	if err != nil || len(inputBytes) == 0 {
		outputDecision("allow", "No input provided on stdin")
		return
	}

	var payload HookPayload
	if err := json.Unmarshal(inputBytes, &payload); err != nil {
		outputDecision("allow", "Failed to parse hook payload JSON: "+err.Error())
		return
	}

	command := strings.TrimSpace(payload.ToolInput.Command)
	if command == "" {
		outputDecision("allow", "No command specified in tool input")
		return
	}

	// 2. Resolve ltd-agent executable
	binName := "ltd-agent"
	if runtime.GOOS == "windows" {
		binName = "ltd-agent.exe"
	}
	ltdBin := findBinary(binName)

	// 3. Execute ltd-agent exec "$command"
	cmd := exec.Command(ltdBin, "exec", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// cmd.Run will return exit status 1 when Cedar denies, which is expected
	_ = cmd.Run()

	// 4. Parse ltd-agent JSON output from stdout
	var execResp ExecResponse
	if err := json.Unmarshal(stdout.Bytes(), &execResp); err != nil {
		// If stdout did not contain expected JSON, report denial with error details
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		if errMsg == "" {
			errMsg = "Unable to execute command via ltd-agent"
		}
		outputDecision("deny", "ltd-agent error: "+errMsg)
		return
	}

	// 5. Format decision matching Claude Code PreToolUse schema
	if execResp.Allowed {
		outputDecision("allow", strings.TrimRight(execResp.Output, "\r\n"))
	} else {
		reason := execResp.Reason
		if reason == "" {
			reason = "Command execution denied by ltd-agent Cedar security policy"
		}
		outputDecision("deny", reason)
	}
}

// findBinary locates the ltd-agent binary across standard search paths.
func findBinary(name string) string {
	// 1. Check current working directory
	if _, err := os.Stat(name); err == nil {
		return "./" + name
	}
	// 2. Check sibling ltd-agent directory
	relPath := filepath.Join("..", "ltd-agent", name)
	if _, err := os.Stat(relPath); err == nil {
		return relPath
	}
	// 3. Check executable directory and ancestor folders
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		// Next to interceptor binary
		cand := filepath.Join(exeDir, name)
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
		// In .claude/
		candParent := filepath.Join(exeDir, "..", name)
		if _, err := os.Stat(candParent); err == nil {
			return candParent
		}
		// In project root (cchook/)
		candRoot := filepath.Join(exeDir, "..", "..", name)
		if _, err := os.Stat(candRoot); err == nil {
			return candRoot
		}
		// In sibling ltd-agent directory
		candSibling := filepath.Join(exeDir, "..", "..", "..", "ltd-agent", name)
		if _, err := os.Stat(candSibling); err == nil {
			return candSibling
		}
	}
	// 4. Check system PATH
	if p, err := exec.LookPath(name); err == nil {
		return p
	}

	return "./" + name
}

// outputDecision prints formatted JSON response to stdout and exits 0.
func outputDecision(decision, context string) {
	resp := HookResponse{
		HookSpecificOutput: HookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: decision,
			AdditionalContext:  context,
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
	os.Exit(0)
}

