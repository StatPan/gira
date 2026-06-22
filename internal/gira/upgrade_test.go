package gira

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeLatestReleaseFetcher struct {
	tag string
	err error
}

func (f fakeLatestReleaseFetcher) LatestReleaseTag() (string, error) {
	return f.tag, f.err
}

func TestBuildUpgradeReportUpdateAvailable(t *testing.T) {
	originalVersion := Version
	t.Cleanup(func() { Version = originalVersion })
	Version = "v1.1.1"

	report, err := BuildUpgradeReport("pipx", "/tmp/gira", fakeLatestReleaseFetcher{tag: "v1.2.0"})
	if err != nil {
		t.Fatalf("BuildUpgradeReport error = %v", err)
	}
	if report.Current != "v1.1.1" || report.Latest != "v1.2.0" || report.Status != "update_available" || report.Channel != "pipx" {
		t.Fatalf("unexpected report: %#v", report)
	}
	if report.NextStep != "pipx upgrade gira-cli" {
		t.Fatalf("next step = %q", report.NextStep)
	}
}

func TestBuildUpgradeReportUpToDate(t *testing.T) {
	originalVersion := Version
	t.Cleanup(func() { Version = originalVersion })
	Version = "v1.2.0"

	report, err := BuildUpgradeReport("install.sh", "/tmp/gira", fakeLatestReleaseFetcher{tag: "v1.2.0"})
	if err != nil {
		t.Fatalf("BuildUpgradeReport error = %v", err)
	}
	if report.Status != "up_to_date" {
		t.Fatalf("status = %q, want up_to_date", report.Status)
	}
}

func TestBuildUpgradeReportDevVersionStatusUnknown(t *testing.T) {
	originalVersion := Version
	t.Cleanup(func() { Version = originalVersion })
	Version = "dev"

	report, err := BuildUpgradeReport("go", "/tmp/gira", fakeLatestReleaseFetcher{tag: "v1.2.0"})
	if err != nil {
		t.Fatalf("BuildUpgradeReport error = %v", err)
	}
	if report.Status != "unknown" {
		t.Fatalf("status = %q, want unknown", report.Status)
	}
}

func TestBuildUpgradeReportLatestFailure(t *testing.T) {
	_, err := BuildUpgradeReport("unknown", "/tmp/gira", fakeLatestReleaseFetcher{err: errors.New("network down")})
	if err == nil {
		t.Fatal("BuildUpgradeReport error = nil, want error")
	}
}

func TestBuildUpgradeReportWithNoticeEmitsOncePerLatestVersion(t *testing.T) {
	originalVersion := Version
	t.Cleanup(func() { Version = originalVersion })
	Version = "v1.1.1"
	root := t.TempDir()

	report, err := BuildUpgradeReportWithOptions(UpgradeOptions{
		ChannelOverride: "pipx",
		ExecutablePath:  "/tmp/gira",
		Fetcher:         fakeLatestReleaseFetcher{tag: "v1.2.0"},
		NotifyOnce:      true,
		NoticeRoot:      root,
	})
	if err != nil {
		t.Fatalf("BuildUpgradeReportWithOptions first error = %v", err)
	}
	if report.Notice == nil || report.Notice.Kind != "new_version" || report.Notice.Version != "v1.2.0" || report.Notice.Status != "emitted" {
		t.Fatalf("unexpected first notice: %#v", report.Notice)
	}
	if report.Notice.StatePath != filepath.Join(root, "notices", "upgrade.json") {
		t.Fatalf("state path = %q", report.Notice.StatePath)
	}
	if _, err := os.Stat(report.Notice.StatePath); err != nil {
		t.Fatalf("notice state was not written: %v", err)
	}

	report, err = BuildUpgradeReportWithOptions(UpgradeOptions{
		ChannelOverride: "pipx",
		ExecutablePath:  "/tmp/gira",
		Fetcher:         fakeLatestReleaseFetcher{tag: "v1.2.0"},
		NotifyOnce:      true,
		NoticeRoot:      root,
	})
	if err != nil {
		t.Fatalf("BuildUpgradeReportWithOptions second error = %v", err)
	}
	if report.Notice == nil || report.Notice.Status != "suppressed" {
		t.Fatalf("unexpected repeated notice: %#v", report.Notice)
	}
	if strings.Contains(FormatUpgradeReport(report), "notice:") {
		t.Fatalf("suppressed notice should not be rendered in human output:\n%s", FormatUpgradeReport(report))
	}
}

