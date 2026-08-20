package gira

import (
	"strconv"
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
	if report.Packet == nil || len(report.Packet.WorkOrder) == 0 || len(report.Packet.ExpectedEvidence) == 0 || len(report.Packet.Guidance) == 0 {
		t.Fatalf("implementer packet missing work order/evidence/guidance: %+v", report.Packet)
	}
	for _, want := range []string{"# Gira implementer prompt", "Assume no prior chat state", "pytest", "ruff", "mypy", "## Goal\nRender prompts", "Role Packet", "Work Order", "Expected Evidence"} {
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
	if report.Packet == nil || !containsString(report.Packet.Readiness, "issue_open") || !containsString(report.Packet.Readiness, "acceptance_criteria_missing") {
		t.Fatalf("planner packet missing readiness context: %+v", report.Packet)
	}
	if strings.Contains(report.Prompt, "pytest") || strings.Contains(report.Prompt, "ruff") || strings.Contains(report.Prompt, "mypy") {
		t.Fatalf("default profile included Python-only guidance:\n%s", report.Prompt)
	}
}

func TestBuildAgentPromptReportReviewerWithExplicitPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/436": `{"number":436,"title":"Add prompts","state":"open","body":"## Goal\nRender prompts","labels":[{"name":"type:story"}]}`,
		"gh pr view 45 --repo StatPan/gira --json number,title,body,state,url,headRefName,baseRefName,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup": `{"number":45,"title":"feat: prompts","body":"Closes #436","state":"OPEN","url":"https://github.com/StatPan/gira/pull/45","headRefName":"issue-436-add-prompts","baseRefName":"main","headRefOid":"head220","reviewDecision":"REVIEW_REQUIRED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"name":"test","workflowName":"ci","status":"completed","conclusion":"success","detailsUrl":"https://ci.example"}]}`,
		"gh pr diff 45 --repo StatPan/gira --name-only": "internal/gira/agent_prompt.go\ninternal/cli/cli.go\n",
	}, errors: map[string]error{}}

	report, err := BuildAgentPromptReport(AgentPromptInput{Repo: repo, Ticket: 436, Role: "reviewer", PRNumber: 45}, runner)
	if err != nil {
		t.Fatalf("BuildAgentPromptReport error: %v", err)
	}
	if report.PR == nil || report.PR.Number != 45 || report.PR.HeadRefName != "issue-436-add-prompts" || report.PR.BaseRefName != "main" || len(report.PR.Checks) != 1 || len(report.PR.ChangedFiles) != 2 {
		t.Fatalf("unexpected PR context: %+v", report.PR)
	}
	if report.Review == nil || len(report.Review.DiffReferences) != 3 || len(report.Review.Guidance) == 0 || len(report.Review.VerdictSchema.RecommendedAction) == 0 {
		t.Fatalf("unexpected review contract: %+v", report.Review)
	}
	if report.Packet == nil || len(report.Packet.WorkOrder) == 0 || len(report.Packet.ExpectedEvidence) == 0 {
		t.Fatalf("reviewer packet missing work order/evidence: %+v", report.Packet)
	}
	if report.PR.FinishReady || !containsString(report.PR.Blockers, "review") {
		t.Fatalf("expected review blocker and not finish ready: %+v", report.PR)
	}
	if report.PRReady == nil || report.PRReady.SchemaVersion != PRReadinessSchemaVersion || report.PRReady.Readiness != "needs_revision" || !prReadinessHasFinding(*report.PRReady, "review_blocked") {
		t.Fatalf("unexpected PR readiness: %+v", report.PRReady)
	}
	for _, want := range []string{
		"do not modify files, commit, push, or resolve comments",
		"Inspect the actual diff",
		"gh pr diff 45",
		"AGENTS.md",
		"AI Delivery Telemetry",
		"Gira label/workflow",
		"tool contract",
		"tests required by the changed surface",
		"Review findings first",
		"Pull Request Context",
		"Review Decision: `REVIEW_REQUIRED`",
		"Head: `issue-436-add-prompts`",
		"Review Packet Contract",
		"goal_fulfilled",
		"recommended_action",
		"Changed Files:",
		"internal/gira/agent_prompt.go",
		"Closes #436",
		"PR Readiness",
		"Findings first",
	} {
		if !strings.Contains(report.Prompt, want) {
			t.Fatalf("reviewer prompt missing %q:\n%s", want, report.Prompt)
		}
	}
}

