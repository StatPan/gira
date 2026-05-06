package gira

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type OnboardStage string

const (
	OnboardStageInit        OnboardStage = "init"
	OnboardStageBootstrap   OnboardStage = "bootstrap"
	OnboardStageFirstSprint OnboardStage = "first-sprint"
	OnboardStageSteadyState OnboardStage = "steady-state"
)

var onboardStageOrder = []OnboardStage{
	OnboardStageInit,
	OnboardStageBootstrap,
	OnboardStageFirstSprint,
	OnboardStageSteadyState,
}

type OnboardCheckStatus string

const (
	OnboardCheckPass OnboardCheckStatus = "pass"
	OnboardCheckFail OnboardCheckStatus = "fail"
)

type OnboardVerifyReport struct {
	Repo              string               `json:"repo"`
	Command           string               `json:"command"`
	Stage             string               `json:"stage"`
	CheckedAt         string               `json:"checked_at"`
	Ready             bool                 `json:"ready"`
	BlockingChecklist []OnboardCheck       `json:"blocking_checklist"`
	Stages            []OnboardStageReport `json:"stages"`
}

type OnboardStageReport struct {
	Stage  string         `json:"stage"`
	Ready  bool           `json:"ready"`
	Checks []OnboardCheck `json:"checks"`
}

type OnboardCheck struct {
	ID          string             `json:"id"`
	Description string             `json:"description"`
	Status      OnboardCheckStatus `json:"status"`
	Detail      string             `json:"detail"`
	Remediation string             `json:"remediation,omitempty"`
}

type onboardRepoView struct {
	NameWithOwner    string `json:"nameWithOwner"`
	ViewerPermission string `json:"viewerPermission"`
	DefaultBranchRef *struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
}

func ParseOnboardStage(value string) (OnboardStage, error) {
	stage := OnboardStage(strings.TrimSpace(value))
	for _, candidate := range onboardStageOrder {
		if stage == candidate {
			return stage, nil
		}
	}
	return "", fmt.Errorf("stage must be one of init, bootstrap, first-sprint, steady-state")
}

func BuildOnboardVerifyReport(repo RepoRef, stage OnboardStage, runner CommandRunner, checkedAt time.Time) OnboardVerifyReport {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	report := OnboardVerifyReport{
		Repo:      repo.FullName(),
		Command:   "onboard verify",
		Stage:     string(stage),
		CheckedAt: checkedAt.UTC().Format(time.RFC3339),
		Ready:     true,
	}

	ghCheck := commandCheck(runner, repo)
	_, repoCheck := repoAccessCheck(runner, repo)

	stageReports := make([]OnboardStageReport, 0, len(onboardStageOrder))
	for _, current := range onboardingStagesUpTo(stage) {
		stageReport := OnboardStageReport{Stage: string(current), Ready: true}
		switch current {
		case OnboardStageInit:
			stageReport.Checks = append(stageReport.Checks, ghCheck, repoCheck)
		case OnboardStageBootstrap:
			stageReport.Checks = append(stageReport.Checks,
				bootstrapObjectsReadinessCheck(repo, runner),
			)
		case OnboardStageFirstSprint:
			stageReport.Checks = append(stageReport.Checks,
				openIssueCheck(repo, runner, checkedAt),
			)
		case OnboardStageSteadyState:
			stageReport.Checks = append(stageReport.Checks,
				dailyRunValidationCheck(repo, runner, checkedAt),
				milestoneCoverageCheck(repo, runner, checkedAt),
			)
		}
		stageReport.Ready = checksReady(stageReport.Checks)
		if !stageReport.Ready {
			report.Ready = false
			report.BlockingChecklist = append(report.BlockingChecklist, failedChecks(stageReport.Checks)...)
		}
		stageReports = append(stageReports, stageReport)
	}
	report.Stages = stageReports
	return report
}

func FormatOnboardVerifyReport(report OnboardVerifyReport) string {
	var b strings.Builder
	verdict := "READY"
	if !report.Ready {
		verdict = "NOT READY"
	}
	fmt.Fprintf(&b, "onboard verify: %s (%s)\n", verdict, report.Stage)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	for _, stage := range report.Stages {
		stageVerdict := "ready"
		if !stage.Ready {
			stageVerdict = "blocked"
		}
		fmt.Fprintf(&b, "stage %s: %s\n", stage.Stage, stageVerdict)
		for _, check := range stage.Checks {
			fmt.Fprintf(&b, "  - [%s] %s\n", check.Status, check.Description)
			if strings.TrimSpace(check.Detail) != "" {
				fmt.Fprintf(&b, "    detail: %s\n", check.Detail)
			}
			if check.Status == OnboardCheckFail && strings.TrimSpace(check.Remediation) != "" {
				fmt.Fprintf(&b, "    remediation: %s\n", check.Remediation)
			}
		}
	}
	fmt.Fprintf(&b, "next step: %s\n", onboardNextStep(report))
	return b.String()
}

