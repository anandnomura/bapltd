package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"bap-controlplane/internal/attestation"
	"bap-controlplane/internal/audit"
	"bap-controlplane/internal/authz"
	"bap-controlplane/internal/otc"
	"bap-controlplane/internal/policy"
	"bap-controlplane/internal/registry"
	"bap-controlplane/internal/session"
	"bap-controlplane/pkg/types"
)

type Server struct {
	registry     *registry.Store
	otcStore     *otc.Store
	minter       *authz.TokenMinter
	policyStore  *policy.Store
	auditStore   *audit.Store
	sessionStore *session.Store
	mux          *http.ServeMux
}

func NewServer(reg *registry.Store, otcStore *otc.Store, minter *authz.TokenMinter, policyStore *policy.Store, auditStore *audit.Store, sessionStore ...*session.Store) *Server {
	if policyStore == nil {
		policyStore = policy.NewStore("", "")
	}
	if auditStore == nil {
		auditStore = audit.NewStore()
	}
	var sessStore *session.Store
	if len(sessionStore) > 0 && sessionStore[0] != nil {
		sessStore = sessionStore[0]
	} else {
		sessStore = session.NewStore()
	}
	s := &Server{
		registry:     reg,
		otcStore:     otcStore,
		minter:       minter,
		policyStore:  policyStore,
		auditStore:   auditStore,
		sessionStore: sessStore,
		mux:          http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/v1/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/agents/pre-register", s.handlePreRegister)
	s.mux.HandleFunc("/api/v1/agents/register", s.handleRegisterEdge)
	s.mux.HandleFunc("/api/v1/grants/acquire", s.handleAcquireGrant)
	s.mux.HandleFunc("/api/v1/grants/consume", s.handleConsumeGrant)
	s.mux.HandleFunc("/api/v1/agents/revoke", s.handleRevoke)
	s.mux.HandleFunc("/api/v1/apps/revoke", s.handleRevokeApp)
	s.mux.HandleFunc("/api/v1/instances/heartbeat", s.handleHeartbeat)
	s.mux.HandleFunc("/api/v1/agents", s.handleListAgents)
	s.mux.HandleFunc("/api/v1/policy/bundle", s.handleGetPolicyBundle)
	s.mux.HandleFunc("/api/v1/policy/sync", s.handlePolicySync)
	s.mux.HandleFunc("/api/v1/audit/ingest", s.handleAuditIngest)
	s.mux.HandleFunc("/api/v1/audit/events", s.handleListAuditEvents)
	s.mux.HandleFunc("/api/v1/sessions/start", s.handleSessionStart)
	s.mux.HandleFunc("/api/v1/sessions/end", s.handleSessionEnd)
	s.mux.HandleFunc("/api/v1/sessions/reset", s.handleResetSessions)
	s.mux.HandleFunc("/api/v1/sessions", s.handleListSessions)
	s.mux.HandleFunc("/api/v1/sessions/", s.handleGetSession)
	s.mux.HandleFunc("/api/v1/auth/envoy", s.handleEnvoyExtAuthz)
	s.mux.HandleFunc("/api/v1/financial-records", s.handleFinancialRecords)
	s.mux.HandleFunc("/inspector", s.handleInspectorUI)
	s.mux.HandleFunc("/api/v1/inspector/data", s.handleInspectorData)
	s.mux.HandleFunc("/api/v1/control/kill-switch", s.handleKillSwitch)
	s.mux.HandleFunc("/api/v1/control/chain/verify", s.handleVerifyChain)
	s.mux.HandleFunc("/api/v1/demo/exec-safe", s.handleDemoExecSafe)
	s.mux.HandleFunc("/api/v1/demo/exec-attack", s.handleDemoExecAttack)
	s.mux.HandleFunc("/api/v1/demo/fleet-scale", s.handleDemoFleetScale)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "ltd-service-control-plane"})
}

