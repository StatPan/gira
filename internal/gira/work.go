package gira

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const WorkStartResultSchemaVersion = "work-start-result/v1"
const WorkPRResultSchemaVersion = "work-pr-result/v1"

const (
	WorkStartExecutionPlanned               = "planned"
	WorkStartExecutionBlockedBeforeMutation = "blocked_before_mutation"
	WorkStartExecutionPartiallyApplied      = "partially_applied"
	WorkStartExecutionApplied               = "applied"
)

type WorkStartResult struct {
	SchemaVersion     string                     `json:"schema_version"`
	Repo              string                     `json:"repo"`
	Issue             int                        `json:"issue"`
	JiraKey           string                     `json:"jira_key,omitempty"`
	MirrorIssue       int                        `json:"mirror_issue,omitempty"`
	Title             string                     `json:"title"`
	Branch            string                     `json:"branch"`
	BaseBranch        string                     `json:"base_branch,omitempty"`
	BaseSource        string                     `json:"base_source,omitempty"`
	PolicyMode        string                     `json:"branch_policy_mode,omitempty"`
	BranchStrategy    string                     `json:"branch_strategy,omitempty"`
	BranchSource      string                     `json:"branch_source,omitempty"`
	BranchSelection   string                     `json:"branch_selection,omitempty"`
	SuggestedBranch   string                     `json:"suggested_branch,omitempty"`
	SelectionRequired bool                       `json:"selection_required,omitempty"`
	Policy            *ResolvedOperationPolicy   `json:"policy,omitempty"`
	TicketReadiness   *TicketReadinessReport     `json:"ticket_readiness,omitempty"`
	DryRun            bool                       `json:"dry_run"`
	Started           bool                       `json:"started"`
	ExecutionState    string                     `json:"execution_state"`
	CreatedBranch     bool                       `json:"created_branch"`
	Status            string                     `json:"status"`
	NextStatus        string                     `json:"next_status"`
	NextStep          string                     `json:"next_step,omitempty"`
	Checks            map[string]bool            `json:"checks"`
	BranchReuse       *DevStartBranchReuseCheck  `json:"branch_reuse,omitempty"`
	Preflight         *DevStartWorktreePreflight `json:"preflight,omitempty"`
	Approval          *ApprovalEvidence          `json:"approval,omitempty"`
}

func EnsureWorkStartResultSchema(result *WorkStartResult) {
	if result == nil {
		return
	}
	if strings.TrimSpace(result.SchemaVersion) == "" {
		result.SchemaVersion = WorkStartResultSchemaVersion
	}
	if strings.TrimSpace(result.ExecutionState) == "" {
		if result.DryRun {
			result.ExecutionState = WorkStartExecutionPlanned
		} else if result.Started {
			result.ExecutionState = WorkStartExecutionApplied
		} else {
			result.ExecutionState = WorkStartExecutionBlockedBeforeMutation
		}
	}
}

type WorkStartOptions struct {
	DryRun       bool
	BaseOverride string
	// Branch accepts auto, new, current, or an existing branch name. The
	// legacy Create/Current/AdoptBranch fields remain supported for callers
	// compiled against the pre-unified interface.
	Branch      string
	Create      bool
	Current     bool
	AdoptBranch string
}

type WorkPRResult struct {
	Repo               string `json:"repo"`
	Issue              int    `json:"issue"`
	DryRun             bool   `json:"dry_run"`
	Draft              bool   `json:"draft"`
	PRNumber           int    `json:"pr_number,omitempty"`
	PRURL              string `json:"pr_url,omitempty"`
	Created            bool   `json:"created"`
	Status             string `json:"status"`
	NextStatus         string `json:"next_status"`
	Branch             string `json:"branch,omitempty"`
	BranchPush         string `json:"branch_push,omitempty"`
	PushRemote         string `json:"push_remote,omitempty"`
	LocalGit           string `json:"local_git,omitempty"`
	RecordedBase       string `json:"recorded_base,omitempty"`
	RecordedBaseSource string `json:"recorded_base_source,omitempty"`
	ActualBase         string `json:"actual_base,omitempty"`
	BaseMismatch       bool   `json:"base_mismatch,omitempty"`
	NextStep           string `json:"next_step,omitempty"`

	Blockers    []string          `json:"blockers"`
	Warnings    []string          `json:"warnings,omitempty"`
	ClosingBody string            `json:"closing_body"`
	Approval    *ApprovalEvidence `json:"approval,omitempty"`
}

type WorkStatusResult struct {
	Command          string                    `json:"command,omitempty"`
	SchemaVersion    string                    `json:"schema_version,omitempty"`
	Repo             string                    `json:"repo"`
	Issue            int                       `json:"issue"`
	Title            string                    `json:"title"`
	State            string                    `json:"state"`
	Status           string                    `json:"status"`
	Labels           []string                  `json:"labels,omitempty"`
	Milestone        string                    `json:"milestone"`
	PRNumber         int                       `json:"pr_number,omitempty"`
	PRURL            string                    `json:"pr_url,omitempty"`
	PRState          string                    `json:"pr_state,omitempty"`
	PRLookupAttempts int                       `json:"pr_lookup_attempts,omitempty"`
	Blockers         []string                  `json:"blockers"`
	NextAction       string                    `json:"next_action"`
	NextStep         string                    `json:"next_step"`
	Branch           *TicketStatusBranch       `json:"branch,omitempty"`
	BranchPolicy     *TicketStatusBranchPolicy `json:"branch_policy,omitempty"`
	PullRequest      *TicketStatusPullRequest  `json:"pull_request,omitempty"`
	ChecksStatus     string                    `json:"checks_status,omitempty"`
	Checks           []DevPRCheck              `json:"checks,omitempty"`
	ReviewStatus     string                    `json:"review_status,omitempty"`
	ReviewPolicy     *FinishReviewPolicy       `json:"review_policy,omitempty"`
	ReviewEvidence   *FinishReviewEvidence     `json:"review_evidence,omitempty"`
	Evidence         *TicketStatusEvidence     `json:"evidence,omitempty"`
	Acceptance       *TicketStatusAcceptance   `json:"acceptance_criteria,omitempty"`
	ReleaseImpact    *TicketReleaseImpact      `json:"release_impact,omitempty"`
	Telemetry        *TicketStatusTelemetry    `json:"telemetry,omitempty"`
	Policy           *ResolvedOperationPolicy  `json:"policy,omitempty"`
	TicketReadiness  *TicketReadinessReport    `json:"ticket_readiness,omitempty"`
	PRReadiness      *PRReadinessReport        `json:"pr_readiness,omitempty"`
	Warnings         []string                  `json:"warnings,omitempty"`
}

type TicketStatusBranch struct {
	Expected     string `json:"expected"`
	Suggestion   string `json:"suggestion,omitempty"`
	BindingState string `json:"binding_state,omitempty"`
	Current      string `json:"current"`
	Trusted      bool   `json:"trusted"`
	Source       string `json:"source"`
}

type TicketStatusBranchPolicy struct {
	RecordedBase       string   `json:"recorded_base,omitempty"`
	RecordedBaseSource string   `json:"recorded_base_source,omitempty"`
	PolicyMode         string   `json:"policy_mode,omitempty"`
	StartMode          string   `json:"start_mode,omitempty"`
	BranchStrategy     string   `json:"branch_strategy,omitempty"`
	WorkBranch         string   `json:"work_branch,omitempty"`
	WorkBranchSource   string   `json:"work_branch_source,omitempty"`
	ActualPRBase       string   `json:"actual_pr_base,omitempty"`
	BaseMismatch       bool     `json:"base_mismatch"`
	Diagnostics        []string `json:"diagnostics"`
}

type TicketStatusPullRequest struct {
	Available        bool   `json:"available"`
	Number           int    `json:"number"`
	URL              string `json:"url"`
	State            string `json:"state"`
	Mergeable        string `json:"mergeable"`
	HeadRefName      string `json:"head_ref_name"`
	BaseRefName      string `json:"base_ref_name"`
	ReviewDecision   string `json:"review_decision"`
	IsDraft          bool   `json:"is_draft"`
	HeadSHA          string `json:"head_sha,omitempty"`
	MergeCommitSHA   string `json:"merge_commit_sha,omitempty"`
	ClosingReference bool   `json:"closing_reference"`
}

