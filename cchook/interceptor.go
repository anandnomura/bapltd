package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ToolInput represents the input object from Claude Code.
// Supports both command-execution (Bash) and direct file tools (Read, View, Edit, Write).
type ToolInput struct {
	Command  string `json:"command,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Path     string `json:"path,omitempty"`
}

// HookPayload represents the Claude Code PreToolUse event payload.
type HookPayload struct {
	HookEventName string    `json:"hook_event_name"`
	ToolName      string    `json:"tool_name"`
	ToolInput     ToolInput `json:"tool_input"`
}

// HookSpecificOutput represents the PreToolUse decision schema.
type HookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"` // "allow" or "deny"
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string `json:"additionalContext,omitempty"`
}

// HookResponse is the top-level response expected by Claude Code.
type HookResponse struct {
	Continue           *bool              `json:"continue,omitempty"`
	StopReason         string             `json:"stopReason,omitempty"`
	SystemMessage      string             `json:"systemMessage,omitempty"`
	Decision           string             `json:"decision,omitempty"`
	Reason             string             `json:"reason,omitempty"`
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
		outputDecision("deny", "Empty hook payload on stdin", "No input received")
		return
	}

	// Clean UTF-8 BOM if present (common in Windows piping)
	inputBytes = bytes.TrimPrefix(inputBytes, []byte("\xef\xbb\xbf"))
	inputBytes = bytes.TrimSpace(inputBytes)

	var payload HookPayload
	if err := json.Unmarshal(inputBytes, &payload); err != nil {
		outputDecision("deny", "Failed to parse hook payload JSON: "+err.Error(), "Invalid JSON payload")
		return
	}

	// 2. Intercept direct file inspection tools (Read, View, Edit, Write)
	targetFile := payload.ToolInput.FilePath
	if targetFile == "" {
		targetFile = payload.ToolInput.Path
	}
	if targetFile != "" {
		normPath := strings.ToLower(filepath.ToSlash(targetFile))
		if strings.Contains(normPath, ".env") || strings.Contains(normPath, ".aws") || strings.Contains(normPath, ".ssh") {
			outputDecision("deny", fmt.Sprintf("Access to sensitive credential file %q is strictly forbidden by policy", targetFile), "Sensitive credential file access blocked")
			return
		}
		// If it's a safe file read/view, allow
		if payload.ToolName == "Read" || payload.ToolName == "View" {
			outputDecision("allow", "", "File access permitted")
			return
		}
	}

	// 3. Process Command execution (Bash or generic command)
	command := strings.TrimSpace(payload.ToolInput.Command)
	if command == "" {
		if payload.ToolName == "Read" || payload.ToolName == "View" {
			outputDecision("allow", "", "File read allowed")
			return
		}
		outputDecision("deny", "No command or target specified in tool input", "Empty command")
		return
	}

	// 4. Resolve bapedge (LTD) or ltd-agent executable
	binName := "bapedge"
	fallbackName := "ltd-agent"
	if runtime.GOOS == "windows" {
		binName = "bapedge.exe"
		fallbackName = "ltd-agent.exe"
	}
	ltdBin := findBinary(binName)
	if _, err := os.Stat(ltdBin); err != nil {
		ltdBin = findBinary(fallbackName)
	}

	// 5. Execute bapedge exec --source claude-code --json "$command"
	cmd := exec.Command(ltdBin, "exec", "--source", "claude-code", "--json", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// cmd.Run will return exit status 1 when Cedar denies, which is expected
	_ = cmd.Run()

	// 6. Parse ltd-agent JSON output from stdout
	var execResp ExecResponse
	if err := json.Unmarshal(stdout.Bytes(), &execResp); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		if errMsg == "" {
			errMsg = "Unable to execute command via ltd-agent"
		}
		outputDecision("deny", "ltd-agent error: "+errMsg, errMsg)
		return
	}

	// 7. Format decision matching Claude Code PreToolUse schema
	if execResp.Allowed {
		outputDecision("allow", "", strings.TrimRight(execResp.Output, "\r\n"))
	} else {
		reason := execResp.Reason
		if reason == "" {
			reason = "Command execution denied by ltd-agent Cedar security policy"
		}
		outputDecision("deny", reason, reason)
	}
}

// findBinary locates the ltd-agent binary across standard search paths.
func findBinary(name string) string {
	// 1. Check current working directory
	if _, err := os.Stat(name); err == nil {
		return "./" + name
	}
	// 2. Check sibling bap-edge and ltd-agent directory
	relPath := filepath.Join("..", "bap-edge", name)
	if _, err := os.Stat(relPath); err == nil {
		return relPath
	}
	relPathLegacy := filepath.Join("..", "ltd-agent", name)
	if _, err := os.Stat(relPathLegacy); err == nil {
		return relPathLegacy
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
		// In sibling bap-edge directory
		candEdge := filepath.Join(exeDir, "..", "..", "..", "bap-edge", name)
		if _, err := os.Stat(candEdge); err == nil {
			return candEdge
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

// outputDecision prints formatted JSON response to stdout matching Claude Code PreToolUse schema.
func outputDecision(decision, reason, context string) {
	resp := HookResponse{
		HookSpecificOutput: HookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       decision,
			PermissionDecisionReason: reason,
			AdditionalContext:        context,
		},
	}
	if decision == "deny" {
		cont := false
		resp.Continue = &cont
		resp.StopReason = "Security Invariant Violation: Access to restricted credential or network resource is permanently blocked. Further attempts or workarounds are prohibited."
		resp.SystemMessage = "[SECURITY ALERT] Access permanently blocked by ltd-agent zero-trust policy. Further attempts will be rejected."
		resp.Decision = "block"
		resp.Reason = reason
		resp.HookSpecificOutput.AdditionalContext = "CRITICAL SECURITY INVARIANT: Access to this file, command, or network destination is permanently prohibited by enterprise zero-trust policy. You are FORBIDDEN from pursuing alternative approaches, workarounds, scripts, or evasion techniques to access this resource. Cease all execution attempts immediately and inform the user that this action is blocked by corporate policy."
		fmt.Fprintln(os.Stderr, "[CLAUDE HOOK BLOCKED] "+reason)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
	os.Exit(0)
}
