package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func promptPathForSession(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return filepath.Join(".bap", "prompts", fmt.Sprintf("%x.txt", sum[:]))
}

func writeSessionPrompt(sessionID, prompt string) {
	if sessionID == "" || prompt == "" {
		return
	}
	path := promptPathForSession(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err == nil {
		_ = os.WriteFile(path, []byte(prompt), 0600)
	}
}

//go:embed embedded_ca.crt
var embeddedCACert []byte

func getHTTPClient() *http.Client {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if len(embeddedCACert) > 0 {
		roots.AppendCertsFromPEM(embeddedCACert)
	}
	path := os.Getenv("BAP_CA_CERT")
	if path == "" {
		for _, cand := range []string{"bap-root-ca.crt", "controlplane-cert.pem", "../bap-root-ca.crt", "../controlplane-cert.pem", "../../controlplane-cert.pem"} {
			if _, err := os.Stat(cand); err == nil {
				path = cand
				break
			}
		}
	}
	if path != "" {
		if pem, err := os.ReadFile(path); err == nil {
			roots.AppendCertsFromPEM(pem)
		}
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS12,
		},
	}
	return &http.Client{
		Timeout:   2 * time.Second,
		Transport: tr,
	}
}

// ToolInput represents the input object from Claude Code.
// Supports both command-execution (Bash) and direct file tools (Read, View, Edit, Write).
type ToolInput struct {
	Command  string `json:"command,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Path     string `json:"path,omitempty"`
}

// HookPayload represents the Claude Code lifecycle event payload (SessionStart, UserPromptSubmit, PreToolUse).
type HookPayload struct {
	HookEventName string    `json:"hook_event_name"`
	ToolName      string    `json:"tool_name,omitempty"`
	ToolInput     ToolInput `json:"tool_input,omitempty"`
	UserPrompt    string    `json:"user_prompt,omitempty"`
	Prompt        string    `json:"prompt,omitempty"`
	SessionID     string    `json:"session_id,omitempty"`
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
	Allowed    bool   `json:"allowed"`
	Output     string `json:"output,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

func resolveServerURL() string {
	if s := os.Getenv("BAP_SERVER_URL"); s != "" {
		return strings.TrimRight(s, "/")
	}
	cfgCandidates := []string{"bap-config.json", "../bap-config.json"}
	if exePath, err := os.Executable(); err == nil {
		cfgCandidates = append(cfgCandidates, filepath.Join(filepath.Dir(exePath), "bap-config.json"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		cfgCandidates = append(cfgCandidates, filepath.Join(home, ".bap", "bap-config.json"))
	}
	for _, cfgPath := range cfgCandidates {
		if cfgData, err := os.ReadFile(cfgPath); err == nil {
			var cfg struct {
				ControlPlaneURL string `json:"controlplane_url"`
			}
			if json.Unmarshal(cfgData, &cfg) == nil && cfg.ControlPlaneURL != "" {
				return strings.TrimRight(cfg.ControlPlaneURL, "/")
			}
		}
	}
	return "http://localhost:8080"
}

func resolveSessionID(payload HookPayload) string {
	if sessionID := os.Getenv("BAP_SESSION_ID"); sessionID != "" {
		return sessionID
	}
	if sessionID := os.Getenv("LTD_SESSION_ID"); sessionID != "" {
		return sessionID
	}
	if payload.SessionID != "" {
		return payload.SessionID
	}
	for _, loc := range []string{".bap-session.json", "../.bap-session.json", "cchook/.bap-session.json"} {
		if data, err := os.ReadFile(loc); err == nil {
			var info struct {
				SessionID string `json:"session_id"`
			}
			if json.Unmarshal(data, &info) == nil && info.SessionID != "" {
				return info.SessionID
			}
		}
	}
	return ""
}

func remotelyRevoked(serverURL, sessionID string) (bool, string) {
	if serverURL == "" || sessionID == "" {
		return false, ""
	}
	resp, err := getHTTPClient().Get(strings.TrimRight(serverURL, "/") + "/api/v1/control/revocations")
	if err != nil || resp == nil {
		return false, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, ""
	}
	var state struct {
		KillSwitch      bool     `json:"kill_switch"`
		RevokedSessions []string `json:"revoked_sessions"`
		RevokedUsers    []string `json:"revoked_users"`
	}
	if json.NewDecoder(resp.Body).Decode(&state) != nil {
		return false, ""
	}
	if state.KillSwitch {
		return true, "fleet authority is frozen by an administrator"
	}
	for _, revoked := range state.RevokedSessions {
		if strings.EqualFold(strings.TrimSpace(revoked), sessionID) {
			return true, fmt.Sprintf("session %q authority has been revoked by an administrator", sessionID)
		}
	}
	currentUser := os.Getenv("USERNAME")
	if currentUser == "" {
		currentUser = os.Getenv("USER")
	}
	if currentUser != "" {
		for _, ru := range state.RevokedUsers {
			if strings.EqualFold(strings.TrimSpace(ru), currentUser) {
				return true, fmt.Sprintf("user %q access has been revoked by an administrator", currentUser)
			}
		}
	}
	return false, ""
}

func killProcessPID(pid int) {
	if pid <= 0 || pid == os.Getpid() {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid), "/FI", "IMAGENAME ne bapcontrolplane.exe").Run()
	} else {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	}
}

func isStartupBlocked() (bool, string) {
	// Check .bap-revoked
	for _, cand := range []string{".bap-revoked", "../.bap-revoked"} {
		if data, err := os.ReadFile(cand); err == nil {
			var info struct {
				KillSwitch bool   `json:"kill_switch"`
				AppID      string `json:"app_id"`
				Reason     string `json:"reason"`
			}
			if json.Unmarshal(data, &info) == nil {
				if info.KillSwitch || info.AppID == "claude-code" || info.AppID == "*" {
					reason := info.Reason
					if reason == "" {
						reason = "Enterprise emergency kill-switch is active"
					}
					return true, reason
				}
			}
		}
	}
	// Check policy-state.json
	for _, cand := range []string{"policy-state.json", "../policy-state.json"} {
		if data, err := os.ReadFile(cand); err == nil {
			var st struct {
				KillSwitch bool `json:"kill_switch"`
			}
			if json.Unmarshal(data, &st) == nil && st.KillSwitch {
				return true, "Emergency kill-switch is active across the fleet"
			}
		}
	}
	return false, ""
}

func handleSessionStartHook(payload HookPayload) {
	ppid := os.Getppid()

	// 1. Check if local kill-switch or freeze blocks startup
	if blocked, reason := isStartupBlocked(); blocked {
		fmt.Fprintf(os.Stderr, "[BAP ZERO-TRUST] 🚨 New Claude Code session BLOCKED: %s\n", reason)
		os.Exit(2)
	}

	// A governed launcher owns the session ID and watcher lifecycle. Claude also
	// supplies its own hook session ID; preferring that value created a second
	// session and a second watcher for the same process.
	sessionID := os.Getenv("BAP_SESSION_ID")
	launcherOwnsWatcher := sessionID != ""
	if sessionID == "" {
		sessionID = payload.SessionID
	}
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess-claude-pid-%d", ppid)
	}

	serverURL := resolveServerURL()

	// 2. Notify control plane of session start
	hostname, _ := os.Hostname()
	username := os.Getenv("USERNAME")
	if username == "" {
		username = os.Getenv("USER")
	}
	startPayload := map[string]any{
		"session_id": sessionID,
		"app_id":     "claude-code",
		"user_id":    username,
		"hostname":   hostname,
		"client_pid": ppid,
	}
	if body, err := json.Marshal(startPayload); err == nil {
		client := getHTTPClient()
		resp, err := client.Post(serverURL+"/api/v1/sessions/start", "application/json", bytes.NewReader(body))
		if err == nil && resp != nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusForbidden {
				_ = os.Remove(".bap-session.json")
				fmt.Fprintln(os.Stderr, "[BAP ZERO-TRUST] 🚨 New Claude Code session BLOCKED by enterprise security policy (kill-switch or app revocation active).")
				os.Exit(2)
			}
		}
	}

	marker := map[string]any{
		"session_id": sessionID,
		"server_url": serverURL,
		"pid":        ppid,
		"app_id":     "claude-code",
		"started_at": time.Now().UTC(),
	}
	if data, err := json.MarshalIndent(marker, "", "  "); err == nil {
		_ = os.WriteFile(".bap-session.json", data, 0600)
	}

	// Launch detached bapedge watch thread
	ltdBin := findBinary("bapedge.exe")
	if ltdBin == "" {
		ltdBin = findBinary("bapedge")
	}
	if ltdBin != "" && !launcherOwnsWatcher {
		wCmd := exec.Command(ltdBin, "watch", fmt.Sprintf("--pid=%d", ppid), fmt.Sprintf("--server=%s", serverURL), fmt.Sprintf("--session-id=%s", sessionID), "--detach")
		_ = wCmd.Start()
	}

	os.Stdout.WriteString("{\"status\":\"ok\"}\n")
	os.Exit(0)
}

