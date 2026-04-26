package gira

import (
	"errors"
	"testing"
)

type fakeCommandRunner struct {
	calls       int
	authStatus  string
	repoData    string
	graphqlErr  bool
	graphqlData string
}

func (r *fakeCommandRunner) Run(name string, args ...string) ([]byte, error) {
	r.calls++
	if len(args) >= 2 && args[0] == "auth" && args[1] == "status" {
		return []byte(r.authStatus), nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "repos/StatPan/gira" {
		return []byte(r.repoData), nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "graphql" {
		if r.graphqlErr {
			return nil, errors.New("projectsv2 query denied")
		}
		return []byte(r.graphqlData), nil
	}
	return nil, nil
}

func TestBuildProjectCapabilityReportAllowed(t *testing.T) {
	auth := `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice","tokenSource":"/home/user/.config/gh/hosts.yml","scopes":"repo, project"}]}}`
	repo := `{"permissions":{"admin":true,"maintain":true,"pull":true,"push":true,"triage":true}}`
	graphql := `{"data":{"repository":{"viewerPermission":"ADMIN","viewerCanAdminister":true,"projectsV2":{"nodes":[{"title":"p1"}]}}}}`

	report, err := BuildProjectCapabilityReport(ParseRepoRefMust("StatPan/gira"), &fakeCommandRunner{
		authStatus:  auth,
		repoData:    repo,
		graphqlData: graphql,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if report.Mode != "write" {
		t.Fatalf("expected mode write, got %s", report.Mode)
	}
	if report.Capabilities["issues:write"] != ProjectCapabilityAllowed {
		t.Fatalf("expected issues:write allowed, got %s", report.Capabilities["issues:write"])
	}
	if report.Capabilities["projectsv2:write"] != ProjectCapabilityAllowed {
		t.Fatalf("expected projectsv2:write allowed, got %s", report.Capabilities["projectsv2:write"])
	}
	if report.Token.Kind != "pat" {
		t.Fatalf("expected pat token kind, got %s", report.Token.Kind)
	}
	if len(report.BlockedActions) != 0 {
		t.Fatalf("expected no blocked actions, got %d", len(report.BlockedActions))
	}
}

func TestBuildProjectCapabilityReportProjectDenied(t *testing.T) {
	auth := `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice","tokenSource":"env://GITHUB_TOKEN","scopes":""}]}}`
	repo := `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":false,"triage":true}}`

	report, err := BuildProjectCapabilityReport(ParseRepoRefMust("StatPan/gira"), &fakeCommandRunner{
		authStatus: auth,
		repoData:   repo,
		graphqlErr: true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if report.Token.Kind != "actions_secret" {
		t.Fatalf("expected actions_secret token kind, got %s", report.Token.Kind)
	}
	if report.Mode != "read-only" {
		t.Fatalf("expected read-only mode, got %s", report.Mode)
	}
	if report.Capabilities["projectsv2:read"] != ProjectCapabilityDeniedScope {
		t.Fatalf("expected projectsv2:read denied, got %s", report.Capabilities["projectsv2:read"])
	}
	if report.Capabilities["projectsv2:write"] != ProjectCapabilityDeniedScope {
		t.Fatalf("expected projectsv2:write denied, got %s", report.Capabilities["projectsv2:write"])
	}
	if len(report.BlockedActions) == 0 {
		t.Fatalf("expected blocked actions for denied capabilities")
	}
}

func ParseRepoRefMust(raw string) RepoRef {
	repo, err := ParseRepoRef(raw)
	if err != nil {
		panic(err)
	}
	return repo
}
