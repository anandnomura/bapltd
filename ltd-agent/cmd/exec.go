package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ltd-agent/internal/audit"
	"ltd-agent/internal/authz"
	"ltd-agent/internal/sandbox"
	"ltd-agent/pkg/types"
)

type execContext struct {
	startTime    time.Time
	source       string
	auditLogPath string
	fullCommand  string
	executable   string
	cmdArguments string
	forceJSON    bool
	forceRaw     bool
}

func (ec *execContext) exit(resp types.ExecResponse, code int) {
	durationMs := time.Since(ec.startTime).Milliseconds()
	decision := "deny"
	if resp.Allowed {
		decision = "allow"
	}
	entry := audit.AuditEntry{
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
	_ = audit.Log(entry, ec.auditLogPath)

	exitWithResponse(resp, code, ec.forceJSON, ec.forceRaw)
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

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing arguments: %v\n", err)
		os.Exit(1)
	}

	ec := &execContext{
		startTime:    startTime,
		source:       *sourceFlag,
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

func exitWithResponse(resp types.ExecResponse, code int, forceJSON, forceRaw bool) {
	outputJSON := forceJSON || (!forceRaw && !isTerminal(os.Stdout))

	if outputJSON {
		printJSONAndExit(resp, code)
		return
	}

	// Human-friendly / beautified output (what `jq -r .output` does)
	if !resp.Allowed {
		fmt.Fprintf(os.Stderr, "[DENIED] %s\n", resp.Reason)
		os.Exit(code)
	}

	if resp.Output != "" {
		fmt.Println(resp.Output)
	}
	if resp.Reason != "" {
		fmt.Fprintf(os.Stderr, "[ERROR] %s\n", resp.Reason)
	}
	os.Exit(code)
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

