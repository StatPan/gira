package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DoctorCheckStatus string

const (
	DoctorCheckPass DoctorCheckStatus = "pass"
	DoctorCheckFail DoctorCheckStatus = "fail"
	DoctorCheckWarn DoctorCheckStatus = "warn"
	DoctorCheckSkip DoctorCheckStatus = "skip"
)

type DoctorReport struct {
	Repo      string        `json:"repo"`
	Command   string        `json:"command"`
	CheckedAt string        `json:"checked_at"`
	Ready     bool          `json:"ready"`
	Checks    []DoctorCheck `json:"checks"`
}

type DoctorCheck struct {
	ID          string            `json:"id"`
	Status      DoctorCheckStatus `json:"status"`
	Detail      string            `json:"detail"`
	Remediation string            `json:"remediation"`
}

func BuildDoctorReport(repoValue string, runner CommandRunner, checkedAt time.Time) DoctorReport {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	report := DoctorReport{
		Command:   "doctor",
		CheckedAt: checkedAt.UTC().Format(time.RFC3339),
		Ready:     true,
	}

	report.Checks = append(report.Checks, giraCLIVisibleDoctorCheck())

	ghOK := false
	if output, err := runner.Run("gh", "--version"); err != nil {
		report.Checks = append(report.Checks, DoctorCheck{
			ID:          "gh_available",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: "install GitHub CLI and ensure `gh --version` succeeds",
		})
	} else {
		report.Checks = append(report.Checks, DoctorCheck{
			ID:          "gh_available",
			Status:      DoctorCheckPass,
			Detail:      firstLine(output),
			Remediation: "",
		})
		ghOK = true
	}

	repo, repoCheck := resolveDoctorRepo(repoValue, runner, ghOK)
	report.Checks = append(report.Checks, repoCheck)
	if repo.FullName() != "/" {
		report.Repo = repo.FullName()
	}

	if !ghOK {
		report.Checks = append(report.Checks,
			skippedDoctorCheck("gh_auth", "GitHub CLI is unavailable", "fix `gh_available`, then run `gh auth status`"),
			skippedDoctorCheck("repo_access", "GitHub CLI is unavailable", "fix `gh_available`, then run `gh repo view OWNER/REPO`"),
			skippedDoctorCheck("metadata_drift", "GitHub CLI is unavailable", "fix `gh_available`, then rerun `gira doctor --repo OWNER/REPO`"),
			skippedDoctorCheck("workflow_policy_labels", "GitHub CLI is unavailable", "fix `gh_available`, then rerun `gira doctor --repo OWNER/REPO`"),
			skippedDoctorCheck("closed_issue_status_labels", "GitHub CLI is unavailable", "fix `gh_available`, then rerun `gira doctor --repo OWNER/REPO`"),
			skippedDoctorCheck("workflow_nonconformance", "GitHub CLI is unavailable", "fix `gh_available`, then rerun `gira doctor --repo OWNER/REPO`"),
			skippedDoctorCheck("onboard_readiness", "GitHub CLI is unavailable", "fix `gh_available`, then rerun `gira doctor --repo OWNER/REPO`"),
		)
		report.Checks = append(report.Checks, companionDoctorsCheck(repo))
		report.Checks = append(report.Checks, localGitStateCheck(runner))
		report.Ready = doctorReady(report.Checks)
		return report
	}

	if output, err := runner.Run("gh", "auth", "status"); err != nil {
		report.Checks = append(report.Checks, DoctorCheck{
			ID:          "gh_auth",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: "run `gh auth login` until `gh auth status` succeeds",
		})
	} else {
		report.Checks = append(report.Checks, DoctorCheck{
			ID:          "gh_auth",
			Status:      DoctorCheckPass,
			Detail:      firstLine(output),
			Remediation: "",
		})
	}

	if repoCheck.Status == DoctorCheckPass {
		report.Checks = append(report.Checks, repoAccessDoctorCheck(repo, runner))
		report.Checks = append(report.Checks, metadataDriftDoctorCheck(repo, runner))
		report.Checks = append(report.Checks, workflowPolicyLabelsDoctorCheck(repo, runner))
		report.Checks = append(report.Checks, closedIssueStatusLabelsDoctorCheck(repo, runner))
		report.Checks = append(report.Checks, workflowNonconformanceDoctorCheck(repo, runner))
		report.Checks = append(report.Checks, onboardReadinessDoctorCheck(repo, runner, checkedAt))
	} else {
		report.Checks = append(report.Checks,
			skippedDoctorCheck("repo_access", "repo context is unavailable", "provide `--repo OWNER/REPO` or run from a GitHub repository"),
			skippedDoctorCheck("metadata_drift", "repo context is unavailable", "fix `repo_context`, then rerun `gira doctor`"),
			skippedDoctorCheck("workflow_policy_labels", "repo context is unavailable", "fix `repo_context`, then rerun `gira doctor`"),
			skippedDoctorCheck("closed_issue_status_labels", "repo context is unavailable", "fix `repo_context`, then rerun `gira doctor`"),
			skippedDoctorCheck("workflow_nonconformance", "repo context is unavailable", "fix `repo_context`, then rerun `gira doctor`"),
			skippedDoctorCheck("onboard_readiness", "repo context is unavailable", "fix `repo_context`, then rerun `gira doctor`"),
		)
	}

	report.Checks = append(report.Checks, companionDoctorsCheck(repo))
	report.Checks = append(report.Checks, localGitStateCheck(runner))
	report.Ready = doctorReady(report.Checks)
	return report
}

