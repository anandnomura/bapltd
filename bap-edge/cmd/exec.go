package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bap-edge/internal/audit"
	"bap-edge/internal/authz"
	"bap-edge/internal/config"
	"bap-edge/internal/sandbox"
	"bap-edge/pkg/types"
)

type execContext struct {
	startTime    time.Time
	source       string
	sessionID    string
	serverURL    string
	auditLogPath string
	fullCommand  string
	executable   string
	cmdArguments string
	forceJSON    bool
	forceRaw     bool
}

func (ec *execContext) exit(resp types.ExecResponse, code int) {
	if !resp.Allowed && resp.Suggestion == "" {
		resp.Suggestion = GenerateSuggestion(ec.fullCommand, ec.executable, resp.Reason)
	}
	durationMs := time.Since(ec.startTime).Milliseconds()
	decision := "deny"
	if resp.Allowed {
		decision = "allow"
	}
	uID, uEmail, spiffeID := resolveLocalIdentity()
	entry := audit.AuditEntry{
		SessionID:   ec.sessionID,
		UserID:      uID,
		UserEmail:   uEmail,
		SPIFFEID:    spiffeID,
		Timestamp:   ec.startTime.UTC(),
		Source:      ec.source,
		ClientPID:   os.Getpid(),
		Executable:  ec.executable,
		Arguments:   ec.cmdArguments,
		FullCommand: ec.fullCommand,
		Decision:    decision,
		Reason:      resp.Reason,
		DurationMs:  durationMs,
		ExitCode:    code,
	}
	if !resp.Allowed && entry.Reason == "" {
		entry.Reason = "Blocked by security policy"
	}
	_ = audit.Log(&entry, ec.auditLogPath)
	_, _, _ = audit.Transmit(entry, ec.serverURL, ec.auditLogPath)

	exitWithResponse(resp, code, ec.forceJSON, ec.forceRaw, isAgentEnvironment(ec.source))
}

// RunExec handles the 'exec' subcommand.
func RunExec(args []string) {
	startTime := time.Now()

	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	policyPath := fs.String("policy", "", "Path to policy.cedar file (defaults to ./policy.cedar)")
	jsonFlag := fs.Bool("json", false, "Output in JSON format (default: auto-detect TTY)")
	rawFlag := fs.Bool("raw", false, "Force output in raw text format")

	defaultSource := os.Getenv("LTD_SOURCE")
	if defaultSource == "" {
		defaultSource = "cli"
	}
	sourceFlag := fs.String("source", defaultSource, "Identifier of agent invoking command (e.g. claude-code, copilot, cli)")
	auditLogFlag := fs.String("audit-log", "", "Path to audit log file in JSON lines (defaults to LTD_AUDIT_LOG or ltd-audit.jsonl, 'off' to disable)")

	defaultSession := os.Getenv("BAP_SESSION_ID")
	if defaultSession == "" {
		defaultSession = os.Getenv("LTD_SESSION_ID")
	}
	sessionFlag := fs.String("session-id", defaultSession, "Session identifier for grouping agent actions")

	epCfg := config.ResolveEndpoints()
	serverFlag := fs.String("server", epCfg.ControlPlaneURL, "Central control plane URL for telemetry streaming")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing arguments: %v\n", err)
		os.Exit(1)
	}

	ec := &execContext{
		startTime:    startTime,
		source:       *sourceFlag,
		sessionID:    *sessionFlag,
		serverURL:    *serverFlag,
		auditLogPath: *auditLogFlag,
		forceJSON:    *jsonFlag,
		forceRaw:     *rawFlag,
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		resp := types.ExecResponse{
			Allowed: false,
			Reason:  "No command provided to exec. Usage: ltd-agent exec <shell_command>",
		}
		ec.exit(resp, 1)
	}

	// Join all remaining args as the shell command string, preserving quotes for arguments with spaces
	var fullCommand string
	if len(cmdArgs) == 1 {
		fullCommand = strings.TrimSpace(cmdArgs[0])
	} else {
		var parts []string
		for _, arg := range cmdArgs {
			parts = append(parts, quoteArg(arg))
		}
		fullCommand = strings.Join(parts, " ")
	}
	fullCommand = sandbox.CleanCommandString(fullCommand)
	ec.fullCommand = fullCommand

	// Fast sync of remote revocations from control plane if reachable
	syncRevocationsFast(ec.serverURL, *policyPath)

	// Check if this session or the system has been revoked by CISO administrator
	if revErr := authz.CheckSessionRevocation(*policyPath, ec.sessionID); revErr != nil {
		resp := types.ExecResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("EXECUTION BLOCKED: %v", revErr),
		}
		ec.exit(resp, 1)
	}

	// 1. Initialize Cedar authorizer
	authorizer, err := authz.NewAuthorizer(*policyPath)
	if err != nil {
		resp := types.ExecResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("Failed to load Cedar policy: %v", err),
		}
		ec.exit(resp, 1)
	}

	// 2. Parse command for Cedar context
	executable, cmdArguments := sandbox.ParseCommand(fullCommand)
	ec.executable = executable
	ec.cmdArguments = cmdArguments

	// 3. Evaluate command against Cedar policies
	allowed, reason, err := authorizer.Evaluate(executable, fullCommand, cmdArguments)
	if err != nil {
		resp := types.ExecResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("Error during Cedar policy evaluation: %v", err),
		}
		ec.exit(resp, 1)
	}

	if !allowed {
		resp := types.ExecResponse{
			Allowed: false,
			Reason:  reason,
		}
		ec.exit(resp, 1)
	}

	// 4. Execute sandboxed command (allowed by Cedar)
	output, execErr := sandbox.RunSandboxedCommand(fullCommand)

	resp := types.ExecResponse{
		Allowed: true,
		Output:  output,
	}
	if execErr != nil {
		resp.Reason = fmt.Sprintf("Command execution failed: %v", execErr)
		ec.exit(resp, 1)
	}
	ec.exit(resp, 0)
}

