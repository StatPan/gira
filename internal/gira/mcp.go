package gira

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

const MCPServerSchemaVersion = "gira-mcp-read-only/v1"

type MCPOptions struct {
	Runner   CommandRunner
	Executor MCPCommandExecutor
}

type MCPCommandEnvelope struct {
	SchemaVersion string          `json:"schema_version"`
	Tool          string          `json:"tool"`
	Command       []string        `json:"command"`
	ReadOnly      bool            `json:"read_only"`
	Payload       json.RawMessage `json:"payload"`
}

type MCPToolError struct {
	SchemaVersion string   `json:"schema_version"`
	Tool          string   `json:"tool,omitempty"`
	Command       []string `json:"command,omitempty"`
	ExitCode      int      `json:"exit_code,omitempty"`
	Stderr        string   `json:"stderr,omitempty"`
	Stdout        string   `json:"stdout,omitempty"`
	Error         string   `json:"error"`
}

type MCPCLICommandEnvelope struct {
	SchemaVersion string   `json:"schema_version"`
	Tool          string   `json:"tool"`
	Command       []string `json:"command"`
	Workdir       string   `json:"workdir,omitempty"`
	ExitCode      int      `json:"exit_code"`
	Stdout        string   `json:"stdout"`
	Stderr        string   `json:"stderr,omitempty"`
	DryRun        bool     `json:"dry_run"`
	Apply         bool     `json:"apply"`
	JSONRequested bool     `json:"json_requested"`
}

type MCPCommandExecution struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type MCPCommandExecutor interface {
	ExecuteGira(args []string, workdir string) MCPCommandExecution
}

type MCPDefaultCommandExecutor struct {
	Auth MCPAuthConfig
}

type MCPToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type MCPWorkflowGuide struct {
	SchemaVersion  string            `json:"schema_version"`
	Principle      string            `json:"principle"`
	LocalFlow      []MCPWorkflowStep `json:"local_flow"`
	Auth           []string          `json:"auth"`
	Evidence       []string          `json:"evidence"`
	Safety         []string          `json:"safety"`
	HostedBoundary []string          `json:"hosted_boundary"`
}

type MCPWorkflowStep struct {
	Name      string `json:"name"`
	UseMCP    string `json:"use_mcp"`
	UseCLI    string `json:"use_cli"`
	UserCheck string `json:"user_check"`
}

type mcpToolTemplate struct {
	Name        string
	Description string
	Ticket      bool
	Queue       bool
	Build       func(repo string, ticket int, limit int, queue string) []string
}

var mcpTools = []mcpToolTemplate{
	{Name: "gira_ticket_view", Description: "Read Gira ticket packet context.", Ticket: true, Build: func(repo string, ticket int, limit int, queue string) []string {
		return []string{"ticket", "view", strconv.Itoa(ticket), "--repo", repo, "--json"}
	}},
	{Name: "gira_ticket_status", Description: "Read Gira ticket lifecycle status.", Ticket: true, Build: func(repo string, ticket int, limit int, queue string) []string {
		return []string{"ticket", "status", strconv.Itoa(ticket), "--repo", repo, "--json"}
	}},
	{Name: "gira_ticket_checks", Description: "Read linked PR checks and finish blockers.", Ticket: true, Build: func(repo string, ticket int, limit int, queue string) []string {
		return []string{"ticket", "checks", strconv.Itoa(ticket), "--repo", repo, "--json"}
	}},
	{Name: "gira_ticket_finish_plan", Description: "Read dry-run-only finish plan and blockers.", Ticket: true, Build: func(repo string, ticket int, limit int, queue string) []string {
		return []string{"ticket", "finish", strconv.Itoa(ticket), "--repo", repo, "--dry-run", "--json"}
	}},
	{Name: "gira_ticket_handoff", Description: "Read worker-handoff/v1 for a ticket.", Ticket: true, Build: func(repo string, ticket int, limit int, queue string) []string {
		return []string{"ticket", "handoff", strconv.Itoa(ticket), "--repo", repo, "--json"}
	}},
	{Name: "gira_queue_list", Description: "Read workspace queue items.", Queue: true, Build: func(repo string, ticket int, limit int, queue string) []string {
		args := []string{"queue", "list", "--repo", repo, "--json"}
		if queue != "" {
			args = append(args, "--queue", queue)
		}
		if limit > 0 {
			args = append(args, "--limit", strconv.Itoa(limit))
		}
		return args
	}},
	{Name: "gira_queue_next", Description: "Read next recommended queue item without claiming it.", Queue: true, Build: func(repo string, ticket int, limit int, queue string) []string {
		return []string{"queue", "next", "--repo", repo, "--json"}
	}},
	{Name: "gira_queue_handoff", Description: "Read queue handoff without starting branches or workers.", Queue: true, Build: func(repo string, ticket int, limit int, queue string) []string {
		args := []string{"queue", "handoff", "--repo", repo, "--json"}
		if ticket > 0 {
			args = append(args, "--ticket", strconv.Itoa(ticket))
		}
		return args
	}},
}