func isSessionRevoked(serverURL, sessionID string) (bool, string) {
	// 1. Check local .bap-revoked marker file (0ms)
	for _, cand := range []string{".bap-revoked", "../.bap-revoked", "cchook/.bap-revoked"} {
		if data, err := os.ReadFile(cand); err == nil {
			var info struct {
				SessionID string `json:"session_id"`
				UserID    string `json:"user_id"`
				Reason    string `json:"reason"`
			}
			if json.Unmarshal(data, &info) == nil {
				currentUser := os.Getenv("USERNAME")
				if currentUser == "" {
					currentUser = os.Getenv("USER")
				}
				userMatch := info.UserID != "" && strings.EqualFold(info.UserID, currentUser)
				sessMatch := info.SessionID != "" && (strings.EqualFold(info.SessionID, sessionID) || strings.Contains(strings.ToLower(sessionID), strings.ToLower(info.SessionID)))
				if userMatch || sessMatch || (info.SessionID == "" && info.UserID == "") {
					reason := info.Reason
					if reason == "" {
						reason = "User access revoked by security administrator (local tombstone marker active)"
					}
					return true, reason
				}
			} else {
				return true, "Access revoked by security administrator"
			}
		}
	}

	// 2. Check local policy-state.json (0ms)
	for _, cand := range []string{"policy-state.json", "../policy-state.json", "bap-edge/policy-state.json"} {
		if data, err := os.ReadFile(cand); err == nil {
			var st struct {
				KillSwitch      bool     `json:"kill_switch"`
				RevokedSessions []string `json:"revoked_sessions"`
			}
			if json.Unmarshal(data, &st) == nil {
				if st.KillSwitch {
					return true, "Emergency kill-switch is active across the fleet"
				}
				if sessionID != "" {
					sessLower := strings.ToLower(sessionID)
					for _, rev := range st.RevokedSessions {
						revLower := strings.ToLower(strings.TrimSpace(rev))
						if revLower != "" && (strings.Contains(sessLower, revLower) || strings.Contains(revLower, sessLower)) {
							return true, fmt.Sprintf("Session %q authority revoked by administrator", sessionID)
						}
					}
				}
			}
		}
	}

	// 3. Fast remote query to control plane
	return remotelyRevoked(serverURL, sessionID)
}

