package cmd

import (
	"flag"
	"fmt"
	"os"
	"time"

	"bap-edge/internal/config"
	"bap-edge/internal/policystore"
)

// RunSync implements the 'bapedge sync' command to synchronize or inspect local policy cache.
func RunSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	epCfg := config.ResolveEndpoints()
	serverURL := fs.String("server", epCfg.ControlPlaneURL, "bapcontrolplane URL (e.g. http://localhost:8080)")
	policyDir := fs.String("policy-dir", policystore.DefaultPolicyDir(), "Path to local edge policy cache directory")
	agentID := fs.String("agent-id", os.Getenv("BAP_AGENT_ID"), "Registered Agent ID (optional)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	store := policystore.New(*policyDir)

	fmt.Printf("[bapedge sync] Checking policy synchronization against %s...\n", *serverURL)
	bundle, isFallback, err := store.SyncWithServer(*serverURL, *agentID, 3*time.Second)
	if err != nil {
		if err == policystore.ErrKillSwitchActive {
			fmt.Println("==================================================")
			fmt.Println("  [EMERGENCY WARNING] POLICY KILL-SWITCH IS ACTIVE!")
			fmt.Println("  All local agent executions are strictly blocked.")
			fmt.Println("==================================================")
			return err
		}
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Println("==================================================")
	if isFallback {
		fmt.Println("  [OFFLINE RESILIENCE ACTIVE] Control plane is DOWN.")
		fmt.Println("  bapedge is operating with prior secure settings.")
	} else {
		fmt.Println("  Policy synchronization successful (Online).")
	}
	fmt.Println("==================================================")
	fmt.Printf("Policy Version: %d\n", bundle.Version)
	fmt.Printf("Rules Digest:   %s\n", bundle.Digest)
	fmt.Printf("Cache Location: %s\n", *policyDir)
	fmt.Printf("Kill Switch:    %v\n", bundle.KillSwitch)
	fmt.Println("==================================================")

	return nil
}
