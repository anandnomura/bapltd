//go:build windows

package attest

import (
	"fmt"
	"net"
	"os"
)

// InspectPeer provides Windows compatibility for Unix domain sockets.
// Because Windows AF_UNIX sockets do not provide SO_PEERCRED, this returns the current
// process executable information or simulated caller info for testing.
func InspectPeer(conn net.Conn) (*PeerInfo, error) {
	pid := os.Getpid()
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	hash, err := ComputeFileSHA256(exePath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash of %s: %w", exePath, err)
	}

	return &PeerInfo{
		PID:        pid,
		UID:        0,
		GID:        0,
		BinaryPath: exePath,
		BinaryHash: hash,
	}, nil
}

