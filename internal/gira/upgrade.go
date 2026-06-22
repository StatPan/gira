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
	Current  string         `json:"current"`
	Latest   string         `json:"latest"`
	Status   string         `json:"status"`
	Channel  string         `json:"channel"`
	NextStep string         `json:"next_step"`
	Notice   *UpgradeNotice `json:"notice,omitempty"`
}

type UpgradeNotice struct {
	Kind      string `json:"kind"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	StatePath string `json:"state_path,omitempty"`
}

type UpgradeOptions struct {
	ChannelOverride string
	ExecutablePath  string
	Fetcher         LatestReleaseFetcher
	NotifyOnce      bool
	NoticeRoot      string
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
	return BuildUpgradeReportWithOptions(UpgradeOptions{ChannelOverride: channelOverride, ExecutablePath: executablePath, Fetcher: fetcher})
}

func BuildUpgradeReportWithOptions(options UpgradeOptions) (UpgradeReport, error) {
	fetcher := options.Fetcher
	if fetcher == nil {
		fetcher = GitHubLatestReleaseFetcher{}
	}
	current := BuildVersionInfo().Version
	latest, err := fetcher.LatestReleaseTag()
	if err != nil {
		return UpgradeReport{}, err
	}
	channel, err := ResolveUpgradeChannel(options.ChannelOverride, options.ExecutablePath)
	if err != nil {
		return UpgradeReport{}, err
	}
	report := UpgradeReport{
		Current:  current,
		Latest:   latest,
		Status:   upgradeStatus(current, latest),
		Channel:  channel,
		NextStep: UpgradeNextStep(channel),
	}
	if options.NotifyOnce {
		notice, err := BuildUpgradeNotice(report, options.NoticeRoot)
		if err != nil {
			return UpgradeReport{}, err
		}
		report.Notice = notice
	}
	return report, nil
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
	var b strings.Builder
	fmt.Fprintf(&b, "upgrade: gira\ncurrent: %s\nlatest:  %s\nstatus:  %s\nchannel: %s\n", report.Current, report.Latest, report.Status, report.Channel)
	if report.Notice != nil && report.Notice.Status == "emitted" {
		fmt.Fprintf(&b, "notice: %s\n", report.Notice.Message)
	}
	fmt.Fprintf(&b, "\nnext step:\n  %s\n", report.NextStep)
	return b.String()
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

type upgradeNoticeState struct {
	Latest     string `json:"latest"`
	NotifiedAt string `json:"notified_at"`
	CheckedAt  string `json:"checked_at,omitempty"`
}

func BuildUpgradeNotice(report UpgradeReport, noticeRoot string) (*UpgradeNotice, error) {
	if report.Status != "update_available" {
		return nil, nil
	}
	statePath, err := UpgradeNoticeStatePath(noticeRoot)
	if err != nil {
		return nil, err
	}
	state, err := readUpgradeNoticeState(statePath)
	if err != nil {
		return nil, err
	}
	notice := &UpgradeNotice{
		Kind:      "new_version",
		Version:   report.Latest,
		StatePath: statePath,
	}
	if state.Latest == report.Latest {
		state.CheckedAt = time.Now().UTC().Format(time.RFC3339)
		if err := writeUpgradeNoticeState(statePath, state); err != nil {
			return nil, err
		}
		notice.Status = "suppressed"
		return notice, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := writeUpgradeNoticeState(statePath, upgradeNoticeState{Latest: report.Latest, NotifiedAt: now, CheckedAt: now}); err != nil {
		return nil, err
	}
	notice.Status = "emitted"
	notice.Message = fmt.Sprintf("new Gira release %s is available; inspect next_step before upgrading", report.Latest)
	return notice, nil
}

func ShouldCheckUpgradeNotice(noticeRoot string, now time.Time, interval time.Duration) (bool, error) {
	if interval <= 0 {
		return true, nil
	}
	statePath, err := UpgradeNoticeStatePath(noticeRoot)
	if err != nil {
		return false, err
	}
	state, err := readUpgradeNoticeState(statePath)
	if err != nil {
		return false, err
	}
	checkedAt := strings.TrimSpace(state.CheckedAt)
	if checkedAt == "" {
		return true, nil
	}
	checked, err := time.Parse(time.RFC3339, checkedAt)
	if err != nil {
		return true, nil
	}
	return now.Sub(checked) >= interval, nil
}

func MarkUpgradeNoticeChecked(noticeRoot string, now time.Time) error {
	statePath, err := UpgradeNoticeStatePath(noticeRoot)
	if err != nil {
		return err
	}
	state, err := readUpgradeNoticeState(statePath)
	if err != nil {
		return err
	}
	state.CheckedAt = now.UTC().Format(time.RFC3339)
	return writeUpgradeNoticeState(statePath, state)
}

func UpgradeNoticeStatePath(noticeRoot string) (string, error) {
	root := strings.TrimSpace(noticeRoot)
	if root == "" {
		var err error
		root, err = DefaultGlobalConfigRoot()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(root, "notices", "upgrade.json"), nil
}

func readUpgradeNoticeState(path string) (upgradeNoticeState, error) {
	var state upgradeNoticeState
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, fmt.Errorf("read upgrade notice state: %w", err)
	}
	if err := json.Unmarshal(content, &state); err != nil {
		return state, fmt.Errorf("parse upgrade notice state: %w", err)
	}
	return state, nil
}

func writeUpgradeNoticeState(path string, state upgradeNoticeState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create upgrade notice state directory: %w", err)
	}
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode upgrade notice state: %w", err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write upgrade notice state: %w", err)
	}
	return nil
}
