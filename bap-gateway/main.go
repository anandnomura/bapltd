package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type GatewayConfig struct {
	Port         int
	ControlPlane string
	SecretKey    string
	UseConsume   bool
}

type ConsumeResponse struct {
	Consumed  bool     `json:"consumed"`
	GrantID   string   `json:"grant_id"`
	AgentID   string   `json:"agent_id"`
	AppID     string   `json:"app_id"`
	Scopes    []string `json:"scopes"`
	ExpiresAt int64    `json:"expires_at"`
}

type TokenClaims struct {
	GrantID    string   `json:"jti"`
	Sub        string   `json:"sub"`
	AppID      string   `json:"app_id"`
	InstanceID string   `json:"instance_id,omitempty"`
	SPIFFEID   string   `json:"spiffe_id,omitempty"`
	AgentName  string   `json:"agent_name"`
	EnvProfile string   `json:"env_profile"`
	Scopes     []string `json:"scopes"`
	Exp        int64    `json:"exp"`
}

func main() {
	defaultCP := resolveDefaultControlPlane()
	port := flag.Int("port", 9090, "Port for BAP Gateway PEP HTTP server")
	cpURL := flag.String("controlplane", defaultCP, "BAP Control Plane base URL")
	secret := flag.String("secret", "ltd-service-bounded-authority-secret-key-32b!", "HMAC secret key for offline JWT verification")
	consume := flag.Bool("consume", true, "Atomically consume single-use grants via control plane")
	flag.Parse()

	cfg := GatewayConfig{
		Port:         *port,
		ControlPlane: strings.TrimRight(*cpURL, "/"),
		SecretKey:    *secret,
		UseConsume:   *consume,
	}

	mux := http.NewServeMux()

	// Public Health
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/api/v1/health", handleHealth)

	// Protected Backend Endpoints (Guarded by Gateway PEP)
	mux.HandleFunc("/api/v1/financial-records", pepGuard(cfg, handleFinancialRecords))
	mux.HandleFunc("/api/v1/core-banking/", pepGuard(cfg, handleCoreBanking))

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("===============================================================================")
	log.Printf("   BAP ZERO-TRUST GATEWAY POLICY ENFORCEMENT POINT (PEP)")
	log.Printf("   Emulating Envoy Proxy / Istio Ingress with ext_authz Semantics")
	log.Printf("===============================================================================")
	log.Printf("[bap-gateway] Listening on http://localhost%s", addr)
	log.Printf("[bap-gateway] Control Plane PEP Target : %s", cfg.ControlPlane)
	log.Printf("[bap-gateway] Atomic Grant Burning    : %v", cfg.UseConsume)
	log.Printf("[bap-gateway] Protected Endpoints      : /api/v1/financial-records, /api/v1/core-banking/*")

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[bap-gateway] Fatal server error: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":   "bap-gateway-pep",
		"status":    "healthy",
		"mode":      "envoy-ext-authz-emulator",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// pepGuard enforces BAP Grant authentication on every incoming request.
// If the caller is a rogue agent (no BAP grant or invalid/consumed token), it drops the request with 401/403.
func pepGuard(cfg GatewayConfig, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		authHeader := r.Header.Get("Authorization")

		// 1. Rogue Detection: Missing or malformed Authorization header
		if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			durationMs := time.Since(start).Milliseconds()
			log.Printf("[BAP-GATEWAY-PEP] [BLOCKED ROGUE AGENT] 401 Unauthorized | Path: %s | Source: %s | Latency: %dms | Reason: Missing BAP Grant Bearer Token",
				r.URL.Path, r.RemoteAddr, durationMs)

			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":         "AccessDenied",
				"gateway":       "bap-gateway-pep",
				"pep_decision":  "DENY",
				"message":       "Blocked by Zero-Trust Gateway PEP: Rogue agent request lacking BAP Bearer Grant.",
				"security_note": "Internal microservice was NEVER contacted. Access requires a valid BAP Grant from bapcontrolplane.",
				"required_auth": "Bearer <bap_grant_token>",
			})
			return
		}

		token := strings.TrimSpace(authHeader[7:])
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":        "AccessDenied",
				"pep_decision": "DENY",
				"message":      "Empty Bearer token provided.",
			})
			return
		}

		// 2. Grant Validation & Atomic Consumption
		valid, claims, err := validateGrant(cfg, token, r.URL.Path)
		durationMs := time.Since(start).Milliseconds()

		if !valid || err != nil {
			reason := "Invalid, expired, or replayed BAP Grant"
			if err != nil {
				reason = err.Error()
			}
			log.Printf("[BAP-GATEWAY-PEP] [BLOCKED FORBIDDEN] 403 Forbidden | Path: %s | Source: %s | Latency: %dms | Reason: %s",
				r.URL.Path, r.RemoteAddr, durationMs, reason)

			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":         "Forbidden",
				"gateway":       "bap-gateway-pep",
				"pep_decision":  "DENY",
				"message":       "BAP Grant verification failed: " + reason,
				"security_note": "Token signature invalid, expired, or already burned.",
			})
			return
		}

		// 3. Permitted Governed Request: Inject verified identity headers into request context
		log.Printf("[BAP-GATEWAY-PEP] [PERMIT GOVERNED] 200 OK | Path: %s | Workload: %s | App: %s | Latency: %dms",
			r.URL.Path, claims.Sub, claims.AppID, durationMs)

		r.Header.Set("X-BAP-Verified-Workload", claims.Sub)
		r.Header.Set("X-BAP-Verified-App", claims.AppID)

		next(w, r)
	}
}

