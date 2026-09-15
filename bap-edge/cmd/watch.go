package cmd

import (
	"bap-edge/internal/httptransport"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
	// 1. Check if user prompt exists
	var initialPrompt string
	if promptData, err := os.ReadFile(".bap-prompt.txt"); err == nil {
		initialPrompt = strings.TrimSpace(string(promptData))
	}
	if initialPrompt == "" {
		initialPrompt = os.Getenv("BAP_USER_PROMPT")
	}

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
	lastSyncedPrompt := initialPrompt

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
					_ = os.Remove(".bap-prompt.txt")
					return
				}
			} else {
				deadCheckCount = 0
			}
		}

		// Keep alive & pulse heartbeat continuously (no matter whether CC is talking or idle)
		_ = postJSONQuick(serverURL+"/api/v1/sessions/heartbeat", map[string]any{
			"session_id": sessionID,
		})
		_ = postJSONQuick(serverURL+"/api/v1/instances/heartbeat", map[string]any{
			"agent_id": sessionID,
		})

		// Check for prompt updates from .bap-prompt.txt
		if pBytes, err := os.ReadFile(".bap-prompt.txt"); err == nil {
			currentPrompt := strings.TrimSpace(string(pBytes))
			if currentPrompt != "" && currentPrompt != lastSyncedPrompt {
				lastSyncedPrompt = currentPrompt
				_ = postJSONQuick(serverURL+"/api/v1/sessions/prompt", map[string]any{
					"session_id":  sessionID,
					"user_prompt": currentPrompt,
				})
			}
		}

		syncRevocationsFast(serverURL, "policy.cedar")
	}
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