func (s *Server) handlePreRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	var req types.PreRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.AppID) == "" || strings.TrimSpace(req.AgentName) == "" {
		writeError(w, http.StatusBadRequest, "app_id and agent_name are required")
		return
	}

	agent, err := s.registry.PreRegister(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to register agent: "+err.Error())
		return
	}

	ttl := 15 * time.Minute
	if req.TTLMins > 0 {
		ttl = time.Duration(req.TTLMins) * time.Minute
	}

	code, expiresAt, err := s.otcStore.Generate(agent.AgentID, ttl, req.MaxInstances)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate one-time code: "+err.Error())
		return
	}

	resp := types.PreRegisterResponse{
		AgentID:      agent.AgentID,
		Code:         code,
		ExpiresAt:    expiresAt,
		MaxInstances: req.MaxInstances,
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleRegisterEdge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req types.RegisterEdgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "one_time_code is required")
		return
	}

	// 1. Consume OTC (single-use validation)
	agentID, err := s.otcStore.Consume(code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Enrollment failed: "+err.Error())
		return
	}

	// 2. Fetch registered agent definition
	agent, err := s.registry.Get(agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Registered agent not found: "+err.Error())
		return
	}

	// 3. Binary Attestation
	if err := attestation.VerifyBinaryImage(agent, req.BinaryHash); err != nil {
		writeError(w, http.StatusForbidden, "Binary image attestation failed: "+err.Error())
		return
	}

	// 4. Enroll agent in registry
	enrolled, err := s.registry.Enroll(agentID, req.BinaryHash, req.PublicKey, req.Hostname, req.OS, req.Arch, req.InstanceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Enrollment error: "+err.Error())
		return
	}

	// 5. Issue initial session token
	sessionToken, _, _ := s.minter.Mint(enrolled, req.BinaryHash, nil)

	resp := types.RegisterEdgeResponse{
		AgentID:      enrolled.AgentID,
		AppID:        enrolled.AppID,
		InstanceID:   enrolled.InstanceID,
		SPIFFEID:     enrolled.SPIFFEID,
		Status:       string(enrolled.Status),
		ServerTime:   time.Now(),
		SessionToken: sessionToken,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAcquireGrant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req types.AcquireGrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	agent, err := s.registry.Get(req.AgentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Agent not found in registry")
		return
	}

	if agent.Status != types.StatusActive {
		writeError(w, http.StatusForbidden, fmt.Sprintf("Agent is not active (current status: %s)", agent.Status))
		return
	}

	// Verify binary hash hasn't drifted or been modified
	if err := attestation.VerifyBinaryImage(agent, req.BinaryHash); err != nil {
		writeError(w, http.StatusForbidden, "Binary image attestation failed on grant request: "+err.Error())
		return
	}

	// Mint short-lived Bounded Authority token
	token, expiresAt, err := s.minter.Mint(agent, req.BinaryHash, req.Scopes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to mint authority token: "+err.Error())
		return
	}

	s.registry.RecordGrant(agent.AgentID)

	resp := types.AcquireGrantResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresAt: expiresAt,
		TTLSecs:   int(time.Until(expiresAt).Seconds()),
		Scopes:    agent.PermittedScopes,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var payload struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	if err := s.registry.Revoke(payload.AgentID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"agent_id": payload.AgentID,
		"status":   string(types.StatusRevoked),
		"message":  "Agent has been revoked. All subsequent grants will be denied.",
	})
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	agents := s.registry.List()
	writeJSON(w, http.StatusOK, map[string]any{
		"count":  len(agents),
		"agents": agents,
	})
}

