package gira

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildFeatureMapListReportEmptyIsOptional(t *testing.T) {
	runner := &featureMapRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/backlog --state all --limit 1000 --json number,title,state,labels,body,url": []byte(`[]`),
	}}
	report, err := BuildFeatureMapListReport(FeatureMapOptions{Repo: RepoRef{Owner: "StatPan", Name: "backlog"}}, runner)
	if err != nil {
		t.Fatalf("BuildFeatureMapListReport returned error: %v", err)
	}
	if report.Mode != "none" || report.Counts.Features != 0 {
		t.Fatalf("unexpected empty feature map report: %+v", report)
	}
	out := FormatFeatureMapList(report)
	if !strings.Contains(out, "features: none") || !strings.Contains(out, "create an issue-backed feature record") {
		t.Fatalf("empty output missing optional guidance:\n%s", out)
	}
}

func TestBuildFeatureMapCheckReportDiagnostics(t *testing.T) {
	runner := &featureMapRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/backlog --state all --limit 1000 --json number,title,state,labels,body,url": []byte(`[
			{"number":31,"title":"Capability: Ticket lifecycle","state":"OPEN","body":"Key: tl\nStatus: stable\n\n## User Need\nStart and finish ticket work.\n\n## Capability\nTicket lifecycle.\n\n## Surface\nCLI and JSON.\n\n## Docs\ndocs-site/ticket-workflow.md\n\n## Evidence\nTests and merged PRs.","url":"u31","labels":[{"name":"area:execution"}]},
			{"number":32,"title":"Capability: Goal mode","state":"OPEN","body":"## Capability\nGoal work.","url":"u32","labels":[{"name":"type:capability"},{"name":"capability:experimental"}]},
			{"number":41,"title":"Add finish receipt validation","state":"OPEN","body":"Related capability: #31","url":"u41","labels":[{"name":"type:task"}]},
			{"number":42,"title":"Unlinked task","state":"OPEN","body":"## Goal\nWork.","url":"u42","labels":[{"name":"type:task"}]},
			{"number":43,"title":"Bad link","state":"OPEN","body":"Feature: #999","url":"u43","labels":[{"name":"type:task"}]}
		]`),
	}}
	report, err := BuildFeatureMapCheckReport(FeatureMapOptions{Repo: RepoRef{Owner: "StatPan", Name: "backlog"}}, runner)
	if err != nil {
		t.Fatalf("BuildFeatureMapCheckReport returned error: %v", err)
	}
	if report.Mode != "optional" || report.Counts.Features != 2 || report.Counts.LinkedWork != 2 || report.Counts.MissingLinkWork != 1 {
		t.Fatalf("unexpected check counts: %+v", report.Counts)
	}
	codes := map[string]bool{}
	for _, diagnostic := range report.Diagnostics {
		codes[diagnostic.Code] = true
	}
	for _, want := range []string{"invalid_maturity", "missing_key", "missing_section", "linked_feature_not_found"} {
		if !codes[want] {
			t.Fatalf("missing diagnostic %q in %+v", want, report.Diagnostics)
		}
	}
}

func TestBuildFeatureMapForReportFindsLinkedFeature(t *testing.T) {
	runner := &featureMapRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/backlog --state all --limit 1000 --json number,title,state,labels,body,url": []byte(`[
			{"number":31,"title":"Capability: Ticket lifecycle","state":"OPEN","body":"Key: tl\nStatus: stable\n\n## User Need\nNeed.\n\n## Capability\nCap.\n\n## Surface\nCLI.\n\n## Docs\nDoc.\n\n## Evidence\nEvidence.","url":"u31","labels":[{"name":"area:execution"}]},
			{"number":41,"title":"Add finish receipt validation","state":"OPEN","body":"Related capability: #31","url":"u41","labels":[{"name":"type:task"}]}
		]`),
	}}
	report, err := BuildFeatureMapForReport(FeatureForOptions{Repo: RepoRef{Owner: "StatPan", Name: "backlog"}, Issue: 41}, runner)
	if err != nil {
		t.Fatalf("BuildFeatureMapForReport returned error: %v", err)
	}
	if report.Feature == nil || report.Feature.Number != 31 || report.Feature.Key != "tl" {
		t.Fatalf("unexpected feature link report: %+v", report)
	}
	out := FormatFeatureMapFor(report)
	if !strings.Contains(out, "feature: #31") || !strings.Contains(out, "key=tl") {
		t.Fatalf("output missing feature link:\n%s", out)
	}
}

type featureMapRunner struct {
	outputs map[string][]byte
	calls   []string
}

func (r *featureMapRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}
