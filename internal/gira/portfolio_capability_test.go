package gira

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuildPortfolioCapabilityReportAllowed(t *testing.T) {
	runner := &fakePortfolioCapabilityRunner{
		authStatus: `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice","tokenSource":"/home/user/.config/gh/hosts.yml","scopes":"repo"}]}}`,
		repos: map[string]string{
			"repos/StatPan/portfolio":        `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":false,"triage":true}}`,
			"repos/StatPan/portfolio/issues": `[]`,
			"repos/StatPan/gira":             `{"permissions":{"admin":false,"maintain":true,"pull":true,"push":true,"triage":true}}`,
			"repos/StatPan/gira/issues":      `[]`,
			"repos/StatPan/docs":             `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":true,"triage":false}}`,
			"repos/StatPan/docs/issues":      `[]`,
		},
	}

	report, err := BuildPortfolioCapabilityReport(
		ParseRepoRefMust("StatPan/portfolio"),
		[]RepoRef{ParseRepoRefMust("StatPan/docs"), ParseRepoRefMust("StatPan/gira")},
		runner,
		time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("BuildPortfolioCapabilityReport error: %v", err)
	}
	if report.Command != "portfolio capability" || report.PortfolioRepo != "StatPan/portfolio" {
		t.Fatalf("unexpected report identity: %+v", report)
	}
	if len(report.Repos) != 3 {
		t.Fatalf("repos = %+v, want portfolio plus two execution repos", report.Repos)
	}
	if len(report.BlockedActions) != 0 {
		t.Fatalf("blocked actions = %+v, want none", report.BlockedActions)
	}
	if report.Repos[1].Repo != "StatPan/docs" || report.Repos[2].Repo != "StatPan/gira" {
		t.Fatalf("execution repos not sorted: %+v", report.Repos)
	}
}

func TestBuildPortfolioCapabilityReportPartialDenied(t *testing.T) {
	runner := &fakePortfolioCapabilityRunner{
		authStatus: `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice","tokenSource":"env://GITHUB_TOKEN","scopes":""}]}}`,
		repos: map[string]string{
			"repos/StatPan/portfolio":        `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":false,"triage":true}}`,
			"repos/StatPan/portfolio/issues": `[]`,
			"repos/StatPan/gira":             `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":false,"triage":false}}`,
			"repos/StatPan/gira/issues":      `[]`,
			"repos/StatPan/docs":             `{"permissions":{"admin":false,"maintain":false,"pull":false,"push":false,"triage":false}}`,
		},
	}

	report, err := BuildPortfolioCapabilityReport(ParseRepoRefMust("StatPan/portfolio"), []RepoRef{ParseRepoRefMust("StatPan/gira"), ParseRepoRefMust("StatPan/docs")}, runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildPortfolioCapabilityReport error: %v", err)
	}
	var foundDocsRead, foundDocsWrite, foundGiraWrite bool
	for _, block := range report.BlockedActions {
		if block.Repo == "StatPan/docs" && block.Required == "issues:read" {
			foundDocsRead = true
		}
		if block.Repo == "StatPan/docs" && block.Required == "issues:write" {
			foundDocsWrite = true
		}
		if block.Repo == "StatPan/gira" && block.Required == "issues:write" {
			foundGiraWrite = true
		}
	}
	if !foundDocsRead || !foundDocsWrite || !foundGiraWrite {
		t.Fatalf("blocked actions = %+v, want docs read/write and gira write", report.BlockedActions)
	}
	text := FormatPortfolioCapabilityReport(report)
	if !strings.Contains(text, "fix blocked repo permissions before implementing portfolio lower --apply") {
		t.Fatalf("text missing remediation:\n%s", text)
	}
}

func TestFormatPortfolioCapabilityReportUsesImplementedNextStep(t *testing.T) {
	text := FormatPortfolioCapabilityReport(PortfolioCapabilityReport{
		PortfolioRepo: "StatPan/portfolio",
		Token:         ProjectCapabilityTokenSummary{Kind: "pat", Identity: "alice"},
		Repos:         []PortfolioRepoCapability{},
		FetchedAt:     "2026-05-05T12:00:00Z",
	})
	if !strings.Contains(text, "next step: gira portfolio plan --dry-run --config .gira/config.yaml") {
		t.Fatalf("text missing implemented next step:\n%s", text)
	}
}