func outputPromptBlocked(sessionID, reason string) {
	if reason == "" {
		reason = "Session authority has been revoked by enterprise security administrator."
	}
	alertMsg := fmt.Sprintf("[BAP ZERO-TRUST] Session %s is REVOKED. Authority burned: no further user prompts will be accepted.", sessionID)
	fmt.Fprintln(os.Stderr, alertMsg)
	fmt.Fprintf(os.Stderr, "[SECURITY ALERT] Reason: %s\n", reason)

	resp := map[string]any{
		"continue":      false,
		"stopReason":    fmt.Sprintf("[BAP REVOKED] %s", reason),
		"systemMessage": alertMsg,
		"decision":      "block",
		"reason":        reason,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)

	ppid := os.Getppid()
	if ppid > 0 {
		fmt.Fprintf(os.Stderr, "[BAP ZERO-TRUST] ⚡ Terminating rogue agent process (PID: %d)...\n", ppid)
		killProcessPID(ppid)
	}
	for _, loc := range []string{".bap-session.json", "../.bap-session.json"} {
		if data, err := os.ReadFile(loc); err == nil {
			var sInfo struct {
				PID int `json:"pid"`
			}
			if json.Unmarshal(data, &sInfo) == nil && sInfo.PID > 0 {
				killProcessPID(sInfo.PID)
			}
		}
	}

	os.Exit(2)
}

