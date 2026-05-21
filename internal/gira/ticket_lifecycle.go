package gira

import (
	"fmt"
	"strings"
)

const (
	TicketLifecycleBlockStart = "<!-- gira:lifecycle:start -->"
	TicketLifecycleBlockEnd   = "<!-- gira:lifecycle:end -->"
)

type TicketLifecycleState struct {
	BaseBranch       string `json:"base_branch,omitempty"`
	BaseSource       string `json:"base_source,omitempty"`
	BranchPolicyMode string `json:"branch_policy_mode,omitempty"`
	Target           string `json:"target,omitempty"`
	WorkBranch       string `json:"work_branch,omitempty"`
}

func ParseTicketLifecycleState(text string) TicketLifecycleState {
	block := extractTicketLifecycleManagedBlock(text, TicketLifecycleBlockStart, TicketLifecycleBlockEnd)
	if block == "" {
		return TicketLifecycleState{}
	}
	state := TicketLifecycleState{}
	for _, line := range strings.Split(block, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "base_branch":
			state.BaseBranch = value
		case "base_source":
			state.BaseSource = value
		case "branch_policy_mode":
			state.BranchPolicyMode = value
		case "target":
			state.Target = value
		case "work_branch":
			state.WorkBranch = value
		}
	}
	return state
}

func RenderTicketLifecycleBlock(state TicketLifecycleState) string {
	var b strings.Builder
	b.WriteString(TicketLifecycleBlockStart)
	b.WriteString("\n")
	writeLifecycleLine(&b, "base_branch", state.BaseBranch)
	writeLifecycleLine(&b, "base_source", state.BaseSource)
	writeLifecycleLine(&b, "branch_policy_mode", state.BranchPolicyMode)
	writeLifecycleLine(&b, "target", state.Target)
	writeLifecycleLine(&b, "work_branch", state.WorkBranch)
	b.WriteString(TicketLifecycleBlockEnd)
	return b.String()
}

func UpdateTicketLifecycleBlock(body string, state TicketLifecycleState) string {
	return replaceTicketLifecycleManagedBlock(strings.TrimRight(body, "\n"), TicketLifecycleBlockStart, TicketLifecycleBlockEnd, RenderTicketLifecycleBlock(state))
}

func recordTicketLifecycleState(repo RepoRef, issueNumber int, body string, state TicketLifecycleState, runner CommandRunner) error {
	updated := UpdateTicketLifecycleBlock(body, state)
	if updated == body {
		return nil
	}
	_, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues/"+fmt.Sprintf("%d", issueNumber), "-X", "PATCH", "-f", "body="+updated)
	if err != nil {
		return fmt.Errorf("record ticket lifecycle state: %w", err)
	}
	return nil
}

func extractTicketLifecycleManagedBlock(text string, startMarker string, endMarker string) string {
	start := strings.Index(text, startMarker)
	if start < 0 {
		return ""
	}
	start += len(startMarker)
	end := strings.Index(text[start:], endMarker)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(text[start : start+end])
}

func replaceTicketLifecycleManagedBlock(body string, startMarker string, endMarker string, block string) string {
	start := strings.Index(body, startMarker)
	if start >= 0 {
		endOffset := strings.Index(body[start:], endMarker)
		if endOffset >= 0 {
			end := start + endOffset + len(endMarker)
			updated := strings.TrimRight(body[:start], "\n") + "\n\n" + strings.TrimSpace(block) + "\n" + strings.TrimLeft(body[end:], "\n")
			return strings.TrimSpace(updated) + "\n"
		}
	}
	if strings.TrimSpace(body) == "" {
		return strings.TrimSpace(block) + "\n"
	}
	return strings.TrimRight(body, "\n") + "\n\n" + strings.TrimSpace(block) + "\n"
}

func writeLifecycleLine(b *strings.Builder, key string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", key, strings.TrimSpace(value))
}
