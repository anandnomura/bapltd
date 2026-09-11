package types

// AttestationResponse is returned by the attestation server when verification succeeds.
type AttestationResponse struct {
	Token string `json:"token"`
}

// ExecResponse is output by the exec command.
type ExecResponse struct {
	Allowed bool   `json:"allowed"`
	Output  string `json:"output,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

