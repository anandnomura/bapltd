package authz

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
)

// Authorizer evaluates execution requests against Cedar policies and schemas.
type Authorizer struct {
	policySet *cedar.PolicySet
	schema    *schema.Schema
}

// NewAuthorizer loads the Cedar policy and optional schema from specified or standard locations.
func NewAuthorizer(policyPath string) (*Authorizer, error) {
	resolvedPolicyPath, err := findFile(policyPath, "policy.cedar")
	if err != nil {
		return nil, err
	}

	// Check if kill-switch is active in policy-state.json
	policyDir := filepath.Dir(resolvedPolicyPath)
	statePath := filepath.Join(policyDir, "policy-state.json")
	if stateData, stateErr := os.ReadFile(statePath); stateErr == nil {
		var state struct {
			KillSwitch bool `json:"kill_switch"`
		}
		if json.Unmarshal(stateData, &state) == nil && state.KillSwitch {
			return nil, fmt.Errorf("emergency kill-switch is active; all executions are blocked")
		}
	}

	policyData, err := os.ReadFile(resolvedPolicyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file %s: %w", resolvedPolicyPath, err)
	}

	ps, err := cedar.NewPolicySetFromBytes(resolvedPolicyPath, policyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Cedar policy file %s: %w", resolvedPolicyPath, err)
	}

	authz := &Authorizer{policySet: ps}

	// Try loading schema.json from the same directory or standard locations
	schemaPathCandidate := filepath.Join(policyDir, "schema.json")
	if resolvedSchemaPath, err := findFile(schemaPathCandidate, "schema.json"); err == nil {
		if schemaData, err := os.ReadFile(resolvedSchemaPath); err == nil {
			var s schema.Schema
			if err := s.UnmarshalJSON(schemaData); err == nil {
				if _, err := s.Resolve(); err == nil {
					authz.schema = &s
					log.Printf("[authz] Loaded and validated Cedar schema from %s", resolvedSchemaPath)
				} else {
					log.Printf("[authz] Warning: Failed to resolve schema %s: %v", resolvedSchemaPath, err)
				}
			} else {
				log.Printf("[authz] Warning: Failed to parse schema %s: %v", resolvedSchemaPath, err)
			}
		}
	}

	return authz, nil
}

// Evaluate checks whether executing the given command is permitted.
// The Cedar schema specifies:
// - Principal: Agent::"Local"
// - Action:    Action::"Execute"
// - Resource:  Command::"CLI"
func (a *Authorizer) Evaluate(executable, fullCommand, args string) (bool, string, error) {
	normalizedExec := strings.ToLower(executable)
	normalizedFull := strings.ToLower(fullCommand)
	normalizedArgs := strings.ToLower(args)

	req := cedar.Request{
		Principal: cedar.NewEntityUID("Agent", "Local"),
		Action:    cedar.NewEntityUID("Action", "Execute"),
		Resource:  cedar.NewEntityUID("Command", "CLI"),
		Context: cedar.NewRecord(cedar.RecordMap{
			cedar.String("executable"):   cedar.String(normalizedExec),
			cedar.String("full_command"): cedar.String(normalizedFull),
			cedar.String("args"):         cedar.String(normalizedArgs),
		}),
	}

	entities := types.EntityMap{}
	decision, diag := cedar.Authorize(a.policySet, entities, req)

	if decision == cedar.Allow {
		return true, "", nil
	}

	var reason string
	if len(diag.Reasons) > 0 {
		var reasonList []string
		for _, r := range diag.Reasons {
			reasonList = append(reasonList, fmt.Sprintf("policy %s", r.PolicyID))
		}
		reason = fmt.Sprintf("Denial triggered by: %s", strings.Join(reasonList, ", "))
	} else if len(diag.Errors) > 0 {
		var errList []string
		for _, e := range diag.Errors {
			errList = append(errList, e.Message)
		}
		reason = fmt.Sprintf("Policy evaluation error: %s", strings.Join(errList, "; "))
	} else {
		reason = "Explicit deny or default deny (no permit policy matched)"
	}

	return false, reason, nil
}

// findFile resolves a file path by checking:
// 1. Specified custom path (if non-empty and exists)
// 2. Current working directory
// 3. Executable's directory
func findFile(customPath, defaultName string) (string, error) {
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			return customPath, nil
		}
	}

	// 1. Current directory
	cwdFile := defaultName
	if _, err := os.Stat(cwdFile); err == nil {
		return cwdFile, nil
	}

	// 2. Executable directory
	exe, err := os.Executable()
	if err == nil {
		exeDirFile := filepath.Join(filepath.Dir(exe), defaultName)
		if _, err := os.Stat(exeDirFile); err == nil {
			return exeDirFile, nil
		}
	}

	// 3. User cached policy directory (~/.ltd/policy/ or ~/.bap/policy/)
	if home, err := os.UserHomeDir(); err == nil {
		ltdCache := filepath.Join(home, ".ltd", "policy", defaultName)
		if _, err := os.Stat(ltdCache); err == nil {
			return ltdCache, nil
		}
		bapCache := filepath.Join(home, ".bap", "policy", defaultName)
		if _, err := os.Stat(bapCache); err == nil {
			return bapCache, nil
		}
	}

	return "", fmt.Errorf("%s not found in custom path, current directory, executable directory, or ~/.ltd/policy/", defaultName)
}
