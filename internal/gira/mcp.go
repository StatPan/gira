package gira

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const MCPServerSchemaVersion = "gira-mcp-read-only/v1"

type MCPOptions struct {
	Runner CommandRunner
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

type MCPToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
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
	out := make([]MCPToolSpec, 0, len(mcpTools))
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
	if runner == nil {
		runner = NewMCPCommandRunnerFromEnv()
	}
	tool, ok := findMCPTool(name)
	if !ok {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: "unsupported read-only MCP tool"}
	}
	repo, err := mcpStringArg(arguments, "repo", true)
	if err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}
	}
	if _, err := ParseRepoRef(repo); err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}
	}
	ticket, err := mcpIntArg(arguments, "ticket", tool.Ticket)
	if err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}
	}
	limit, err := mcpIntArg(arguments, "limit", false)
	if err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}
	}
	queue, err := mcpStringArg(arguments, "queue", false)
	if err != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Error: err.Error()}
	}
	args := tool.Build(repo, ticket, limit, queue)
	command := append([]string{"gira"}, args...)
	stdout, runErr := runner.Run("gira", args...)
	if runErr != nil {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Command: command, ExitCode: 1, Stderr: runErr.Error(), Error: "gira command failed"}
	}
	trimmed := bytes.TrimSpace(stdout)
	if !json.Valid(trimmed) {
		return MCPCommandEnvelope{}, &MCPToolError{SchemaVersion: MCPServerSchemaVersion, Tool: name, Command: command, Stdout: string(stdout), Error: "gira command did not emit valid JSON"}
	}
	return MCPCommandEnvelope{SchemaVersion: MCPServerSchemaVersion, Tool: name, Command: command, ReadOnly: true, Payload: append(json.RawMessage(nil), trimmed...)}, nil
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
		envelope, toolErr := ExecuteMCPTool(params.Name, params.Arguments, options.Runner)
		if toolErr != nil {
			resp.Result = mcpToolResult(toolErr, true)
			return resp
		}
		resp.Result = mcpToolResult(envelope, false)
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
