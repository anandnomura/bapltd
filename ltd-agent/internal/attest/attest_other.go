//go:build !linux && !darwin && !windows

package attest

import (
	"fmt"
	"net"
	"os"
)

func InspectPeer(conn net.Conn) (*PeerInfo, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	hash, err := ComputeFileSHA256(exePath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash: %w", err)
	}
	return &PeerInfo{
		PID:        os.Getpid(),
		BinaryPath: exePath,
		BinaryHash: hash,
	}, nil
}