type TicketStatusEvidence struct {
	ClosingReference bool     `json:"closing_reference"`
	BranchTrusted    bool     `json:"branch_trusted"`
	FinishReady      bool     `json:"finish_ready"`
	Sources          []string `json:"sources"`
}

type TicketStatusAcceptance struct {
	Status     string `json:"status"`
	Total      int    `json:"total"`
	Complete   int    `json:"complete"`
	Incomplete int    `json:"incomplete"`
}

type TicketStatusTelemetry struct {
	Required bool     `json:"required"`
	Present  bool     `json:"present"`
	Status   string   `json:"status"`
	Sources  []string `json:"sources"`
	Warnings []string `json:"warnings,omitempty"`
}

var workStatusMissingPRRetryAttempts = 3
var workStatusMissingPRRetryDelay = time.Second

type ticketStartBaseResolution struct {
	BaseBranch           string
	BaseSource           string
	PolicyMode           string
	Target               string
	WorkBranch           string
	WorkBranchSource     string
	FeatureBranchPattern string
	StartMode            string
}

func resolveTicketStartBase(repo RepoRef, issue devStartIssue, explicitBase string, runner CommandRunner) (ticketStartBaseResolution, error) {
	if base := strings.TrimSpace(explicitBase); base != "" {
		policy, err := resolveRepoBranchPolicy(repo, runner)
		if err != nil {
			return ticketStartBaseResolution{}, err
		}
		return ticketStartBaseResolution{
			BaseBranch:           base,
			BaseSource:           "explicit --base",
			PolicyMode:           policy.Mode,
			Target:               "override",
			FeatureBranchPattern: configuredFeatureBranchPattern(policy),
			StartMode:            policy.StartMode,
		}, nil
	}
	recorded := ParseTicketLifecycleState(issue.Body)
	if strings.TrimSpace(recorded.BaseBranch) != "" {
		source := strings.TrimSpace(recorded.BaseSource)
		if source == "" {
			source = "recorded_ticket_base"
		}
		startMode := strings.TrimSpace(recorded.StartMode)
		if startMode == "" {
			startMode = BranchStartModeLegacyCreate
		}
		return ticketStartBaseResolution{BaseBranch: strings.TrimSpace(recorded.BaseBranch), BaseSource: source, PolicyMode: recorded.BranchPolicyMode, Target: recorded.Target, WorkBranch: strings.TrimSpace(recorded.WorkBranch), WorkBranchSource: strings.TrimSpace(recorded.WorkBranchSource), StartMode: startMode}, nil
	}
	policy, err := resolveRepoBranchPolicy(repo, runner)
	if err != nil {
		return ticketStartBaseResolution{}, err
	}
	target := strings.TrimSpace(policy.DefaultTarget)
	base := strings.TrimSpace(policy.Targets[target])
	if base == "" {
		base = strings.TrimSpace(policy.DefaultBase)
	}
	return ticketStartBaseResolution{
		BaseBranch:           base,
		BaseSource:           branchPolicyBaseSource(policy, target),
		PolicyMode:           policy.Mode,
		Target:               target,
		FeatureBranchPattern: configuredFeatureBranchPattern(policy),
		StartMode:            policy.StartMode,
	}, nil
}

func resolveTicketPRBase(repo RepoRef, issue devStartIssue, runner CommandRunner) (ticketStartBaseResolution, error) {
	recorded := ParseTicketLifecycleState(issue.Body)
	if strings.TrimSpace(recorded.BaseBranch) != "" {
		source := strings.TrimSpace(recorded.BaseSource)
		if source == "" {
			source = "recorded_ticket_base"
		}
		startMode := strings.TrimSpace(recorded.StartMode)
		if startMode == "" {
			startMode = BranchStartModeLegacyCreate
		}
		return ticketStartBaseResolution{BaseBranch: strings.TrimSpace(recorded.BaseBranch), BaseSource: source, PolicyMode: recorded.BranchPolicyMode, Target: recorded.Target, WorkBranch: strings.TrimSpace(recorded.WorkBranch), WorkBranchSource: strings.TrimSpace(recorded.WorkBranchSource), StartMode: startMode}, nil
	}
	policy, err := resolveRepoBranchPolicy(repo, runner)
	if err != nil {
		return ticketStartBaseResolution{}, err
	}
	target := strings.TrimSpace(policy.DefaultTarget)
	base := strings.TrimSpace(policy.Targets[target])
	if base == "" {
		base = strings.TrimSpace(policy.DefaultBase)
	}
	return ticketStartBaseResolution{BaseBranch: base, BaseSource: branchPolicyBaseSource(policy, target), PolicyMode: policy.Mode, Target: target, FeatureBranchPattern: configuredFeatureBranchPattern(policy), StartMode: policy.StartMode}, nil
}

func branchPolicyBaseSource(policy ResolvedBranchPolicy, target string) string {
	parts := []string{"branch_policy"}
	if source := strings.TrimSpace(policy.ConfigSource); source != "" {
		parts = append(parts, source)
	}
	return strings.Join(append(parts, strings.TrimSpace(target)), ".")
}

func configuredFeatureBranchPattern(policy ResolvedBranchPolicy) string {
	if policy.Source != "config" {
		return ""
	}
	return strings.TrimSpace(policy.FeatureBranchPattern)
}

func applyPRBaseMismatch(result *WorkPRResult, expectedBase string, actualBase string) {
	result.RecordedBase = strings.TrimSpace(expectedBase)
	result.ActualBase = strings.TrimSpace(actualBase)
	if result.RecordedBase == "" || result.ActualBase == "" || result.RecordedBase == result.ActualBase {
		return
	}
	result.BaseMismatch = true
	result.Blockers = appendMissingWorkBlocker(result.Blockers, "pr_base_mismatch")
}

func resolveRepoBranchPolicy(repo RepoRef, runner CommandRunner) (ResolvedBranchPolicy, error) {
	defaultBranch := "main"
	if view, err := fetchRepoView(runner, repo); err == nil && view.DefaultBranchRef != nil && strings.TrimSpace(view.DefaultBranchRef.Name) != "" {
		defaultBranch = strings.TrimSpace(view.DefaultBranchRef.Name)
	} else if err != nil {
		return ResolvedBranchPolicy{}, fmt.Errorf("resolve GitHub default branch: %w", err)
	}
	config, source, err := loadBranchPolicyConfig(repo, runner)
	if err != nil {
		return ResolvedBranchPolicy{}, err
	}
	policy, err := ResolveBranchPolicy(config, defaultBranch)
	if err != nil {
		return ResolvedBranchPolicy{}, err
	}
	policy.ConfigSource = source
	return policy, nil
}

func loadBranchPolicyConfig(repo RepoRef, runner CommandRunner) (*BranchPolicyConfig, string, error) {
	config, err := loadLocalBranchPolicyConfig(repo, runner)
	if err != nil || config != nil {
		return config, "repo_local_contract", err
	}
	return loadRegisteredBranchPolicyConfig(repo)
}

func loadLocalBranchPolicyConfig(repo RepoRef, runner CommandRunner) (*BranchPolicyConfig, error) {
	paths := []string{DefaultInitConfigPath("."), ".gira/config.toml"}
	if output, err := runner.Run("git", "rev-parse", "--show-toplevel"); err == nil {
		root := strings.TrimSpace(string(output))
		if root != "" && root != "." {
			paths = append(paths, filepath.Join(root, ".gira", "config.yaml"), filepath.Join(root, ".gira", "config.toml"))
		}
	}
	seen := map[string]struct{}{}
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		if _, err := os.Stat(clean); err != nil {
			continue
		}
		cfg, err := LoadInitConfig(clean)
		if err != nil {
			return nil, err
		}
		if configuredRepo := strings.TrimSpace(cfg.Repo); configuredRepo != "" && configuredRepo != repo.FullName() {
			continue
		}
		if cfg.BranchPolicy != nil {
			return cfg.BranchPolicy, nil
		}
	}
	return nil, nil
}

