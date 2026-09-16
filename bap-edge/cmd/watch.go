package cmd

import (
	"bap-edge/internal/httptransport"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"bap-edge/internal/config"
)

// RunSessionStart executes a zero-trust pre-flight check and starts the session.
// If the user or session is revoked, prints an alert and exits with code 2.
// If authorized, persists .bap-session.json and spawns the background watcher.
func RunSessionStart(args []string) error {
	fs := flag.NewFlagSet("session-start", flag.ContinueOnError)
	serverFlag := fs.String("server", "", "Central control plane URL")
	sessionFlag := fs.String("session-id", "", "Session ID to watch and govern")
	appFlag := fs.String("app-id", "claude-code", "Governed workload identifier")
	pidFlag := fs.Int("pid", 0, "Target process ID to watch (default: parent PID)")
	promptFlag := fs.String("prompt", "", "Initial user prompt")

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
		sessionID = fmt.Sprintf("sess-%s-%d", *appFlag, watchPID)
	}

	username := os.Getenv("USERNAME")
	if username == "" {
		username = os.Getenv("USER")
	}
	hostname, _ := os.Hostname()

	initialPrompt := *promptFlag
	if initialPrompt == "" {
		initialPrompt = strings.TrimSpace(os.Getenv("BAP_USER_PROMPT"))
	}

	// 1. Check local .bap-revoked file
	for _, cand := range []string{".bap-revoked", "../.bap-revoked"} {
		if data, err := os.ReadFile(cand); err == nil {
			var info struct {
				UserID string `json:"user_id"`
				Reason string `json:"reason"`
			}
			if json.Unmarshal(data, &info) == nil {
				if info.UserID == "" || strings.EqualFold(info.UserID, username) {
					reason := info.Reason
					if reason == "" {
						reason = "User access has been revoked by administrator"
					}
					fmt.Fprintf(os.Stderr, "\n===============================================================================\n")
					fmt.Fprintf(os.Stderr, "  [BAP ZERO-TRUST] 🚨 SESSION STARTUP BLOCKED\n")
					fmt.Fprintf(os.Stderr, "  User '%s' access has been REVOKED by administrator.\n", username)
					fmt.Fprintf(os.Stderr, "  Reason: %s\n", reason)
					fmt.Fprintf(os.Stderr, "  All new Claude Code sessions are denied until access is restored.\n")
					fmt.Fprintf(os.Stderr, "===============================================================================\n\n")
					os.Exit(2)
				}
			}
		}
	}

	// 2. Pre-flight check with Central Control Plane
	startPayload := map[string]any{
		"session_id":  sessionID,
		"app_id":      *appFlag,
		"user_id":     username,
		"hostname":    hostname,
		"client_pid":  watchPID,
		"user_prompt": initialPrompt,
	}
	body, _ := json.Marshal(startPayload)
	client := httptransport.New(3 * time.Second)
	resp, err := client.Post(serverURL+"/api/v1/sessions/start", "application/json", bytes.NewReader(body))
	if err == nil && resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusForbidden {
			var errResp struct {
				Error string `json:"error"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&errResp)
			reason := errResp.Error
			if reason == "" {
				reason = "User access has been revoked by administrator"
			}
			writeLocalRevoked(sessionID, reason)
			fmt.Fprintf(os.Stderr, "\n===============================================================================\n")
			fmt.Fprintf(os.Stderr, "  [BAP ZERO-TRUST] 🚨 SESSION STARTUP BLOCKED\n")
			fmt.Fprintf(os.Stderr, "  User '%s' access has been REVOKED by administrator.\n", username)
			fmt.Fprintf(os.Stderr, "  Reason: %s\n", reason)
			fmt.Fprintf(os.Stderr, "  All new Claude Code sessions are denied until access is restored.\n")
			fmt.Fprintf(os.Stderr, "===============================================================================\n\n")
			os.Exit(2)
		}
	}

	// 3. Persist workspace session marker with PID
	marker := map[string]any{
		"session_id":  sessionID,
		"server_url":  serverURL,
		"pid":         watchPID,
		"app_id":      *appFlag,
		"user":        username,
		"hostname":    hostname,
		"user_prompt": initialPrompt,
		"started_at":  time.Now().UTC(),
	}
	if data, err := json.MarshalIndent(marker, "", "  "); err == nil {
		_ = os.WriteFile(".bap-session.json", data, 0600)
	}

	// 4. Start continuous background heartbeat watcher
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
	_ = detachedCmd.Start()

	return nil
}

// RunSessionEnd gracefully closes a session on the control plane and removes local session markers.
func RunSessionEnd(args []string) error {
	fs := flag.NewFlagSet("session-end", flag.ContinueOnError)
	serverFlag := fs.String("server", "", "Central control plane URL")
	sessionFlag := fs.String("session-id", "", "Session ID to end")
	reasonFlag := fs.String("reason", "Claude Code closed gracefully", "Exit reason")

	if err := fs.Parse(args); err != nil {
		return err
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

	if sessionID != "" {
		endPayload := map[string]any{
			"session_id": sessionID,
			"reason":     *reasonFlag,
		}
		_ = postJSONQuick(serverURL+"/api/v1/sessions/end", endPayload)
	}

	_ = os.Remove(".bap-session.json")
	_ = os.Remove("../.bap-session.json")
	_ = os.Remove(".bap-prompt.txt")
	_ = os.Remove("../.bap-prompt.txt")
	return nil
}

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
	initialPrompt := strings.TrimSpace(os.Getenv("BAP_USER_PROMPT"))

	// 1. Ensure local workspace marker exists
	if _, err := os.Stat(".bap-session.json"); os.IsNotExist(err) {
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
	}

	// 2. Enroll session into central control plane (idempotent)
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

	// 3. Continuous Heartbeat and Liveness Monitor
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	deadCheckCount := 0
	for range ticker.C {
		// If session marker was removed locally (e.g. session-end called), workload finished
		if _, err := os.Stat(".bap-session.json"); os.IsNotExist(err) {
			return
		}

		// If watchPID was not specified at launch, look for it in local session marker
		if watchPID <= 0 {
			for _, loc := range []string{".bap-session.json", "../.bap-session.json"} {
				if data, err := os.ReadFile(loc); err == nil {
					var sInfo struct {
						PID int `json:"pid"`
					}
					if json.Unmarshal(data, &sInfo) == nil && sInfo.PID > 0 {
						watchPID = sInfo.PID
						break
					}
				}
			}
		}

		// Only check process termination if a valid target PID was specified
		if watchPID > 0 {
			if !isProcessAlive(watchPID) {
				deadCheckCount++
				if deadCheckCount >= 2 {
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

		// Keep alive & pulse heartbeat continuously
		statusHB, reasonHB := pulseHeartbeat(serverURL, sessionID)
		if statusHB == "closed" {
			if watchPID > 0 {
				fmt.Fprintf(os.Stderr, "[bapedge watch] 🛑 Session %s STOPPED: Terminating workload (PID: %d)...\n", sessionID, watchPID)
				killProcessPID(watchPID)
			}
			_ = os.Remove(".bap-session.json")
			return
		}
		syncRevocationsFast(serverURL, "policy.cedar")
		revokedByState := isSessionRevokedInState(sessionID)

		if statusHB == "revoked" || revokedByState {
			reason := reasonHB
			if reason == "" {
				reason = "Session authority revoked by administrator (synced policy state)"
			}
			fmt.Fprintf(os.Stderr, "[bapedge watch] 🚨 Session %s REVOKED by control plane: %s\n", sessionID, reason)
			writeLocalRevoked(sessionID, reason)
			if watchPID > 0 {
				fmt.Fprintf(os.Stderr, "[bapedge watch] ⚡ Terminating revoked agent workload (PID: %d)...\n", watchPID)
				killProcessPID(watchPID)
			}
			_ = os.Remove(".bap-session.json")
			return
		}
	}
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

func writeLocalRevoked(sessionID, reason string) {
	username := os.Getenv("USERNAME")
	if username == "" {
		username = os.Getenv("USER")
	}
	marker := map[string]any{
		"session_id": sessionID,
		"user_id":    username,
		"status":     "revoked",
		"revoked_at": time.Now().UTC(),
		"reason":     reason,
	}
	if data, err := json.MarshalIndent(marker, "", "  "); err == nil {
		_ = os.WriteFile(".bap-revoked", data, 0600)
		_ = os.WriteFile("../.bap-revoked", data, 0600)
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

func pulseHeartbeat(serverURL, sessionID string) (string, string) {
	body, err := json.Marshal(map[string]any{"session_id": sessionID})
	if err != nil {
		return "", ""
	}
	client := httptransport.New(2 * time.Second)
	resp, err := client.Post(serverURL+"/api/v1/sessions/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	var res struct {
		Status string `json:"status"`
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
		if res.Status == "closed" {
			return "closed", res.Reason
		}
		if res.Status == "revoked" || res.Action == "terminate" {
			return "revoked", res.Reason
		}
	}
	return "ok", ""
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
