package session

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"bap-controlplane/internal/audit"

	_ "modernc.org/sqlite"
)

// Session represents an active or historical agent execution session (e.g., a Claude Code or Copilot run).
type Session struct {
	SessionID    string        `json:"session_id"`
	AppID        string        `json:"app_id"`
	InstanceID   string        `json:"instance_id,omitempty"`
	AgentName    string        `json:"agent_name,omitempty"`
	UserID       string        `json:"user_id,omitempty"`
	UserEmail    string        `json:"user_email,omitempty"`
	SPIFFEID     string        `json:"spiffe_id,omitempty"`
	Status       string        `json:"status"` // "active", "closed", or "revoked"
	StartedAt    time.Time     `json:"started_at"`
	EndedAt      *time.Time    `json:"ended_at,omitempty"`
	LastActiveAt time.Time     `json:"last_active_at"`
	ClientPID    int           `json:"client_pid,omitempty"`
	Hostname     string        `json:"hostname,omitempty"`
	TotalEvents  int           `json:"total_events"`
	AllowedCount int           `json:"allowed_count"`
	DeniedCount  int           `json:"denied_count"`
	CloseReason  string        `json:"close_reason,omitempty"`
	UserPrompt   string        `json:"user_prompt,omitempty"`
	Events       []audit.Event `json:"events,omitempty"`
}

