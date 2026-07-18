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

type mcpCLIExecutor struct {
	args    []string
	workdir string
	result  MCPCommandExecution
}

func (r *mcpRunner) Run(name string, args ...string) ([]byte, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	if r.err != nil {
		return nil, r.err
	}
	return r.out, nil
}

func (e *mcpCLIExecutor) ExecuteGira(args []string, workdir string) MCPCommandExecution {
	e.args = append([]string(nil), args...)
	e.workdir = workdir
	return e.result
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

func TestExecuteMCPToolCLIParityExecutesGiraArgs(t *testing.T) {
	executor := &mcpCLIExecutor{result: MCPCommandExecution{Stdout: "ticket start: dry-run\n", ExitCode: 0}}
	payload, toolErr := ExecuteMCPToolWithOptions("gira_cli", mcpArgs(map[string]any{
		"args":    []string{"ticket", "start", "754", "--repo", "StatPan/gira", "--dry-run"},
		"workdir": "/tmp/repo",
	}), MCPOptions{Executor: executor})
	if toolErr != nil {
		t.Fatalf("ExecuteMCPToolWithOptions error: %+v", toolErr)
	}
	envelope, ok := payload.(MCPCLICommandEnvelope)
	if !ok {
		t.Fatalf("payload type = %T", payload)
	}
	if strings.Join(executor.args, " ") != "ticket start 754 --repo StatPan/gira --dry-run" {
		t.Fatalf("args = %#v", executor.args)
	}
	if executor.workdir != "/tmp/repo" {
		t.Fatalf("workdir = %q", executor.workdir)
	}
	if envelope.ExitCode != 0 || !envelope.DryRun || envelope.Apply {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if !strings.Contains(envelope.Stdout, "ticket start") {
		t.Fatalf("stdout = %q", envelope.Stdout)
	}
}

func TestExecuteMCPToolCLIParityReportsFailureWithoutRPCError(t *testing.T) {
	executor := &mcpCLIExecutor{result: MCPCommandExecution{Stderr: "not ready\n", ExitCode: 2}}
	payload, toolErr := ExecuteMCPToolWithOptions("gira_cli", mcpArgs(map[string]any{
		"args": []string{"ticket", "finish", "754", "--repo", "StatPan/gira", "--apply"},
	}), MCPOptions{Executor: executor})
	if toolErr != nil {
		t.Fatalf("CLI failures should be command envelopes, got %+v", toolErr)
	}
	envelope := payload.(MCPCLICommandEnvelope)
	if envelope.ExitCode != 2 || !envelope.Apply || envelope.DryRun {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if !strings.Contains(envelope.Stderr, "not ready") {
		t.Fatalf("stderr = %q", envelope.Stderr)
	}
}

func TestExecuteMCPToolCLIParityValidatesArgv(t *testing.T) {
	executor := &mcpCLIExecutor{result: MCPCommandExecution{Stdout: "should not run"}}
	_, toolErr := ExecuteMCPToolWithOptions("gira_cli", mcpArgs(map[string]any{
		"args": []string{"gira", "ticket", "status"},
	}), MCPOptions{Executor: executor})
	if toolErr == nil || !strings.Contains(toolErr.Error, "omit") {
		t.Fatalf("expected validation error, got %+v", toolErr)
	}
	if len(executor.args) != 0 {
		t.Fatalf("invalid argv should not execute: %#v", executor.args)
	}
}

func TestExecuteMCPToolCLIParityRejectsRecursiveServe(t *testing.T) {
	executor := &mcpCLIExecutor{result: MCPCommandExecution{Stdout: "should not run"}}
	_, toolErr := ExecuteMCPToolWithOptions("gira_cli", mcpArgs(map[string]any{
		"args": []string{"mcp", "serve"},
	}), MCPOptions{Executor: executor})
	if toolErr == nil || !strings.Contains(toolErr.Error, "recursively") {
		t.Fatalf("expected recursive serve validation error, got %+v", toolErr)
	}
	if len(executor.args) != 0 {
		t.Fatalf("invalid argv should not execute: %#v", executor.args)
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

func TestMCPFocusedPMToolsAreReadOnlyCLIAdapters(t *testing.T) {
	cases := map[string]string{
		"gira_pm_bootstrap":   "pm bootstrap --repo StatPan/gira --ticket 868 --role ai --json",
		"gira_pm_compile":     "pm compile --repo StatPan/gira --goal 868 --json",
		"gira_pm_observe":     "pm observe --repo StatPan/gira --ticket 868 --json",
		"gira_pm_replan_plan": "pm replan --repo StatPan/gira --ticket 868 --dry-run --json",
		"gira_pm_validate":    "pm qa --repo StatPan/gira --ticket 868 --json",
		"gira_pm_report":      "goal report 868 --repo StatPan/gira --view ai --json",
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			runner := &mcpRunner{out: []byte(`{"schema_version":"fixture/v1"}`)}
			envelope, toolErr := ExecuteMCPTool(name, mcpArgs(map[string]any{"repo": "StatPan/gira", "ticket": 868}), runner)
			if toolErr != nil {
				t.Fatal(toolErr)
			}
			got := strings.Join(runner.args, " ")
			if got != want || strings.Contains(got, "--apply") || !envelope.ReadOnly {
				t.Fatalf("adapter diverged or mutated: got=%q envelope=%#v", got, envelope)
			}
		})
	}
}

func TestExecuteMCPToolWorkflowGuideIsReadOnlyAndDoesNotRunCommand(t *testing.T) {
	runner := &mcpRunner{out: []byte(`{}`)}
	envelope, toolErr := ExecuteMCPTool("gira_workflow_guide", nil, runner)
	if toolErr != nil {
		t.Fatalf("ExecuteMCPTool error: %+v", toolErr)
	}
	if runner.name != "" {
		t.Fatalf("workflow guide should not run a command: %+v", runner)
	}
	if !envelope.ReadOnly || envelope.Tool != "gira_workflow_guide" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if !bytes.Contains(envelope.Payload, []byte(`"gira-mcp-workflow-guide/v1"`)) {
		t.Fatalf("payload = %s", envelope.Payload)
	}
	if !bytes.Contains(envelope.Payload, []byte(`dry-run/apply`)) {
		t.Fatalf("payload should explain dry-run/apply boundary: %s", envelope.Payload)
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
	if !strings.Contains(lines[0], "gira_cli") || !strings.Contains(lines[0], "gira_workflow_guide") || !strings.Contains(lines[0], "gira_ticket_view") || !strings.Contains(lines[1], "queue-next/v1") {
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
