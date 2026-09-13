package authz

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"bap-controlplane/pkg/types"
)

type TokenMinter struct {
	signingKey     []byte
	defaultTTL     time.Duration
	mu             sync.Mutex
	consumedGrants map[string]time.Time
}

func NewTokenMinter(secretKey string, defaultTTL time.Duration) *TokenMinter {
	if defaultTTL <= 0 {
		defaultTTL = 30 * time.Minute
	}
	tm := &TokenMinter{
		signingKey:     []byte(secretKey),
		defaultTTL:     defaultTTL,
		consumedGrants: make(map[string]time.Time),
	}
	go tm.cleanupLoop()
	return tm
}

func (tm *TokenMinter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		tm.mu.Lock()
		now := time.Now()
		for id, consumedAt := range tm.consumedGrants {
			if now.Sub(consumedAt) > 2*tm.defaultTTL {
				delete(tm.consumedGrants, id)
			}
		}
		tm.mu.Unlock()
	}
}

type GrantClaims struct {
	GrantID    string   `json:"jti"`
	Sub        string   `json:"sub"`
	AppID      string   `json:"app_id"`
	InstanceID string   `json:"instance_id,omitempty"`
	SPIFFEID   string   `json:"spiffe_id,omitempty"`
	AgentName  string   `json:"agent_name"`
	EnvProfile string   `json:"env_profile"`
	BinaryHash string   `json:"binary_hash"`
	Scopes     []string `json:"scopes"`
	Iss        string   `json:"iss"`
	Aud        string   `json:"aud"`
	Iat        int64    `json:"iat"`
	Exp        int64    `json:"exp"`
}

func (tm *TokenMinter) Mint(agent *types.RegisteredAgent, candidateHash string, requestedScopes []string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(tm.defaultTTL)

	scopes := agent.PermittedScopes
	if len(requestedScopes) > 0 {
		scopes = requestedScopes
	}
	if len(scopes) == 0 {
		scopes = []string{"cli:exec", "zero-trust"}
	}

	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate random token id: %w", err)
	}
	grantID := fmt.Sprintf("grant-%x", idBytes)

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	sub := agent.AgentID
	if agent.SPIFFEID != "" {
		sub = agent.SPIFFEID
	}

	claims := GrantClaims{
		GrantID:    grantID,
		Sub:        sub,
		AppID:      agent.AppID,
		InstanceID: agent.InstanceID,
		SPIFFEID:   agent.SPIFFEID,
		AgentName:  agent.AgentName,
		EnvProfile: string(agent.EnvProfile),
		BinaryHash: candidateHash,
		Scopes:     scopes,
		Iss:        "bap-controlplane",
		Aud:        "bap-edge-broker",
		Iat:        now.Unix(),
		Exp:        expiresAt.Unix(),
	}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	unsignedToken := headerB64 + "." + claimsB64
	mac := hmac.New(sha256.New, tm.signingKey)
	mac.Write([]byte(unsignedToken))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	jwtToken := unsignedToken + "." + sigB64
	return jwtToken, expiresAt, nil
}

func (tm *TokenMinter) Verify(tokenStr string) (*GrantClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed token structure")
	}

	unsignedToken := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, tm.signingKey)
	mac.Write([]byte(unsignedToken))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid token signature")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode claims: %w", err)
	}

	var claims GrantClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims JSON: %w", err)
	}

	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token has expired")
	}

	return &claims, nil
}

func (tm *TokenMinter) Consume(tokenStr, resource string) (*GrantClaims, error) {
	claims, err := tm.Verify(tokenStr)
	if err != nil {
		return nil, err
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, consumed := tm.consumedGrants[claims.GrantID]; consumed {
		return nil, fmt.Errorf("grant %s has already been consumed (replay blocked)", claims.GrantID)
	}

	if resource != "" {
		matched := false
		for _, s := range claims.Scopes {
			if s == "*" || strings.EqualFold(s, resource) {
				matched = true
				break
			}
			if strings.HasSuffix(s, "*") && strings.HasPrefix(resource, strings.TrimSuffix(s, "*")) {
				matched = true
				break
			}
			if (s == "api:read" || s == "api:write") && (strings.HasPrefix(resource, "/api/") || strings.EqualFold(s, resource)) {
				matched = true
				break
			}
		}
		if !matched && resource != "cli:exec" {
			return nil, fmt.Errorf("grant scope does not authorize requested resource %q", resource)
		}
	}

	tm.consumedGrants[claims.GrantID] = time.Now()
	return claims, nil
}
