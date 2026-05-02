package gira

import (
	"fmt"
	"strings"
	"testing"
)

type initRunner struct {
	outputs map[string][]byte
	errs    map[string]error
}

func (r *initRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestBuildInitReportReady(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	path := "/repo"
	runner := &initRunner{outputs: map[string][]byte{
		"gh --version":                                 []byte("gh version 2"),
		"git --version":                                []byte("git version 2"),
		"gh auth status":                               []byte("ok"),
		"gh repo view StatPan/gira --json name":        []byte(`{"name":"gira"}`),
		"git -C /repo rev-parse --is-inside-work-tree": []byte("true"),
		"git -C /repo diff --quiet":                    nil,
		"git -C /repo diff --cached --quiet":           nil,
	}}
	report, err := BuildInitReport(repo, path, true, runner)
	if err != nil {
		t.Fatalf("BuildInitReport error: %v", err)
	}
	if !report.Ready {
		t.Fatalf("expected ready report: %+v", report)
	}
	if report.NextStep == "" || !strings.Contains(report.NextStep, "gira bootstrap") {
		t.Fatalf("unexpected next step: %q", report.NextStep)
	}
}

func TestBuildInitReportFailsWithRemediation(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &initRunner{
		outputs: map[string][]byte{"git --version": []byte("git version 2")},
		errs:    map[string]error{"gh --version": fmt.Errorf("not found")},
	}
	report, err := BuildInitReport(repo, "/repo", true, runner)
	if err == nil {
		t.Fatal("expected error for missing prerequisites")
	}
	if report.Ready {
		t.Fatal("report unexpectedly ready")
	}
	if report.Remediations["gh_installed"] == "" {
		t.Fatalf("missing remediation in report: %+v", report)
	}
}
