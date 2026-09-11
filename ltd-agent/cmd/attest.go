package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"ltd-agent/internal/client"
)

// RunAttest handles testing client attestation against the serve daemon.
func RunAttest(args []string) error {
	fs := flag.NewFlagSet("attest", flag.ExitOnError)
	socketPath := fs.String("socket", "/tmp/ltd.sock", "Path to Unix domain socket")
	timeoutSec := fs.Int("timeout", 5, "Connection timeout in seconds")

	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("[ltd-agent attest] Requesting attestation from socket: %s\n", *socketPath)
	resp, err := client.RequestToken(*socketPath, time.Duration(*timeoutSec)*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ltd-agent attest] Attestation failed: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

