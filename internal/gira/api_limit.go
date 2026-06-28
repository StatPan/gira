package gira

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const APILimitReportSchemaVersion = "api-limit-report/v1"
const APILimitWorkflowSafetyFactorPercent = 80
const APILimitLowBudgetPercent = 10

type APILimitReport struct {
	SchemaVersion string             `json:"schema_version"`
	Command       string             `json:"command"`
	Repo          string             `json:"repo"`
	FetchedAt     string             `json:"fetched_at"`
	Core          APILimitBucket     `json:"core"`
	GraphQL       APILimitBucket     `json:"graphql"`
	Search        APILimitBucket     `json:"search"`
	Secondary     SecondaryLimitInfo `json:"secondary"`
	Workflow      *APILimitWorkflow  `json:"workflow,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
	NextStep      string             `json:"next_step"`
}

type APILimitBucket struct {
	Limit     int    `json:"limit"`
	Remaining int    `json:"remaining"`
	Used      int    `json:"used,omitempty"`
	ResetAt   string `json:"reset_at,omitempty"`
}

type SecondaryLimitInfo struct {
	Status   string   `json:"status"`
	Signals  []string `json:"signals"`
	Guidance string   `json:"guidance"`
}

type APILimitWorkflow struct {
	Name                   string                     `json:"name"`
	Mode                   string                     `json:"mode"`
	SafetyFactorPercent    int                        `json:"safety_factor_percent"`
	Cost                   WorkflowCostBucketEstimate `json:"cost"`
	BucketRuns             APILimitWorkflowBucketRuns `json:"bucket_runs"`
	LimitingBucket         string                     `json:"limiting_bucket"`
	SafeRuns               int                        `json:"safe_runs"`
	WriteContentMeasurable bool                       `json:"write_content_measurable"`
	SecondaryNote          string                     `json:"secondary_note,omitempty"`
}

type APILimitWorkflowBucketRuns struct {
	RESTCore     int `json:"rest_core"`
	GraphQL      int `json:"graphql"`
	Search       int `json:"search"`
	WriteContent int `json:"write_content"`
}

func BuildAPILimitReport(repo RepoRef, runner CommandRunner, now time.Time) (APILimitReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if !repoRefIsSet(repo) {
		return APILimitReport{}, fmt.Errorf("repo is required")
	}
	output, err := runner.Run("gh", "api", "rate_limit")
	if err != nil {
		return APILimitReport{}, actionableGitHubStatusError(err)
	}
	return ParseAPILimitReport(repo, output, now)
}

func ParseAPILimitReport(repo RepoRef, data []byte, now time.Time) (APILimitReport, error) {
	var raw struct {
		Resources struct {
			Core    rawAPILimitBucket `json:"core"`
			GraphQL rawAPILimitBucket `json:"graphql"`
			Search  rawAPILimitBucket `json:"search"`
		} `json:"resources"`
		Rate rawAPILimitBucket `json:"rate"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return APILimitReport{}, fmt.Errorf("parse GitHub rate limit JSON: %w", err)
	}
	core := apiLimitBucketFromRaw(raw.Resources.Core)
	if core.Limit == 0 && raw.Rate.Limit > 0 {
		core = apiLimitBucketFromRaw(raw.Rate)
	}
	report := APILimitReport{
		SchemaVersion: APILimitReportSchemaVersion,
		Command:       "ops limit",
		Repo:          repo.FullName(),
		FetchedAt:     now.UTC().Format(time.RFC3339),
		Core:          core,
		GraphQL:       apiLimitBucketFromRaw(raw.Resources.GraphQL),
		Search:        apiLimitBucketFromRaw(raw.Resources.Search),
		Secondary: SecondaryLimitInfo{
			Status: "unobservable",
			Signals: []string{
				"http_403",
				"http_429",
				"retry-after",
				"x-ratelimit-remaining=0",
				"secondary rate limit error",
			},
			Guidance: "Back off or wait for reset/retry-after; Gira cannot query remaining secondary budget.",
		},
		NextStep: fmt.Sprintf("gira ops limit --repo %s --json", QuoteShellArg(repo.FullName())),
	}
	report.Warnings = append(report.Warnings, APILimitBucketWarnings(report.Core, report.GraphQL, report.Search)...)
	return report, nil
}

