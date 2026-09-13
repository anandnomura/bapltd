package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"bap-controlplane/internal/audit"
)

// Session represents an active or historical agent execution session (e.g., a Claude Code or Copilot run).
type Session struct {
	SessionID    string        `json:"session_id"`
	AppID        string        `json:"app_id"`
	InstanceID   string        `json:"instance_id,omitempty"`
	UserID       string        `json:"user_id,omitempty"`
	UserEmail    string        `json:"user_email,omitempty"`
	SPIFFEID     string        `json:"spiffe_id,omitempty"`
	Status       string        `json:"status"` // "active" or "closed"
	StartedAt    time.Time     `json:"started_at"`
	EndedAt      *time.Time    `json:"ended_at,omitempty"`
	LastActiveAt time.Time     `json:"last_active_at"`
	ClientPID    int           `json:"client_pid,omitempty"`
	Hostname     string        `json:"hostname,omitempty"`
	TotalEvents  int           `json:"total_events"`
	AllowedCount int           `json:"allowed_count"`
	DeniedCount  int           `json:"denied_count"`
	CloseReason  string        `json:"close_reason,omitempty"`
	Events       []audit.Event `json:"events,omitempty"`
}

// SessionStartRequest contains fields to initiate a session.
type SessionStartRequest struct {
	SessionID  string `json:"session_id,omitempty"`
	AppID      string `json:"app_id"`
	InstanceID string `json:"instance_id,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	UserEmail  string `json:"user_email,omitempty"`
	SPIFFEID   string `json:"spiffe_id,omitempty"`
	ClientPID  int    `json:"client_pid,omitempty"`
	Hostname   string `json:"hostname,omitempty"`
}

// Store manages sessions in memory with thread safety.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	order    []string // chronological order of session IDs
}

// NewStore creates an initialized Store.
func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
		order:    make([]string, 0),
	}
}

// GenerateSessionID creates a unique session identifier.
func GenerateSessionID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	if prefix == "" {
		prefix = "sess"
	}
	return fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("150405"), hex.EncodeToString(b))
}

// Start creates and registers a new session or re-activates an existing one.
func (s *Store) Start(req SessionStartRequest) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID := req.SessionID
	if sessionID == "" {
		prefix := "sess"
		if req.AppID != "" {
			prefix = fmt.Sprintf("sess-%s", req.AppID)
		}
		sessionID = GenerateSessionID(prefix)
	}

	now := time.Now().UTC()
	if existing, found := s.sessions[sessionID]; found {
		existing.Status = "active"
		existing.LastActiveAt = now
		if req.ClientPID > 0 {
			existing.ClientPID = req.ClientPID
		}
		if req.Hostname != "" {
			existing.Hostname = req.Hostname
		}
		if req.UserID != "" {
			existing.UserID = req.UserID
		}
		if req.UserEmail != "" {
			existing.UserEmail = req.UserEmail
		}
		if req.SPIFFEID != "" {
			existing.SPIFFEID = req.SPIFFEID
		}
		return existing, nil
	}

	appID := req.AppID
	if appID == "" {
		appID = "agent"
	}

	userID := req.UserID
	if userID == "" {
		userID = "NA"
	}
	userEmail := req.UserEmail
	if userEmail == "" {
		userEmail = "NA"
	}
	spiffeID := req.SPIFFEID
	if spiffeID == "" {
		spiffeID = "NA"
	}

	sess := &Session{
		SessionID:    sessionID,
		AppID:        appID,
		InstanceID:   req.InstanceID,
		UserID:       userID,
		UserEmail:    userEmail,
		SPIFFEID:     spiffeID,
		Status:       "active",
		StartedAt:    now,
		LastActiveAt: now,
		ClientPID:    req.ClientPID,
		Hostname:     req.Hostname,
		TotalEvents:  0,
		AllowedCount: 0,
		DeniedCount:  0,
		Events:       make([]audit.Event, 0),
	}

	s.sessions[sessionID] = sess
	s.order = append(s.order, sessionID)
	return sess, nil
}

// End marks a session as closed.
func (s *Store) End(sessionID string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, found := s.sessions[sessionID]
	if !found {
		return fmt.Errorf("session %q not found", sessionID)
	}

	now := time.Now().UTC()
	sess.Status = "closed"
	sess.EndedAt = &now
	sess.LastActiveAt = now
	sess.CloseReason = reason
	return nil
}

// RecordEvent associates an audit event with a session and updates counts.
func (s *Store) RecordEvent(sessionID string, ev audit.Event) {
	if sessionID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sess, found := s.sessions[sessionID]
	if !found {
		// Auto-instantiate session if it does not yet exist
		now := time.Now().UTC()
		appID := ev.Source
		if appID == "" {
			appID = "agent"
		}
		sess = &Session{
			SessionID:    sessionID,
			AppID:        appID,
			UserID:       ev.UserID,
			UserEmail:    ev.UserEmail,
			SPIFFEID:     ev.SPIFFEID,
			Status:       "active",
			StartedAt:    now,
			LastActiveAt: now,
			ClientPID:    ev.ClientPID,
			TotalEvents:  0,
			AllowedCount: 0,
			DeniedCount:  0,
			Events:       make([]audit.Event, 0),
		}
		s.sessions[sessionID] = sess
		s.order = append(s.order, sessionID)
	}

	// Re-activate session on new activity if it was previously closed due to idle timeout
	if sess.Status != "active" {
		sess.Status = "active"
		sess.EndedAt = nil
		sess.CloseReason = ""
	}

	if ev.UserID != "" && sess.UserID == "" {
		sess.UserID = ev.UserID
	}
	if ev.UserEmail != "" && sess.UserEmail == "" {
		sess.UserEmail = ev.UserEmail
	}
	if ev.SPIFFEID != "" && sess.SPIFFEID == "" {
		sess.SPIFFEID = ev.SPIFFEID
	}

	sess.LastActiveAt = time.Now().UTC()
	sess.TotalEvents++
	if ev.Decision == "allow" {
		sess.AllowedCount++
	} else if ev.Decision == "deny" {
		sess.DeniedCount++
	}
	sess.Events = append(sess.Events, ev)
}

// List returns the latest N sessions in reverse chronological order.
func (s *Store) List(limit int) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.order)
	if limit <= 0 || limit > total {
		limit = total
	}

	result := make([]*Session, 0, limit)
	for i := total - 1; i >= 0 && len(result) < limit; i-- {
		id := s.order[i]
		if sess, found := s.sessions[id]; found {
			// Return shallow copy without embedding all raw events to keep payloads light
			cp := *sess
			cp.Events = nil // summary only
			result = append(result, &cp)
		}
	}
	return result
}

// Get returns full session details including all linked events.
func (s *Store) Get(sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, found := s.sessions[sessionID]
	if !found {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}
	cp := *sess
	eventsCopy := make([]audit.Event, len(sess.Events))
	copy(eventsCopy, sess.Events)
	cp.Events = eventsCopy
	return &cp, nil
}

// PurgeStale marks any active sessions that have been idle for longer than maxIdle as closed.
func (s *Store) PurgeStale(maxIdle time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	closed := 0
	for _, sess := range s.sessions {
		if sess.Status == "active" && now.Sub(sess.LastActiveAt) > maxIdle {
			sess.Status = "closed"
			sess.EndedAt = &now
			sess.CloseReason = "idle_timeout"
			closed++
		}
	}
	return closed
}

// Reset marks all active sessions as closed.
func (s *Store) Reset() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	closed := 0
	for _, sess := range s.sessions {
		if sess.Status == "active" {
			sess.Status = "closed"
			sess.EndedAt = &now
			sess.CloseReason = "system_reset"
			closed++
		}
	}
	return closed
}