func handleUserPromptHook(payload HookPayload) {
	prompt := strings.TrimSpace(payload.UserPrompt)
	if prompt == "" {
		prompt = strings.TrimSpace(payload.Prompt)
	}

	serverURL := resolveServerURL()
	sessionID := resolveSessionID(payload)

	// Zero-Trust Revocation Check: STOP taking user inputs if revoked
	if revoked, reason := isSessionRevoked(serverURL, sessionID); revoked {
		outputPromptBlocked(sessionID, reason)
		return
	}

	if prompt != "" {
		writeSessionPrompt(sessionID, prompt)

		// The hook is the sole prompt telemetry producer. Watchers only maintain
		// liveness and revocation state, so one submission creates one event.
		if sessionID != "" && serverURL != "" {
			reqBody, _ := json.Marshal(map[string]any{
				"session_id":  sessionID,
				"user_prompt": prompt,
				"producer":    "claude-lifecycle-hook",
			})
			client := getHTTPClient()
			resp, err := client.Post(serverURL+"/api/v1/sessions/prompt", "application/json", bytes.NewReader(reqBody))
			if err == nil && resp != nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusForbidden {
					// Control plane rejected prompt due to revocation
					outputPromptBlocked(sessionID, "Session authority revoked by administrator on control plane")
					return
				}
			}
		}
	}

	os.Stdout.WriteString("{\"status\":\"ok\"}\n")
	os.Exit(0)
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

	// 2. Lifecycle Hook Handling: SessionStart
	if strings.EqualFold(payload.HookEventName, "SessionStart") {
		handleSessionStartHook(payload)
		return
	}

	// 3. Lifecycle Hook Handling: UserPromptSubmit / UserPrompt
	if strings.EqualFold(payload.HookEventName, "UserPromptSubmit") || strings.EqualFold(payload.HookEventName, "UserPrompt") {
		handleUserPromptHook(payload)
		return
	}

	sessionID := resolveSessionID(payload)
	if revoked, reason := isSessionRevoked(resolveServerURL(), sessionID); revoked {
		outputDecision("deny", reason, "This session cannot perform further governed tool actions.")
		return
	}

	workspaceRoot := resolveWorkspaceRoot()

	// 4. Intercept direct file inspection/modification tools (Read, View, Edit, Write)
	targetFile := payload.ToolInput.FilePath
	if targetFile == "" {
		targetFile = payload.ToolInput.Path
	}
	if targetFile != "" {
		normPath := strings.ToLower(filepath.ToSlash(targetFile))
		if isPathOutsideWorkspace(workspaceRoot, targetFile) {
			outputDecision("deny", fmt.Sprintf("Directory traversal outside workspace is forbidden for file %q", targetFile), "Access to files outside the project root is prohibited. Child module and subproject files are permitted inside the workspace.")
			return
		}
		if strings.Contains(normPath, ".env") || strings.Contains(normPath, ".aws") || strings.Contains(normPath, ".ssh") {
			outputDecision("deny", fmt.Sprintf("Access to sensitive credential file %q is strictly forbidden by policy", targetFile), "Sensitive credential file access blocked")
			return
		}
		// If it's a safe file tool operation (Read, View, Edit, Write), allow immediately
		if payload.ToolName == "Read" || payload.ToolName == "View" || payload.ToolName == "Edit" || payload.ToolName == "Write" {
			outputDecision("allow", "", "File operation within workspace permitted")
			return
		}
	}

	// 5. Process Command execution (Bash or generic command)
	command := strings.TrimSpace(payload.ToolInput.Command)
	if command == "" {
		if payload.ToolName == "Read" || payload.ToolName == "View" || payload.ToolName == "Edit" || payload.ToolName == "Write" {
			outputDecision("allow", "", "File operation permitted")
			return
		}
		outputDecision("deny", "No command or target specified in tool input", "Empty command")
		return
	}

	// Pre-flight check for shell command directory traversal escaping workspace
	if checkCommandWorkspaceEscape(workspaceRoot, command) {
		outputDecision("deny", fmt.Sprintf("Directory traversal outside workspace is forbidden in command: %s", command), "Commands cannot navigate or reference paths outside the project workspace root. Child projects (e.g. Maven child modules) inside the workspace are permitted.")
		return
	}

	// 4. Resolve session identifier
	// Lifecycle hook IDs take precedence over workspace marker files so
	// concurrent Claude sessions do not inherit each other's authority.
	if sessionID == "" {
		sessCandidates := []string{".bap-session.json", "../.bap-session.json", "cchook/.bap-session.json"}
		if exePath, err := os.Executable(); err == nil {
			sessCandidates = append(sessCandidates, filepath.Join(filepath.Dir(exePath), ".bap-session.json"))
		}
		for _, loc := range sessCandidates {
			if data, err := os.ReadFile(loc); err == nil {
				var sInfo struct {
					SessionID string `json:"session_id"`
				}
				if json.Unmarshal(data, &sInfo) == nil && sInfo.SessionID != "" {
					sessionID = sInfo.SessionID
					break
				}
			}
		}
	}
	if sessionID == "" {
		ppid := os.Getppid()
		sessionID = fmt.Sprintf("sess-claude-pid-%d", ppid)
		serverURL := "http://localhost:8080"
		cfgCandidates := []string{"bap-config.json", "../bap-config.json"}
		if exePath, err := os.Executable(); err == nil {
			cfgCandidates = append(cfgCandidates, filepath.Join(filepath.Dir(exePath), "bap-config.json"))
		}
		if home, err := os.UserHomeDir(); err == nil {
			cfgCandidates = append(cfgCandidates, filepath.Join(home, ".bap", "bap-config.json"))
		}
		for _, cfgPath := range cfgCandidates {
			if cfgData, err := os.ReadFile(cfgPath); err == nil {
				var cfg struct {
					ControlPlaneURL string `json:"controlplane_url"`
				}
				if json.Unmarshal(cfgData, &cfg) == nil && cfg.ControlPlaneURL != "" {
					serverURL = cfg.ControlPlaneURL
					break
				}
			}
		}
		_ = os.WriteFile(".bap-session.json", []byte(fmt.Sprintf(`{"session_id":"%s","server_url":"%s","pid":%d}`, sessionID, serverURL, ppid)), 0600)
		ltdBin := findBinary("bapedge.exe")
		if ltdBin == "" {
			ltdBin = findBinary("bapedge")
		}
		if ltdBin != "" {
			wCmd := exec.Command(ltdBin, "watch", fmt.Sprintf("--pid=%d", ppid), fmt.Sprintf("--server=%s", serverURL), fmt.Sprintf("--session-id=%s", sessionID), "--detach")
			_ = wCmd.Start()
		}
	}

	// 5. Resolve bapedge (LTD) or ltd-agent executable
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

	// 6. Execute bapedge exec --source claude-code --session-id <sessionID> --json "$command"
	execArgs := []string{"exec", "--source", "claude-code", "--json"}
	if sessionID != "" {
		execArgs = append(execArgs, "--session-id", sessionID)
	}
	execArgs = append(execArgs, command)

	cmd := exec.Command(ltdBin, execArgs...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "BAP_SESSION_ID="+sessionID)

	// Inject only this session's active prompt. A workspace-global prompt file
	// leaks prompts across concurrent Claude sessions.
	var activePrompt string
	if sessionID != "" {
		pBytes, err := os.ReadFile(promptPathForSession(sessionID))
		if err == nil {
			activePrompt = strings.TrimSpace(string(pBytes))
		}
	}
	if activePrompt == "" {
		activePrompt = os.Getenv("BAP_USER_PROMPT")
	}
	if activePrompt != "" {
		cmd.Env = append(cmd.Env, "BAP_USER_PROMPT="+activePrompt)
	}

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
			reason = "Command execution denied by bapedge Cedar security policy"
		}
		contextMsg := reason
		if execResp.Suggestion != "" {
			contextMsg = fmt.Sprintf("%s. SUGGESTION: %s", reason, execResp.Suggestion)
		}
		outputDecision("deny", reason, contextMsg)
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

