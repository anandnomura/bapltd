package sandbox

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildExecCmdUsesHostShell(t *testing.T) {
	cmd := BuildExecCmd("echo hello")

	if runtime.GOOS == "windows" {
		if !strings.EqualFold(filepath.Base(cmd.Path), "cmd.exe") {
			t.Fatalf("expected cmd.exe on Windows, got %q", cmd.Path)
		}
		expectedArgs := []string{"cmd.exe", "/c", "echo hello"}
		if len(cmd.Args) != len(expectedArgs) {
			t.Fatalf("expected args %v, got %v", expectedArgs, cmd.Args)
		}
		for i := range expectedArgs {
			if cmd.Args[i] != expectedArgs[i] {
				t.Fatalf("expected args %v, got %v", expectedArgs, cmd.Args)
			}
		}
		return
	}

	if cmd.Path != "/bin/sh" {
		t.Fatalf("expected /bin/sh on POSIX, got %q", cmd.Path)
	}
	expectedArgs := []string{"/bin/sh", "-c", "echo hello"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, cmd.Args)
	}
	for i := range expectedArgs {
		if cmd.Args[i] != expectedArgs[i] {
			t.Fatalf("expected args %v, got %v", expectedArgs, cmd.Args)
		}
	}
}

func TestRunSandboxedCommandReturnsExecutionError(t *testing.T) {
	_, err := RunSandboxedCommand("definitely-not-a-real-ltd-agent-command")
	if err == nil {
		t.Fatal("expected command execution error")
	}
}

func TestRunSandboxedCommandNormalizesWhitespace(t *testing.T) {
	output, err := RunSandboxedCommand("echo hello")
	if err != nil {
		t.Fatalf("unexpected error running echo: %v", err)
	}
	if strings.Contains(output, "\r") {
		t.Errorf("output still contains carriage return (\\r): %q", output)
	}
	if strings.HasPrefix(output, " ") || strings.HasPrefix(output, "\n") ||
		strings.HasSuffix(output, " ") || strings.HasSuffix(output, "\n") {
		t.Errorf("output has leading or trailing whitespace: %q", output)
	}
	if output != "hello" {
		t.Errorf("expected %q, got %q", "hello", output)
	}
}

func TestCleanOutput(t *testing.T) {
	input := "\r\n\r\nline1   \r\n\r\n\r\nline2\t  \r\n\r\n"
	expected := "line1\n\nline2"
	got := CleanOutput(input)
	if got != expected {
		t.Errorf("CleanOutput mismatch:\nexpected: %q\ngot:      %q", expected, got)
	}
}

func TestParseCommandNormalization(t *testing.T) {
	tests := []struct {
		input        string
		expectedExec string
		expectedArgs string
	}{
		{"pytest -s tests/test_leak.py", "pytest", "-s tests/test_leak.py"},
		{"pytest.exe -v", "pytest", "-v"},
		{`C:\Python312\Scripts\pytest.exe -s tests/test_leak.py`, "pytest", "-s tests/test_leak.py"},
		{`"C:\Program Files\Python\Scripts\pytest.exe" -v`, "pytest", "-v"},
		{"/usr/local/bin/pytest -q", "pytest", "-q"},
		{"/c/users/appdata/roaming/script/pytest -s test.py", "pytest", "-s test.py"},
		{"./venv/bin/pytest tests/", "pytest", "tests/"},
		{"python -m pytest tests/test_leak.py", "pytest", "-m pytest tests/test_leak.py"},
		{"python3 -m pytest -s test.py", "pytest", "-m pytest -s test.py"},
		{"py -m pytest -s test.py", "pytest", "-m pytest -s test.py"},
		{`"C:\Python312\python.exe" -m pytest -v`, "pytest", "-m pytest -v"},
		{"git status", "git", "status"},
		{`"C:\Program Files\Git\bin\git.exe" status`, "git", "status"},
	}

	for _, tc := range tests {
		exec, args := ParseCommand(tc.input)
		if exec != tc.expectedExec {
			t.Errorf("for input %q, expected executable %q, got %q", tc.input, tc.expectedExec, exec)
		}
		if args != tc.expectedArgs {
			t.Errorf("for input %q, expected args %q, got %q", tc.input, tc.expectedArgs, args)
		}
	}
}