func loadRegisteredBranchPolicyConfig(repo RepoRef) (*BranchPolicyConfig, string, error) {
	root, err := globalConfigRoot("")
	if err != nil {
		return nil, "", fmt.Errorf("resolve global branch policy registry: %w", err)
	}

	entry, err := LoadGlobalRepoRegistryEntry(root, repo)
	if err == nil {
		if entry.BranchPolicy != nil {
			return entry.BranchPolicy, "global_repo_registry", nil
		}
		if workspace := strings.TrimSpace(entry.Workspace.Name); workspace != "" {
			return loadWorkspaceBranchPolicyConfig(root, repo, workspace, true)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("load global repo branch policy: %w", err)
	}

	global, err := LoadGlobalConfig(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("load global branch policy config: %w", err)
	}
	workspace := strings.TrimSpace(global.DefaultWorkspace)
	if workspace == "" {
		return nil, "", nil
	}
	return loadWorkspaceBranchPolicyConfig(root, repo, workspace, false)
}

func loadWorkspaceBranchPolicyConfig(root string, repo RepoRef, workspace string, required bool) (*BranchPolicyConfig, string, error) {
	entry, err := LoadGlobalWorkspaceRegistryEntry(root, workspace)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("load global workspace branch policy %q: %w", workspace, err)
	}
	if !workspaceConfigContainsRepo(entry.Workspace, repo) {
		if required {
			return nil, "", fmt.Errorf("global repo registry workspace %q does not contain %s", workspace, repo.FullName())
		}
		return nil, "", nil
	}
	if entry.BranchPolicy == nil {
		return nil, "", nil
	}
	return entry.BranchPolicy, "global_workspace_registry", nil
}

func workspaceConfigContainsRepo(workspace WorkspaceConfig, repo RepoRef) bool {
	for _, configured := range workspace.Repos {
		parsed, err := ParseRepoRef(configured)
		if err == nil && strings.EqualFold(parsed.FullName(), repo.FullName()) {
			return true
		}
	}
	return false
}

func StartWork(repo RepoRef, issueNumber int, dryRun bool, runner CommandRunner) (WorkStartResult, error) {
	return StartWorkWithOptions(repo, issueNumber, WorkStartOptions{DryRun: dryRun}, runner)
}

func StartWorkWithOptions(repo RepoRef, issueNumber int, options WorkStartOptions, runner CommandRunner) (WorkStartResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if strings.TrimSpace(options.Branch) != "" && countWorkStartLegacyStrategies(options) > 0 {
		return WorkStartResult{}, fmt.Errorf("--branch cannot be combined with --create, --current, or --adopt")
	}
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return WorkStartResult{}, err
	}
	operationPolicy, err := ResolveRepoOperationPolicy(repo, runner)
	if err != nil {
		return WorkStartResult{}, fmt.Errorf("resolve operation policy for %s before ticket start: %w", repo.FullName(), err)
	}
	ticketReadiness := EvaluateTicketReadinessWithPolicy(issue.Body, issue.Labels, issue.State, operationPolicy)
	status := displayStatus(managedStatusFromLabels(issue.Labels))
	alreadyStarted := status == "In progress"
	if alreadyStarted && !strings.EqualFold(issue.State, "open") {
		return WorkStartResult{}, fmt.Errorf("issue #%d is not open", issue.Number)
	}
	if !alreadyStarted && (!strings.EqualFold(issue.State, "open") || (!hasReadyLabel(issue.Labels) && operationPolicy.RequiresManagedDelivery())) {
		start, err := StartDevBranchWithOptions(repo, issueNumber, DevStartOptions{Pattern: DefaultDevBranchPattern, DryRun: options.DryRun}, runner)
		result := WorkStartResult{
			SchemaVersion:   WorkStartResultSchemaVersion,
			Repo:            repo.FullName(),
			Issue:           issueNumber,
			Title:           start.Title,
			Branch:          start.Branch,
			DryRun:          options.DryRun,
			Status:          status,
			NextStatus:      status,
			NextStep:        workStartNextStep(repo.FullName(), issueNumber, issue.State, status, options.DryRun),
			Checks:          start.Checked,
			Preflight:       start.Preflight,
			ExecutionState:  WorkStartExecutionBlockedBeforeMutation,
			Policy:          &operationPolicy,
			TicketReadiness: &ticketReadiness,
		}
		if options.DryRun && err == nil {
			result.ExecutionState = WorkStartExecutionPlanned
		}
		if err != nil && !strings.HasPrefix(strings.TrimSpace(result.NextStep), "gira adopt issues ") {
			result.NextStep = retryWorkStartNextStep(repo.FullName(), issueNumber)
		}
		return result, err
	}

	base, err := resolveTicketStartBase(repo, issue, options.BaseOverride, runner)
	if err != nil {
		return WorkStartResult{}, err
	}
	pattern := strings.TrimSpace(base.FeatureBranchPattern)
	if pattern == "" {
		pattern = DefaultDevBranchPattern
	}
	if countWorkStartStrategies(options) > 1 {
		return WorkStartResult{}, fmt.Errorf("choose only one of --create, --current, or --adopt")
	}
	suggested := strings.TrimSpace(base.WorkBranch)
	if suggested == "" {
		suggested = formatDevBranch(pattern, issue.Number, issue.Title)
	}
	result := WorkStartResult{
		SchemaVersion:   WorkStartResultSchemaVersion,
		Repo:            repo.FullName(),
		Issue:           issueNumber,
		Title:           issue.Title,
		Branch:          suggested,
		BaseBranch:      base.BaseBranch,
		BaseSource:      base.BaseSource,
		PolicyMode:      base.PolicyMode,
		SuggestedBranch: suggested,
		DryRun:          options.DryRun,
		Status:          status,
		NextStatus:      status,
		NextStep:        workStartNextStep(repo.FullName(), issueNumber, issue.State, status, options.DryRun),
		Checks:          map[string]bool{"issue_exists": true, "issue_open": true, "ready_label": hasReadyLabel(issue.Labels)},
		ExecutionState:  WorkStartExecutionBlockedBeforeMutation,
		Policy:          &operationPolicy,
		TicketReadiness: &ticketReadiness,
	}
	result.BranchSelection = strings.TrimSpace(options.Branch)
	if base.WorkBranch == "" && base.StartMode == BranchStartModeExplicit && strings.TrimSpace(options.Branch) == "" && !options.Create && !options.Current && strings.TrimSpace(options.AdoptBranch) == "" {
		result.BranchStrategy = "selection-required"
		result.SelectionRequired = true
		result.NextStep = explicitBranchSelectionNextStep(repo.FullName(), issueNumber)
		if options.DryRun {
			result.ExecutionState = WorkStartExecutionPlanned
			return result, nil
		}
		return result, fmt.Errorf("branch strategy is required: choose --branch auto|new|current|NAME, or --create, --current, or --adopt BRANCH")
	}

	strategy, selection, err := resolveWorkStartStrategy(options, base, runner)
	if err != nil {
		return result, err
	}
	if strategy == "create" || strategy == "current" || strategy == "adopt" {
		if err := validateWorkBranchRepository(repo, runner); err != nil {
			return result, err
		}
	}
	if strategy == "current" || strategy == "adopt" || strategy == "new" || strategy == "auto" {
		if err := validateRemoteBranchExists("origin", base.BaseBranch, runner); err != nil {
			return result, err
		}
		result.Checks["base_branch_exists"] = true
	}

	var start DevStartResult
	branchSource := strings.TrimSpace(base.WorkBranchSource)
	switch strategy {
	case "current":
		branch, err := currentWorkBranch(runner)
		if err != nil {
			return result, err
		}
		if err := validateAdoptedWorkBranch(branch, base.BaseBranch, runner, false); err != nil {
			return result, err
		}
		result.Branch = branch
		result.BranchStrategy = strategy
		result.BranchSelection = selection
		result.BranchSource = "current"
		branchSource = "current"
	case "adopt":
		branch := strings.TrimSpace(options.AdoptBranch)
		if branch == "" {
			branch = strings.TrimSpace(options.Branch)
		}
		if err := validateAdoptedWorkBranch(branch, base.BaseBranch, runner, true); err != nil {
			return result, err
		}
		result.Branch = branch
		result.BranchStrategy = strategy
		result.BranchSelection = selection
		result.BranchSource = "adopted"
		branchSource = "adopted"
	default:
		start, err = StartDevBranchWithOptions(repo, issueNumber, DevStartOptions{Pattern: pattern, Branch: base.WorkBranch, Base: base.BaseBranch, DryRun: options.DryRun, Force: alreadyStarted || (!operationPolicy.RequiresManagedDelivery() && strings.EqualFold(issue.State, "open")), RequireCleanWorktree: true}, runner)
		result.Title = start.Title
		result.Branch = start.Branch
		result.CreatedBranch = start.Created
		result.Checks = start.Checked
		result.BranchReuse = start.BranchReuse
		result.Preflight = start.Preflight
		result.BranchStrategy = strategy
		result.BranchSelection = selection
		result.BranchSource = "generated"
		if branchSource == "" {
			branchSource = "generated"
		}
	}
	if err != nil {
		if result.Preflight != nil && result.Preflight.Dirty {
			result.NextStep = blockedWorkStartNextStep(repo.FullName(), issueNumber, result.Preflight)
		} else {
			result.NextStep = retryWorkStartNextStep(repo.FullName(), issueNumber)
		}
		return result, err
	}
	if options.DryRun {
		result.ExecutionState = WorkStartExecutionPlanned
		result.NextStatus = "In progress"
		result.Approval = WorkStartApprovalEvidence(result, "gira work start")
		return result, nil
	}
	if err := recordTicketLifecycleState(repo, issueNumber, issue.Body, TicketLifecycleState{
		BaseBranch:       base.BaseBranch,
		BaseSource:       base.BaseSource,
		BranchPolicyMode: base.PolicyMode,
		StartMode:        base.StartMode,
		BranchStrategy:   result.BranchStrategy,
		Target:           base.Target,
		WorkBranch:       result.Branch,
		WorkBranchSource: branchSource,
	}, runner); err != nil {
		result.ExecutionState = WorkStartExecutionPartiallyApplied
		result.NextStep = partialWorkStartNextStep(repo.FullName(), issueNumber)
		return result, err
	}
	if err := setIssueStatus(repo, issueNumber, issue.Labels, "status:in-progress", runner); err != nil {
		result.ExecutionState = WorkStartExecutionPartiallyApplied
		result.NextStep = partialWorkStartNextStep(repo.FullName(), issueNumber)
		return result, err
	}
	result.Status = "In progress"
	result.NextStatus = "In progress"
	result.NextStep = workStartNextStep(repo.FullName(), issueNumber, issue.State, result.Status, false)
	result.Started = true
	result.ExecutionState = WorkStartExecutionApplied
	return result, nil
}

