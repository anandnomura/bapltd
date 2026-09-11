package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"ltd-agent/internal/attest"
	"ltd-agent/pkg/types"
)

// ServerConfig holds configuration options for the attestation server.
type ServerConfig struct {
	SocketPath    string
	AllowedHashes []string
}

// Server implements the Unix domain socket attestation server.
type Server struct {
	config   ServerConfig
	listener net.Listener
}

// NewServer creates a new attestation server.
func NewServer(cfg ServerConfig) *Server {
	if cfg.SocketPath == "" {
		cfg.SocketPath = "/tmp/ltd.sock"
	}
	// Always include the hardcoded allowed mock hash
	hasHardcoded := false
	for _, h := range cfg.AllowedHashes {
		if strings.EqualFold(h, attest.HardcodedAllowedMockHash) {
			hasHardcoded = true
			break
		}
	}
	if !hasHardcoded {
		cfg.AllowedHashes = append(cfg.AllowedHashes, attest.HardcodedAllowedMockHash)
	}
	return &Server{config: cfg}
}

// Run starts the listener and handles incoming connections until interrupted.
func (s *Server) Run() error {
	// Ensure parent directory of socket exists
	sockDir := filepath.Dir(s.config.SocketPath)
	if err := os.MkdirAll(sockDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory %s: %w", sockDir, err)
	}

	// Remove any existing socket file
	_ = os.Remove(s.config.SocketPath)

	ln, err := net.Listen("unix", s.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket %s: %w", s.config.SocketPath, err)
	}
	s.listener = ln
	defer func() {
		_ = ln.Close()
		_ = os.Remove(s.config.SocketPath)
	}()

	// Ensure proper file permissions on socket
	_ = os.Chmod(s.config.SocketPath, 0666)

	log.Printf("[ltd-agent serve] Listening on %s", s.config.SocketPath)
	log.Printf("[ltd-agent serve] Hardcoded mock allowed hash: %s", attest.HardcodedAllowedMockHash)
	if len(s.config.AllowedHashes) > 1 {
		log.Printf("[ltd-agent serve] Additional allowed hashes: %v", s.config.AllowedHashes[1:])
	}

	// Trap signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("[ltd-agent serve] Shutting down...")
		_ = s.listener.Close()
		_ = os.Remove(s.config.SocketPath)
		os.Exit(0)
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// Check if listener closed
			select {
			case <-sigChan:
				return nil
			default:
				return err
			}
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	peerInfo, err := attest.InspectPeer(conn)
	if err != nil {
		log.Printf("[ltd-agent serve] Peer inspection failed: %v. Dropping connection.", err)
		return
	}

	log.Printf("[ltd-agent serve] New client connected: PID=%d, Binary=%s, SHA-256=%s",
		peerInfo.PID, peerInfo.BinaryPath, peerInfo.BinaryHash)

	// Check if hash matches hardcoded allowed mock hash or any configured allowed hash
	matched := false
	for _, allowed := range s.config.AllowedHashes {
		if strings.EqualFold(peerInfo.BinaryHash, strings.TrimSpace(allowed)) {
			matched = true
			break
		}
	}

	if !matched {
		log.Printf("[ltd-agent serve] REJECTED: Hash mismatch for PID %d (got %s). Dropping connection.",
			peerInfo.PID, peerInfo.BinaryHash)
		return
	}

	log.Printf("[ltd-agent serve] ACCEPTED: Hash verified for PID %d. Issuing On-Behalf-Of JWT.", peerInfo.PID)

	resp := types.AttestationResponse{
		Token: attest.MockOBOToken,
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[ltd-agent serve] JSON encoding error: %v", err)
		return
	}

	// Write response followed by newline
	_, _ = conn.Write(append(respBytes, '\n'))
}

