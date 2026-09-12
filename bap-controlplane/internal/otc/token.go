package otc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type TokenEntry struct {
	Code          string
	AgentID       string
	ExpiresAt     time.Time
	Consumed      bool
	MaxInstances  int
	EnrolledCount int
}

type Store struct {
	mu     sync.RWMutex
	tokens map[string]*TokenEntry
}

func NewStore() *Store {
	s := &Store{
		tokens: make(map[string]*TokenEntry),
	}
	// Background cleaner
	go s.cleanupLoop()
	return s
}

func (s *Store) Generate(agentID string, ttl time.Duration, maxInstances int) (string, time.Time, error) {
	if maxInstances <= 0 {
		maxInstances = 1
	}
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	prefix := "LTD-OTC"
	if maxInstances > 1 {
		prefix = "BAP-FLEET"
	}
	code := fmt.Sprintf("%s-%s-%s",
		prefix,
		hex.EncodeToString(bytes[0:4]),
		hex.EncodeToString(bytes[4:8]))

	expiresAt := time.Now().Add(ttl)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[code] = &TokenEntry{
		Code:          code,
		AgentID:       agentID,
		ExpiresAt:     expiresAt,
		Consumed:      false,
		MaxInstances:  maxInstances,
		EnrolledCount: 0,
	}

	return code, expiresAt, nil
}

func (s *Store) Consume(code string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.tokens[code]
	if !exists {
		return "", fmt.Errorf("invalid one-time code")
	}

	if entry.Consumed {
		if entry.MaxInstances > 1 {
			return "", fmt.Errorf("fleet enrollment code quota reached (max %d instances enrolled)", entry.MaxInstances)
		}
		return "", fmt.Errorf("one-time code has already been consumed (replay attempt blocked)")
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(s.tokens, code)
		return "", fmt.Errorf("one-time code has expired")
	}

	entry.EnrolledCount++
	if entry.EnrolledCount >= entry.MaxInstances {
		entry.Consumed = true
	}
	return entry.AgentID, nil
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for code, entry := range s.tokens {
			if now.After(entry.ExpiresAt) || entry.Consumed {
				delete(s.tokens, code)
			}
		}
		s.mu.Unlock()
	}
}