func validateGrant(cfg GatewayConfig, token, resource string) (bool, *TokenClaims, error) {
	// Mode A: Central Control Plane Atomic Consumption (/api/v1/grants/consume)
	if cfg.UseConsume {
		payload := map[string]string{
			"token":    token,
			"resource": resource,
		}
		body, _ := json.Marshal(payload)
		url := fmt.Sprintf("%s/api/v1/grants/consume", cfg.ControlPlane)

		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var consumeResp ConsumeResponse
				if err := json.NewDecoder(resp.Body).Decode(&consumeResp); err == nil && consumeResp.Consumed {
					claims := &TokenClaims{
						GrantID: consumeResp.GrantID,
						Sub:     consumeResp.AgentID,
						AppID:   consumeResp.AppID,
						Scopes:  consumeResp.Scopes,
						Exp:     consumeResp.ExpiresAt,
					}
					return true, claims, nil
				}
			} else {
				respBody, _ := io.ReadAll(resp.Body)
				return false, nil, fmt.Errorf("control plane rejected grant (HTTP %d): %s", resp.StatusCode, string(respBody))
			}
		}
	}

	// Mode B: Local Cryptographic Fallback (Decentralized HMAC verification)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false, nil, fmt.Errorf("malformed JWT token")
	}

	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(cfg.SecretKey))
	mac.Write([]byte(unsigned))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return false, nil, fmt.Errorf("cryptographic signature mismatch")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false, nil, fmt.Errorf("failed to decode claims base64")
	}

	var claims TokenClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return false, nil, fmt.Errorf("failed to parse claims JSON")
	}

	if time.Now().Unix() > claims.Exp {
		return false, nil, fmt.Errorf("BAP grant expired at %s", time.Unix(claims.Exp, 0).Format(time.RFC3339))
	}

	return true, &claims, nil
}

func handleFinancialRecords(w http.ResponseWriter, r *http.Request) {
	workload := r.Header.Get("X-BAP-Verified-Workload")
	appID := r.Header.Get("X-BAP-Verified-App")

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "success",
		"gateway":           "bap-gateway-pep",
		"pep_decision":      "ALLOW",
		"verified_workload": workload,
		"verified_app":      appID,
		"security_boundary": "Perimeter Gateway Enforcement Verified",
		"accounts": []map[string]any{
			{"account_id": "ACC-98124", "holder": "Apex Capital Management", "balance": 42500000.00, "currency": "USD", "risk_tier": "Tier-1"},
			{"account_id": "ACC-54219", "holder": "Global Sovereign Fund LTD", "balance": 18200000.00, "currency": "EUR", "risk_tier": "Tier-1"},
			{"account_id": "ACC-11048", "holder": "Enterprise Treasury Reserve", "balance": 95000000.00, "currency": "USD", "risk_tier": "Critical"},
		},
	})
}

func handleCoreBanking(w http.ResponseWriter, r *http.Request) {
	workload := r.Header.Get("X-BAP-Verified-Workload")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "success",
		"gateway":           "bap-gateway-pep",
		"verified_workload": workload,
		"action":            "core_banking_query",
		"transaction_id":    fmt.Sprintf("TXN-%d", time.Now().UnixNano()%1000000),
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func resolveDefaultControlPlane() string {
	if v := os.Getenv("BAP_SERVER_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := os.Getenv("BAP_CONTROL_PLANE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	for _, p := range []string{"bap-config.json", "../bap-config.json"} {
		if data, err := os.ReadFile(p); err == nil {
			var cfg struct {
				ControlPlaneURL string `json:"controlplane_url"`
			}
			if err := json.Unmarshal(data, &cfg); err == nil && cfg.ControlPlaneURL != "" {
				return strings.TrimRight(cfg.ControlPlaneURL, "/")
			}
		}
	}
	return "http://localhost:8080"
}