func countWorkStartStrategies(options WorkStartOptions) int {
	return countWorkStartLegacyStrategies(options)
}

func countWorkStartLegacyStrategies(options WorkStartOptions) int {
	count := 0
	if options.Create {
		count++
	}
	if options.Current {
		count++
	}
	if strings.TrimSpace(options.AdoptBranch) != "" {
		count++
	}
	return count
}

// resolveWorkStartStrategy resolves both the unified --branch spelling and
// the compatibility flags. It deliberately decides auto before any branch
// mutation, so a non-base checkout is only bound and a base/detached checkout
// is safely created from the recorded remote base.
func resolveWorkStartStrategy(options WorkStartOptions, base ticketStartBaseResolution, runner CommandRunner) (string, string, error) {
	requested := strings.TrimSpace(options.Branch)
	if requested != "" {
		switch strings.ToLower(requested) {
		case "auto":
			current, err := currentWorkBranchOptional(runner)
			if err != nil {
				return "", "", err
			}
			if current != "" && current != strings.TrimSpace(base.BaseBranch) {
				if err := validateAdoptedWorkBranch(current, base.BaseBranch, runner, false); err != nil {
					return "", "", err
				}
				return "current", "auto", nil
			}
			return "create", "auto", nil
		case "new":
			return "create", "new", nil
		case "current":
			return "current", "current", nil
		default:
			return "adopt", requested, nil
		}
	}
	if options.Current {
		return "current", "", nil
	}
	if strings.TrimSpace(options.AdoptBranch) != "" {
		return "adopt", "", nil
	}
	if options.Create {
		return "create", "", nil
	}
	// A recorded work branch is authoritative for retries. Legacy configs
	// retain their historical create/reuse behavior; new presets default to
	// automatic selection.
	if strings.TrimSpace(base.WorkBranch) != "" || base.StartMode == BranchStartModeLegacyCreate {
		return "create", "", nil
	}
	current, err := currentWorkBranchOptional(runner)
	if err != nil {
		return "", "", err
	}
	if current != "" && current != strings.TrimSpace(base.BaseBranch) {
		if err := validateAdoptedWorkBranch(current, base.BaseBranch, runner, false); err != nil {
			return "", "", err
		}
		return "current", "auto", nil
	}
	return "create", "auto", nil
}