// SessionStartRequest contains fields to initiate a session.
type SessionStartRequest struct {
	SessionID  string `json:"session_id,omitempty"`
	AppID      string `json:"app_id"`
	InstanceID string `json:"instance_id,omitempty"`
	AgentName  string `json:"agent_name,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	UserEmail  string `json:"user_email,omitempty"`
	SPIFFEID   string `json:"spiffe_id,omitempty"`
	ClientPID  int    `json:"client_pid,omitempty"`
	Hostname   string `json:"hostname,omitempty"`
	UserPrompt string `json:"user_prompt,omitempty"`
}

// Store manages sessions in memory with thread safety and optional SQLite durability.
type Store struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	order        []string // chronological order of session IDs
	revokedUsers map[string]bool
	db           *sql.DB // persistent SQLite storage
}

// NewStore creates an in-memory Store (for tests or backward compatibility).
func NewStore() *Store {
	return &Store{
		sessions:     make(map[string]*Session),
		order:        make([]string, 0),
		revokedUsers: make(map[string]bool),
	}
}

// NewStoreWithDB creates an initialized Store with SQLite persistence.
func NewStoreWithDB(dbPath string) (*Store, error) {
	if dbPath == "" || dbPath == ":memory:" || dbPath == "memory" {
		return NewStore(), nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA busy_timeout=5000;")

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		app_id TEXT NOT NULL,
		instance_id TEXT,
		user_id TEXT,
		user_email TEXT,
		spiffe_id TEXT,
		status TEXT NOT NULL,
		started_at TEXT NOT NULL,
		ended_at TEXT,
		last_active_at TEXT NOT NULL,
		client_pid INTEGER,
		hostname TEXT,
		total_events INTEGER DEFAULT 0,
		allowed_count INTEGER DEFAULT 0,
		denied_count INTEGER DEFAULT 0,
		close_reason TEXT
	);
	CREATE TABLE IF NOT EXISTS revoked_users (
		username TEXT PRIMARY KEY,
		revoked_at TEXT NOT NULL,
		reason TEXT
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize sqlite sessions schema: %w", err)
	}

	store := &Store{
		sessions:     make(map[string]*Session),
		order:        make([]string, 0),
		revokedUsers: make(map[string]bool),
		db:           db,
	}

	if err := store.loadFromDB(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to restore sessions from sqlite: %w", err)
	}

	return store, nil
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
		if existing.Status == "revoked" {
			return nil, fmt.Errorf("session is revoked; administrator restore required")
		}
		if req.AgentName != "" {
			existing.AgentName = req.AgentName
		}
		if req.UserPrompt != "" {
			existing.UserPrompt = req.UserPrompt
		}
		if existing.Status != "active" || existing.EndedAt != nil || existing.CloseReason != "" {
			existing.EndedAt = nil
			existing.CloseReason = ""
			existing.AllowedCount = 0
			existing.DeniedCount = 0
			existing.TotalEvents = 0
			existing.StartedAt = now
		}
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
		s.saveSessionToDB(existing)
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
	agentName := req.AgentName
	if agentName == "" {
		agentName = appID
	}

	sess := &Session{
		SessionID:    sessionID,
		AppID:        appID,
		InstanceID:   req.InstanceID,
		AgentName:    agentName,
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
		UserPrompt:   req.UserPrompt,
	}

	s.sessions[sessionID] = sess
	s.order = append(s.order, sessionID)
	s.saveSessionToDB(sess)
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

	if sess.Status == "revoked" {
		return fmt.Errorf("revoked session requires administrator restore")
	}

	now := time.Now().UTC()
	sess.Status = "closed"
	sess.EndedAt = &now
	sess.LastActiveAt = now
	sess.CloseReason = reason
	s.saveSessionToDB(sess)
	return nil
}

// RevokeSession explicitly revokes a session by exact ID.
func (s *Store) RevokeSession(sessionID string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, found := s.sessions[sessionID]
	if !found {
		// Even if not found, create a placeholder revoked session to prevent future enrollment
		now := time.Now().UTC()
		s.sessions[sessionID] = &Session{
			SessionID:    sessionID,
			AppID:        "unknown",
			Status:       "revoked",
			StartedAt:    now,
			LastActiveAt: now,
			CloseReason:  reason,
		}
		s.order = append(s.order, sessionID)
		s.saveSessionToDB(s.sessions[sessionID])
		return nil
	}

	now := time.Now().UTC()
	sess.Status = "revoked"
	sess.CloseReason = reason
	sess.LastActiveAt = now
	s.saveSessionToDB(sess)
	return nil
}

// ListRevoked returns a slice of all currently revoked session IDs.
func (s *Store) ListRevoked() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	revoked := make([]string, 0)
	for id, sess := range s.sessions {
		if sess.Status == "revoked" {
			revoked = append(revoked, id)
		}
	}
	return revoked
}

// IsRevoked checks if a specific session ID is currently revoked.
func (s *Store) IsRevoked(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sess, found := s.sessions[sessionID]; found {
		return sess.Status == "revoked"
	}
	return false
}

// IsUserRevoked checks if a username is in the revoked users list.
func (s *Store) IsUserRevoked(username string) bool {
	if username == "" || username == "NA" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.revokedUsers[strings.ToLower(strings.TrimSpace(username))]
}

// RevokeUser places a user identity into revoked status, persisting it and revoking any active sessions.
func (s *Store) RevokeUser(username, reason string) {
	if username == "" || username == "NA" {
		return
	}
	u := strings.ToLower(strings.TrimSpace(username))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokedUsers[u] = true
	if s.db != nil {
		_, _ = s.db.Exec(`INSERT INTO revoked_users (username, revoked_at, reason) VALUES (?, ?, ?) ON CONFLICT(username) DO UPDATE SET revoked_at=excluded.revoked_at, reason=excluded.reason`, u, time.Now().UTC().Format(time.RFC3339), reason)
	}
	now := time.Now().UTC()
	for _, sess := range s.sessions {
		if strings.EqualFold(sess.UserID, username) || strings.EqualFold(sess.UserEmail, username) {
			sess.Status = "revoked"
			sess.CloseReason = reason
			sess.LastActiveAt = now
			s.saveSessionToDB(sess)
		}
	}
}

// RestoreUser restores a user identity and clears revocation on their sessions.
func (s *Store) RestoreUser(username string) {
	if username == "" || username == "NA" {
		return
	}
	u := strings.ToLower(strings.TrimSpace(username))
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.revokedUsers, u)
	if s.db != nil {
		_, _ = s.db.Exec(`DELETE FROM revoked_users WHERE username = ?`, u)
	}
	now := time.Now().UTC()
	for _, sess := range s.sessions {
		if strings.EqualFold(sess.UserID, username) || strings.EqualFold(sess.UserEmail, username) {
			if sess.Status == "revoked" {
				sess.Status = "closed"
				sess.CloseReason = "Restored by administrator"
				sess.LastActiveAt = now
				s.saveSessionToDB(sess)
			}
		}
	}
}

// ListRevokedUsers returns a slice of all currently revoked usernames.
func (s *Store) ListRevokedUsers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]string, 0, len(s.revokedUsers))
	for u := range s.revokedUsers {
		users = append(users, u)
	}
	return users
}

// RevokeTarget finds an active session matching target (by session_id, instance_id, user_id, or user_email) and marks it revoked.
func (s *Store) RevokeTarget(target string, reason string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("target is required")
	}
	now := time.Now().UTC()
	for _, sess := range s.sessions {
		if sess.SessionID == target {
			sess.Status = "revoked"
			sess.CloseReason = reason
			sess.LastActiveAt = now
			s.saveSessionToDB(sess)
			return sess, nil
		}
	}
	return nil, fmt.Errorf("no session found matching %q", target)
}

// RestoreTarget restores a previously revoked session back to active (or closed if process already terminated).
func (s *Store) RestoreTarget(target string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("target is required")
	}
	now := time.Now().UTC()
	for _, sess := range s.sessions {
		if sess.SessionID == target {
			sess.Status = "active"
			sess.CloseReason = ""
			sess.LastActiveAt = now
			s.saveSessionToDB(sess)
			return sess, nil
		}
	}
	return nil, fmt.Errorf("no session found matching %q", target)
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

	// Re-activate session on new activity only if it was closed due to idle timeout; NEVER if revoked!
	if sess.Status != "active" && sess.Status != "revoked" {
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
	s.saveSessionToDB(sess)
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

// ListVisible returns sessions in reverse chronological order, pruning non-revoked sessions older than maxAge.
// Revoked sessions and sessions belonging to revoked users are NEVER pruned so administrators can inspect and restore them.
func (s *Store) ListVisible(limit int, maxAge time.Duration) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	total := len(s.order)
	if limit <= 0 {
		limit = total
	}

	result := make([]*Session, 0, limit)
	for i := total - 1; i >= 0 && len(result) < limit; i-- {
		id := s.order[i]
		if sess, found := s.sessions[id]; found {
			isRevoked := sess.Status == "revoked" || s.revokedUsers[strings.ToLower(sess.UserID)] || s.revokedUsers[strings.ToLower(sess.UserEmail)]
			if !isRevoked && maxAge > 0 && now.Sub(sess.LastActiveAt) > maxAge {
				continue
			}
			cp := *sess
			cp.Events = nil
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

// Heartbeat updates the LastActiveAt timestamp for a session, and resurrects it if prematurely timed out.
func (s *Store) Heartbeat(sessionID string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	now := time.Now().UTC()
	sess.LastActiveAt = now
	// If prematurely closed by idle timeout while process is still actively pinging, resurrect to active
	if sess.Status == "closed" && sess.CloseReason == "idle_timeout" {
		sess.Status = "active"
		sess.EndedAt = nil
		sess.CloseReason = ""
	}
	s.saveSessionToDB(sess)
	cp := *sess
	return &cp, nil
}

// SetPrompt updates the UserPrompt and LastActiveAt timestamp for a session.
func (s *Store) SetPrompt(sessionID, prompt string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	now := time.Now().UTC()
	sess.UserPrompt = prompt
	sess.LastActiveAt = now
	if sess.Status == "closed" && sess.CloseReason == "idle_timeout" {
		sess.Status = "active"
		sess.EndedAt = nil
		sess.CloseReason = ""
	}
	s.saveSessionToDB(sess)
	cp := *sess
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
			s.saveSessionToDB(sess)
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
			s.saveSessionToDB(sess)
			closed++
		}
	}
	return closed
}

func (s *Store) loadFromDB() error {
	if s.db == nil {
		return nil
	}
	rows, err := s.db.Query(`SELECT session_id, app_id, instance_id, user_id, user_email, spiffe_id, status, started_at, ended_at, last_active_at, client_pid, hostname, total_events, allowed_count, denied_count, close_reason FROM sessions ORDER BY started_at ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			sessID, appID, instID, uID, uEmail, spiffeID, status, startedAtStr, endedAtStr, lastActiveStr, hostname, closeReason sql.NullString
			clientPID, totalEv, allowCnt, denyCnt                                                                                sql.NullInt64
		)
		if err := rows.Scan(&sessID, &appID, &instID, &uID, &uEmail, &spiffeID, &status, &startedAtStr, &endedAtStr, &lastActiveStr, &clientPID, &hostname, &totalEv, &allowCnt, &denyCnt, &closeReason); err != nil {
			continue
		}
		startedAt, _ := time.Parse(time.RFC3339, startedAtStr.String)
		lastActiveAt, _ := time.Parse(time.RFC3339, lastActiveStr.String)
		var endedAt *time.Time
		if endedAtStr.Valid && endedAtStr.String != "" {
			if t, err := time.Parse(time.RFC3339, endedAtStr.String); err == nil {
				endedAt = &t
			}
		}

		sess := &Session{
			SessionID:    sessID.String,
			AppID:        appID.String,
			InstanceID:   instID.String,
			UserID:       uID.String,
			UserEmail:    uEmail.String,
			SPIFFEID:     spiffeID.String,
			Status:       status.String,
			StartedAt:    startedAt,
			EndedAt:      endedAt,
			LastActiveAt: lastActiveAt,
			ClientPID:    int(clientPID.Int64),
			Hostname:     hostname.String,
			TotalEvents:  int(totalEv.Int64),
			AllowedCount: int(allowCnt.Int64),
			DeniedCount:  int(denyCnt.Int64),
			CloseReason:  closeReason.String,
			Events:       make([]audit.Event, 0),
		}
		s.sessions[sess.SessionID] = sess
		s.order = append(s.order, sess.SessionID)
	}

	uRows, err := s.db.Query(`SELECT username FROM revoked_users`)
	if err == nil {
		defer uRows.Close()
		for uRows.Next() {
			var u string
			if uRows.Scan(&u) == nil && u != "" {
				s.revokedUsers[strings.ToLower(u)] = true
			}
		}
	}

	return nil
}

