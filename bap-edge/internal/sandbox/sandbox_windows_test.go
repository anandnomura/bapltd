//go:build windows

package sandbox

import (
	"strings"
	"testing"
)

func TestRestrictedTokenPrivileges(t *testing.T) {
	out, code, err := RunSandboxedCommandWithExitCode("whoami /priv")
	if err != nil {
		t.Fatalf("RunSandboxedCommandWithExitCode failed: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	t.Logf("whoami /priv output:\n%s", out)

	// In a restricted token with DISABLE_MAX_PRIVILEGE, sensitive administrative
	// and debug privileges (e.g. SeDebugPrivilege, SeTakeOwnershipPrivilege, SeSecurityPrivilege)
	// must NOT be present or enabled.
	for _, priv := range []string{"SeDebugPrivilege", "SeTakeOwnershipPrivilege", "SeSecurityPrivilege", "SeBackupPrivilege", "SeRestorePrivilege"} {
		if strings.Contains(out, priv) {
			t.Errorf("Unexpected high privilege present in restricted token: %s", priv)
		}
	}
}
