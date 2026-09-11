package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"ltd-agent/pkg/types"
)

// RequestToken connects to the ltd-agent attestation socket at socketPath
// and waits for the server to verify the caller's binary and return an OBO JWT.
func RequestToken(socketPath string, timeout time.Duration) (*types.AttestationResponse, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	conn, err := net.DialTimeout("unix", socketPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to attestation socket at %s: %w", socketPath, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		if err == io.EOF || n == 0 {
			return nil, fmt.Errorf("connection dropped by server: binary attestation failed (hash not authorized)")
		}
		return nil, fmt.Errorf("failed to read from attestation socket: %w", err)
	}

	var resp types.AttestationResponse
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		return nil, fmt.Errorf("failed to parse attestation response JSON: %w (raw: %s)", err, string(buf[:n]))
	}

	if resp.Token == "" {
		return nil, fmt.Errorf("attestation server returned empty token: %s", string(buf[:n]))
	}

	return &resp, nil
}

