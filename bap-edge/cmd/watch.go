package cmd

import (
	"bap-edge/internal/httptransport"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"bap-edge/internal/config"
)

// RunWatch starts a process watcher thread / background monitor for an agent session.
func RunWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	pidFlag := fs.Int("pid", 0, "Target process ID to watch (default: parent PID)")
	serverFlag := fs.String("server", "", "Central control plane URL")
	sessionFlag := fs.String("session-id", "", "Session ID to watch and govern")
	appFlag := fs.String("app-id", "claude-code", "Governed workload identifier")
	detachFlag := fs.Bool("detach", false, "Spawn background detached process and exit immediately")

	if err := fs.Parse(args); err != nil {
		return err
	}

	watchPID := *pidFlag
	if watchPID <= 0 {
		watchPID = os.Getppid()
	}

	serverURL := *serverFlag
	if serverURL == "" {
		ep := config.ResolveEndpoints()
		serverURL = ep.ControlPlaneURL
	}
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}
	serverURL = strings.TrimRight(serverURL, "/")

	sessionID := *sessionFlag
	if sessionID == "" {
		sessionID = os.Getenv("BAP_SESSION_ID")
	}
	if sessionID == "" {
		for _, loc := range []string{".bap-session.json", "../.bap-session.json"} {
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
		sessionID = fmt.Sprintf("sess-%s-%d", *appFlag, watchPID)
	}

	if *detachFlag {
		exe, err := os.Executable()
		if err != nil {
			exe = "bapedge.exe"
		}
		cmdArgs := []string{
			"watch",
			fmt.Sprintf("--pid=%d", watchPID),
			fmt.Sprintf("--server=%s", serverURL),
			fmt.Sprintf("--session-id=%s", sessionID),
			fmt.Sprintf("--app-id=%s", *appFlag),
		}
		detachedCmd := exec.Command(exe, cmdArgs...)
		detachedCmd.SysProcAttr = getSysProcAttrDetached()
		if err := detachedCmd.Start(); err != nil {
			return fmt.Errorf("failed to start detached watcher: %w", err)
		}
		return nil
	}

	runWatchLoop(watchPID, serverURL, sessionID, *appFlag)
	return nil
}

func runWatchLoop(watchPID int, serverURL, sessionID, appID string) {
	// Prompt telemetry is emitted by the lifecycle hook. The watcher must not
	// replay workspace prompt files because multiple sessions can share a cwd.
	initialPrompt := strings.TrimSpace(os.Getenv("BAP_USER_PROMPT"))

	// 2. Persist local workspace marker
	marker := map[string]any{
		"session_id":  sessionID,
		"server_url":  serverURL,
		"pid":         watchPID,
		"app_id":      appID,
		"user_prompt": initialPrompt,
		"started_at":  time.Now().UTC(),
	}
	if data, err := json.MarshalIndent(marker, "", "  "); err == nil {
		_ = os.WriteFile(".bap-session.json", data, 0600)
	}

	// 3. Enroll session into central control plane
	hostname, _ := os.Hostname()
	username := os.Getenv("USERNAME")
	if username == "" {
		username = os.Getenv("USER")
	}
	startPayload := map[string]any{
		"session_id":  sessionID,
		"app_id":      appID,
		"user_id":     username,
		"hostname":    hostname,
		"client_pid":  watchPID,
		"user_prompt": initialPrompt,
	}
	_ = postJSONQuick(serverURL+"/api/v1/sessions/start", startPayload)

	// 4. Continuous Heartbeat and Liveness Monitor
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	deadCheckCount := 0
	revokedCount := 0
	for range ticker.C {
		// Only check process termination if a valid target PID was specified
		if watchPID > 0 {
			if !isProcessAlive(watchPID) {
				deadCheckCount++
				if deadCheckCount >= 3 {
					// Workload confirmed terminated (exit, Ctrl+C, or window close)
					endPayload := map[string]any{
						"session_id": sessionID,
						"reason":     "Workload process exited cleanly",
					}
					_ = postJSONQuick(serverURL+"/api/v1/sessions/end", endPayload)
					_ = os.Remove(".bap-session.json")
					return
				}
			} else {
				deadCheckCount = 0
			}
		}

		// Keep alive & pulse heartbeat continuously (no matter whether CC is talking or idle)
		revokedByHB, reasonHB := pulseHeartbeat(serverURL, sessionID)
		syncRevocationsFast(serverURL, "policy.cedar")
		revokedByState := isSessionRevokedInState(sessionID)

		if revokedByHB || revokedByState {
			reason := reasonHB
			if reason == "" {
				reason = "Session authority revoked by administrator (synced policy state)"
			}
			fmt.Fprintf(os.Stderr, "[bapedge watch] 🚨 Session %s REVOKED by control plane: %s\n", sessionID, reason)
			writeLocalRevoked(sessionID, reason)
			revokedCount++

			// Option 3: Warn on tick 1, hard kill rogue workload process on tick 2
			if revokedCount >= 2 && watchPID > 0 {
				fmt.Fprintf(os.Stderr, "[bapedge watch] ⚡ Terminating rogue agent workload (PID: %d)...\n", watchPID)
				killProcessPID(watchPID)
				_ = os.Remove(".bap-session.json")
				return
			}
		} else {
			revokedCount = 0
		}
	}
}

func killProcessPID(pid int) {
	if pid <= 0 {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid)).Run()
	} else {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	}
}

func writeLocalRevoked(sessionID, reason string) {
	marker := map[string]any{
		"session_id": sessionID,
		"status":     "revoked",
		"revoked_at": time.Now().UTC(),
		"reason":     reason,
	}
	if data, err := json.MarshalIndent(marker, "", "  "); err == nil {
		_ = os.WriteFile(".bap-revoked", data, 0600)
	}
}

func isSessionRevokedInState(sessionID string) bool {
	data, err := os.ReadFile("policy-state.json")
	if err != nil {
		return false
	}
	var st struct {
		KillSwitch      bool     `json:"kill_switch"`
		RevokedSessions []string `json:"revoked_sessions"`
	}
	if json.Unmarshal(data, &st) != nil {
		return false
	}
	if st.KillSwitch {
		return true
	}
	sessLower := strings.ToLower(sessionID)
	for _, rev := range st.RevokedSessions {
		revLower := strings.ToLower(strings.TrimSpace(rev))
		if revLower != "" && (strings.Contains(sessLower, revLower) || strings.Contains(revLower, sessLower)) {
			return true
		}
	}
	return false
}

func pulseHeartbeat(serverURL, sessionID string) (bool, string) {
	body, err := json.Marshal(map[string]any{"session_id": sessionID})
	if err != nil {
		return false, ""
	}
	client := httptransport.New(2 * time.Second)
	resp, err := client.Post(serverURL+"/api/v1/sessions/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	var res struct {
		Status string `json:"status"`
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
		if res.Status == "revoked" || res.Action == "terminate" {
			return true, res.Reason
		}
	}
	return false, ""
}

func postJSONQuick(url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := httptransport.New(2 * time.Second)
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[bapedge watch] Error posting to %s: %v\n", url, err)
		return err
	}
	_ = resp.Body.Close()
	return nil
}
