package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"ltd-agent/internal/authz"
	"ltd-agent/internal/sandbox"
	"ltd-agent/pkg/types"
)

// RunExec handles the 'exec' subcommand.
func RunExec(args []string) {
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	policyPath := fs.String("policy", "", "Path to policy.cedar file (defaults to ./policy.cedar)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing arguments: %v\n", err)
		os.Exit(1)
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		resp := types.ExecResponse{
			Allowed: false,
			Reason:  "No command provided to exec. Usage: ltd-agent exec <shell_command>",
		}
		printJSONAndExit(resp, 1)
	}

	// Join all remaining args as the shell command string
	fullCommand := strings.Join(cmdArgs, " ")

	// 1. Initialize Cedar authorizer
	authorizer, err := authz.NewAuthorizer(*policyPath)
	if err != nil {
		resp := types.ExecResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("Failed to load Cedar policy: %v", err),
		}
		printJSONAndExit(resp, 1)
	}

	// 2. Parse command for Cedar context
	executable, cmdArguments := sandbox.ParseCommand(fullCommand)

	// 3. Evaluate command against Cedar policies
	allowed, reason, err := authorizer.Evaluate(executable, fullCommand, cmdArguments)
	if err != nil {
		resp := types.ExecResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("Error during Cedar policy evaluation: %v", err),
		}
		printJSONAndExit(resp, 1)
	}

	if !allowed {
		resp := types.ExecResponse{
			Allowed: false,
			Reason:  reason,
		}
		printJSONAndExit(resp, 1)
	}

	// 4. Execute sandboxed command (allowed by Cedar)
	output, execErr := sandbox.RunSandboxedCommand(fullCommand)

	resp := types.ExecResponse{
		Allowed: true,
		Output:  output,
	}
	if execErr != nil {
		resp.Reason = fmt.Sprintf("Command execution failed: %v", execErr)
		printJSONAndExit(resp, 1)
	}
	printJSONAndExit(resp, 0)
}

func printJSONAndExit(resp types.ExecResponse, code int) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
	os.Exit(code)
}
