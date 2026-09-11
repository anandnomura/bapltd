package attest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// HardcodedAllowedMockHash is the default hardcoded SHA-256 hash allowed by the attestation server.
// For testing/mocking, this represents the expected hash of an authorized agent binary.
const HardcodedAllowedMockHash = "a1b2c3d4e5f60718293a4b5c6d7e8f90123456789abcdef0123456789abcdef0"

// MockOBOToken is the mock On-Behalf-Of JWT returned to attested callers.
const MockOBOToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJsb2NhbC1haS1hZ2VudCIsImlzcyI6Imx0ZC1hdHRlc3RhdGlvbi1zZXJ2ZXIiLCJhdWQiOiJjb3JwLWV4ZWMiLCJleHAiOjE5OTk5OTk5OTksInNjb3BlcyI6WyJjbGk6ZXhlYyIsInplcm8tdHJ1c3QiXX0.mock_signature_z3r0_trust_obo_token_987654321"

// PeerInfo holds process metadata extracted from the socket connection.
type PeerInfo struct {
	PID        int
	UID        int
	GID        int
	BinaryPath string
	BinaryHash string
}

// ComputeFileSHA256 reads the file at the given path and computes its SHA-256 hexadecimal checksum.
func ComputeFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open binary %q: %w", filePath, err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("failed to hash file %q: %w", filePath, err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