func APILimitBucketWarnings(core APILimitBucket, graphql APILimitBucket, search APILimitBucket) []string {
	warnings := []string{}
	if warning := apiLimitBucketWarning("REST core", core); warning != "" {
		warnings = append(warnings, warning)
	}
	if warning := apiLimitBucketWarning("GraphQL", graphql); warning != "" {
		warnings = append(warnings, warning)
	}
	if warning := apiLimitBucketWarning("search", search); warning != "" {
		warnings = append(warnings, warning)
	}
	return warnings
}

func apiLimitBucketWarning(name string, bucket APILimitBucket) string {
	if bucket.Limit <= 0 {
		return ""
	}
	reset := ""
	if bucket.ResetAt != "" {
		reset = " reset=" + bucket.ResetAt
	}
	if bucket.Remaining <= 0 {
		return fmt.Sprintf("GitHub %s budget exhausted;%s inspect with gira ops limit", name, reset)
	}
	if bucket.Remaining*100 <= bucket.Limit*APILimitLowBudgetPercent {
		return fmt.Sprintf("GitHub API budget low (%s remaining=%d/%d); inspect with gira ops limit", strings.ToLower(name), bucket.Remaining, bucket.Limit)
	}
	return ""
}

func WithAPILimitWorkflow(report APILimitReport, workflowName string) (APILimitReport, error) {
	profile, ok := LookupWorkflowCostProfile(workflowName)
	if !ok {
		return APILimitReport{}, fmt.Errorf("unknown workflow cost profile %q", workflowName)
	}
	cost, err := DefaultWorkflowCostEstimate(profile)
	if err != nil {
		return APILimitReport{}, err
	}
	workflow := BuildAPILimitWorkflowEstimate(report, profile.Name, WorkflowCostModeConservative, cost)
	report.Workflow = &workflow
	report.NextStep = fmt.Sprintf("gira ops limit --repo %s --workflow %s --json", QuoteShellArg(report.Repo), QuoteShellArg(profile.Name))
	return report, nil
}

func BuildAPILimitWorkflowEstimate(report APILimitReport, workflowName string, mode string, cost WorkflowCostBucketEstimate) APILimitWorkflow {
	bucketRuns := APILimitWorkflowBucketRuns{
		RESTCore:     safeRunsForAPIBucket(report.Core.Remaining, cost.RESTCore),
		GraphQL:      safeRunsForAPIBucket(report.GraphQL.Remaining, cost.GraphQL),
		Search:       safeRunsForAPIBucket(report.Search.Remaining, cost.Search),
		WriteContent: -1,
	}
	limitingBucket, safeRuns := limitingAPIBucketRuns(cost, bucketRuns)
	workflow := APILimitWorkflow{
		Name:                   workflowName,
		Mode:                   mode,
		SafetyFactorPercent:    APILimitWorkflowSafetyFactorPercent,
		Cost:                   cost,
		BucketRuns:             bucketRuns,
		LimitingBucket:         limitingBucket,
		SafeRuns:               safeRuns,
		WriteContentMeasurable: false,
	}
	if cost.WriteContent > 0 {
		workflow.SecondaryNote = "write/content pressure is not directly measurable; safe_runs uses primary REST, GraphQL, and search budgets only."
	}
	return workflow
}

func safeRunsForAPIBucket(remaining int, cost int) int {
	if cost <= 0 {
		return -1
	}
	if remaining <= 0 {
		return 0
	}
	return (remaining * APILimitWorkflowSafetyFactorPercent / 100) / cost
}