func currentWorkBranchOptional(runner CommandRunner) (string, error) {
	output, err := runner.Run("git", "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("read current work branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func currentWorkBranch(runner CommandRunner) (string, error) {
	output, err := runner.Run("git", "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("read current work branch: %w", err)
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", fmt.Errorf("cannot adopt the current branch from a detached HEAD")
	}
	return branch, nil
}

func validateAdoptedWorkBranch(branch string, base string, runner CommandRunner, requireExisting bool) error {
	if err := validateGitBranchPushName(branch); err != nil {
		return fmt.Errorf("invalid adopted work branch: %w", err)
	}
	if branch == strings.TrimSpace(base) {
		return fmt.Errorf("cannot use resolved base branch %q as a work branch", base)
	}
	if !requireExisting {
		return nil
	}
	local, err := gitLocalBranchExists(branch, runner)
	if err != nil {
		return err
	}
	if local {
		return nil
	}
	remote, err := gitRemoteBranchExists(branch, runner)
	if err != nil {
		return err
	}
	if !local && !remote {
		return fmt.Errorf("cannot adopt work branch %q: it does not exist locally or on origin", branch)
	}
	return nil
}

// validateWorkBranchRepository prevents a --repo target from binding a local
// checkout that belongs to a different GitHub repository. Binding current or
// adopted branches does not perform a checkout or push, so the origin check is
// the safety boundary before lifecycle state or labels can be mutated.
func validateWorkBranchRepository(repo RepoRef, runner CommandRunner) error {
	output, err := runner.Run("git", "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("verify local repository before binding work branch: %w", err)
	}
	origin, err := ParseGitHubRemoteRepo(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("verify local repository before binding work branch: %w", err)
	}
	if !sameRepoRef(origin, repo) {
		return fmt.Errorf("cannot bind work branch from local origin %q to target repo %q", origin.FullName(), repo.FullName())
	}
	return nil
}

func explicitBranchSelectionNextStep(repo string, issue int) string {
	return fmt.Sprintf("choose a branch strategy (auto, new, current, or NAME); preview: gira ticket start %d --repo %s --branch auto --dry-run", issue, repo)
}

func blockedWorkStartNextStep(repo string, issue int, preflight *DevStartWorktreePreflight) string {
	if preflight != nil && strings.TrimSpace(preflight.SuggestedWorktree) != "" {
		return fmt.Sprintf("cd %s && gira work start --repo %s --issue %d --dry-run", QuoteShellArg(preflight.SuggestedWorktree), repo, issue)
	}
	return fmt.Sprintf("clean the current worktree, then gira work start --repo %s --issue %d --dry-run", repo, issue)
}

func retryWorkStartNextStep(repo string, issue int) string {
	return fmt.Sprintf("resolve ticket start preflight failures, then gira work start --repo %s --issue %d --dry-run", repo, issue)
}

func partialWorkStartNextStep(repo string, issue int) string {
	return fmt.Sprintf("gira work status --repo %s --issue %d --json", repo, issue)
}

func OpenWorkPR(repo RepoRef, issueNumber int, dryRun bool, draft bool, runner CommandRunner) (WorkPRResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return WorkPRResult{}, err
	}
	base, err := resolveTicketPRBase(repo, issue, runner)
	if err != nil {
		return WorkPRResult{}, err
	}
	status := displayStatus(managedStatusFromLabels(issue.Labels))
	targetStatus := "In review"
	if draft {
		targetStatus = "In progress"
	}
	result := WorkPRResult{
		Repo:               repo.FullName(),
		Issue:              issueNumber,
		DryRun:             dryRun,
		Draft:              draft,
		Status:             status,
		NextStatus:         targetStatus,
		RecordedBase:       base.BaseBranch,
		RecordedBaseSource: base.BaseSource,
		Blockers:           []string{},
		ClosingBody:        releaseImpactPRBody(issueNumber, issue.Body),
	}

	prStatus, err := DevPRStatus(repo, issueNumber, runner)
	if err != nil {
		return result, err
	}
	applyDevPRBindingPolicy(&prStatus, issueNumber, devPRBindingPolicyFromIssue(issue, runner, repo))
	if prStatus.PRNumber != 0 {
		actualDraft := hasWorkBlocker(prStatus.Blockers, "draft")
		targetStatus = "In review"
		if actualDraft {
			targetStatus = "In progress"
		}
		result.Draft = actualDraft
		result.NextStatus = targetStatus
		result.PRNumber = prStatus.PRNumber
		result.PRURL = prStatus.PRURL
		result.Blockers = prStatus.Blockers
		applyPRBaseMismatch(&result, base.BaseBranch, prStatus.Binding.BaseRef)
		if result.BaseMismatch {
			result.NextStep = correctPRBaseNextStep(repo, result.PRNumber, result.RecordedBase)
			return result, fmt.Errorf("existing PR base %q does not match recorded ticket base %q", result.ActualBase, result.RecordedBase)
		}
		if dryRun {
			result.Approval = WorkPRApprovalEvidence(result, "gira work pr")
			return result, nil
		}
		if err := setIssueStatus(repo, issueNumber, issue.Labels, statusLabelForDraft(actualDraft), runner); err != nil {
			return result, err
		}
		result.Status = targetStatus
		return result, nil
	}

	result.Blockers = prStatus.Blockers
	if err := validateRemoteBranchExists("origin", base.BaseBranch, runner); err != nil {
		return result, err
	}
	if dryRun {
		push, err := prepareWorkPRBranchPush(issue, issueNumber, base.BaseBranch, dryRun, runner)
		if err != nil {
			return result, err
		}
		result.Branch = push.Branch
		result.BranchPush = push.Status
		result.PushRemote = push.Remote
		result.LocalGit = push.LocalGit
		result.Warnings = appendUniqueStrings(result.Warnings, push.Warnings...)
		if push.Status == "planned" {
			result.Blockers = appendMissingWorkBlocker(result.Blockers, "branch_push_required")
		}
		result.Approval = WorkPRApprovalEvidence(result, "gira work pr")
		return result, nil
	}

	push, err := prepareWorkPRBranchPush(issue, issueNumber, base.BaseBranch, dryRun, runner)
	if err != nil {
		return result, err
	}
	result.Branch = push.Branch
	result.BranchPush = push.Status
	result.PushRemote = push.Remote
	result.LocalGit = push.LocalGit
	result.Warnings = appendUniqueStrings(result.Warnings, push.Warnings...)

	opened, err := OpenDevPRWithCreateOptions(repo, issueNumber, DevPRCreateOptions{Draft: draft, Base: base.BaseBranch}, runner)
	if err != nil {
		return result, err
	}
	result.PRNumber = opened.PRNumber
	result.PRURL = opened.PRURL
	result.ActualBase = opened.Base
	result.Created = true
	result.Blockers = []string{}
	if draft {
		result.Blockers = []string{"draft"}
	}
	if err := setIssueStatus(repo, issueNumber, issue.Labels, statusLabelForDraft(draft), runner); err != nil {
		return result, err
	}
	result.Status = targetStatus
	return result, nil
}

func GetWorkStatus(repo RepoRef, issueNumber int, runner CommandRunner) (WorkStatusResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	var issue devStartIssue
	var prStatus DevPRStatusResult
	var issueErr error
	var prErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		issue, issueErr = fetchDevIssue(repo, issueNumber, runner)
	}()
	go func() {
		defer wg.Done()
		prStatus, prErr = DevPRStatus(repo, issueNumber, runner)
	}()
	wg.Wait()
	if issueErr != nil {
		return WorkStatusResult{}, issueErr
	}
	if prErr != nil {
		return WorkStatusResult{}, prErr
	}
	if shouldRetryWorkStatusMissingPR(issue, prStatus) {
		prStatus, prErr = retryDevPRStatusAfterMissing(repo, issueNumber, runner, prStatus, workStatusMissingPRRetryAttempts, workStatusMissingPRRetryDelay, nil)
		if prErr != nil {
			return WorkStatusResult{}, prErr
		}
	}
	policy, err := ResolveRepoOperationPolicy(repo, runner)
	if err != nil {
		return WorkStatusResult{}, fmt.Errorf("resolve operation policy for %s: %w", repo.FullName(), err)
	}
	return workStatusFromIssueAndPRWithReviewEvidenceAndPolicy(repo, issueNumber, issue, prStatus, runner, policy), nil
}

func GetWorkStatusWithPRStatus(repo RepoRef, issueNumber int, prStatus DevPRStatusResult, runner CommandRunner) (WorkStatusResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return WorkStatusResult{}, err
	}
	policy, err := ResolveRepoOperationPolicy(repo, runner)
	if err != nil {
		return WorkStatusResult{}, fmt.Errorf("resolve operation policy for %s: %w", repo.FullName(), err)
	}
	return workStatusFromIssueAndPRWithReviewEvidenceAndPolicy(repo, issueNumber, issue, prStatus, runner, policy), nil
}

func workStatusFromIssueAndPR(repo RepoRef, issueNumber int, issue devStartIssue, prStatus DevPRStatusResult) WorkStatusResult {
	return workStatusFromIssueAndPRWithReviewEvidenceAndPolicy(repo, issueNumber, issue, prStatus, nil, compatibilityManagedRequiredPolicy())
}

func workStatusFromIssueAndPRWithReviewEvidence(repo RepoRef, issueNumber int, issue devStartIssue, prStatus DevPRStatusResult, runner CommandRunner) WorkStatusResult {
	policy := compatibilityManagedRequiredPolicy()
	if runner != nil {
		if resolved, err := ResolveRepoOperationPolicy(repo, runner); err == nil {
			policy = resolved
		}
	}
	return workStatusFromIssueAndPRWithReviewEvidenceAndPolicy(repo, issueNumber, issue, prStatus, runner, policy)
}

func workStatusFromIssueAndPRWithReviewEvidenceAndPolicy(repo RepoRef, issueNumber int, issue devStartIssue, prStatus DevPRStatusResult, runner CommandRunner, operationPolicy ResolvedOperationPolicy) WorkStatusResult {
	applyDevPRBindingPolicy(&prStatus, issueNumber, devPRBindingPolicyFromIssue(issue, nil, repo))
	policy := loadFinishReviewPolicy(repo)
	var review *FinishReviewEvidence
	status := displayStatus(managedStatusFromLabels(issue.Labels))
	nextAction := nextWorkAction(issue.State, status, prStatus)
	if runner != nil && prStatus.PRNumber > 0 && strings.EqualFold(status, "In review") && (nextAction == "merge_when_policy_allows" || containsString(prStatus.Blockers, "review")) {
		evidence := finishReviewEvidence(repo, prStatus, policy, runner)
		review = &evidence
		if policy.Value == FinishReviewPolicyNone {
			prStatus.Blockers = removeString(prStatus.Blockers, "review")
			nextAction = nextWorkAction(issue.State, status, prStatus)
		} else if evidence.Blocker != "" {
			prStatus.Blockers = removeString(prStatus.Blockers, "review")
			prStatus.Blockers = appendUniqueStrings(prStatus.Blockers, evidence.Blocker)
			nextAction = nextWorkAction(issue.State, status, prStatus)
		}
	}
	branchPolicy := ticketStatusBranchPolicy(issue, prStatus)
	if branchPolicy.BaseMismatch {
		prStatus.Blockers = appendMissingWorkBlocker(prStatus.Blockers, "pr_base_mismatch")
		nextAction = nextWorkAction(issue.State, status, prStatus)
	}
	if nextAction == "done" {
		status = "Done"
	} else if nextAction == "closed" && (status == "" || status == "null") {
		status = "Closed"
		prStatus.Blockers = nil
	}
	result := WorkStatusResult{
		Command:          "ticket status",
		SchemaVersion:    TicketStatusSchemaVersion,
		Repo:             repo.FullName(),
		Issue:            issueNumber,
		Title:            issue.Title,
		State:            issue.State,
		Status:           status,
		Labels:           append([]string(nil), issue.Labels...),
		Milestone:        issue.Milestone,
		PRNumber:         prStatus.PRNumber,
		PRURL:            prStatus.PRURL,
		PRState:          prStatus.State,
		PRLookupAttempts: prStatus.LookupAttempts,
		Blockers:         prStatus.Blockers,
		NextAction:       nextAction,
		Branch:           ticketStatusBranch(issue, prStatus),
		BranchPolicy:     branchPolicy,
		PullRequest:      ticketStatusPullRequest(prStatus),
		ChecksStatus:     ticketStatusChecksStatus(prStatus),
		Checks:           append([]DevPRCheck(nil), prStatus.Checks...),
		ReviewStatus:     ticketStatusReviewStatusWithEvidence(prStatus, review),
		ReviewPolicy:     reviewPolicyPtr(policy),
		ReviewEvidence:   review,
		Evidence:         ticketStatusEvidence(prStatus, nextAction),
		Acceptance:       ticketStatusAcceptance(issue.Body),
		ReleaseImpact:    ticketReleaseImpactPtr(ParseTicketReleaseImpact(issue.Body)),
		Telemetry:        ticketStatusTelemetry(issue.Body, issue.Labels),
		Policy:           &operationPolicy,
		TicketReadiness:  ticketStatusReadinessWithPolicy(issue, operationPolicy),
		Warnings:         ticketStatusWarnings(issue, prStatus),
	}
	result.NextStep = workStatusNextStep(result)
	prReadiness := EvaluatePRReadinessFromStatus(result)
	result.PRReadiness = &prReadiness
	return result
}