func onboardNextStep(report OnboardVerifyReport) string {
	if !report.Ready {
		for _, check := range report.BlockingChecklist {
			if strings.TrimSpace(check.Remediation) != "" {
				return check.Remediation
			}
		}
		return "fix blocking checks and rerun `gira onboard verify --repo " + report.Repo + " --stage " + report.Stage + "`"
	}
	switch OnboardStage(report.Stage) {
	case OnboardStageInit:
		return "gira bootstrap --repo " + report.Repo + " --path ."
	case OnboardStageBootstrap:
		return "gira ops sync --repo " + report.Repo + " --dry-run"
	case OnboardStageFirstSprint, OnboardStageSteadyState:
		return "gira status --repo " + report.Repo
	default:
		return "gira status --repo " + report.Repo
	}
}

func onboardingStagesUpTo(target OnboardStage) []OnboardStage {
	stages := make([]OnboardStage, 0, len(onboardStageOrder))
	for _, stage := range onboardStageOrder {
		stages = append(stages, stage)
		if stage == target {
			break
		}
	}
	return stages
}

func commandCheck(runner CommandRunner, repo RepoRef) OnboardCheck {
	_, err := runner.Run("gh", "--version")
	if err != nil {
		return OnboardCheck{
			ID:          "gh_cli_available",
			Description: "GitHub CLI is available",
			Status:      OnboardCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("install/authenticate gh, then rerun `gira onboard verify --repo %s --stage init`", repo.FullName()),
		}
	}
	return OnboardCheck{
		ID:          "gh_cli_available",
		Description: "GitHub CLI is available",
		Status:      OnboardCheckPass,
		Detail:      "gh responded to --version",
	}
}

func repoAccessCheck(runner CommandRunner, repo RepoRef) (onboardRepoView, OnboardCheck) {
	view, err := fetchRepoView(runner, repo)
	if err != nil {
		return onboardRepoView{}, OnboardCheck{
			ID:          "repo_access",
			Description: "repository is reachable through gh",
			Status:      OnboardCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gh auth status` and `gh repo view %s`; fix access/auth before onboarding", repo.FullName()),
		}
	}
	branch := "unknown"
	if view.DefaultBranchRef != nil && strings.TrimSpace(view.DefaultBranchRef.Name) != "" {
		branch = view.DefaultBranchRef.Name
	}
	permission := strings.TrimSpace(view.ViewerPermission)
	if permission == "" {
		permission = "unknown"
	}
	return view, OnboardCheck{
		ID:          "repo_access",
		Description: "repository is reachable through gh",
		Status:      OnboardCheckPass,
		Detail:      fmt.Sprintf("default branch=%s, permission=%s", branch, permission),
	}
}

func contentCheck(runner CommandRunner, repo RepoRef, branch string, path string, id string, description string) OnboardCheck {
	if strings.TrimSpace(branch) == "" {
		return OnboardCheck{
			ID:          id,
			Description: description,
			Status:      OnboardCheckFail,
			Detail:      "default branch unavailable because repository access check failed",
			Remediation: fmt.Sprintf("fix repository access first, then rerun onboarding verification for %s", repo.FullName()),
		}
	}
	if err := repoContentExists(runner, repo, branch, path); err != nil {
		return OnboardCheck{
			ID:          id,
			Description: description,
			Status:      OnboardCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gira bootstrap --repo %s --path PATH`, commit %s on %s, then rerun verification", repo.FullName(), path, branch),
		}
	}
	return OnboardCheck{
		ID:          id,
		Description: description,
		Status:      OnboardCheckPass,
		Detail:      fmt.Sprintf("found %s on %s", path, branch),
	}
}

