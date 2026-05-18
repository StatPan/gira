package gira

import (
	"strings"
	"testing"
)

func TestBuildAgentPromptReportPythonImplementer(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/436": `{"number":436,"title":"Add prompts","state":"open","body":"## Goal\nRender prompts\n\n## Acceptance Criteria\n- json output","labels":[{"name":"type:story"},{"name":"status:in-progress"}]}`,
	}, errors: map[string]error{}}

	report, err := BuildAgentPromptReport(AgentPromptInput{Repo: repo, Ticket: 436, Role: "implementer", Profile: "python"}, runner)
	if err != nil {
		t.Fatalf("BuildAgentPromptReport error: %v", err)
	}
	if report.Command != "ticket prompt" || report.Role != AgentPromptRoleImplementer || report.Profile != AgentPromptProfilePython {
		t.Fatalf("unexpected report metadata: %+v", report)
	}
	for _, want := range []string{"# Gira implementer prompt", "Assume no prior chat state", "pytest", "ruff", "mypy", "## Goal\nRender prompts"} {
		if !strings.Contains(report.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, report.Prompt)
		}
	}
}

func TestBuildAgentPromptReportDefaultProfileExcludesPythonSpecificTools(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/436": `{"number":436,"title":"Add prompts","state":"open","body":"## Goal\nRender prompts","labels":[{"name":"type:story"}]}`,
	}, errors: map[string]error{}}

	report, err := BuildAgentPromptReport(AgentPromptInput{Repo: repo, Ticket: 436, Role: "planner"}, runner)
	if err != nil {
		t.Fatalf("BuildAgentPromptReport error: %v", err)
	}
	if report.Profile != AgentPromptProfileDefault {
		t.Fatalf("profile = %q, want default", report.Profile)
	}
	if strings.Contains(report.Prompt, "pytest") || strings.Contains(report.Prompt, "ruff") || strings.Contains(report.Prompt, "mypy") {
		t.Fatalf("default profile included Python-only guidance:\n%s", report.Prompt)
	}
}

func TestBuildAgentPromptReportReviewerWithExplicitPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/436": `{"number":436,"title":"Add prompts","state":"open","body":"## Goal\nRender prompts","labels":[{"name":"type:story"}]}`,
		"gh pr view 45 --repo StatPan/gira --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup": `{"number":45,"title":"feat: prompts","body":"Closes #436","state":"OPEN","url":"https://github.com/StatPan/gira/pull/45","reviewDecision":"REVIEW_REQUIRED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"name":"test","workflowName":"ci","status":"completed","conclusion":"success","detailsUrl":"https://ci.example"}]}`,
		"gh pr diff 45 --repo StatPan/gira --name-only": "internal/gira/agent_prompt.go\ninternal/cli/cli.go\n",
	}, errors: map[string]error{}}

	report, err := BuildAgentPromptReport(AgentPromptInput{Repo: repo, Ticket: 436, Role: "reviewer", PRNumber: 45}, runner)
	if err != nil {
		t.Fatalf("BuildAgentPromptReport error: %v", err)
	}
	if report.PR == nil || report.PR.Number != 45 || len(report.PR.Checks) != 1 || len(report.PR.ChangedFiles) != 2 {
		t.Fatalf("unexpected PR context: %+v", report.PR)
	}
	if report.PR.FinishReady || !containsString(report.PR.Blockers, "review") {
		t.Fatalf("expected review blocker and not finish ready: %+v", report.PR)
	}
	for _, want := range []string{"Review findings first", "Pull Request Context", "Review Decision: `REVIEW_REQUIRED`", "Changed Files:", "internal/gira/agent_prompt.go", "Closes #436", "Findings first"} {
		if !strings.Contains(report.Prompt, want) {
			t.Fatalf("reviewer prompt missing %q:\n%s", want, report.Prompt)
		}
	}
}

func TestBuildAgentPromptReportReviewerResolvesLinkedPRAndAcceptance(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/436": `{"number":436,"title":"Add prompts","state":"open","body":"## Goal\nRender prompts\n\n## Scope\nPrompt UX\n\n## Acceptance Criteria\n- includes issue goal\n- includes PR evidence","labels":[{"name":"type:story"},{"name":"status:in-review"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 436 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": `[{"number":45,"title":"feat: prompts","body":"Closes #436","state":"OPEN","url":"https://github.com/StatPan/gira/pull/45","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-436-add-prompts","baseRefName":"main","statusCheckRollup":[{"name":"test","workflowName":"ci","status":"completed","conclusion":"success","detailsUrl":"https://ci.example"}]}]`,
		"gh pr view 45 --repo StatPan/gira --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup":                                                                                  `{"number":45,"title":"feat: prompts","body":"Closes #436","state":"OPEN","url":"https://github.com/StatPan/gira/pull/45","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"name":"test","workflowName":"ci","status":"completed","conclusion":"success","detailsUrl":"https://ci.example"}]}`,
		"gh pr diff 45 --repo StatPan/gira --name-only": "internal/gira/agent_prompt.go\n",
	}, errors: map[string]error{}}

	report, err := BuildAgentPromptReport(AgentPromptInput{Repo: repo, Ticket: 436, Role: "reviewer"}, runner)
	if err != nil {
		t.Fatalf("BuildAgentPromptReport error: %v", err)
	}
	if report.Issue.Goal != "Render prompts" || report.Issue.Scope != "Prompt UX" || len(report.Issue.Acceptance) != 2 {
		t.Fatalf("unexpected parsed issue context: %+v", report.Issue)
	}
	if report.PR == nil || report.PR.Number != 45 || !report.PR.FinishReady {
		t.Fatalf("unexpected PR context: %+v", report.PR)
	}
	if report.Evidence == nil || !report.Evidence.FinishReady || len(report.Evidence.ClosingIssues) != 1 || report.Evidence.ClosingIssues[0] != 436 || len(report.Evidence.ChangedFiles) != 1 {
		t.Fatalf("unexpected evidence: %+v", report.Evidence)
	}
	for _, want := range []string{"Goal: Render prompts", "Acceptance Criteria:", "includes PR evidence", "Finish Ready: `true`"} {
		if !strings.Contains(report.Prompt, want) {
			t.Fatalf("reviewer prompt missing %q:\n%s", want, report.Prompt)
		}
	}
}

func TestBuildAgentPromptReportReviewerErrorsWhenNoLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/436": `{"number":436,"title":"Add prompts","state":"open","body":"## Goal\nRender prompts","labels":[{"name":"status:in-review"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 436 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": `[]`,
	}, errors: map[string]error{}}

	_, err := BuildAgentPromptReport(AgentPromptInput{Repo: repo, Ticket: 436, Role: "reviewer"}, runner)
	if err == nil {
		t.Fatal("expected missing linked PR error")
	}
	for _, want := range []string{"requires a linked PR", "gira ticket pr --repo StatPan/gira --ticket 436 --dry-run"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestBuildAgentPromptReportRejectsUnknownRole(t *testing.T) {
	_, err := BuildAgentPromptReport(AgentPromptInput{Repo: RepoRef{Owner: "StatPan", Name: "gira"}, Ticket: 436, Role: "fixer"}, nil)
	if err == nil || !strings.Contains(err.Error(), "--role must be one of planner, implementer, reviewer") {
		t.Fatalf("unexpected error: %v", err)
	}
}
