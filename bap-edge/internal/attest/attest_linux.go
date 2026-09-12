//go:build linux

package attest

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

// InspectPeer extracts the calling PID from a Unix domain socket on Linux using SO_PEERCRED,
// resolves /proc/<pid>/exe to find the executable path, and computes its SHA-256 hash.
func InspectPeer(conn net.Conn) (*PeerInfo, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("connection is not a unix domain socket")
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain raw syscall connection: %w", err)
	}

	var ucred *syscall.Ucred
	var credErr error
	err = rawConn.Control(func(fd uintptr) {
		ucred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return nil, fmt.Errorf("socket control failed: %w", err)
	}
	if credErr != nil {
		return nil, fmt.Errorf("SO_PEERCRED getsockopt failed: %w", credErr)
	}

	pid := int(ucred.Pid)
	uid := int(ucred.Uid)
	gid := int(ucred.Gid)

	exeLink := fmt.Sprintf("/proc/%d/exe", pid)
	exePath, err := os.Readlink(exeLink)
	if err != nil {
		return nil, fmt.Errorf("failed to readlink %s: %w", exeLink, err)
	}

	hash, err := ComputeFileSHA256(exePath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash of %s: %w", exePath, err)
	}

	return &PeerInfo{
		PID:        pid,
		UID:        uid,
		GID:        gid,
		BinaryPath: exePath,
		BinaryHash: hash,
	}, nil
}