func FormatDoctorReport(report DoctorReport) string {
	verdict := "READY"
	if !report.Ready {
		verdict = "NOT READY"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "doctor: %s\n", verdict)
	if strings.TrimSpace(report.Repo) != "" {
		fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	}
	for _, check := range report.Checks {
		fmt.Fprintf(&b, "- [%s] %s: %s\n", check.Status, check.ID, check.Detail)
		if check.Status == DoctorCheckFail && strings.TrimSpace(check.Remediation) != "" {
			fmt.Fprintf(&b, "  remediation: %s\n", check.Remediation)
		}
	}
	fmt.Fprintf(&b, "next step: %s\n", doctorNextStep(report))
	return b.String()
}

func doctorNextStep(report DoctorReport) string {
	if report.Ready {
		if strings.TrimSpace(report.Repo) != "" {
			return "gira status --repo " + report.Repo
		}
		return "gira status"
	}
	for _, check := range report.Checks {
		if check.Status == DoctorCheckFail && strings.TrimSpace(check.Remediation) != "" {
			return fmt.Sprintf("fix %s: %s", check.ID, check.Remediation)
		}
	}
	return "fix failed checks and rerun `gira doctor`"
}

func resolveDoctorRepo(repoValue string, runner CommandRunner, ghOK bool) (RepoRef, DoctorCheck) {
	value := strings.TrimSpace(repoValue)
	if value != "" {
		repo, err := ParseRepoRef(value)
		if err != nil {
			return RepoRef{}, DoctorCheck{
				ID:          "repo_context",
				Status:      DoctorCheckFail,
				Detail:      err.Error(),
				Remediation: "pass `--repo OWNER/REPO`",
			}
		}
		return repo, DoctorCheck{
			ID:          "repo_context",
			Status:      DoctorCheckPass,
			Detail:      "using --repo " + repo.FullName(),
			Remediation: "",
		}
	}
	if !ghOK {
		return RepoRef{}, DoctorCheck{
			ID:          "repo_context",
			Status:      DoctorCheckSkip,
			Detail:      "cannot infer repo because gh is unavailable",
			Remediation: "pass `--repo OWNER/REPO` after fixing GitHub CLI",
		}
	}
	output, err := runner.Run("gh", "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return RepoRef{}, DoctorCheck{
			ID:          "repo_context",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: "pass `--repo OWNER/REPO` or run from a GitHub repository",
		}
	}
	var view struct {
		NameWithOwner string `json:"nameWithOwner"`
	}
	if err := json.Unmarshal(output, &view); err != nil {
		return RepoRef{}, DoctorCheck{
			ID:          "repo_context",
			Status:      DoctorCheckFail,
			Detail:      fmt.Sprintf("parse gh JSON: %v", err),
			Remediation: "pass `--repo OWNER/REPO`",
		}
	}
	repo, err := ParseRepoRef(view.NameWithOwner)
	if err != nil {
		return RepoRef{}, DoctorCheck{
			ID:          "repo_context",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: "pass `--repo OWNER/REPO`",
		}
	}
	return repo, DoctorCheck{
		ID:          "repo_context",
		Status:      DoctorCheckPass,
		Detail:      "inferred " + repo.FullName(),
		Remediation: "",
	}
}

