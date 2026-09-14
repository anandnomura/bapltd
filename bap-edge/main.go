package main

import (
	"fmt"
	"os"
	"strings"

	"bap-edge/cmd"
)

const usage = `bapedge - BAP Local Trusted Daemon (LTD) & Zero-Trust Execution Broker
(Alias: ltd-agent)

Usage:
  bapedge <command> [arguments]

Available Commands:
  config    View or update central control plane and gateway host URLs
  register  Enroll edge LTD with central BAP Control Plane using one-time code (OTC)
  sync      Synchronize or inspect local policy cache from control plane (with offline fallback)
  serve     Start the zero-trust attestation server on Unix domain socket
  exec      Evaluate command against Cedar policy and run in sandboxed kernel namespace
  mcp       Run as Model Context Protocol (MCP) stdio server for Claude, Copilot, and Cursor
  attest      Client test command: connect to attestation server and request OBO JWT
  verify-log  Verify cryptographic integrity and anti-tamper hash-chain of local audit log
  help        Display help information

Examples:
  bapedge register --server http://localhost:8080 --code LTD-OTC-XXXX-XXXX
  bapedge sync --server http://localhost:8080
  bapedge exec "echo hello world"
  bapedge exec "ls -la /"
  bapedge exec "rm -rf /"
  bapedge serve --socket /tmp/ltd.sock
  bapedge attest --socket /tmp/ltd.sock
`

func main() {
	if len(os.Args) < 2 {
		// If invoked as bapmcp.exe or containing "mcp", default to running the MCP server
		if strings.Contains(strings.ToLower(os.Args[0]), "mcp") {
			if err := cmd.RunMCP(nil); err != nil {
				fmt.Fprintf(os.Stderr, "Error running MCP server: %v\n", err)
				os.Exit(1)
			}
			return
		}
		fmt.Print(usage)
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "register":
		if err := cmd.RunRegister(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error registering agent: %v\n", err)
			os.Exit(1)
		}
	case "sync":
		if err := cmd.RunSync(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error synchronizing policy: %v\n", err)
			os.Exit(1)
		}
	case "serve":
		if err := cmd.RunServe(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error running server: %v\n", err)
			os.Exit(1)
		}
	case "exec":
		cmd.RunExec(args)
	case "mcp":
		if err := cmd.RunMCP(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error running MCP server: %v\n", err)
			os.Exit(1)
		}
	case "attest":
		if err := cmd.RunAttest(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error running attestation client: %v\n", err)
			os.Exit(1)
		}
	case "config":
		if err := cmd.RunConfig(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error managing configuration: %v\n", err)
			os.Exit(1)
		}
	case "watch":
		if err := cmd.RunWatch(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error running watcher: %v\n", err)
			os.Exit(1)
		}
	case "verify-log":
		if err := cmd.RunVerifyLog(args); err != nil {
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
