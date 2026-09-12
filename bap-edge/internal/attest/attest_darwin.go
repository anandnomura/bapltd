//go:build darwin

package attest

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"syscall"
)

// InspectPeer extracts the calling PID from a Unix domain socket on Darwin (macOS)
// using LOCAL_PEERPID, inspects the executable path, and computes its SHA-256 hash.
func InspectPeer(conn net.Conn) (*PeerInfo, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("connection is not a unix domain socket")
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain raw syscall connection: %w", err)
	}

	var pid int
	var sockErr error
	err = rawConn.Control(func(fd uintptr) {
		// On Darwin, SOL_LOCAL = 0, LOCAL_PEERPID = 0x002
		val, err := syscall.GetsockoptInt(int(fd), 0, 0x002)
		if err != nil {
			sockErr = err
			return
		}
		pid = val
	})
	if err != nil {
		return nil, fmt.Errorf("socket control failed: %w", err)
	}
	if sockErr != nil {
		return nil, fmt.Errorf("LOCAL_PEERPID getsockopt failed: %w", sockErr)
	}

	// Resolve executable path on Darwin
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve process binary for pid %d: %w", pid, err)
	}
	exePath := strings.TrimSpace(string(out))
	if exePath == "" {
		return nil, fmt.Errorf("empty executable path for pid %d", pid)
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