func TestBuildAgentReviewDiffSummaryParsesUnifiedDiff(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh pr diff 77 --repo StatPan/gira": `diff --git a/internal/cli/cli.go b/internal/cli/cli.go
index 1111111..2222222 100644
--- a/internal/cli/cli.go
+++ b/internal/cli/cli.go
@@ -10,2 +10,3 @@ func runTicketReview() {
-	old()
+new()
+more()
diff --git a/docs/dogfood.md b/docs/dogfood.md
index 3333333..4444444 100644
--- a/docs/dogfood.md
+++ b/docs/dogfood.md
@@ -1,2 +1,2 @@
-old docs
+new docs
`,
	}, errors: map[string]error{}}

	summary := BuildAgentReviewDiffSummary(repo, AgentPromptPR{Number: 77, ChangedFiles: []string{"internal/cli/cli.go", "docs/dogfood.md"}}, []string{"dogfood docs include review summary"}, false, runner)
	if summary.UnsupportedMessage != "" {
		t.Fatalf("unexpected unsupported message: %s", summary.UnsupportedMessage)
	}
	if summary.TotalAdditions != 3 || summary.TotalDeletions != 2 {
		t.Fatalf("unexpected totals: +%d/-%d", summary.TotalAdditions, summary.TotalDeletions)
	}
	if len(summary.Files) != 2 || summary.Files[0].Path != "internal/cli/cli.go" || len(summary.Files[0].Hunks) != 1 {
		t.Fatalf("unexpected files: %+v", summary.Files)
	}
	if len(summary.AcceptanceMapping) != 1 || len(summary.AcceptanceMapping[0].Files) != 1 || summary.AcceptanceMapping[0].Files[0] != "docs/dogfood.md" {
		t.Fatalf("unexpected acceptance mapping: %+v", summary.AcceptanceMapping)
	}
	if !containsString(summary.RiskAreas, "Go runtime code changed") || !containsString(summary.RiskAreas, "documentation changed") {
		t.Fatalf("unexpected risk areas: %+v", summary.RiskAreas)
	}
}

func TestBuildAgentPromptReportReviewerResolvesLinkedPRAndAcceptance(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := "## Goal\nRender prompts\n\n## Scope\nPrompt UX\n\n## Acceptance Criteria\n- [x] includes issue goal\n- [x] includes PR evidence\n\n" + RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", BaseSource: "branch_policy.default"})
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/436": `{"number":436,"title":"Add prompts","state":"open","body":` + strconv.Quote(body) + `,"labels":[{"name":"type:story"},{"name":"status:in-review"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 436 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[{"number":45,"title":"feat: prompts","body":"Closes #436","state":"OPEN","url":"https://github.com/StatPan/gira/pull/45","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-436-add-prompts","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[{"name":"test","workflowName":"ci","status":"completed","conclusion":"success","detailsUrl":"https://ci.example"}]}]`,
		"gh pr view 45 --repo StatPan/gira --json number,title,body,state,url,headRefName,baseRefName,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup":                                                                     `{"number":45,"title":"feat: prompts","body":"Closes #436","state":"OPEN","url":"https://github.com/StatPan/gira/pull/45","headRefName":"issue-436-add-prompts","baseRefName":"main","headRefOid":"head220","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"name":"test","workflowName":"ci","status":"completed","conclusion":"success","detailsUrl":"https://ci.example"}]}`,
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
	if report.PR.RecordedBase != "main" || report.PR.RecordedBaseSource != "branch_policy.default" || report.PR.BaseMismatch {
		t.Fatalf("unexpected branch base context: %+v", report.PR)
	}
	if report.PRReady == nil || report.PRReady.Readiness != "ready_for_finish" || report.PRReady.NextAction != "finish_ticket" {
		t.Fatalf("unexpected PR readiness: %+v", report.PRReady)
	}
	if report.Evidence == nil || !report.Evidence.FinishReady || len(report.Evidence.ClosingIssues) != 1 || report.Evidence.ClosingIssues[0] != 436 || len(report.Evidence.ChangedFiles) != 1 {
		t.Fatalf("unexpected evidence: %+v", report.Evidence)
	}
	for _, want := range []string{"Goal: Render prompts", "Acceptance Criteria:", "includes PR evidence", "Finish Ready: `true`", "Recorded Base: `main`", "Verify the PR targets the recorded base `main`"} {
		if !strings.Contains(report.Prompt, want) {
			t.Fatalf("reviewer prompt missing %q:\n%s", want, report.Prompt)
		}
	}
}

func TestBuildAgentPromptReportReviewerSurfacesBaseMismatch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := "## Goal\nRender prompts\n\n" + RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", BaseSource: "branch_policy.default"})
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/436": `{"number":436,"title":"Add prompts","state":"open","body":` + strconv.Quote(body) + `,"labels":[{"name":"type:story"},{"name":"status:in-review"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 436 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[{"number":45,"title":"feat: prompts","body":"Closes #436","state":"OPEN","url":"https://github.com/StatPan/gira/pull/45","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-436-add-prompts","baseRefName":"develop","headRefOid":"head220","statusCheckRollup":[]}]`,
		"gh pr view 45 --repo StatPan/gira --json number,title,body,state,url,headRefName,baseRefName,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup":                                                                     `{"number":45,"title":"feat: prompts","body":"Closes #436","state":"OPEN","url":"https://github.com/StatPan/gira/pull/45","headRefName":"issue-436-add-prompts","baseRefName":"develop","headRefOid":"head220","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}`,
		"gh pr diff 45 --repo StatPan/gira --name-only": "internal/gira/agent_prompt.go\n",
	}, errors: map[string]error{}}

	report, err := BuildAgentPromptReport(AgentPromptInput{Repo: repo, Ticket: 436, Role: "reviewer"}, runner)
	if err != nil {
		t.Fatalf("BuildAgentPromptReport error: %v", err)
	}
	if report.PR == nil || !report.PR.BaseMismatch || report.PR.RecordedBase != "main" || report.PR.BaseRefName != "develop" || report.PR.FinishReady {
		t.Fatalf("expected base mismatch in PR context: %+v", report.PR)
	}
	if report.Evidence == nil || !containsString(report.Evidence.Blockers, "pr_base_mismatch") {
		t.Fatalf("expected base mismatch evidence blocker: %+v", report.Evidence)
	}
	if report.PRReady == nil || report.PRReady.Readiness != "needs_revision" || !prReadinessHasFinding(*report.PRReady, "base_mismatch") {
		t.Fatalf("expected base mismatch PR readiness: %+v", report.PRReady)
	}
	for _, want := range []string{"Base: `develop`", "Recorded Base: `main`", "Base Matches Recorded: `false`"} {
		if !strings.Contains(report.Prompt, want) {
			t.Fatalf("reviewer prompt missing %q:\n%s", want, report.Prompt)
		}
	}
}

func TestBuildAgentPromptReportReviewerEmitsPacketWhenNoLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/436": `{"number":436,"title":"Add prompts","state":"open","body":"## Goal\nRender prompts","labels":[{"name":"status:in-review"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 436 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}, errors: map[string]error{}}

	report, err := BuildAgentPromptReport(AgentPromptInput{Repo: repo, Ticket: 436, Role: "reviewer"}, runner)
	if err != nil {
		t.Fatalf("BuildAgentPromptReport error: %v", err)
	}
	if report.PR != nil {
		t.Fatalf("expected no PR context: %+v", report.PR)
	}
	if report.Evidence == nil || !containsString(report.Evidence.Blockers, "missing_linked_pr") {
		t.Fatalf("expected missing linked PR evidence: %+v", report.Evidence)
	}
	if report.PRReady == nil || report.PRReady.Readiness != "blocked" || !prReadinessHasFinding(*report.PRReady, "missing_linked_pr") {
		t.Fatalf("expected missing linked PR readiness: %+v", report.PRReady)
	}
	if report.Review == nil || len(report.Review.DiffReferences) != 0 || len(report.Review.VerdictSchema.GoalFulfilled) == 0 {
		t.Fatalf("expected review packet without PR evidence: %+v", report.Review)
	}
	if !strings.Contains(report.Prompt, "Review Packet Contract") {
		t.Fatalf("prompt missing review contract:\n%s", report.Prompt)
	}
}

func TestBuildAgentPromptReportRejectsUnknownRole(t *testing.T) {
	_, err := BuildAgentPromptReport(AgentPromptInput{Repo: RepoRef{Owner: "StatPan", Name: "gira"}, Ticket: 436, Role: "fixer"}, nil)
	if err == nil || !strings.Contains(err.Error(), "--role must be one of planner, implementer, reviewer") {
		t.Fatalf("unexpected error: %v", err)
	}
}
