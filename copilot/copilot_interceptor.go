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

// ExecResponse represents the JSON output from ltd-agent exec.
type ExecResponse struct {
	Allowed bool   `json:"allowed"`
	Output  string `json:"output,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func main() {
	var command string

	// 1. Check if command is passed via CLI arguments
	if len(os.Args) > 1 {
		command = strings.TrimSpace(strings.Join(os.Args[1:], " "))
	} else {
		// 2. Otherwise read from stdin (JSON payload or raw command line)
		inputBytes, err := io.ReadAll(os.Stdin)
		if err == nil && len(inputBytes) > 0 {
			raw := strings.TrimSpace(string(inputBytes))
			// Attempt to parse as JSON if it looks like JSON
			if strings.HasPrefix(raw, "{") {
				var payload struct {
					Command string `json:"command"`
					Input   string `json:"input"`
				}
				if err := json.Unmarshal(inputBytes, &payload); err == nil {
					if payload.Command != "" {
						command = payload.Command
					} else {
						command = payload.Input
					}
				}
			}
			if command == "" {
				command = raw
			}
		}
	}

	if command == "" {
		fmt.Fprintln(os.Stderr, "[copilot-interceptor] Error: No command provided to execute.")
		os.Exit(1)
	}

	// 3. Resolve ltd-agent executable
	binName := "ltd-agent"
	if runtime.GOOS == "windows" {
		binName = "ltd-agent.exe"
	}
	ltdBin := findBinary(binName)

	// 4. Execute ltd-agent exec --source copilot --json "$command"
	cmd := exec.Command(ltdBin, "exec", "--source", "copilot", "--json", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()

	// 5. Parse output
	var execResp ExecResponse
	if err := json.Unmarshal(stdout.Bytes(), &execResp); err != nil {
		fmt.Fprintf(os.Stderr, "[copilot-interceptor] ltd-agent execution error: %s\n", stdout.String())
		os.Exit(1)
	}

	if !execResp.Allowed {
		fmt.Fprintf(os.Stderr, "[COPILOT BLOCKED BY POLICY] %s\n", execResp.Reason)
		fmt.Fprintln(os.Stderr, "[CRITICAL SECURITY INVARIANT] Access to this resource is permanently prohibited by enterprise zero-trust policy.")
		fmt.Fprintln(os.Stderr, "[INSTRUCTION TO AI AGENT] You must CEASE all attempts to access, read, copy, rename, or exfiltrate this resource. Do NOT attempt alternative commands, scripts, or workarounds. Abort this task immediately and notify the user.")
		os.Exit(1)
	}

	if execResp.Output != "" {
		fmt.Print(execResp.Output)
		if !strings.HasSuffix(execResp.Output, "\n") {
			fmt.Println()
		}
	}
	os.Exit(0)
}

func findBinary(name string) string {
	// 1. Current working directory
	if _, err := os.Stat(name); err == nil {
		abs, err := filepath.Abs(name)
		if err == nil {
			return abs
		}
		return name
	}
	// 2. Look in parent directories (e.g. ../ltd-agent.exe, ../ltd-agent/ltd-agent.exe)
	candidates := []string{
		filepath.Join("..", name),
		filepath.Join("..", "ltd-agent", name),
		filepath.Join("ltd-agent", name),
		filepath.Join("cchook", name),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
			return c
		}
	}
	// 3. System PATH
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}
