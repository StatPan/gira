package gira

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type mcpRunner struct {
	name string
	args []string
	out  []byte
	err  error
}

func (r *mcpRunner) Run(name string, args ...string) ([]byte, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	if r.err != nil {
		return nil, r.err
	}
	return r.out, nil
}

func TestExecuteMCPToolWrapsReadOnlyGiraJSON(t *testing.T) {
	runner := &mcpRunner{out: []byte(`{"schema_version":"ticket-status/v1","issue":730}`)}
	envelope, toolErr := ExecuteMCPTool("gira_ticket_status", mcpArgs(map[string]any{"repo": "StatPan/gira", "ticket": 730}), runner)
	if toolErr != nil {
		t.Fatalf("ExecuteMCPTool error: %+v", toolErr)
	}
	if runner.name != "gira" {
		t.Fatalf("runner name = %q", runner.name)
	}
	wantArgs := []string{"ticket", "status", "730", "--repo", "StatPan/gira", "--json"}
	if strings.Join(runner.args, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
	}
	if !envelope.ReadOnly || envelope.Tool != "gira_ticket_status" || envelope.Command[0] != "gira" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if !bytes.Contains(envelope.Payload, []byte(`"ticket-status/v1"`)) {
		t.Fatalf("payload = %s", envelope.Payload)
	}
}

func TestExecuteMCPToolFinishPlanIsDryRunOnly(t *testing.T) {
	runner := &mcpRunner{out: []byte(`{"schema_version":"finish-readiness/v1"}`)}
	_, toolErr := ExecuteMCPTool("gira_ticket_finish_plan", mcpArgs(map[string]any{"repo": "StatPan/gira", "ticket": 640}), runner)
	if toolErr != nil {
		t.Fatalf("ExecuteMCPTool error: %+v", toolErr)
	}
	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "--dry-run") || strings.Contains(joined, "--apply") {
		t.Fatalf("finish plan args should be dry-run-only: %#v", runner.args)
	}
}

func TestExecuteMCPToolRejectsUnsupportedMutation(t *testing.T) {
	runner := &mcpRunner{out: []byte(`{}`)}
	_, toolErr := ExecuteMCPTool("gira_ticket_finish_apply", mcpArgs(map[string]any{"repo": "StatPan/gira", "ticket": 1}), runner)
	if toolErr == nil || !strings.Contains(toolErr.Error, "unsupported") {
		t.Fatalf("expected unsupported tool error, got %+v", toolErr)
	}
	if runner.name != "" {
		t.Fatalf("unsupported tool should not run command: %+v", runner)
	}
}

func TestExecuteMCPToolValidatesInputBeforeRunning(t *testing.T) {
	runner := &mcpRunner{out: []byte(`{}`)}
	_, toolErr := ExecuteMCPTool("gira_ticket_view", mcpArgs(map[string]any{"repo": "not-a-repo", "ticket": 1}), runner)
	if toolErr == nil || toolErr.Error == "" {
		t.Fatalf("expected validation error")
	}
	if runner.name != "" {
		t.Fatalf("invalid input should not run command: %+v", runner)
	}
}

func TestExecuteMCPToolReturnsCommandFailureEnvelope(t *testing.T) {
	runner := &mcpRunner{err: errors.New("boom")}
	_, toolErr := ExecuteMCPTool("gira_queue_next", mcpArgs(map[string]any{"repo": "StatPan/gira"}), runner)
	if toolErr == nil || toolErr.ExitCode != 1 || !strings.Contains(toolErr.Stderr, "boom") {
		t.Fatalf("unexpected tool error: %+v", toolErr)
	}
}

func TestServeMCPListsAndCallsTools(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gira_queue_next","arguments":{"repo":"StatPan/gira"}}}` + "\n")
	var output bytes.Buffer
	runner := &mcpRunner{out: []byte(`{"schema_version":"queue-next/v1"}`)}
	if err := ServeMCP(input, &output, MCPOptions{Runner: runner}); err != nil {
		t.Fatalf("ServeMCP error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("responses = %q", output.String())
	}
	if !strings.Contains(lines[0], "gira_ticket_view") || !strings.Contains(lines[1], "queue-next/v1") {
		t.Fatalf("unexpected responses:\n%s", output.String())
	}
}

func mcpArgs(values map[string]any) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for key, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}
		out[key] = encoded
	}
	return out
}