func ticketReleaseImpactPtr(impact TicketReleaseImpact) *TicketReleaseImpact {
	return &impact
}

func reviewPolicyPtr(policy FinishReviewPolicy) *FinishReviewPolicy { return &policy }

func shouldRetryWorkStatusMissingPR(issue devStartIssue, prStatus DevPRStatusResult) bool {
	return containsString(prStatus.Blockers, "missing_linked_pr") && strings.EqualFold(managedStatusFromLabels(issue.Labels), "In review")
}

func ticketStatusBranch(issue devStartIssue, pr DevPRStatusResult) *TicketStatusBranch {
	state := ParseTicketLifecycleState(issue.Body)
	suggestion := formatDevBranch(DefaultDevBranchPattern, issue.Number, issue.Title)
	expected := strings.TrimSpace(state.WorkBranch)
	bindingState := "bound"
	if expected == "" {
		expected = suggestion
		bindingState = "unbound"
	}
	current := "unknown"
	source := "expected_from_issue_title"
	trusted := false
	if pr.Binding.HeadRef != "" {
		current = pr.Binding.HeadRef
		source = pr.Binding.Source
		trusted = pr.Binding.Trusted
	}
	return &TicketStatusBranch{Expected: expected, Suggestion: suggestion, BindingState: bindingState, Current: current, Trusted: trusted, Source: source}
}

func ticketStatusBranchPolicy(issue devStartIssue, pr DevPRStatusResult) *TicketStatusBranchPolicy {
	state := ParseTicketLifecycleState(issue.Body)
	report := &TicketStatusBranchPolicy{
		RecordedBase:       strings.TrimSpace(state.BaseBranch),
		RecordedBaseSource: strings.TrimSpace(state.BaseSource),
		PolicyMode:         strings.TrimSpace(state.BranchPolicyMode),
		StartMode:          strings.TrimSpace(state.StartMode),
		BranchStrategy:     strings.TrimSpace(state.BranchStrategy),
		WorkBranch:         strings.TrimSpace(state.WorkBranch),
		WorkBranchSource:   strings.TrimSpace(state.WorkBranchSource),
		ActualPRBase:       strings.TrimSpace(pr.Binding.BaseRef),
		Diagnostics:        []string{},
	}
	if report.RecordedBase == "" {
		report.Diagnostics = append(report.Diagnostics, "missing_recorded_base")
	}
	if report.PolicyMode == "" {
		report.Diagnostics = append(report.Diagnostics, "missing_branch_policy_mode")
	}
	if report.WorkBranch == "" {
		report.Diagnostics = append(report.Diagnostics, "missing_recorded_work_branch")
	} else if pr.Binding.HeadRef != "" && report.WorkBranch != pr.Binding.HeadRef {
		report.Diagnostics = append(report.Diagnostics, "branch_name_differs_from_suggestion")
	}
	if report.RecordedBase != "" && report.ActualPRBase != "" && report.RecordedBase != report.ActualPRBase {
		report.BaseMismatch = true
		report.Diagnostics = append(report.Diagnostics, "recorded_base_actual_pr_base_mismatch")
	}
	if len(report.Diagnostics) == 0 {
		report.Diagnostics = []string{}
	}
	return report
}

func ticketStatusPullRequest(pr DevPRStatusResult) *TicketStatusPullRequest {
	if pr.PRNumber == 0 {
		return &TicketStatusPullRequest{Available: false, State: "missing", Mergeable: "unknown", ReviewDecision: "unknown", HeadRefName: "unknown", BaseRefName: "unknown"}
	}
	return &TicketStatusPullRequest{
		Available:        true,
		Number:           pr.PRNumber,
		URL:              pr.PRURL,
		State:            valueOrUnknown(pr.State),
		Mergeable:        valueOrUnknown(pr.Mergeable),
		HeadRefName:      valueOrUnknown(pr.Binding.HeadRef),
		BaseRefName:      valueOrUnknown(pr.Binding.BaseRef),
		ReviewDecision:   valueOrUnknown(pr.ReviewDecision),
		IsDraft:          pr.IsDraft,
		HeadSHA:          pr.HeadSHA,
		MergeCommitSHA:   pr.MergeCommitSHA,
		ClosingReference: pr.ClosingReference,
	}
}

func ticketStatusChecksStatus(pr DevPRStatusResult) string {
	if len(pr.Checks) == 0 {
		return "unknown"
	}
	if pr.ChecksUnavailable {
		return "unknown"
	}
	for _, check := range pr.Checks {
		if strings.EqualFold(strings.TrimSpace(check.Conclusion), "skipped") {
			continue
		}
		if strings.TrimSpace(check.State) == "" || strings.EqualFold(strings.TrimSpace(check.State), "unknown") {
			return "unknown"
		}
	}
	if containsString(pr.Blockers, "checks") {
		return "failed"
	}
	if containsString(pr.Blockers, "checks_pending") {
		return "pending"
	}
	return "passed"
}

func ticketStatusReviewStatus(pr DevPRStatusResult) string {
	return ticketStatusReviewStatusWithEvidence(pr, nil)
}

func ticketStatusReviewStatusWithEvidence(pr DevPRStatusResult, review *FinishReviewEvidence) string {
	if pr.PRNumber == 0 {
		return "missing"
	}
	if review != nil && review.Status == "not_required" {
		return "not_required"
	}
	if containsString(pr.Blockers, "review") || (review != nil && review.Blocker != "") {
		return "blocked"
	}
	if strings.EqualFold(pr.ReviewDecision, "APPROVED") {
		return "approved"
	}
	if strings.TrimSpace(pr.ReviewDecision) == "" {
		return "unknown"
	}
	return strings.ToLower(pr.ReviewDecision)
}

func ticketStatusEvidence(pr DevPRStatusResult, nextAction string) *TicketStatusEvidence {
	sources := []string{}
	if pr.PRNumber > 0 {
		sources = append(sources, "closing_reference")
	}
	if pr.Binding.Trusted {
		sources = append(sources, "branch_binding")
	}
	if len(pr.Checks) > 0 {
		sources = append(sources, "checks")
	}
	if strings.TrimSpace(pr.ReviewDecision) != "" {
		sources = append(sources, "review_state")
	}
	if len(sources) == 0 {
		sources = append(sources, "none")
	}
	return &TicketStatusEvidence{
		ClosingReference: pr.PRNumber > 0,
		BranchTrusted:    pr.Binding.Trusted,
		FinishReady:      nextAction == "merge_when_policy_allows" || nextAction == "done",
		Sources:          sources,
	}
}