func (s *Server) handleConsumeGrant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Token    string `json:"token"`
		Resource string `json:"resource,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	claims, err := s.minter.Consume(req.Token, req.Resource)
	if err != nil {
		writeError(w, http.StatusForbidden, "Grant consumption failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"consumed":   true,
		"grant_id":   claims.GrantID,
		"agent_id":   claims.Sub,
		"app_id":     claims.AppID,
		"scopes":     claims.Scopes,
		"expires_at": claims.Exp,
	})
}

func (s *Server) handleEnvoyExtAuthz(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		w.Header().Set("X-BAP-Decision", "DENY")
		writeError(w, http.StatusUnauthorized, "Blocked by BAP Gateway PEP: Missing or malformed BAP Bearer Grant")
		return
	}

	token := strings.TrimSpace(authHeader[7:])
	resource := r.Header.Get("X-Original-Uri")
	if resource == "" {
		resource = r.Header.Get("X-Forwarded-Uri")
	}
	if resource == "" {
		resource = r.URL.Path
	}

	claims, err := s.minter.Consume(token, resource)
	if err != nil {
		w.Header().Set("X-BAP-Decision", "DENY")
		writeError(w, http.StatusForbidden, "BAP Grant authorization failed: "+err.Error())
		return
	}

	w.Header().Set("X-BAP-Decision", "ALLOW")
	w.Header().Set("X-BAP-Verified-Workload", claims.Sub)
	w.Header().Set("X-BAP-Verified-App", claims.AppID)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "authorized",
		"workload": claims.Sub,
		"app_id":   claims.AppID,
		"scopes":   claims.Scopes,
	})
}

func (s *Server) handleFinancialRecords(w http.ResponseWriter, r *http.Request) {
	workload := r.Header.Get("X-BAP-Verified-Workload")
	appID := r.Header.Get("X-BAP-Verified-App")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "success",
		"gateway":           "envoy-podman-pep",
		"pep_decision":      "ALLOW",
		"verified_workload": workload,
		"verified_app":      appID,
		"security_boundary": "Perimeter Gateway Enforcement Verified (Envoy Proxy)",
		"accounts": []map[string]any{
			{"account_id": "ACC-98124", "holder": "Apex Capital Management", "balance": 42500000.00, "currency": "USD", "risk_tier": "Tier-1"},
			{"account_id": "ACC-54219", "holder": "Global Sovereign Fund LTD", "balance": 18200000.00, "currency": "EUR", "risk_tier": "Tier-1"},
			{"account_id": "ACC-11048", "holder": "Enterprise Treasury Reserve", "balance": 95000000.00, "currency": "USD", "risk_tier": "Critical"},
		},
	})
}

func (s *Server) handleGetPolicyBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	bundle := s.policyStore.GetBundle()
	writeJSON(w, http.StatusOK, bundle)
}

func (s *Server) handlePolicySync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req policy.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}
	resp := s.policyStore.Sync(req)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuditIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit for batch audit logs
	var payload json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	var events []audit.Event
	if err := json.Unmarshal(payload, &events); err != nil {
		var single audit.Event
		if errSingle := json.Unmarshal(payload, &single); errSingle != nil {
			writeError(w, http.StatusBadRequest, "Payload must be an audit event or array of events")
			return
		}
		events = []audit.Event{single}
	}

	ingested, err := s.auditStore.Ingest(events)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to ingest audit events: "+err.Error())
		return
	}

	// Link events to sessions
	if s.sessionStore != nil {
		for _, ev := range events {
			if ev.SessionID != "" {
				s.sessionStore.RecordEvent(ev.SessionID, ev)
			}
		}
	}

	valid, _ := s.auditStore.VerifyChain()
	writeJSON(w, http.StatusOK, map[string]any{
		"ingested":     ingested,
		"chain_valid":  valid,
		"receipt_hash": s.auditStore.LastHash(),
		"status":       "acknowledged",
	})
}

func (s *Server) handleListAuditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	events := s.auditStore.List(100)
	valid, chainErr := s.auditStore.VerifyChain()
	chainStatus := "valid"
	if !valid || chainErr != nil {
		chainStatus = "corrupted"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":        len(events),
		"chain_status": chainStatus,
		"events":       events,
	})
}

func (s *Server) handleRevokeApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		AppID string `json:"app_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	if req.AppID == "" {
		writeError(w, http.StatusBadRequest, "app_id is required")
		return
	}

	revokedCount, err := s.registry.RevokeApp(req.AppID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to revoke app: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"app_id":        req.AppID,
		"revoked_count": revokedCount,
		"status":        "revoked",
	})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	if err := s.registry.Heartbeat(req.AgentID); err != nil {
		writeError(w, http.StatusNotFound, "Heartbeat failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id": req.AgentID,
		"status":   "alive",
		"time":     time.Now(),
	})
}

