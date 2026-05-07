package gira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/StatPan/gira/releases/latest"

type UpgradeReport struct {
	Current  string `json:"current"`
	Latest   string `json:"latest"`
	Status   string `json:"status"`
	Channel  string `json:"channel"`
	NextStep string `json:"next_step"`
}

type LatestReleaseFetcher interface {
	LatestReleaseTag() (string, error)
}

type GitHubLatestReleaseFetcher struct {
	Client *http.Client
}

func (f GitHubLatestReleaseFetcher) LatestReleaseTag() (string, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequest(http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gira")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("check latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("check latest release: GitHub returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode latest release: %w", err)
	}
	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", fmt.Errorf("latest release response did not include tag_name")
	}
	return tag, nil
}

func BuildUpgradeReport(channelOverride string, executablePath string, fetcher LatestReleaseFetcher) (UpgradeReport, error) {
	if fetcher == nil {
		fetcher = GitHubLatestReleaseFetcher{}
	}
	current := BuildVersionInfo().Version
	latest, err := fetcher.LatestReleaseTag()
	if err != nil {
		return UpgradeReport{}, err
	}
	channel, err := ResolveUpgradeChannel(channelOverride, executablePath)
	if err != nil {
		return UpgradeReport{}, err
	}
	return UpgradeReport{
		Current:  current,
		Latest:   latest,
		Status:   upgradeStatus(current, latest),
		Channel:  channel,
		NextStep: UpgradeNextStep(channel),
	}, nil
}

func ResolveUpgradeChannel(override string, executablePath string) (string, error) {
	if strings.TrimSpace(override) != "" && strings.TrimSpace(override) != "auto" {
		channel := strings.TrimSpace(override)
		if !validUpgradeChannel(channel) {
			return "", fmt.Errorf("channel must be one of auto, install.sh, uv, pipx, pip, homebrew, npm, bun, go, unknown")
		}
		return channel, nil
	}
	if env := strings.TrimSpace(os.Getenv("GIRA_INSTALL_CHANNEL")); env != "" {
		if !validUpgradeChannel(env) || env == "auto" {
			return "", fmt.Errorf("GIRA_INSTALL_CHANNEL must be one of install.sh, uv, pipx, pip, homebrew, npm, bun, go, unknown")
		}
		return env, nil
	}
	paths := upgradeChannelCandidatePaths(executablePath)
	for _, path := range paths {
		if channel, ok := upgradeChannelFromPath(path); ok {
			return channel, nil
		}
	}
	return "unknown", nil
}

func upgradeChannelCandidatePaths(executablePath string) []string {
	raw := strings.TrimSpace(executablePath)
	if raw == "" {
		return nil
	}
	paths := []string{normalizeUpgradePath(raw)}
	if resolved, err := filepath.EvalSymlinks(raw); err == nil && strings.TrimSpace(resolved) != "" {
		normalized := normalizeUpgradePath(resolved)
		if normalized != paths[0] {
			paths = append(paths, normalized)
		}
	}
	return paths
}

func normalizeUpgradePath(path string) string {
	return strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
}

func upgradeChannelFromPath(path string) (string, bool) {
	switch {
	case strings.Contains(path, "/homebrew/") || strings.Contains(path, "/cellar/") || strings.Contains(path, "/opt/homebrew/"):
		return "homebrew", true
	case strings.Contains(path, "/node_modules/") || strings.Contains(path, "/packages/npm/"):
		return "npm", true
	case strings.Contains(path, "/uv/tools/"):
		return "uv", true
	case strings.Contains(path, "/pipx/"):
		return "pipx", true
	case strings.Contains(path, "/site-packages/") || strings.Contains(path, "/gira-cli/"):
		return "pip", true
	case strings.HasSuffix(path, "/go/bin/gira") || strings.Contains(path, "/gopath/bin/gira"):
		return "go", true
	default:
		return "", false
	}
}

func UpgradeNextStep(channel string) string {
	switch channel {
	case "install.sh":
		return "curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh"
	case "uv":
		return "uv tool upgrade gira-cli"
	case "pipx":
		return "pipx upgrade gira-cli"
	case "pip":
		return "python -m pip install --user --upgrade gira-cli"
	case "homebrew":
		return "brew update && brew upgrade gira"
	case "npm":
		return "npm update -g @statpan/gira"
	case "bun":
		return "bun update -g @statpan/gira"
	case "go":
		return "go install github.com/StatPan/gira/cmd/gira@latest"
	default:
		return "rerun the same installer or package manager that installed gira; use --channel to print a specific command"
	}
}

func FormatUpgradeReport(report UpgradeReport) string {
	return fmt.Sprintf("upgrade: gira\ncurrent: %s\nlatest:  %s\nstatus:  %s\nchannel: %s\n\nnext step:\n  %s\n", report.Current, report.Latest, report.Status, report.Channel, report.NextStep)
}

func validUpgradeChannel(channel string) bool {
	switch channel {
	case "auto", "install.sh", "uv", "pipx", "pip", "homebrew", "npm", "bun", "go", "unknown":
		return true
	default:
		return false
	}
}

func upgradeStatus(current string, latest string) string {
	cmp, ok := compareSemverTags(current, latest)
	if !ok {
		return "unknown"
	}
	if cmp < 0 {
		return "update_available"
	}
	return "up_to_date"
}

func compareSemverTags(a string, b string) (int, bool) {
	left, ok := semverParts(a)
	if !ok {
		return 0, false
	}
	right, ok := semverParts(b)
	if !ok {
		return 0, false
	}
	for i := 0; i < 3; i++ {
		if left[i] < right[i] {
			return -1, true
		}
		if left[i] > right[i] {
			return 1, true
		}
	}
	return 0, true
}

func semverParts(value string) ([3]int, bool) {
	var parts [3]int
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	fields := strings.Split(value, ".")
	if len(fields) != 3 {
		return parts, false
	}
	for i, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || strings.ContainsAny(field, "-+") {
			return parts, false
		}
		n, err := strconv.Atoi(field)
		if err != nil || n < 0 {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}
