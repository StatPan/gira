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
	}, errors: map[string]error{}}

	report, err := BuildAgentPromptReport(AgentPromptInput{Repo: repo, Ticket: 436, Role: "reviewer", PRNumber: 45}, runner)
	if err != nil {
		t.Fatalf("BuildAgentPromptReport error: %v", err)
	}
	if report.PR == nil || report.PR.Number != 45 || len(report.PR.Checks) != 1 {
		t.Fatalf("unexpected PR context: %+v", report.PR)
	}
	for _, want := range []string{"Review findings first", "Pull Request Context", "Closes #436", "Findings first"} {
		if !strings.Contains(report.Prompt, want) {
			t.Fatalf("reviewer prompt missing %q:\n%s", want, report.Prompt)
		}
	}
}

func TestBuildAgentPromptReportRejectsUnknownRole(t *testing.T) {
	_, err := BuildAgentPromptReport(AgentPromptInput{Repo: RepoRef{Owner: "StatPan", Name: "gira"}, Ticket: 436, Role: "fixer"}, nil)
	if err == nil || !strings.Contains(err.Error(), "--role must be one of planner, implementer, reviewer") {
		t.Fatalf("unexpected error: %v", err)
	}
}