func ticketStatusAcceptance(body string) *TicketStatusAcceptance {
	report := &TicketStatusAcceptance{Status: "missing"}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if !strings.HasPrefix(lower, "- [") && !strings.HasPrefix(lower, "* [") {
			continue
		}
		if len(lower) < 5 || lower[1] != ' ' || lower[2] != '[' || lower[4] != ']' {
			continue
		}
		marker := lower[3]
		switch marker {
		case 'x':
			report.Total++
			report.Complete++
		case ' ':
			report.Total++
			report.Incomplete++
		}
	}
	if report.Total == 0 {
		return report
	}
	if report.Incomplete > 0 {
		report.Status = "incomplete"
	} else {
		report.Status = "complete"
	}
	return report
}

func ticketStatusTelemetry(body string, labels []string) *TicketStatusTelemetry {
	report := &TicketStatusTelemetry{
		Required: aiDeliveryTelemetryRequired(labels),
		Sources:  []string{},
	}
	lowerBody := strings.ToLower(body)
	if strings.Contains(lowerBody, "ai delivery telemetry") {
		report.Present = true
		report.Sources = append(report.Sources, "issue_body:ai_delivery_telemetry")
	}
	if extractProvenanceBlock(body) != "" {
		report.Present = true
		report.Sources = append(report.Sources, "issue_body:gira_provenance")
	}
	if !report.Present {
		report.Sources = append(report.Sources, "none")
	}
	switch {
	case report.Required && report.Present:
		report.Status = "present"
	case report.Required:
		report.Status = "missing"
		report.Warnings = append(report.Warnings, "missing_ai_delivery_telemetry")
	case report.Present:
		report.Status = "present"
	default:
		report.Status = "not_required"
	}
	return report
}

func aiDeliveryTelemetryRequired(labels []string) bool {
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		if lower == "lane:agent" || lower == "lane:hybrid" {
			return true
		}
		if !strings.HasPrefix(lower, "agent:") {
			continue
		}
		switch strings.TrimPrefix(lower, "agent:") {
		case "worker", "codex", "gira", "reviewer":
			return true
		}
	}
	return false
}

func ticketStatusWarnings(issue devStartIssue, pr DevPRStatusResult) []string {
	warnings := []string{}
	if len(managedStatusLabels(issue.Labels)) > 1 {
		warnings = append(warnings, "multiple_status_labels")
	}
	if pr.PRNumber == 0 {
		warnings = append(warnings, "missing_linked_pr")
	}
	if acceptance := ticketStatusAcceptance(issue.Body); acceptance.Status == "missing" {
		warnings = append(warnings, "missing_acceptance_criteria")
	} else if acceptance.Status == "incomplete" {
		warnings = append(warnings, "incomplete_acceptance_criteria")
	}
	warnings = appendUniqueStrings(warnings, ticketReleaseImpactWarnings(issue.Body, issue.Labels)...)
	if telemetry := ticketStatusTelemetry(issue.Body, issue.Labels); len(telemetry.Warnings) > 0 {
		warnings = appendUniqueStrings(warnings, telemetry.Warnings...)
	}
	if containsString(pr.Blockers, "pr_binding") {
		warnings = append(warnings, "untrusted_pr_branch_binding")
	}
	warnings = appendUniqueStrings(warnings, pr.Binding.Warnings...)
	if branchPolicy := ticketStatusBranchPolicy(issue, pr); branchPolicy.BaseMismatch {
		warnings = append(warnings, "recorded_base_actual_pr_base_mismatch")
	}
	if len(warnings) == 0 {
		return []string{}
	}
	return warnings
}

func ticketReleaseImpactWarnings(body string, labels []string) []string {
	impact := ParseTicketReleaseImpact(body)
	if impact.Declared {
		return []string{}
	}
	if containsString(labels, "type:story") {
		return []string{"missing_release_impact"}
	}
	return []string{}
}

func ticketStatusReadiness(issue devStartIssue) *TicketReadinessReport {
	report := EvaluateTicketReadiness(issue.Body, issue.Labels, issue.State)
	return &report
}

func ticketStatusReadinessWithPolicy(issue devStartIssue, policy ResolvedOperationPolicy) *TicketReadinessReport {
	report := EvaluateTicketReadinessWithPolicy(issue.Body, issue.Labels, issue.State, policy)
	return &report
}

func setIssueStatus(repo RepoRef, issueNumber int, labels []string, targetLabel string, runner CommandRunner) error {
	existing := managedStatusLabels(labels)
	if len(existing) == 1 && strings.EqualFold(existing[0], targetLabel) {
		return nil
	}
	return replaceIssueStatusLabel(repo, issueNumber, existing, targetLabel, runner)
}

func statusLabelForDraft(draft bool) string {
	if draft {
		return "status:in-progress"
	}
	return "status:in-review"
}

func nextWorkAction(issueState string, status string, pr DevPRStatusResult) string {
	if strings.EqualFold(pr.State, "MERGED") {
		if !strings.EqualFold(issueState, "closed") {
			return "converge_completion_state"
		}
		return "done"
	}
	if pr.PRNumber > 0 && hasWorkBlocker(pr.Blockers, "pr_base_mismatch") {
		return "correct_pr_base"
	}
	if strings.EqualFold(issueState, "closed") {
		return "closed"
	}
	if pr.PRNumber == 0 {
		if status == "Blocked" {
			return "resolve_blockers"
		}
		if status == "Ready" {
			return "start_work"
		}
		if status == "In progress" {
			return "open_pr"
		}
		return "start_work"
	}
	for _, blocker := range pr.Blockers {
		switch blocker {
		case "pr_base_mismatch":
			return "correct_pr_base"
		case "draft":
			return "mark_pr_ready"
		case "review", "review_policy_not_configured", "review_required_but_absent", "review_evidence_unavailable", "review_approval_stale":
			return "address_review"
		case "checks", "checks_pending":
			return "wait_for_checks"
		}
	}
	return "merge_when_policy_allows"
}

func hasWorkBlocker(blockers []string, target string) bool {
	for _, blocker := range blockers {
		if blocker == target {
			return true
		}
	}
	return false
}

type workPRBranchPush struct {
	Branch   string
	Remote   string
	Status   string
	LocalGit string
	Warnings []string
}

func prepareWorkPRBranchPush(issue devStartIssue, issueNumber int, resolvedBase string, dryRun bool, runner CommandRunner) (workPRBranchPush, error) {
	state := ParseTicketLifecycleState(issue.Body)
	expectedBranch := strings.TrimSpace(state.WorkBranch)
	if expectedBranch == "" {
		expectedBranch = formatDevBranch(DefaultDevBranchPattern, issue.Number, issue.Title)
	}
	currentOut, err := runner.Run("git", "branch", "--show-current")
	if err != nil {
		return workPRBranchPush{}, fmt.Errorf("read current branch before PR create: %w", err)
	}
	currentBranch := strings.TrimSpace(string(currentOut))
	if currentBranch == "" {
		return workPRBranchPush{}, fmt.Errorf("cannot create PR from detached HEAD; checkout the ticket branch, then run `gira ticket pr --apply`")
	}
	if currentBranch == strings.TrimSpace(resolvedBase) {
		return workPRBranchPush{}, fmt.Errorf("refusing to create ticket PR from resolved base branch %s; choose a work branch before opening the PR", currentBranch)
	}
	const remote = "origin"
	if err := validateGitPushTarget(remote, currentBranch); err != nil {
		return workPRBranchPush{}, err
	}
	warnings := []string{}
	if currentBranch != expectedBranch && !strings.HasPrefix(currentBranch, fmt.Sprintf("issue-%d-", issueNumber)) {
		warnings = append(warnings, "branch_name_differs_from_suggestion")
	}
	localGit := fmt.Sprintf("git push -u %s <validated-ticket-branch>", remote)
	if upstreamOut, err := runner.Run("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		upstream := strings.TrimSpace(string(upstreamOut))
		if upstream == remote+"/"+currentBranch {
			return workPRBranchPush{Branch: currentBranch, Remote: remote, Status: "skipped", LocalGit: localGit, Warnings: warnings}, nil
		}
	}
	if dryRun {
		return workPRBranchPush{Branch: currentBranch, Remote: remote, Status: "planned", LocalGit: localGit, Warnings: warnings}, nil
	}
	if _, err := runner.Run("git", "push", "-u", remote, currentBranch); err != nil {
		return workPRBranchPush{Branch: currentBranch, Remote: remote, Status: "failed", LocalGit: localGit, Warnings: warnings}, fmt.Errorf("push ticket branch before PR create failed; inspect local git output and credentials outside Gira logs")
	}
	return workPRBranchPush{Branch: currentBranch, Remote: remote, Status: "applied", LocalGit: localGit, Warnings: warnings}, nil
}