func repoAccessDoctorCheck(repo RepoRef, runner CommandRunner) DoctorCheck {
	view, err := fetchRepoView(runner, repo)
	if err != nil {
		return DoctorCheck{
			ID:          "repo_access",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gh auth status` and `gh repo view %s`; fix access/auth before using Gira", repo.FullName()),
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
	return DoctorCheck{
		ID:          "repo_access",
		Status:      DoctorCheckPass,
		Detail:      fmt.Sprintf("default branch=%s, permission=%s", branch, permission),
		Remediation: "",
	}
}

func metadataDriftDoctorCheck(repo RepoRef, runner CommandRunner) DoctorCheck {
	plan, err := BuildSyncPlan(NewGHSyncClient(repo, runner), SyncPlanOptions{EnableBootstrapIssues: true, PolicyMode: SyncPolicyMerge})
	if err != nil {
		return DoctorCheck{
			ID:          "metadata_drift",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gira ops sync --repo %s --dry-run`; fix access errors, then apply with `gira ops sync --repo %s`", repo.FullName(), repo.FullName()),
		}
	}
	labelCreates := countLabelActions(plan.Labels, PlanCreate)
	labelUpdates := countLabelActions(plan.Labels, PlanUpdate)
	milestoneCreates := countMilestoneActions(plan.Milestones, PlanCreate)
	milestoneUpdates := countMilestoneActions(plan.Milestones, PlanUpdate)
	bootstrapIssueCreates := countBootstrapIssueActions(plan.BootstrapIssues, PlanCreate)
	drift := labelCreates + labelUpdates + milestoneCreates + milestoneUpdates
	if drift > 0 {
		return DoctorCheck{
			ID:          "metadata_drift",
			Status:      DoctorCheckFail,
			Detail:      fmt.Sprintf("labels create=%d update=%d; milestones create=%d update=%d; bootstrap issues create=%d", labelCreates, labelUpdates, milestoneCreates, milestoneUpdates, bootstrapIssueCreates),
			Remediation: fmt.Sprintf("run `gira ops sync --repo %s --dry-run`, then apply with `gira ops sync --repo %s`; add `--bootstrap-issues` only for demo/self-dogfood seed issues", repo.FullName(), repo.FullName()),
		}
	}
	if bootstrapIssueCreates > 0 {
		return DoctorCheck{
			ID:          "metadata_drift",
			Status:      DoctorCheckWarn,
			Detail:      fmt.Sprintf("labels create=0 update=0; milestones create=0 update=0; optional bootstrap issues create=%d", bootstrapIssueCreates),
			Remediation: fmt.Sprintf("run `gira ops sync --repo %s --bootstrap-issues --dry-run` only when you want Gira sample bootstrap issues", repo.FullName()),
		}
	}
	return DoctorCheck{
		ID:          "metadata_drift",
		Status:      DoctorCheckPass,
		Detail:      "labels and milestones match the default Gira contract; bootstrap sample issues are optional",
		Remediation: "",
	}
}