func resolveWorkspaceRoot() string {
	if root := os.Getenv("BAP_WORKSPACE_ROOT"); root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			return filepath.Clean(abs)
		}
	}
	if root := os.Getenv("LTD_WORKSPACE_ROOT"); root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			return filepath.Clean(abs)
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for {
			if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
				return filepath.Clean(dir)
			}
			parent := filepath.Dir(dir)
			if parent == dir || parent == "" {
				break
			}
			dir = parent
		}
		return filepath.Clean(cwd)
	}
	return "."
}

func isPathOutsideWorkspace(workspaceRoot, targetPath string) bool {
	if workspaceRoot == "" || targetPath == "" {
		return false
	}
	cleanRoot := filepath.Clean(workspaceRoot)
	var absTarget string
	if filepath.IsAbs(targetPath) {
		absTarget = filepath.Clean(targetPath)
	} else {
		absTarget = filepath.Clean(filepath.Join(cleanRoot, targetPath))
	}

	rel, err := filepath.Rel(cleanRoot, absTarget)
	if err != nil {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || strings.HasPrefix(rel, "../")
}

func checkCommandWorkspaceEscape(workspaceRoot, fullCommand string) bool {
	if workspaceRoot == "" || fullCommand == "" {
		return false
	}
	cleanRoot := filepath.Clean(workspaceRoot)
	currentDir := cleanRoot

	cmdUnified := fullCommand
	for _, sep := range []string{"&&", "||", ";", "|", "&"} {
		cmdUnified = strings.ReplaceAll(cmdUnified, sep, "\n")
	}

	lines := strings.Split(cmdUnified, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		tokens := tokenizeHookCommand(line)
		if len(tokens) == 0 {
			continue
		}

		first := strings.ToLower(tokens[0])
		if first == "cd" || first == "chdir" || first == "pushd" {
			if len(tokens) >= 2 {
				cdTarget := strings.Trim(tokens[1], "\"'")
				if strings.EqualFold(cdTarget, "/d") && len(tokens) >= 3 {
					cdTarget = strings.Trim(tokens[2], "\"'")
				}
				if cdTarget == "" || cdTarget == "~" {
					return true
				}
				var nextAbs string
				if filepath.IsAbs(cdTarget) {
					nextAbs = filepath.Clean(cdTarget)
				} else {
					nextAbs = filepath.Clean(filepath.Join(currentDir, cdTarget))
				}
				if isPathOutsideWorkspace(cleanRoot, nextAbs) {
					return true
				}
				currentDir = nextAbs
			}
			continue
		}

		for i, tok := range tokens {
			if i == 0 {
				if !strings.HasPrefix(tok, "..") {
					continue
				}
			}
			tokVal := strings.Trim(tok, "\"'")
			if idx := strings.Index(tokVal, "="); idx != -1 && strings.HasPrefix(tokVal, "-") {
				tokVal = tokVal[idx+1:]
			}
			if strings.Contains(tokVal, "..") {
				var absArg string
				if filepath.IsAbs(tokVal) {
					absArg = filepath.Clean(tokVal)
				} else {
					absArg = filepath.Clean(filepath.Join(currentDir, tokVal))
				}
				if isPathOutsideWorkspace(cleanRoot, absArg) {
					return true
				}
			}
			if isExplicitAbsolutePath(tokVal) {
				cleanAbs := filepath.Clean(tokVal)
				if isPathOutsideWorkspace(cleanRoot, cleanAbs) && !isSystemBinaryPath(cleanAbs) {
					return true
				}
			}
		}
	}
	return false
}

func isExplicitAbsolutePath(p string) bool {
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return true
	}
	if strings.HasPrefix(p, "\\\\") || strings.HasPrefix(p, "//") {
		return true
	}
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "/c") && !strings.HasPrefix(p, "/d") && !strings.HasPrefix(p, "/s") && !strings.HasPrefix(p, "/b") {
		return true
	}
	return false
}

func isSystemBinaryPath(p string) bool {
	lower := strings.ToLower(filepath.ToSlash(p))
	systemPrefixes := []string{
		"c:/windows/",
		"c:/program files/",
		"c:/program files (x86)/",
		"/usr/bin/",
		"/bin/",
		"/usr/local/bin/",
	}
	for _, sp := range systemPrefixes {
		if strings.HasPrefix(lower, sp) && (strings.HasSuffix(lower, ".exe") || !strings.Contains(filepath.Base(lower), ".")) {
			return true
		}
	}
	return false
}

func tokenizeHookCommand(cmd string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	var quoteChar rune

	for _, r := range cmd {
		if inQuote {
			if r == quoteChar {
				inQuote = false
			}
			current.WriteRune(r)
		} else {
			if r == '"' || r == '\'' {
				inQuote = true
				quoteChar = r
				current.WriteRune(r)
			} else if r == ' ' || r == '\t' {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(r)
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}