func MCPToolSpecs() []MCPToolSpec {
	out := []MCPToolSpec{{
		Name:        "gira_workflow_guide",
		Description: "Read the recommended conversational agent workflow for using MCP with the Gira CLI lifecycle.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties":           map[string]any{},
			"required":             []string{},
		},
	}, {
		Name:        "gira_cli",
		Description: "Execute the installed Gira CLI with explicit argv. This is CLI parity over MCP: no shell, no raw gh, no separate lifecycle.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"args": map[string]any{
					"type":        "array",
					"description": "Gira CLI argv excluding the `gira` binary, for example [`ticket`,`status`,`42`,`--repo`,`OWNER/REPO`,`--json`].",
					"minItems":    1,
					"items":       map[string]any{"type": "string"},
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Optional working directory for repo/branch inference. Defaults to the MCP server working directory.",
				},
			},
			"required": []string{"args"},
		},
	}}
	for _, tool := range mcpTools {
		properties := map[string]any{
			"repo": map[string]any{"type": "string", "description": "GitHub repository in OWNER/REPO form."},
		}
		required := []string{"repo"}
		if tool.Ticket {
			properties["ticket"] = map[string]any{"type": "integer", "minimum": 1, "description": "GitHub issue number."}
			required = append(required, "ticket")
		}
		if tool.Name == "gira_queue_list" {
			properties["limit"] = map[string]any{"type": "integer", "minimum": 1, "description": "Maximum queue items to return."}
			properties["queue"] = map[string]any{"type": "string", "description": "Optional queue alias such as ready, review, finish, blocked, failed, or human."}
		}
		if tool.Name == "gira_queue_handoff" {
			properties["ticket"] = map[string]any{"type": "integer", "minimum": 1, "description": "Optional explicit GitHub issue number."}
		}
		out = append(out, MCPToolSpec{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties":           properties,
				"required":             required,
			},
		})
	}
	return out
}

func ExecuteMCPTool(name string, arguments map[string]json.RawMessage, runner CommandRunner) (MCPCommandEnvelope, *MCPToolError) {
	envelope, toolErr, _ := executeMCPTool(name, arguments, MCPOptions{Runner: runner})
	return envelope, toolErr
}

func ExecuteMCPToolWithOptions(name string, arguments map[string]json.RawMessage, options MCPOptions) (any, *MCPToolError) {
	envelope, toolErr, payload := executeMCPTool(name, arguments, options)
	if toolErr != nil {
		return nil, toolErr
	}
	if payload != nil {
		return payload, nil
	}
	return envelope, nil
}

func executeMCPTool(name string, arguments map[string]json.RawMessage, options MCPOptions) (MCPCommandEnvelope, *MCPToolError, any) {
	runner := options.Runner
	if name == "gira_workflow_guide" {
		if len(arguments) > 0 {
			return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: "gira_workflow_guide does not accept arguments"}, nil
		}
		payload, err := json.Marshal(MCPAgentWorkflowGuide())
		if err != nil {
			return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}, nil
		}
		return MCPCommandEnvelope{
			SchemaVersion: MCPServerSchemaVersion,
			Tool:          name,
			Command:       []string{"gira", "mcp", "serve", "tool:gira_workflow_guide"},
			ReadOnly:      true,
			Payload:       payload,
		}, nil, nil
	}
	if name == "gira_cli" {
		payload, toolErr := executeMCPCLI(arguments, options.Executor)
		if toolErr != nil {
			return MCPCommandEnvelope{}, toolErr, nil
		}
		return MCPCommandEnvelope{}, nil, payload
	}
	if runner == nil {
		runner = NewMCPCommandRunnerFromEnv()
	}
	tool, ok := findMCPTool(name)
	if !ok {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: "unsupported Gira MCP tool"}, nil
	}
	repo, err := mcpStringArg(arguments, "repo", true)
	if err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}, nil
	}
	if _, err := ParseRepoRef(repo); err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}, nil
	}
	ticket, err := mcpIntArg(arguments, "ticket", tool.Ticket)
	if err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}, nil
	}
	limit, err := mcpIntArg(arguments, "limit", false)
	if err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}, nil
	}
	queue, err := mcpStringArg(arguments, "queue", false)
	if err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}, nil
	}
	args := tool.Build(repo, ticket, limit, queue)
	command := append([]string{"gira"}, args...)
	stdout, runErr := runner.Run("gira", args...)
	if runErr != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Command: command, ExitCode: 1, Stderr: runErr.Error(), Error: "gira command failed"}, nil
	}
	trimmed := bytes.TrimSpace(stdout)
	if !json.Valid(trimmed) {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Command: command, Stdout: string(stdout), Error: "gira command did not emit valid JSON"}, nil
	}
	return MCPCommandEnvelope{SchemaVersion: MCPServerSchemaVersion, Tool: name, Command: command, ReadOnly: true, Payload: append(json.RawMessage(nil), trimmed...)}, nil, nil
}