func validateGitPushTarget(remote string, branch string) error {
	if err := validateGitRemoteName(remote); err != nil {
		return err
	}
	if err := validateGitBranchPushName(branch); err != nil {
		return err
	}
	return nil
}

func validateGitRemoteName(remote string) error {
	if strings.TrimSpace(remote) != remote || remote == "" {
		return fmt.Errorf("invalid git push remote: expected a configured remote name")
	}
	if strings.HasPrefix(remote, "-") {
		return fmt.Errorf("invalid git push remote: remote names cannot start with '-'")
	}
	for _, r := range remote {
		if r <= 0x20 || r == 0x7f || r == ':' || r == '/' || r == '\\' {
			return fmt.Errorf("invalid git push remote: expected a configured remote name")
		}
	}
	return nil
}

func validateGitBranchPushName(branch string) error {
	if strings.TrimSpace(branch) != branch || branch == "" {
		return fmt.Errorf("invalid git push branch: expected a local ticket branch")
	}
	if strings.HasPrefix(branch, "-") {
		return fmt.Errorf("invalid git push branch: branch names cannot start with '-'")
	}
	if strings.Contains(branch, "..") ||
		strings.Contains(branch, "@{") ||
		strings.Contains(branch, "\\") ||
		strings.Contains(branch, ":") ||
		strings.Contains(branch, "//") ||
		strings.HasPrefix(branch, "/") ||
		strings.HasSuffix(branch, "/") ||
		strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("invalid git push branch: expected a plain local branch name")
	}
	for _, part := range strings.Split(branch, "/") {
		if part == "" || part == "." || strings.HasSuffix(part, ".") {
			return fmt.Errorf("invalid git push branch: expected a plain local branch name")
		}
	}
	for _, r := range branch {
		if r <= 0x20 || r == 0x7f || r == '~' || r == '^' || r == '?' || r == '*' || r == '[' {
			return fmt.Errorf("invalid git push branch: expected a plain local branch name")
		}
	}
	return nil
}

func appendMissingWorkBlocker(blockers []string, blocker string) []string {
	if hasWorkBlocker(blockers, blocker) {
		return blockers
	}
	return append(blockers, blocker)
}

func FormatWorkStart(result WorkStartResult) string {
	next := strings.TrimSpace(result.NextStep)
	if next == "" {
		next = fmt.Sprintf("gira work pr --repo %s --issue %d --dry-run", result.Repo, result.Issue)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "work start: issue #%d branch=%s status=%s execution_state=%s started=%t\n", result.Issue, result.Branch, result.NextStatus, result.ExecutionState, result.Started)
	if preflight := result.Preflight; preflight != nil {
		fmt.Fprintf(&b, "worktree preflight: path=%s dirty=%t expected_branch=%s reusable_branch=%t\n", preflight.CurrentWorktree, preflight.Dirty, preflight.ExpectedBranch, preflight.ReusableBranch)
		if preflight.SuggestedWorktree != "" {
			fmt.Fprintf(&b, "suggested worktree: %s\n", preflight.SuggestedWorktree)
		}
	}
	fmt.Fprintf(&b, "next step: %s\n", next)
	return b.String()
}

func FormatWorkPR(result WorkPRResult) string {
	created := "reused"
	if result.Created {
		created = "created"
	}
	url := strings.TrimSpace(result.PRURL)
	if url == "" {
		url = "(planned)"
		created = "planned"
	}
	next := fmt.Sprintf("gira work status --repo %s --issue %d", result.Repo, result.Issue)
	if result.DryRun {
		next = fmt.Sprintf("gira work pr --repo %s --issue %d --apply", result.Repo, result.Issue)
		if result.Draft {
			next += " --draft"
		}
	}
	if result.Draft && !result.DryRun {
		next = "mark the PR ready, then " + next
	}
	branchPush := ""
	if result.BranchPush != "" && result.BranchPush != "skipped" {
		branchPush = fmt.Sprintf(" branch_push=%s", result.BranchPush)
		if result.LocalGit != "" {
			branchPush += fmt.Sprintf(" local_git=%q", result.LocalGit)
		}
	}
	return fmt.Sprintf("work pr: issue #%d pr=%s status=%s %s%s\nnext step: %s\n", result.Issue, url, result.NextStatus, created, branchPush, next)
}

func FormatWorkStatus(result WorkStatusResult) string {
	blockers := strings.Join(result.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"work status: issue #%d status=%s pr=%d blockers=%s next=%s\n",
		result.Issue,
		result.Status,
		result.PRNumber,
		blockers,
		result.NextAction,
	)
	if result.TicketReadiness != nil {
		b.WriteString(formatTicketReadinessHuman(*result.TicketReadiness))
	}
	if result.PRReadiness != nil {
		b.WriteString(formatPRReadinessHuman(*result.PRReadiness))
	}
	fmt.Fprintf(&b, "next step: %s\n", workStatusNextStep(result))
	return b.String()
}

func workStartNextStep(repo string, issue int, issueState string, status string, dryRun bool) string {
	if !strings.EqualFold(issueState, "open") {
		return fmt.Sprintf("gira work status --repo %s --issue %d", repo, issue)
	}
	if isMissingWorkStatus(status) {
		return readyStatusNextStep(repo, issue)
	}
	if dryRun && (status == "Ready" || status == "In progress") {
		return fmt.Sprintf("gira work start --repo %s --issue %d --apply", repo, issue)
	}
	if status == "Ready" || status == "In progress" {
		return fmt.Sprintf("gira work pr --repo %s --issue %d --dry-run", repo, issue)
	}
	return fmt.Sprintf("gira work status --repo %s --issue %d", repo, issue)
}

func workStatusNextStep(result WorkStatusResult) string {
	switch result.NextAction {
	case "start_work":
		if isMissingWorkStatus(result.Status) {
			return readyStatusNextStep(result.Repo, result.Issue)
		}
		return fmt.Sprintf("gira work start --repo %s --issue %d --dry-run", result.Repo, result.Issue)
	case "open_pr":
		return fmt.Sprintf("gira work pr --repo %s --issue %d --dry-run", result.Repo, result.Issue)
	case "resolve_blockers":
		return "resolve blockers, then set status:ready before starting work"
	case "mark_pr_ready":
		return "mark the PR ready for review"
	case "address_review":
		return "address review blockers"
	case "correct_pr_base":
		if result.PRNumber > 0 && result.BranchPolicy != nil {
			return correctPRBaseNextStepForRepoName(result.Repo, result.PRNumber, result.BranchPolicy.RecordedBase)
		}
		return "correct the linked PR base to the recorded ticket base, then rerun ticket status"
	case "wait_for_checks":
		return "wait for required checks to finish or fix failing checks"
	case "merge_when_policy_allows":
		return "merge when policy checks pass"
	case "done":
		return "ticket is done"
	case "converge_completion_state":
		return fmt.Sprintf("gira ticket finish --repo %s --ticket %d --dry-run", result.Repo, result.Issue)
	case "closed":
		return "ticket is closed; inspect GitHub history if more evidence is needed"
	default:
		return fmt.Sprintf("gira status --repo %s", result.Repo)
	}
}

func correctPRBaseNextStep(repo RepoRef, prNumber int, base string) string {
	return correctPRBaseNextStepForRepoName(repo.FullName(), prNumber, base)
}

func correctPRBaseNextStepForRepoName(repo string, prNumber int, base string) string {
	if prNumber <= 0 || strings.TrimSpace(repo) == "" || strings.TrimSpace(base) == "" {
		return "correct the linked PR base to the recorded ticket base, then rerun ticket status"
	}
	return fmt.Sprintf("gh pr edit %d --repo %s --base %s", prNumber, QuoteShellArg(repo), QuoteShellArg(base))
}

func readyStatusNextStep(repo string, issue int) string {
	return fmt.Sprintf("gira adopt issues --repo %s --issue %d --label status:ready --dry-run", repo, issue)
}

func isMissingWorkStatus(status string) bool {
	trimmed := strings.TrimSpace(status)
	return trimmed == "" || strings.EqualFold(trimmed, "null")
}
