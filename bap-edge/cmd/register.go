package cmd

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"bap-edge/internal/attest"
	"bap-edge/internal/config"
	"bap-edge/internal/httptransport"
	"bap-edge/internal/policystore"
)

type RegisterRequest struct {
	Code       string `json:"one_time_code"`
	BinaryHash string `json:"binary_hash"`
	Hostname   string `json:"hostname"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	PublicKey  string `json:"public_key,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
}

type RegisterResponse struct {
	AgentID      string    `json:"agent_id"`
	AppID        string    `json:"app_id"`
	InstanceID   string    `json:"instance_id,omitempty"`
	SPIFFEID     string    `json:"spiffe_id,omitempty"`
	Status       string    `json:"status"`
	ServerTime   time.Time `json:"server_time"`
	SessionToken string    `json:"session_token"`
}

type StoredCredentials struct {
	ServerURL    string    `json:"server_url"`
	AgentID      string    `json:"agent_id"`
	AppID        string    `json:"app_id"`
	InstanceID   string    `json:"instance_id,omitempty"`
	SPIFFEID     string    `json:"spiffe_id,omitempty"`
	Status       string    `json:"status"`
	BinaryHash   string    `json:"binary_hash"`
	SessionToken string    `json:"session_token"`
	EnrolledAt   time.Time `json:"enrolled_at"`
}

// DefaultCredentialsPath returns ~/.ltd/credentials.json
func DefaultCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".ltd", "credentials.json")
}

// RunRegister executes the edge agent self-registration against bap-controlplane.
func RunRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	epCfg := config.ResolveEndpoints()
	serverURL := fs.String("server", epCfg.ControlPlaneURL, "bap-controlplane URL")
	code := fs.String("code", "", "One-time registration code (e.g. LTD-OTC-XXXX or BAP-FLEET-XXXX)")
	configPath := fs.String("config", DefaultCredentialsPath(), "Path to store enrolled credentials")
	customInstanceID := fs.String("instance-id", "", "Custom instance identifier (optional)")
	caCertPath := fs.String("ca-cert", os.Getenv("BAP_CA_CERT"), "Path to custom CA certificate for HTTPS verification")
	insecureTLS := fs.Bool("insecure", false, "Skip TLS certificate verification (development only)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *code == "" {
		return fmt.Errorf("missing required --code flag (e.g. --code LTD-OTC-XXXX or BAP-FLEET-XXXX)")
	}

	// 1. Identify running binary and compute SHA-256 hash for attestation
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}

	binHash, err := attest.ComputeFileSHA256(exePath)
	if err != nil {
		return fmt.Errorf("failed to compute binary hash for %q: %w", exePath, err)
	}

	hostname, _ := os.Hostname()

	reqBody := RegisterRequest{
		Code:       *code,
		BinaryHash: binHash,
		Hostname:   hostname,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		InstanceID: *customInstanceID,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to encode registration payload: %w", err)
	}

	// Configure TLS client with embedded Root CA support
	httpClient := httptransport.New(10 * time.Second)
	if *insecureTLS {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 10 * time.Second,
		}
	} else if *caCertPath != "" {
		if caData, err := os.ReadFile(*caCertPath); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(caData) {
				httpClient = &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{RootCAs: pool},
					},
					Timeout: 10 * time.Second,
				}
			}
		}
	}

	endpoint := fmt.Sprintf("%s/api/v1/agents/register", *serverURL)
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		return fmt.Errorf("connection to control plane failed at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registration rejected (status %d): %s", resp.StatusCode, string(respBody))
	}

	var regResp RegisterResponse
	if err := json.Unmarshal(respBody, &regResp); err != nil {
		return fmt.Errorf("failed to parse registration response: %w", err)
	}

	// 2. Save credentials to disk
	creds := StoredCredentials{
		ServerURL:    *serverURL,
		AgentID:      regResp.AgentID,
		AppID:        regResp.AppID,
		InstanceID:   regResp.InstanceID,
		SPIFFEID:     regResp.SPIFFEID,
		Status:       regResp.Status,
		BinaryHash:   binHash,
		SessionToken: regResp.SessionToken,
		EnrolledAt:   time.Now(),
	}

	if err := os.MkdirAll(filepath.Dir(*configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	credsData, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format credentials: %w", err)
	}

	if err := os.WriteFile(*configPath, credsData, 0600); err != nil {
		return fmt.Errorf("failed to write credentials to %s: %w", *configPath, err)
	}

	// 3. Cache initial policy bundle from control plane
	policyStore := policystore.New("")
	bundle, _, syncErr := policyStore.SyncWithServer(*serverURL, regResp.AgentID, 3*time.Second)
	policySynced := syncErr == nil

	fmt.Println("==================================================")
	fmt.Println("  Bounded Authority Plane (BAP) Agent Enrolled!")
	fmt.Println("==================================================")
	fmt.Printf("Agent ID:      %s\n", regResp.AgentID)
	fmt.Printf("App ID:        %s\n", regResp.AppID)
	if regResp.InstanceID != "" {
		fmt.Printf("Instance ID:   %s\n", regResp.InstanceID)
	}
	if regResp.SPIFFEID != "" {
		fmt.Printf("SPIFFE ID:     %s\n", regResp.SPIFFEID)
	}
	fmt.Printf("Status:        %s\n", regResp.Status)
	fmt.Printf("Binary Hash:   %s\n", binHash)
	fmt.Printf("Credentials:   %s\n", *configPath)
	if policySynced {
		fmt.Printf("Policy Synced: Version %d (Cached in %s)\n", bundle.Version, policyStore.Directory())
	}
	fmt.Println("==================================================")

	return nil
}