func onboardReadinessDoctorCheck(repo RepoRef, runner CommandRunner, checkedAt time.Time) DoctorCheck {
	summary, err := BuildStatusSummary(NewGHStatusClient(repo, runner), checkedAt, 14)
	if err != nil {
		return DoctorCheck{
			ID:          "onboard_readiness",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gira status --repo %s --json`; fix read access or repository data before daily operation", repo.FullName()),
		}
	}
	if summary.Counts.Issues.Open == 0 {
		return DoctorCheck{
			ID:          "onboard_readiness",
			Status:      DoctorCheckFail,
			Detail:      "open issues=0",
			Remediation: fmt.Sprintf("create or sync ready issues for %s before using Gira for daily operation", repo.FullName()),
		}
	}
	if summary.Counts.Milestones.Total == 0 {
		return DoctorCheck{
			ID:          "onboard_readiness",
			Status:      DoctorCheckFail,
			Detail:      "milestones total=0",
			Remediation: fmt.Sprintf("run `gira ops sync --repo %s --dry-run`, then `gira ops sync --repo %s` so milestone planning exists", repo.FullName(), repo.FullName()),
		}
	}
	return DoctorCheck{
		ID:          "onboard_readiness",
		Status:      DoctorCheckPass,
		Detail:      fmt.Sprintf("open issues=%d; milestones total=%d open=%d", summary.Counts.Issues.Open, summary.Counts.Milestones.Total, summary.Counts.Milestones.Open),
		Remediation: "",
	}
}

func workflowPolicyLabelsDoctorCheck(repo RepoRef, runner CommandRunner) DoctorCheck {
	output, err := runner.Run("gh", "label", "list", "--repo", repo.FullName(), "--json", "name", "--limit", "1000")
	if err != nil {
		return DoctorCheck{
			ID:          "workflow_policy_labels",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gh label list --repo %s --json name`; fix label read access before relying on agent workflow policy", repo.FullName()),
		}
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return DoctorCheck{
			ID:          "workflow_policy_labels",
			Status:      DoctorCheckFail,
			Detail:      fmt.Sprintf("parse label list: %v", err),
			Remediation: fmt.Sprintf("rerun `gira doctor --repo %s`; if it repeats, inspect `gh label list --repo %s --json name` output", repo.FullName(), repo.FullName()),
		}
	}
	existing := map[string]struct{}{}
	for _, row := range rows {
		label := strings.ToLower(strings.TrimSpace(row.Name))
		if label != "" {
			existing[label] = struct{}{}
		}
	}
	missing := []string{}
	for _, label := range doctorWorkflowPolicyLabels() {
		if _, ok := existing[strings.ToLower(label)]; !ok {
			missing = append(missing, label)
		}
	}
	if len(missing) > 0 {
		return DoctorCheck{
			ID:          "workflow_policy_labels",
			Status:      DoctorCheckFail,
			Detail:      "missing labels: " + strings.Join(missing, ","),
			Remediation: fmt.Sprintf("create reviewed repo labels or extend the managed taxonomy, then rerun `gira doctor --repo %s`", repo.FullName()),
		}
	}
	return DoctorCheck{
		ID:          "workflow_policy_labels",
		Status:      DoctorCheckPass,
		Detail:      fmt.Sprintf("agent workflow labels present=%d", len(doctorWorkflowPolicyLabels())),
		Remediation: "",
	}
}

func doctorWorkflowPolicyLabels() []string {
	return []string{
		"type:epic",
		"type:story",
		"type:task",
		"type:bug",
		"type:spike",
		"type:chore",
		"status:ready",
		"status:in-progress",
		"status:in-review",
		"status:blocked",
		"status:done",
		"priority:p1",
		"priority:p2",
		"area:backend",
		"area:docs",
		"area:ai",
		"agent:human",
		"agent:worker",
	}
}