func limitingAPIBucketRuns(cost WorkflowCostBucketEstimate, bucketRuns APILimitWorkflowBucketRuns) (string, int) {
	type candidate struct {
		name string
		cost int
		runs int
	}
	candidates := []candidate{
		{name: "rest_core", cost: cost.RESTCore, runs: bucketRuns.RESTCore},
		{name: "graphql", cost: cost.GraphQL, runs: bucketRuns.GraphQL},
		{name: "search", cost: cost.Search, runs: bucketRuns.Search},
	}
	limitingBucket := "none"
	safeRuns := -1
	for _, candidate := range candidates {
		if candidate.cost <= 0 {
			continue
		}
		if safeRuns < 0 || candidate.runs < safeRuns {
			limitingBucket = candidate.name
			safeRuns = candidate.runs
		}
	}
	if safeRuns < 0 {
		return limitingBucket, 0
	}
	return limitingBucket, safeRuns
}

type rawAPILimitBucket struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Used      int   `json:"used"`
	Reset     int64 `json:"reset"`
}

func apiLimitBucketFromRaw(raw rawAPILimitBucket) APILimitBucket {
	bucket := APILimitBucket{Limit: raw.Limit, Remaining: raw.Remaining, Used: raw.Used}
	if raw.Reset > 0 {
		bucket.ResetAt = time.Unix(raw.Reset, 0).UTC().Format(time.RFC3339)
	}
	return bucket
}

func FormatAPILimitReport(report APILimitReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ops limit: %s\n", report.Repo)
	fmt.Fprintf(&b, "fetched_at: %s\n", report.FetchedAt)
	formatAPILimitBucket(&b, "core", report.Core)
	formatAPILimitBucket(&b, "graphql", report.GraphQL)
	formatAPILimitBucket(&b, "search", report.Search)
	fmt.Fprintf(&b, "secondary: %s\n", report.Secondary.Status)
	if report.Secondary.Guidance != "" {
		fmt.Fprintf(&b, "secondary guidance: %s\n", report.Secondary.Guidance)
	}
	if len(report.Secondary.Signals) > 0 {
		fmt.Fprintf(&b, "secondary signals: %s\n", strings.Join(report.Secondary.Signals, ", "))
	}
	if report.Workflow != nil {
		formatAPILimitWorkflow(&b, *report.Workflow)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", warning)
	}
	if report.NextStep != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}

func formatAPILimitBucket(b *strings.Builder, name string, bucket APILimitBucket) {
	fmt.Fprintf(b, "%s: remaining=%d/%d", name, bucket.Remaining, bucket.Limit)
	if bucket.Used > 0 {
		fmt.Fprintf(b, " used=%d", bucket.Used)
	}
	if bucket.ResetAt != "" {
		fmt.Fprintf(b, " reset=%s", bucket.ResetAt)
	}
	b.WriteString("\n")
}

func formatAPILimitWorkflow(b *strings.Builder, workflow APILimitWorkflow) {
	fmt.Fprintf(b, "workflow: %s mode=%s safety_factor=%d%%\n", workflow.Name, workflow.Mode, workflow.SafetyFactorPercent)
	fmt.Fprintf(b, "workflow cost: rest_core=%d graphql=%d search=%d write_content=%d\n", workflow.Cost.RESTCore, workflow.Cost.GraphQL, workflow.Cost.Search, workflow.Cost.WriteContent)
	fmt.Fprintf(b, "safe runs: %d limiting_bucket=%s\n", workflow.SafeRuns, workflow.LimitingBucket)
	fmt.Fprintf(b, "bucket runs: rest_core=%s graphql=%s search=%s write_content=unobservable\n", formatAPILimitRunCount(workflow.BucketRuns.RESTCore), formatAPILimitRunCount(workflow.BucketRuns.GraphQL), formatAPILimitRunCount(workflow.BucketRuns.Search))
	if workflow.SecondaryNote != "" {
		fmt.Fprintf(b, "secondary note: %s\n", workflow.SecondaryNote)
	}
}

func formatAPILimitRunCount(runs int) string {
	if runs < 0 {
		return "not_applicable"
	}
	return fmt.Sprintf("%d", runs)
}