func executeMCPCLI(arguments map[string]json.RawMessage, executor MCPCommandExecutor) (MCPCLICommandEnvelope, *MCPToolError) {
	args, err := mcpStringSliceArg(arguments, "args", true)
	if err != nil {
		return MCPCLICommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: "gira_cli", Error: err.Error()}
	}
	workdir, err := mcpStringArg(arguments, "workdir", false)
	if err != nil {
		return MCPCLICommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: "gira_cli", Error: err.Error()}
	}
	if err := validateMCPCLIArgs(args); err != nil {
		return MCPCLICommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: "gira_cli", Error: err.Error()}
	}
	if executor == nil {
		executor = MCPDefaultCommandExecutor{Auth: ResolveMCPAuthConfig(NewMCPCommandRunnerFromEnv().commandEnv())}
	}
	result := executor.ExecuteGira(args, workdir)
	return MCPCLICommandEnvelope{
		SchemaVersion: "gira-mcp-cli-exec/v1",
		Tool:          "gira_cli",
		Command:       append([]string{"gira"}, args...),
		Workdir:       workdir,
		ExitCode:      result.ExitCode,
		Stdout:        result.Stdout,
		Stderr:        result.Stderr,
		DryRun:        mcpArgsContain(args, "--dry-run"),
		Apply:         mcpArgsContain(args, "--apply"),
		JSONRequested: mcpArgsContain(args, "--json"),
	}, nil
}

func (e MCPDefaultCommandExecutor) ExecuteGira(args []string, workdir string) MCPCommandExecution {
	auth := e.Auth
	if len(auth.env) == 0 {
		auth = ResolveMCPAuthConfig(NewMCPCommandRunnerFromEnv().commandEnv())
	}
	cmd := exec.Command("gira", args...)
	cmd.Env = (MCPCommandRunner{Auth: auth}).commandEnv()
	if strings.TrimSpace(workdir) != "" {
		cmd.Dir = strings.TrimSpace(workdir)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return MCPCommandExecution{
		Stdout:   redactSecrets(stdout.String(), auth),
		Stderr:   redactSecrets(stderr.String(), auth),
		ExitCode: exitCode,
	}
}

func validateMCPCLIArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("args is required")
	}
	for i, arg := range args {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("args[%d] must be a non-empty string", i)
		}
		if strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("args[%d] must not contain NUL bytes", i)
		}
	}
	if args[0] == "gira" {
		return fmt.Errorf("args must omit the `gira` binary")
	}
	if len(args) >= 2 && args[0] == "mcp" && args[1] == "serve" {
		return fmt.Errorf("gira mcp serve cannot be started recursively through MCP")
	}
	return nil
}