func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func isAgentEnvironment(source string) bool {
	if source != "" && source != "cli" {
		return true
	}
	agentEnvs := []string{
		"BAP_SESSION_ID",
		"LTD_SESSION_ID",
		"ANTIGRAVITY",
		"AGY_SESSION",
		"GEMINI_AGENT",
		"CLAUDE_CODE",
		"COPILOT_AGENT",
		"AI_AGENT",
		"AGENT_NAME",
		"AUTO_GPT",
	}
	for _, envKey := range agentEnvs {
		if os.Getenv(envKey) != "" {
			return true
		}
	}
	return false
}

func exitWithResponse(resp types.ExecResponse, code int, forceJSON, forceRaw bool, isAgent bool) {
	outputJSON := forceJSON || (!forceRaw && (!isTerminal(os.Stdout) || isAgent))

	if outputJSON {
		printJSONAndExit(resp, code)
		return
	}

	// Human-friendly / interactive terminal output
	if !resp.Allowed {
		fmt.Fprintf(os.Stderr, "[DENIED] %s\n", resp.Reason)
		if resp.Suggestion != "" {
			fmt.Fprintf(os.Stderr, "[SUGGESTION] %s\n", resp.Suggestion)
		}
		fmt.Fprintf(os.Stderr, "[TIP] Pass '-json' to receive machine-readable structured JSON responses.\n")
		os.Exit(code)
	}

	if resp.Output != "" {
		fmt.Println(resp.Output)
	}
	if resp.Reason != "" {
		fmt.Fprintf(os.Stderr, "[ERROR] %s\n", resp.Reason)
		if resp.Suggestion != "" {
			fmt.Fprintf(os.Stderr, "[SUGGESTION] %s\n", resp.Suggestion)
		}
	}
	os.Exit(code)
}