func TestBuildUpgradeReportWithNoticeSkipsWhenNoUpdate(t *testing.T) {
	originalVersion := Version
	t.Cleanup(func() { Version = originalVersion })
	Version = "v1.2.0"

	report, err := BuildUpgradeReportWithOptions(UpgradeOptions{
		ChannelOverride: "pipx",
		ExecutablePath:  "/tmp/gira",
		Fetcher:         fakeLatestReleaseFetcher{tag: "v1.2.0"},
		NotifyOnce:      true,
		NoticeRoot:      t.TempDir(),
	})
	if err != nil {
		t.Fatalf("BuildUpgradeReportWithOptions error = %v", err)
	}
	if report.Notice != nil {
		t.Fatalf("notice = %#v, want nil", report.Notice)
	}
}

func TestResolveUpgradeChannel(t *testing.T) {
	t.Setenv("GIRA_INSTALL_CHANNEL", "")
	cases := []struct {
		name     string
		override string
		path     string
		want     string
	}{
		{name: "override", override: "npm", path: "/tmp/gira", want: "npm"},
		{name: "homebrew", path: "/opt/homebrew/bin/gira", want: "homebrew"},
		{name: "npm", path: "/repo/node_modules/.bin/gira", want: "npm"},
		{name: "uv", path: "/home/me/.local/share/uv/tools/gira-cli/bin/gira", want: "uv"},
		{name: "pipx", path: "/home/me/.local/pipx/venvs/gira/bin/gira", want: "pipx"},
		{name: "go", path: "/home/me/go/bin/gira", want: "go"},
		{name: "unknown", path: "/usr/local/bin/gira", want: "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveUpgradeChannel(tc.override, tc.path)
			if err != nil {
				t.Fatalf("ResolveUpgradeChannel error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("channel = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveUpgradeChannelFollowsUVToolSymlink(t *testing.T) {
	t.Setenv("GIRA_INSTALL_CHANNEL", "")
	root := t.TempDir()
	targetDir := filepath.Join(root, ".local", "share", "uv", "tools", "gira-cli", "bin")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	target := filepath.Join(targetDir, "gira")
	if err := os.WriteFile(target, []byte("#!/usr/bin/env sh\n"), 0o755); err != nil {
		t.Fatalf("write target: %v", err)
	}
	linkDir := filepath.Join(root, ".local", "bin")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatalf("mkdir link dir: %v", err)
	}
	link := filepath.Join(linkDir, "gira")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	got, err := ResolveUpgradeChannel("auto", link)
	if err != nil {
		t.Fatalf("ResolveUpgradeChannel error = %v", err)
	}
	if got != "uv" {
		t.Fatalf("channel = %q, want uv", got)
	}
}

func TestResolveUpgradeChannelFromEnv(t *testing.T) {
	t.Setenv("GIRA_INSTALL_CHANNEL", "bun")
	got, err := ResolveUpgradeChannel("auto", "/usr/local/bin/gira")
	if err != nil {
		t.Fatalf("ResolveUpgradeChannel error = %v", err)
	}
	if got != "bun" {
		t.Fatalf("channel = %q, want bun", got)
	}
}

func TestResolveUpgradeChannelRejectsInvalidValues(t *testing.T) {
	t.Setenv("GIRA_INSTALL_CHANNEL", "")
	if _, err := ResolveUpgradeChannel("apt", "/tmp/gira"); err == nil {
		t.Fatal("ResolveUpgradeChannel invalid override error = nil, want error")
	}

	t.Setenv("GIRA_INSTALL_CHANNEL", "apt")
	if _, err := ResolveUpgradeChannel("auto", "/tmp/gira"); err == nil {
		t.Fatal("ResolveUpgradeChannel invalid env error = nil, want error")
	}
}

func TestUpgradeNextStepChannels(t *testing.T) {
	cases := map[string]string{
		"install.sh": "curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh",
		"uv":         "uv tool upgrade gira-cli",
		"pipx":       "pipx upgrade gira-cli",
		"pip":        "python -m pip install --user --upgrade gira-cli",
		"homebrew":   "brew update && brew upgrade gira",
		"npm":        "npm update -g @statpan/gira",
		"bun":        "bun update -g @statpan/gira",
		"go":         "go install github.com/StatPan/gira/cmd/gira@latest",
	}
	for channel, want := range cases {
		if got := UpgradeNextStep(channel); got != want {
			t.Fatalf("UpgradeNextStep(%q) = %q, want %q", channel, got, want)
		}
	}
}