func MCPAgentWorkflowGuide() MCPWorkflowGuide {
	return MCPWorkflowGuide{
		SchemaVersion: "gira-mcp-workflow-guide/v1",
		Principle:     "MCP should make Gira feel conversational, but it must not make lifecycle changes implicit. Agents may continuously read and summarize project state through MCP while every mutation remains an explicit Gira CLI dry-run/apply transition with GitHub evidence.",
		LocalFlow: []MCPWorkflowStep{
			{
				Name:      "adopt_or_select",
				UseMCP:    "Read queue, ticket, and repository context when available.",
				UseCLI:    "Use `gira adopt`, `gira queue`, or `gira ticket new` with `--dry-run` before creating or changing workflow state.",
				UserCheck: "Confirm the target repo, issue, branch, and intended lifecycle transition before apply.",
			},
			{
				Name:      "inspect_context",
				UseMCP:    "Use ticket view/status/checks/handoff and queue tools for conversational context gathering.",
				UseCLI:    "Use CLI status/review/checks commands when a human-readable or local command receipt is needed.",
				UserCheck: "Surface blockers, stale assumptions, and missing auth before proposing mutation.",
			},
			{
				Name:      "plan_mutation",
				UseMCP:    "Summarize current state and explain the proposed next action.",
				UseCLI:    "Run the matching Gira command with `--dry-run`; do not use MCP as an apply surface.",
				UserCheck: "Show the dry-run result and ask for explicit agreement when the operation changes remote state.",
			},
			{
				Name:      "apply_mutation",
				UseMCP:    "Continue reading status after the mutation to keep the conversation current.",
				UseCLI:    "Run the approved command with `--apply` and capture the resulting issue, PR, check, or release link.",
				UserCheck: "Report exactly what changed and where the evidence was recorded.",
			},
			{
				Name:      "review_and_finish",
				UseMCP:    "Read checks, finish plan, handoff, and queue state while discussing readiness.",
				UseCLI:    "Use `gira ticket finish --dry-run` before `--apply`; release tags and publishing remain explicit CLI/GitHub actions.",
				UserCheck: "Do not finish work with failing checks, missing PR evidence, or unresolved human decisions.",
			},
		},
		Auth: []string{
			"Prefer `GIRA_MCP_GITHUB_TOKEN` for MCP-specific local credentials.",
			"Fall back to `GITHUB_TOKEN`, then `GH_TOKEN`, then local `gh` authentication.",
			"Use `gira mcp doctor --repo OWNER/REPO --json` before relying on MCP reads in a new environment.",
			"Do not persist GitHub tokens in Gira repo config or MCP workflow documents.",
		},
		Evidence: []string{
			"GitHub issues remain task packets.",
			"Pull requests remain change units.",
			"Checks, reviews, merge state, and issue comments remain completion evidence.",
			"MCP context is advisory unless backed by a Gira command result or GitHub state.",
		},
		Safety: []string{
			"MCP tools must stay read-only or dry-run-only unless a future ADR explicitly changes the boundary.",
			"Do not expose raw shell, raw `gh`, or hidden `--apply` behavior through MCP.",
			"Do not create a second MCP-only lifecycle vocabulary that diverges from Gira CLI states.",
			"Prefer short conversational summaries, but include concrete issue, PR, workflow, and release links when state changes.",
		},
		HostedBoundary: []string{
			"A hosted MCP service should preserve the same conversational flow and evidence model.",
			"Hosted authentication should use per-user or per-installation authorization rather than shared environment tokens.",
			"Hosted service behavior must not silently mutate GitHub state without an explicit dry-run/apply equivalent.",
		},
	}
}

func findMCPTool(name string) (mcpToolTemplate, bool) {
	for _, tool := range mcpTools {
		if tool.Name == name {
			return tool, true
		}
	}
	return mcpToolTemplate{}, false
}

func mcpStringArg(args map[string]json.RawMessage, name string, required bool) (string, error) {
	raw, ok := args[name]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		if required {
			return "", fmt.Errorf("%s is required", name)
		}
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func mcpIntArg(args map[string]json.RawMessage, name string, required bool) (int, error) {
	raw, ok := args[name]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		if required {
			return 0, fmt.Errorf("%s is required", name)
		}
		return 0, nil
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", name)
	}
	return value, nil
}

func mcpStringSliceArg(args map[string]json.RawMessage, name string, required bool) ([]string, error) {
	raw, ok := args[name]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		if required {
			return nil, fmt.Errorf("%s is required", name)
		}
		return nil, nil
	}
	var value []string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("%s must be an array of strings", name)
	}
	if required && len(value) == 0 {
		return nil, fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func mcpArgsContain(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpCallParams struct {
	Name      string                     `json:"name"`
	Arguments map[string]json.RawMessage `json:"arguments"`
}

func ServeMCP(input io.Reader, output io.Writer, options MCPOptions) error {
	scanner := bufio.NewScanner(input)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req mcpRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := encoder.Encode(mcpResponse{JSONRPC: "2.0", Error: &mcpRPCError{Code: -32700, Message: "parse error"}}); err != nil {
				return err
			}
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		resp := handleMCPRequest(req, options)
		if err := encoder.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func handleMCPRequest(req mcpRequest, options MCPOptions) mcpResponse {
	resp := mcpResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "gira", "version": MCPServerSchemaVersion},
		}
	case "tools/list":
		resp.Result = map[string]any{"tools": MCPToolSpecs()}
	case "tools/call":
		var params mcpCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil || strings.TrimSpace(params.Name) == "" {
			resp.Error = &mcpRPCError{Code: -32602, Message: "invalid tools/call params"}
			return resp
		}
		if params.Arguments == nil {
			params.Arguments = map[string]json.RawMessage{}
		}
		payload, toolErr := ExecuteMCPToolWithOptions(params.Name, params.Arguments, options)
		if toolErr != nil {
			resp.Result = mcpToolResult(toolErr, true)
			return resp
		}
		resp.Result = mcpToolResult(payload, false)
	default:
		resp.Error = &mcpRPCError{Code: -32601, Message: "method not found"}
	}
	return resp
}

func mcpToolResult(payload any, isError bool) map[string]any {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		encoded = []byte(fmt.Sprintf(`{"error":%q}`, err.Error()))
	}
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": string(encoded)}},
		"isError": isError,
	}
}
