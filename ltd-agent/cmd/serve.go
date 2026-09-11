package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"ltd-agent/internal/attest"
	"ltd-agent/internal/server"
)

// RunServe handles the 'serve' subcommand.
func RunServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	socketPath := fs.String("socket", "/tmp/ltd.sock", "Path to Unix domain socket")
	allowedHash := fs.String("allowed-hash", "", "Additional allowed binary SHA-256 hash(es), comma-separated")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var allowedHashes []string
	if *allowedHash != "" {
		for _, h := range strings.Split(*allowedHash, ",") {
			trimmed := strings.TrimSpace(h)
			if trimmed != "" {
				allowedHashes = append(allowedHashes, trimmed)
			}
		}
	}

	// Check environment variable LTD_ALLOWED_HASH as well
	if envHash := os.Getenv("LTD_ALLOWED_HASH"); envHash != "" {
		for _, h := range strings.Split(envHash, ",") {
			trimmed := strings.TrimSpace(h)
			if trimmed != "" {
				allowedHashes = append(allowedHashes, trimmed)
			}
		}
	}

	cfg := server.ServerConfig{
		SocketPath:    *socketPath,
		AllowedHashes: allowedHashes,
	}

	fmt.Printf("[ltd-agent] Starting Zero-Trust Attestation Server on %s\n", *socketPath)
	fmt.Printf("[ltd-agent] Hardcoded allowed mock hash: %s\n", attest.HardcodedAllowedMockHash)

	srv := server.NewServer(cfg)
	return srv.Run()
}