func TestBuildPortfolioCapabilityReportInconclusiveWriteIsBlocked(t *testing.T) {
	runner := &fakePortfolioCapabilityRunner{
		authStatus: `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice","tokenSource":"env://GITHUB_TOKEN","scopes":""}]}}`,
		repos: map[string]string{
			"repos/StatPan/portfolio":        `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":false,"triage":true}}`,
			"repos/StatPan/portfolio/issues": `[]`,
			"repos/StatPan/gira":             `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":false,"triage":true}}`,
			"repos/StatPan/gira/issues":      `[]`,
		},
	}

	report, err := BuildPortfolioCapabilityReport(ParseRepoRefMust("StatPan/portfolio"), []RepoRef{ParseRepoRefMust("StatPan/gira")}, runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildPortfolioCapabilityReport error: %v", err)
	}
	if report.Repos[1].Capabilities["issues:write"] != ProjectCapabilityUnsupported {
		t.Fatalf("issues:write = %s, want unsupported when token scope cannot prove issue write", report.Repos[1].Capabilities["issues:write"])
	}
	if len(report.BlockedActions) == 0 || !strings.Contains(report.BlockedActions[0].Reason, "cannot be proven") {
		t.Fatalf("blocked actions = %+v, want non-destructive proof block", report.BlockedActions)
	}
}

func TestBuildPortfolioCapabilityReportRepoProbeFailureIsBlocked(t *testing.T) {
	runner := &fakePortfolioCapabilityRunner{
		authStatus: `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice","tokenSource":"env://GITHUB_TOKEN","scopes":""}]}}`,
		repos: map[string]string{
			"repos/StatPan/portfolio":        `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":false,"triage":true}}`,
			"repos/StatPan/portfolio/issues": `[]`,
		},
	}

	report, err := BuildPortfolioCapabilityReport(ParseRepoRefMust("StatPan/portfolio"), []RepoRef{ParseRepoRefMust("StatPan/missing")}, runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildPortfolioCapabilityReport error: %v", err)
	}
	var foundRead, foundWrite bool
	for _, block := range report.BlockedActions {
		if block.Repo == "StatPan/missing" && block.Required == "issues:read" {
			foundRead = true
		}
		if block.Repo == "StatPan/missing" && block.Required == "issues:write" {
			foundWrite = true
		}
	}
	if !foundRead || !foundWrite {
		t.Fatalf("blocked actions = %+v, want missing repo read/write blocks", report.BlockedActions)
	}
}

func TestPortfolioCapabilityBlocksForActions(t *testing.T) {
	report := PortfolioCapabilityReport{
		BlockedActions: []PortfolioCapabilityBlock{
			{CheckID: "execution:StatPan/gira:issues:write", Repo: "StatPan/gira", Role: "execution", Required: "issues:write", Reason: "blocked"},
			{CheckID: "execution:StatPan/docs:issues:write", Repo: "StatPan/docs", Role: "execution", Required: "issues:write", Reason: "blocked"},
		},
	}
	blocks := PortfolioCapabilityBlocksForActions(report, []PortfolioPlanAction{
		{Action: "execution_issue:create", Repo: "StatPan/gira"},
		{Action: "execution_issue:link_existing", Repo: "StatPan/docs"},
	})
	if len(blocks) != 1 || blocks[0].Repo != "StatPan/gira" {
		t.Fatalf("blocks = %+v, want only gira write block for create action", blocks)
	}
}

type fakePortfolioCapabilityRunner struct {
	authStatus string
	repos      map[string]string
}

func (r *fakePortfolioCapabilityRunner) Run(name string, args ...string) ([]byte, error) {
	if len(args) >= 2 && args[0] == "auth" && args[1] == "status" {
		return []byte(r.authStatus), nil
	}
	if len(args) >= 2 && args[0] == "api" {
		if payload, ok := r.repos[args[1]]; ok {
			return []byte(payload), nil
		}
		return nil, errors.New("repo not found: " + args[1])
	}
	return nil, errors.New("unexpected command")
}
