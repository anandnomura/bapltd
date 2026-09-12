package attestation

import (
	"testing"

	"bap-controlplane/pkg/types"
)

func TestVerifyBinaryImage_ProdProfile(t *testing.T) {
	agent := &types.RegisteredAgent{
		AgentID:             "agent-prod-01",
		EnvProfile:          types.ProfileProd,
		AllowedBinaryHashes: []string{"aabbccdd11223344", "eeff001122334455"},
	}

	// 1. Valid hash in list (case-insensitive)
	if err := VerifyBinaryImage(agent, "AABBCCDD11223344"); err != nil {
		t.Fatalf("expected valid prod hash to pass, got: %v", err)
	}

	// 2. Hash not in list
	if err := VerifyBinaryImage(agent, "deadbeef12345678"); err == nil {
		t.Fatalf("expected unlisted prod hash to fail")
	}

	// 3. Prod with empty whitelist should fail
	emptyAgent := &types.RegisteredAgent{
		AgentID:    "agent-prod-no-hash",
		EnvProfile: types.ProfileProd,
	}
	if err := VerifyBinaryImage(emptyAgent, "aabbccdd11223344"); err == nil {
		t.Fatalf("expected empty prod whitelist to fail")
	}
}

func TestVerifyBinaryImage_DevProfile(t *testing.T) {
	// 1. Dev with explicit hash whitelist
	agentWithAllowed := &types.RegisteredAgent{
		AgentID:             "agent-dev-01",
		EnvProfile:          types.ProfileDev,
		AllowedBinaryHashes: []string{"hash123"},
	}
	if err := VerifyBinaryImage(agentWithAllowed, "hash123"); err != nil {
		t.Fatalf("expected dev allowed hash to pass: %v", err)
	}
	if err := VerifyBinaryImage(agentWithAllowed, "hash456"); err == nil {
		t.Fatalf("expected dev mismatched hash to fail when allowed hashes are specified")
	}

	// 2. Dev with TOFU (no allowed hashes yet, not enrolled)
	agentTOFU := &types.RegisteredAgent{
		AgentID:    "agent-dev-tofu",
		EnvProfile: types.ProfileDev,
	}
	if err := VerifyBinaryImage(agentTOFU, "anydevhash999"); err != nil {
		t.Fatalf("expected TOFU dev to pass any hash initially: %v", err)
	}

	// 3. Dev after enrollment: must match enrolled hash
	agentTOFU.EnrolledBinaryHash = "anydevhash999"
	if err := VerifyBinaryImage(agentTOFU, "anydevhash999"); err != nil {
		t.Fatalf("expected enrolled hash match to pass: %v", err)
	}
	if err := VerifyBinaryImage(agentTOFU, "altereddevhash"); err == nil {
		t.Fatalf("expected altered hash after enrollment to fail")
	}
}