func workflowNonconformanceDoctorCheck(repo RepoRef, runner CommandRunner) DoctorCheck {
	report, err := BuildWorkflowAuditReport(repo, runner, time.Now().UTC())
	if err != nil {
		return DoctorCheck{
			ID:          "workflow_nonconformance",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gira audit workflow --repo %s --json`; fix read access or malformed GitHub data", repo.FullName()),
		}
	}
	if len(report.Findings) == 0 {
		return DoctorCheck{
			ID:          "workflow_nonconformance",
			Status:      DoctorCheckPass,
			Detail:      fmt.Sprintf("workflow drift findings=0; issues scanned=%d; prs scanned=%d", report.Counts.IssuesScanned, report.Counts.PRsScanned),
			Remediation: "",
		}
	}
	status := DoctorCheckFail
	if report.Ready {
		status = DoctorCheckWarn
	}
	return DoctorCheck{
		ID:          "workflow_nonconformance",
		Status:      status,
		Detail:      fmt.Sprintf("workflow drift findings=%d (%s)", len(report.Findings), formatWorkflowFindingSummary(report.Findings)),
		Remediation: fmt.Sprintf("run `gira audit workflow --repo %s --json`, then normalize safe status drift with `gira adopt issues --repo %s --state all --normalize-status --dry-run`", repo.FullName(), repo.FullName()),
	}
}

type closedIssueStatusProblem struct {
	Number int
	Labels []string
}

func closedIssueStatusLabelsDoctorCheck(repo RepoRef, runner CommandRunner) DoctorCheck {
	output, err := runner.Run("gh", "issue", "list", "--repo", repo.FullName(), "--state", "closed", "--limit", "1000", "--json", "number,title,labels")
	if err != nil {
		return DoctorCheck{
			ID:          "closed_issue_status_labels",
			Status:      DoctorCheckFail,
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("run `gh issue list --repo %s --state closed --limit 1000 --json number,labels`; fix issue read access", repo.FullName()),
		}
	}
	var rows []struct {
		Number int `json:"number"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return DoctorCheck{
			ID:          "closed_issue_status_labels",
			Status:      DoctorCheckFail,
			Detail:      fmt.Sprintf("parse closed issue list: %v", err),
			Remediation: fmt.Sprintf("rerun `gira doctor --repo %s`; if it repeats, inspect the GitHub issue list JSON", repo.FullName()),
		}
	}
	problems := []closedIssueStatusProblem{}
	for _, row := range rows {
		labels := make([]string, 0, len(row.Labels))
		for _, label := range row.Labels {
			labels = append(labels, label.Name)
		}
		active := activeStatusLabels(labels)
		if len(active) > 0 {
			problems = append(problems, closedIssueStatusProblem{Number: row.Number, Labels: active})
		}
	}
	if len(problems) > 0 {
		numbers := make([]int, 0, len(problems))
		for _, problem := range problems {
			numbers = append(numbers, problem.Number)
		}
		return DoctorCheck{
			ID:          "closed_issue_status_labels",
			Status:      DoctorCheckFail,
			Detail:      fmt.Sprintf("closed issues with active status labels=%d (%s)", len(problems), formatClosedIssueStatusProblems(problems)),
			Remediation: fmt.Sprintf("run `gira adopt issues --repo %s --state all --issues %s --normalize-status --dry-run`, then rerun with `--apply`", repo.FullName(), joinIssueNumbers(numbers)),
		}
	}
	return DoctorCheck{
		ID:          "closed_issue_status_labels",
		Status:      DoctorCheckPass,
		Detail:      fmt.Sprintf("closed issues scanned=%d; active status labels=0", len(rows)),
		Remediation: "",
	}
}

func formatClosedIssueStatusProblems(problems []closedIssueStatusProblem) string {
	parts := []string{}
	limit := minInt(len(problems), 5)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("#%d %s", problems[i].Number, strings.Join(problems[i].Labels, ",")))
	}
	if len(problems) > limit {
		parts = append(parts, fmt.Sprintf("... +%d more", len(problems)-limit))
	}
	return strings.Join(parts, "; ")
}

func companionDoctorsCheck(repo RepoRef) DoctorCheck {
	scope := "run `gira config doctor` for global registry/config source diagnostics; run `gira jira doctor --repo OWNER/REPO` when Jira provider mode is configured"
	if repo.FullName() != "/" {
		scope = fmt.Sprintf("run `gira config doctor --repo %s` for global registry/config source diagnostics; run `gira jira doctor --repo %s` when Jira provider mode is configured", repo.FullName(), repo.FullName())
	}
	return DoctorCheck{
		ID:          "companion_doctors",
		Status:      DoctorCheckPass,
		Detail:      scope,
		Remediation: "",
	}
}