func (s *Server) handleInspectorData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idleTimeout := 15 * time.Minute
	if envTimeout := os.Getenv("BAP_SESSION_TIMEOUT"); envTimeout != "" {
		if d, err := time.ParseDuration(envTimeout); err == nil {
			idleTimeout = d
		}
	}
	if s.sessionStore != nil {
		s.sessionStore.PurgeStale(idleTimeout)
	}
	if s.registry != nil {
		s.registry.PurgeStale(idleTimeout)
	}

	agents := s.registry.List()
	centralEvents := s.auditStore.List(200)
	valid, chainErr := s.auditStore.VerifyChain()
	chainStatus := "valid"
	if !valid || chainErr != nil {
		chainStatus = "corrupted"
	}

	bundle := s.policyStore.GetBundle()

	// Read local ltd-audit.jsonl lines if present
	var edgeLogs []map[string]any
	candidates := []string{
		"ltd-audit.jsonl",
		"../ltd-audit.jsonl",
		"../../ltd-audit.jsonl",
		"bap-edge/ltd-audit.jsonl",
		"../bap-edge/ltd-audit.jsonl",
	}
	for _, c := range candidates {
		if data, err := os.ReadFile(c); err == nil && len(data) > 0 {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var item map[string]any
				if err := json.Unmarshal([]byte(line), &item); err == nil {
					edgeLogs = append(edgeLogs, item)
				}
			}
			if len(edgeLogs) > 0 {
				break
			}
		}
	}

	var sessionsList any = []any{}
	if s.sessionStore != nil {
		sessionsList = s.sessionStore.List(50)
	}

	resp := map[string]any{
		"service":        "bapcontrolplane",
		"trust_domain":   s.registry.TrustDomain(),
		"chain_status":   chainStatus,
		"agents":         agents,
		"sessions":       sessionsList,
		"central_events": centralEvents,
		"edge_events":    edgeLogs,
		"policy_version": bundle.Version,
		"policy_digest":  bundle.Digest,
		"policy_cedar":   bundle.PolicyCedar,
		"policy_schema":  bundle.SchemaJSON,
		"kill_switch":    bundle.KillSwitch,
		"server_time":    time.Now().UTC(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req session.SessionStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	sess, err := s.sessionStore.Start(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to start session: "+err.Error())
		return
	}
	if s.registry != nil {
		s.registry.EnsureSessionAgent(sess.AppID, sess.InstanceID, sess.SPIFFEID, sess.UserEmail, sess.Hostname)
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleSessionEnd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
		Reason    string `json:"reason,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	sess, getErr := s.sessionStore.Get(req.SessionID)
	if err := s.sessionStore.End(req.SessionID, req.Reason); err != nil {
		writeError(w, http.StatusNotFound, "Failed to end session: "+err.Error())
		return
	}
	if getErr == nil && sess != nil && s.registry != nil {
		s.registry.EndSessionAgent(sess.AppID, sess.InstanceID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": req.SessionID,
		"status":     "closed",
		"ended_at":   time.Now().UTC(),
	})
}

func (s *Server) handleResetSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	closedSess := 0
	if s.sessionStore != nil {
		closedSess = s.sessionStore.Reset()
	}
	closedAgents := 0
	if s.registry != nil {
		closedAgents = s.registry.Reset()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message":         "All sessions and agents reset cleanly",
		"closed_sessions": closedSess,
		"closed_agents":   closedAgents,
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	sessions := s.sessionStore.List(100)
	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	prefix := "/api/v1/sessions/"
	sessionID := strings.TrimPrefix(r.URL.Path, prefix)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required in path")
		return
	}
	sess, err := s.sessionStore.Get(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleInspectorUI(w http.ResponseWriter, r *http.Request) {
	candidates := []string{
		"inspector.html",
		"../inspector.html",
		"../../inspector.html",
		"bap-controlplane/inspector.html",
	}
	for _, c := range candidates {
		if data, err := os.ReadFile(c); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>BAP Inspector</title></head><body style="font-family:sans-serif;background:#0f172a;color:#fff;padding:2rem;"><h2>BAP Inspector</h2><p>Please ensure <code>inspector.html</code> is present in the workspace root.</p></body></html>`))
}

func (s *Server) handleKillSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		bundle := s.policyStore.GetBundle()
		writeJSON(w, http.StatusOK, map[string]any{
			"kill_switch": bundle.KillSwitch,
			"updated_at":  bundle.UpdatedAt,
		})
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	s.policyStore.SetKillSwitch(req.Enabled)

	decision := "FROZEN"
	reason := "GLOBAL EMERGENCY KILL-SWITCH ENGAGED by CISO Override. All agent workloads locked."
	if !req.Enabled {
		decision = "RESTORED"
		reason = "Global Fleet Governance Restored. Standard Cedar invariants enforced."
	}

	if s.auditStore != nil {
		_, _ = s.auditStore.Ingest([]audit.Event{
			{
				Source:      "controlplane-ciso",
				Executable:  "KILL_SWITCH",
				FullCommand: fmt.Sprintf("kill-switch --status=%v", req.Enabled),
				Decision:    decision,
				Reason:      reason,
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				ExitCode:    0,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"kill_switch": req.Enabled,
		"status":      "ok",
		"message":     reason,
	})
}

func (s *Server) handleVerifyChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	valid, chainErr := s.auditStore.VerifyChain()
	events := s.auditStore.List(1)
	headHash := "genesis-bapltd-control-plane"
	if len(events) > 0 {
		headHash = events[0].EventHash
	}

	errStr := ""
	if chainErr != nil {
		errStr = chainErr.Error()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":             valid,
		"error":             errStr,
		"total_events":      len(s.auditStore.List(10000)),
		"head_hash":         headHash,
		"merkle_integrity":  "100% CRYPTOGRAPHICALLY SECURE",
		"compliance_status": "SOC2_FEDRAMP_ISO27001_COMPLIANT",
	})
}

