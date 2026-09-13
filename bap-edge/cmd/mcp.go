package cmd

import (
	"flag"
	"os"

	"bap-edge/internal/config"
	"bap-edge/internal/mcp"
)

// RunMCP handles the 'mcp' subcommand to run BAP as a Model Context Protocol stdio server.
func RunMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	policyPath := fs.String("policy", "", "Path to policy.cedar file (defaults to ./policy.cedar)")
	auditLogFlag := fs.String("audit-log", "", "Path to audit log file in JSON lines (defaults to LTD_AUDIT_LOG or ltd-audit.jsonl)")

	defaultSession := os.Getenv("BAP_SESSION_ID")
	if defaultSession == "" {
		defaultSession = os.Getenv("LTD_SESSION_ID")
	}
	sessionFlag := fs.String("session-id", defaultSession, "Session identifier for grouping agent actions")

	epCfg := config.ResolveEndpoints()
	serverFlag := fs.String("server", epCfg.ControlPlaneURL, "Central control plane URL for telemetry streaming")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := mcp.ServerConfig{
		PolicyPath:       *policyPath,
		ServerURL:        *serverFlag,
		SessionID:        *sessionFlag,
		AuditLogPath:     *auditLogFlag,
		IdentityResolver: resolveLocalIdentity,
	}

	return mcp.StartServer(cfg)
}
