package gira

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const APILimitReportSchemaVersion = "api-limit-report/v1"

type APILimitReport struct {
	SchemaVersion string             `json:"schema_version"`
	Command       string             `json:"command"`
	Repo          string             `json:"repo"`
	FetchedAt     string             `json:"fetched_at"`
	Core          APILimitBucket     `json:"core"`
	GraphQL       APILimitBucket     `json:"graphql"`
	Search        APILimitBucket     `json:"search"`
	Secondary     SecondaryLimitInfo `json:"secondary"`
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
	if report.Core.Limit > 0 && report.Core.Remaining == 0 {
		report.Warnings = append(report.Warnings, "GitHub REST core budget exhausted")
	}
	if report.GraphQL.Limit > 0 && report.GraphQL.Remaining == 0 {
		report.Warnings = append(report.Warnings, "GitHub GraphQL budget exhausted")
	}
	if report.Search.Limit > 0 && report.Search.Remaining == 0 {
		report.Warnings = append(report.Warnings, "GitHub search budget exhausted")
	}
	return report, nil
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