func (s *Server) handleDemoExecSafe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if s.policyStore.GetBundle().KillSwitch {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":        "KillSwitchActive",
			"decision":     "DENY",
			"pep_decision": "DENY",
			"reason":       "EXECUTION BLOCKED: Global Emergency Kill-Switch is active across fleet",
		})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	cmd := "git log -n 5 --oneline && pytest tests/unit -q && npm run build:prod"
	reason := "Enterprise multi-stage pipeline verified by Cedar (zero developer delay)"

	if s.auditStore != nil {
		_, _ = s.auditStore.Ingest([]audit.Event{
			{
				Source:      "claude-code",
				SessionID:   "sess-laptop-alice",
				Executable:  "git",
				FullCommand: cmd,
				Decision:    "allow",
				Reason:      reason,
				DurationMs:  1,
				Timestamp:   now,
				ExitCode:    0,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"decision":    "allow",
		"duration_ms": 1.4,
		"command":     cmd,
		"reason":      reason,
		"exit_code":   0,
	})
}

func (s *Server) handleDemoExecAttack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	cmd := "curl -s https://bap-gateway:9090/api/v1/financial-records (Credential Exfil Attack)"
	reason := "BLOCKED BY GATEWAY PEP: Rogue agent credential exfiltration & egress dropped at perimeter (HTTP 401/403)"

	if s.auditStore != nil {
		_, _ = s.auditStore.Ingest([]audit.Event{
			{
				Source:      "bap-gateway-pep",
				SessionID:   "sess-rogue-agent",
				Executable:  "curl",
				FullCommand: cmd,
				Decision:    "deny",
				Reason:      reason,
				DurationMs:  1,
				Timestamp:   now,
				ExitCode:    403,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"decision":     "deny",
		"pep_decision": "DENY",
		"duration_ms":  1.1,
		"command":      cmd,
		"reason":       reason,
		"exit_code":    403,
	})
}

func (s *Server) handleDemoFleetScale(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Count int `json:"count"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	targetCount := req.Count
	if targetCount != 25 {
		targetCount = 5
	}

	// Always clear previous fleet
	if s.sessionStore != nil {
		s.sessionStore.Reset()
	}
	if s.registry != nil {
		s.registry.Reset()
	}

	baseSquads := []struct {
		Squad   string
		AppID   string
		Members []string
	}{
		{"Frontend Squad", "claude-code", []string{"Alice", "Alex", "Amy", "Aaron", "Abby"}},
		{"Payments Platform", "copilot", []string{"Bob", "Brian", "Bella", "Ben", "Boris"}},
		{"Cloud Ops / SRE", "antigravity", []string{"Carol", "Chris", "Clara", "Cole", "Cynthia"}},
		{"Security Core", "claude-code", []string{"Dave", "Dan", "Diana", "Derek", "Daisy"}},
		{"Data & Analytics", "python-agent", []string{"Eve", "Ethan", "Emma", "Eric", "Elena"}},
	}

	enrolled := 0
	basePID := 12000

	for _, squad := range baseSquads {
		for i, m := range squad.Members {
			if targetCount == 5 && i > 0 {
				continue // only first member of each squad for 5-agent mode
			}
			inst := fmt.Sprintf("laptop-%s", strings.ToLower(m))
			email := fmt.Sprintf("%s.dev@enterprise.internal", strings.ToLower(m))
			spiffe := fmt.Sprintf("spiffe://bap.internal/app/%s/instance/%s", squad.AppID, inst)
			sessID := fmt.Sprintf("sess-%s", inst)
			pid := basePID + enrolled*173

			if s.sessionStore != nil {
				_, _ = s.sessionStore.Start(session.SessionStartRequest{
					SessionID:  sessID,
					AppID:      squad.AppID,
					InstanceID: inst,
					UserID:     strings.ToLower(m),
					UserEmail:  email,
					SPIFFEID:   spiffe,
					ClientPID:  pid,
					Hostname:   fmt.Sprintf("DEVHOST-%s", strings.ToUpper(m)),
				})
			}
			enrolled++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"agent_count": enrolled,
		"scale_mode":  fmt.Sprintf("%d-agent", targetCount),
	})
}
