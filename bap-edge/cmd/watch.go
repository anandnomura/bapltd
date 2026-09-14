package cmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
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
	// 1. Persist local workspace marker
	marker := map[string]any{
		"session_id": sessionID,
		"server_url": serverURL,
		"pid":        watchPID,
		"app_id":     appID,
		"started_at": time.Now().UTC(),
	}
	if data, err := json.MarshalIndent(marker, "", "  "); err == nil {
		_ = os.WriteFile(".bap-session.json", data, 0600)
	}

	// 2. Enroll session into central control plane
	hostname, _ := os.Hostname()
	username := os.Getenv("USERNAME")
	if username == "" {
		username = os.Getenv("USER")
	}
	startPayload := map[string]any{
		"session_id": sessionID,
		"app_id":     appID,
		"user_id":    username,
		"hostname":   hostname,
		"client_pid": watchPID,
	}
	_ = postJSONQuick(serverURL+"/api/v1/sessions/start", startPayload)

	// 3. Monitor process liveness
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	heartbeatCount := 0
	for range ticker.C {
		if !isProcessAlive(watchPID) {
			// Workload terminated (exit, Ctrl+C, or window close)
			endPayload := map[string]any{
				"session_id": sessionID,
				"reason":     "Workload process exited cleanly",
			}
			_ = postJSONQuick(serverURL+"/api/v1/sessions/end", endPayload)
			_ = os.Remove(".bap-session.json")
			return
		}

		// Keep alive & sync state periodically
		heartbeatCount++
		if heartbeatCount%2 == 0 { // every ~4 seconds
			_ = postJSONQuick(serverURL+"/api/v1/sessions/heartbeat", map[string]any{
				"session_id": sessionID,
			})
			_ = postJSONQuick(serverURL+"/api/v1/instances/heartbeat", map[string]any{
				"agent_id": sessionID,
			})
			syncRevocationsFast(serverURL, "policy.cedar")
		}
	}
}

func postJSONQuick(url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}
