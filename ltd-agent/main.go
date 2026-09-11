package main

import (
	"fmt"
	"os"

	"ltd-agent/cmd"
)

const usage = `ltd-agent - Zero-Trust Execution Broker for AI Agents

Usage:
  ltd-agent <command> [arguments]

Available Commands:
  serve     Start the zero-trust attestation server on Unix domain socket
  exec      Evaluate command against Cedar policy and run in sandboxed kernel namespace
  attest    Client test command: connect to attestation server and request OBO JWT
  help      Display help information

Examples:
  ltd-agent serve --socket /tmp/ltd.sock
  ltd-agent exec "echo hello world"
  ltd-agent exec "ls -la /"
  ltd-agent exec "rm -rf /"
  ltd-agent attest --socket /tmp/ltd.sock
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "serve":
		if err := cmd.RunServe(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error running server: %v\n", err)
			os.Exit(1)
		}
	case "exec":
		cmd.RunExec(args)
	case "attest":
		if err := cmd.RunAttest(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error running attestation client: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		fmt.Print(usage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n\n%s", command, usage)
		os.Exit(1)
	}
}