func (s *Store) saveSessionToDB(sess *Session) {
	if s.db == nil || sess == nil {
		return
	}
	endedAtStr := ""
	if sess.EndedAt != nil {
		endedAtStr = sess.EndedAt.UTC().Format(time.RFC3339)
	}
	query := `
	INSERT INTO sessions (session_id, app_id, instance_id, user_id, user_email, spiffe_id, status, started_at, ended_at, last_active_at, client_pid, hostname, total_events, allowed_count, denied_count, close_reason)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(session_id) DO UPDATE SET
		status = excluded.status,
		ended_at = excluded.ended_at,
		last_active_at = excluded.last_active_at,
		total_events = excluded.total_events,
		allowed_count = excluded.allowed_count,
		denied_count = excluded.denied_count,
		close_reason = excluded.close_reason;`
	_, _ = s.db.Exec(query,
		sess.SessionID,
		sess.AppID,
		sess.InstanceID,
		sess.UserID,
		sess.UserEmail,
		sess.SPIFFEID,
		sess.Status,
		sess.StartedAt.UTC().Format(time.RFC3339),
		endedAtStr,
		sess.LastActiveAt.UTC().Format(time.RFC3339),
		sess.ClientPID,
		sess.Hostname,
		sess.TotalEvents,
		sess.AllowedCount,
		sess.DeniedCount,
		sess.CloseReason,
	)
}

// Close gracefully closes the SQLite database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
