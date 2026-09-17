package main

import (
	"encoding/base64"
	"testing"
)

func TestUnwrapBrokerCommandRequiresExactSessionBoundWrapper(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte("echo governed"))
	command := `bapedge exec --source claude-code --session-id sess-1 --cmd-b64 ` + payload
	decoded, wrapped, err := unwrapBrokerCommand(command, "sess-1")
	if err != nil || !wrapped || decoded != "echo governed" {
		t.Fatalf("decoded=%q wrapped=%v err=%v", decoded, wrapped, err)
	}
}

func TestUnwrapBrokerCommandRejectsAgentLookalikes(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte("cat .env"))
	for _, command := range []string{
		`bapedge exec --source claude-code --session-id another-session --cmd-b64 ` + payload,
		`bapedge exec --source claude-code --session-id sess-1 --cmd-b64 ` + payload + ` && echo bypass`,
		`bapedge --cmd-b64 ` + payload,
	} {
		if _, wrapped, err := unwrapBrokerCommand(command, "sess-1"); !wrapped || err == nil {
			t.Fatalf("expected wrapped lookalike to be rejected: %q (wrapped=%v err=%v)", command, wrapped, err)
		}
	}
}

func TestUnwrapBrokerCommandDoesNotTrustArbitraryCmdB64Argument(t *testing.T) {
	command := `python tool.py --cmd-b64 attacker-controlled`
	decoded, wrapped, err := unwrapBrokerCommand(command, "sess-1")
	if err != nil || wrapped || decoded != command {
		t.Fatalf("decoded=%q wrapped=%v err=%v", decoded, wrapped, err)
	}
}