// GenerateSuggestion produces intelligent, actionable advice when a command is denied.
func GenerateSuggestion(fullCmd, execName, reason string) string {
	lower := strings.ToLower(fullCmd)

	if strings.Contains(lower, "invoke-restmethod") || strings.Contains(lower, "invoke-webrequest") ||
		strings.Contains(lower, "curl") || strings.Contains(lower, "wget") ||
		strings.Contains(lower, "iwr ") || strings.Contains(lower, "irm ") ||
		strings.Contains(lower, "downloadstring") || strings.Contains(lower, "downloadfile") {
		return "External network egress is restricted by Zero-Trust policy. For local LLM inference, use 'http://localhost:11434' or 'http://127.0.0.1:11434'. For external banking APIs or web services, route through the BAP Gateway PEP (http://localhost:9090) with an authorized BAP Grant."
	}

	if strings.Contains(lower, ".env") {
		return "Direct access or tampering with .env credential files is strictly prohibited. Access required configuration via sandboxed environment variables (e.g., $env:VARIABLE) or the corporate Secret Store."
	}

	if strings.Contains(lower, ".ssh") || strings.Contains(lower, ".aws") || strings.Contains(lower, "id_rsa") {
		return "Direct access to private developer credentials (~/.ssh, ~/.aws) is blocked. Use the injected corporate identity token (CORP_OBO_TOKEN) or SPIFFE workload identities provided by BAP."
	}

	if strings.Contains(lower, "-encodedcommand") || strings.Contains(lower, "-enc ") || strings.Contains(lower, "-encoded ") {
		return "Base64-encoded PowerShell execution is blocked to prevent defense evasion. Provide the decoded PowerShell script or command directly."
	}

	if strings.Contains(lower, "tcpclient") || strings.Contains(lower, "system.net.sockets") || strings.Contains(lower, "udpclient") {
		return "Opening raw network sockets directly on the host is prohibited on edge agents. Egress must be governed through the BAP Gateway PEP."
	}

	if strings.Contains(lower, "set-mppreference") || strings.Contains(lower, "disablerealtimemonitoring") {
		return "Disabling or altering host security controls and antivirus preferences is strictly prohibited."
	}

	if strings.Contains(lower, "remove-item") || strings.Contains(lower, "rmdir") || strings.Contains(lower, "rd /s") {
		return "Mass recursive deletion targeting system paths is blocked to protect workspace integrity."
	}

	if strings.Contains(lower, "comsvcs") || strings.Contains(lower, "minidump") || strings.Contains(lower, "sekurlsa") {
		return "Process memory dumping and credential harvesting techniques are blocked by zero-trust invariant."
	}

	return "Command executable is not in the approved developer whitelist. Permitted toolchains include: python, git, go, npm, maven, gradle, cargo, java, powershell, cmd, and standard inspection utilities."
}

func printJSONAndExit(resp types.ExecResponse, code int) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
	os.Exit(code)
}

func quoteArg(arg string) string {
	if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") && len(arg) >= 2 {
		return arg
	}
	if strings.ContainsAny(arg, " \t\r\n") {
		return `"` + arg + `"`
	}
	return arg
}

func syncRevocationsFast(serverURL, policyPath string) {
	if serverURL == "" || os.Getenv("BAP_OFFLINE") == "1" || os.Getenv("BAP_TEST_MODE") == "1" {
		return
	}
	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Get(serverURL + "/api/v1/control/revocations")
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	var data struct {
		KillSwitch      bool     `json:"kill_switch"`
		RevokedSessions []string `json:"revoked_sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return
	}

	// Update local policy-state.json
	statePath := "policy-state.json"
	if policyPath != "" {
		statePath = filepath.Join(filepath.Dir(policyPath), "policy-state.json")
	}
	var existing map[string]any
	if content, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(content, &existing)
	}
	if existing == nil {
		existing = make(map[string]any)
	}
	existing["kill_switch"] = data.KillSwitch
	existing["revoked_sessions"] = data.RevokedSessions
	existing["last_sync"] = time.Now().UTC()
	if updated, err := json.MarshalIndent(existing, "", "  "); err == nil {
		_ = os.WriteFile(statePath, updated, 0600)
	}
}