func bootstrapObjectsReadinessCheck(repo RepoRef, runner CommandRunner) OnboardCheck {
	plan, err := BuildSyncPlan(NewGHSyncClient(repo, runner), SyncPlanOptions{EnableBootstrapIssues: true})
	if err != nil {
		return OnboardCheck{
			ID:          "bootstrap_operating_objects_ready",
			Description: "bootstrap operating objects are converged (labels, milestones, bootstrap issues)",
			Status:      OnboardCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gira ops sync --repo %s --dry-run`, then apply the required metadata sync; add `--bootstrap-issues` only for sample bootstrap issues", repo.FullName()),
		}
	}
	labelCreates := countLabelActions(plan.Labels, PlanCreate)
	labelUpdates := countLabelActions(plan.Labels, PlanUpdate)
	milestoneCreates := countMilestoneActions(plan.Milestones, PlanCreate)
	milestoneUpdates := countMilestoneActions(plan.Milestones, PlanUpdate)
	bootstrapIssueCreates := countBootstrapIssueActions(plan.BootstrapIssues, PlanCreate)
	if labelCreates+labelUpdates+milestoneCreates+milestoneUpdates+bootstrapIssueCreates > 0 {
		return OnboardCheck{
			ID:          "bootstrap_operating_objects_ready",
			Description: "bootstrap operating objects are converged (labels, milestones, bootstrap issues)",
			Status:      OnboardCheckFail,
			Detail:      fmt.Sprintf("labels create=%d update=%d; milestones create=%d update=%d; bootstrap issues create=%d", labelCreates, labelUpdates, milestoneCreates, milestoneUpdates, bootstrapIssueCreates),
			Remediation: fmt.Sprintf("run `gira ops sync --repo %s --dry-run` and then `gira ops sync --repo %s` until label and milestone drift is zero; add `--bootstrap-issues` only for sample bootstrap issues", repo.FullName(), repo.FullName()),
		}
	}
	return OnboardCheck{
		ID:          "bootstrap_operating_objects_ready",
		Description: "bootstrap operating objects are converged (labels, milestones, bootstrap issues)",
		Status:      OnboardCheckPass,
		Detail:      "labels, milestones, and optional bootstrap issues already match the default Gira contract",
	}
}

func openIssueCheck(repo RepoRef, runner CommandRunner, checkedAt time.Time) OnboardCheck {
	summary, err := BuildStatusSummary(NewGHStatusClient(repo, runner), checkedAt, 14)
	if err != nil {
		return OnboardCheck{
			ID:          "first_sprint_backlog",
			Description: "first sprint has an actionable issue backlog",
			Status:      OnboardCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gira status --repo %s --json`; restore gh access or create ready issues", repo.FullName()),
		}
	}
	if summary.Counts.Issues.Open == 0 {
		return OnboardCheck{
			ID:          "first_sprint_backlog",
			Description: "first sprint has an actionable issue backlog",
			Status:      OnboardCheckFail,
			Detail:      "open issues=0",
			Remediation: fmt.Sprintf("create or sync ready issues for %s before starting the first sprint", repo.FullName()),
		}
	}
	return OnboardCheck{
		ID:          "first_sprint_backlog",
		Description: "first sprint has an actionable issue backlog",
		Status:      OnboardCheckPass,
		Detail:      fmt.Sprintf("open issues=%d, blocked open=%d", summary.Counts.Issues.Open, summary.Counts.Issues.BlockedOpen),
	}
}

func dailyRunValidationCheck(repo RepoRef, runner CommandRunner, checkedAt time.Time) OnboardCheck {
	summary, err := BuildStatusSummary(NewGHStatusClient(repo, runner), checkedAt, 14)
	if err != nil {
		return OnboardCheck{
			ID:          "daily_run_validation",
			Description: "sample daily status run succeeds",
			Status:      OnboardCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("fix `gh` access until `gira status --repo %s --json` succeeds", repo.FullName()),
		}
	}
	return OnboardCheck{
		ID:          "daily_run_validation",
		Description: "sample daily status run succeeds",
		Status:      OnboardCheckPass,
		Detail:      fmt.Sprintf("status summary fetched with %d open issues and %d stale issues", summary.Counts.Issues.Open, summary.Counts.Issues.StaleOpen),
	}
}

func milestoneCoverageCheck(repo RepoRef, runner CommandRunner, checkedAt time.Time) OnboardCheck {
	summary, err := BuildStatusSummary(NewGHStatusClient(repo, runner), checkedAt, 14)
	if err != nil {
		return OnboardCheck{
			ID:          "milestone_coverage",
			Description: "daily operation has milestone coverage",
			Status:      OnboardCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("fix `gira status --repo %s --json`, then ensure milestones are synced", repo.FullName()),
		}
	}
	if summary.Counts.Milestones.Open == 0 && summary.Counts.Milestones.Closed == 0 {
		return OnboardCheck{
			ID:          "milestone_coverage",
			Description: "daily operation has milestone coverage",
			Status:      OnboardCheckFail,
			Detail:      "milestones total=0",
			Remediation: fmt.Sprintf("run `gira ops sync --repo %s` so milestone planning exists before daily operation", repo.FullName()),
		}
	}
	return OnboardCheck{
		ID:          "milestone_coverage",
		Description: "daily operation has milestone coverage",
		Status:      OnboardCheckPass,
		Detail:      fmt.Sprintf("milestones total=%d open=%d", summary.Counts.Milestones.Total, summary.Counts.Milestones.Open),
	}
}

func checksReady(checks []OnboardCheck) bool {
	for _, check := range checks {
		if check.Status != OnboardCheckPass {
			return false
		}
	}
	return true
}

func failedChecks(checks []OnboardCheck) []OnboardCheck {
	failed := make([]OnboardCheck, 0, len(checks))
	for _, check := range checks {
		if check.Status == OnboardCheckFail {
			failed = append(failed, check)
		}
	}
	return failed
}

func fetchRepoView(runner CommandRunner, repo RepoRef) (onboardRepoView, error) {
	output, err := runner.Run("gh", "repo", "view", repo.FullName(), "--json", "nameWithOwner,viewerPermission,defaultBranchRef")
	if err != nil {
		return onboardRepoView{}, err
	}
	var view onboardRepoView
	if err := json.Unmarshal(output, &view); err != nil {
		return onboardRepoView{}, fmt.Errorf("parse gh JSON: %w", err)
	}
	return view, nil
}

func repoContentExists(runner CommandRunner, repo RepoRef, branch string, path string) error {
	_, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/contents/%s?ref=%s", repo.FullName(), path, branch))
	return err
}