func localGitStateCheck(runner CommandRunner) DoctorCheck {
	if _, err := runner.Run("git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return DoctorCheck{
			ID:          "local_git_state",
			Status:      DoctorCheckWarn,
			Detail:      "current directory is not a git worktree: " + err.Error(),
			Remediation: "run doctor from a local repository when checking branch readiness",
		}
	}
	branchOutput, branchErr := runner.Run("git", "branch", "--show-current")
	statusOutput, statusErr := runner.Run("git", "status", "--porcelain")
	if branchErr != nil {
		return DoctorCheck{
			ID:          "local_git_state",
			Status:      DoctorCheckWarn,
			Detail:      "could not read current branch: " + branchErr.Error(),
			Remediation: "run `git status` to inspect the local worktree",
		}
	}
	branch := strings.TrimSpace(string(branchOutput))
	detached := branch == ""
	if statusErr != nil {
		if detached {
			return DoctorCheck{
				ID:          "local_git_state",
				Status:      DoctorCheckFail,
				Detail:      fmt.Sprintf("detached HEAD; could not read worktree status: %v", statusErr),
				Remediation: "checkout a named branch and run `git status` to inspect the local worktree",
			}
		}
		return DoctorCheck{
			ID:          "local_git_state",
			Status:      DoctorCheckWarn,
			Detail:      fmt.Sprintf("branch=%s; could not read worktree status: %v", branch, statusErr),
			Remediation: "run `git status` to inspect the local worktree",
		}
	}
	dirtyLines, auditLines := countGitStatusChanges(statusOutput)
	if detached {
		detail := "detached HEAD; worktree clean"
		if dirtyLines > 0 {
			detail = fmt.Sprintf("detached HEAD; uncommitted changes=%d", dirtyLines)
		}
		return DoctorCheck{
			ID:          "local_git_state",
			Status:      DoctorCheckFail,
			Detail:      detail,
			Remediation: "checkout a named branch before running Gira readiness or mutation workflows",
		}
	}
	if dirtyLines > 0 {
		userLines := dirtyLines - auditLines
		if userLines == 0 {
			return DoctorCheck{
				ID:          "local_git_state",
				Status:      DoctorCheckWarn,
				Detail:      fmt.Sprintf("branch=%s; Gira audit ledger changes=%d; user changes=0", branch, auditLines),
				Remediation: "commit audit ledger changes when you want repository-tracked operation evidence, or leave them uncommitted for local audit history",
			}
		}
		return DoctorCheck{
			ID:          "local_git_state",
			Status:      DoctorCheckFail,
			Detail:      fmt.Sprintf("branch=%s; uncommitted changes=%d; Gira audit ledger changes=%d; user changes=%d", branch, dirtyLines, auditLines, userLines),
			Remediation: "commit, stash, or intentionally keep local changes before running mutating Gira commands",
		}
	}
	return DoctorCheck{
		ID:          "local_git_state",
		Status:      DoctorCheckPass,
		Detail:      fmt.Sprintf("branch=%s; worktree clean", branch),
		Remediation: "",
	}
}

func giraCLIVisibleDoctorCheck() DoctorCheck {
	executable, err := os.Executable()
	if err != nil {
		return DoctorCheck{
			ID:          "gira_cli_visible",
			Status:      DoctorCheckPass,
			Detail:      "gira doctor command is available; executable path unavailable: " + err.Error(),
			Remediation: "",
		}
	}
	return DoctorCheck{
		ID:          "gira_cli_visible",
		Status:      DoctorCheckPass,
		Detail:      "gira doctor command is available; executable=" + executable,
		Remediation: "",
	}
}

func skippedDoctorCheck(id string, detail string, remediation string) DoctorCheck {
	return DoctorCheck{ID: id, Status: DoctorCheckSkip, Detail: detail, Remediation: remediation}
}

func doctorReady(checks []DoctorCheck) bool {
	for _, check := range checks {
		if check.Status == DoctorCheckFail {
			return false
		}
	}
	return true
}

func firstLine(output []byte) string {
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return "command succeeded"
}

func countGitStatusChanges(output []byte) (int, int) {
	total := 0
	audit := 0
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		total++
		if isGiraAuditStatusLine(line) {
			audit++
		}
	}
	return total, audit
}

func isGiraAuditStatusLine(line string) bool {
	if len(line) < 4 {
		return false
	}
	path := strings.TrimSpace(line[3:])
	if strings.Contains(path, " -> ") {
		parts := strings.Split(path, " -> ")
		path = strings.TrimSpace(parts[len(parts)-1])
	}
	path = strings.Trim(path, `"`)
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, ".gira/audit/") {
		return false
	}
	return strings.HasSuffix(path, ".jsonl") || strings.HasSuffix(path, ".jsonl.lasthash")
}
