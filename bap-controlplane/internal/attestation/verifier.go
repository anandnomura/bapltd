package attestation

import (
	"fmt"
	"strings"

	"bap-controlplane/pkg/types"
)

// VerifyBinaryImage validates that the provided binary hash is permitted for the agent.
func VerifyBinaryImage(agent *types.RegisteredAgent, candidateHash string) error {
	candidateHash = strings.TrimSpace(strings.ToLower(candidateHash))
	if candidateHash == "" {
		return fmt.Errorf("missing candidate binary hash for attestation")
	}

	// 1. In Production, binary must be explicitly present in AllowedBinaryHashes
	if agent.EnvProfile == types.ProfileProd {
		if len(agent.AllowedBinaryHashes) == 0 {
			return fmt.Errorf("production profile requires explicit allowed binary hashes, none configured")
		}
		for _, allowed := range agent.AllowedBinaryHashes {
			if strings.EqualFold(candidateHash, strings.TrimSpace(allowed)) {
				return nil
			}
		}
		return fmt.Errorf("unauthorized binary image hash in production: %s", candidateHash)
	}

	// 2. In Development Profile:
	// If allowed hashes are specified, verify against them
	if len(agent.AllowedBinaryHashes) > 0 {
		for _, allowed := range agent.AllowedBinaryHashes {
			if strings.EqualFold(candidateHash, strings.TrimSpace(allowed)) {
				return nil
			}
		}
		return fmt.Errorf("binary hash %s does not match any allowed development hashes", candidateHash)
	}

	// If no hashes specified in Dev, Trust On First Use (TOFU) or if already enrolled, verify it matches enrolled
	if agent.EnrolledBinaryHash != "" && !strings.EqualFold(agent.EnrolledBinaryHash, candidateHash) {
		return fmt.Errorf("binary hash mismatch with enrolled development hash: got %s, expected %s", candidateHash, agent.EnrolledBinaryHash)
	}

	return nil
}
