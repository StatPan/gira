package gira

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	RunManifestSchemaVersion = "gira-run/v1"
	RunStartReportVersion    = "gira-run-start-report/v1"
	RunStatusReportVersion   = "gira-run-status-report/v1"
)

type RunManifest struct {
	SchemaVersion  string   `json:"schema_version"`
	RunID          string   `json:"run_id"`
	Name           string   `json:"name,omitempty"`
	Repo           string   `json:"repo"`
	Ticket         int      `json:"ticket,omitempty"`
	Role           string   `json:"role,omitempty"`
	Profile        string   `json:"profile,omitempty"`
	Executor       string   `json:"executor"`
	Status         string   `json:"status"`
	StartedAt      string   `json:"started_at"`
	FinishedAt     string   `json:"finished_at,omitempty"`
	WorkDir        string   `json:"work_dir,omitempty"`
	ManifestPath   string   `json:"manifest_path,omitempty"`
	PromptPath     string   `json:"prompt_path,omitempty"`
	EventLogPath   string   `json:"event_log_path,omitempty"`
	StderrLogPath  string   `json:"stderr_log_path,omitempty"`
	ResultPath     string   `json:"result_path,omitempty"`
	Command        []string `json:"command,omitempty"`
	CommandSummary string   `json:"command_summary,omitempty"`
	PID            int      `json:"pid,omitempty"`
	LastObservedAt string   `json:"last_observed_at,omitempty"`
	PublicSafe     bool     `json:"public_safe"`
	PrivateStorage bool     `json:"private_storage"`
	SafeSummary    string   `json:"safe_summary,omitempty"`
}

type RunStartInput struct {
	Repo          RepoRef
	Ticket        int
	Role          string
	Profile       string
	Name          string
	StateRoot     string
	WorkDir       string
	Prompt        []byte
	DryRun        bool
	Apply         bool
	Exec          bool
	Now           time.Time
	RunID         string
	Command       []string
	PID           int
	SafeSummary   string
	PromptSummary *RunPromptSummary
}

type RunStartReport struct {
	SchemaVersion string            `json:"schema_version"`
	DryRun        bool              `json:"dry_run"`
	Exec          bool              `json:"exec"`
	Manifest      RunManifest       `json:"manifest"`
	PromptSummary *RunPromptSummary `json:"prompt_summary,omitempty"`
	NextStep      string            `json:"next_step"`
}

type RunPromptSummary struct {
	SchemaVersion        string   `json:"schema_version,omitempty"`
	Role                 string   `json:"role,omitempty"`
	Profile              string   `json:"profile,omitempty"`
	Readiness            string   `json:"readiness,omitempty"`
	NextAction           string   `json:"next_action,omitempty"`
	NextSafeCommand      string   `json:"next_safe_command,omitempty"`
	IncludedContext      []string `json:"included_context,omitempty"`
	GuidancePointers     []string `json:"guidance_pointers,omitempty"`
	ExtraContextCount    int      `json:"extra_context_count,omitempty"`
	PublicSafe           bool     `json:"public_safe"`
	PrivateStorage       bool     `json:"private_storage"`
	PrivateStorageNotice string   `json:"private_storage_notice,omitempty"`
}

type RunSelectInput struct {
	Repo      RepoRef
	Ticket    int
	RunID     string
	Latest    bool
	StateRoot string
}

type RunStatusReport struct {
	SchemaVersion string        `json:"schema_version"`
	Manifest      *RunManifest  `json:"manifest,omitempty"`
	Matches       []RunManifest `json:"matches,omitempty"`
	NextStep      string        `json:"next_step"`
}

func BuildRunStartReport(input RunStartInput) (RunStartReport, error) {
	if !input.DryRun && !input.Apply {
		return RunStartReport{}, fmt.Errorf("run start requires --dry-run or --apply")
	}
	if input.DryRun && input.Apply {
		return RunStartReport{}, fmt.Errorf("use only one of --dry-run or --apply")
	}
	if !repoRefIsSet(input.Repo) {
		return RunStartReport{}, fmt.Errorf("repo is required")
	}
	if input.Ticket <= 0 {
		return RunStartReport{}, fmt.Errorf("ticket is required")
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	root, err := runStateRoot(input.StateRoot)
	if err != nil {
		return RunStartReport{}, err
	}
	customRunID := strings.TrimSpace(input.RunID) != ""
	runID := strings.TrimSpace(input.RunID)
	if runID == "" {
		runID = generateRunID(now, input.Repo, input.Ticket)
	}
	if !isSafeRunID(runID) {
		return RunStartReport{}, fmt.Errorf("run id must contain only letters, digits, dot, underscore, or dash")
	}
	if input.Apply {
		allocatedRunID, err := allocateRunID(root, runID, input.Repo, input.Ticket, now, customRunID)
		if err != nil {
			return RunStartReport{}, err
		}
		runID = allocatedRunID
	}
	runDir := filepath.Join(root, "runs", runID)
	manifestPath := filepath.Join(runDir, "manifest.json")
	promptPath := filepath.Join(runDir, "prompt.json")
	eventLogPath := filepath.Join(runDir, "events.jsonl")
	stderrLogPath := filepath.Join(runDir, "stderr.log")
	resultPath := filepath.Join(runDir, "result.md")
	status := "prepared"
	if input.DryRun {
		status = "dry-run"
	}
	if input.Exec {
		status = "running"
	}
	command := input.Command
	if len(command) == 0 {
		command = DefaultCodexRunCommand(input.WorkDir, resultPath)
	}
	manifest := RunManifest{
		SchemaVersion:  RunManifestSchemaVersion,
		RunID:          runID,
		Name:           strings.TrimSpace(input.Name),
		Repo:           input.Repo.FullName(),
		Ticket:         input.Ticket,
		Role:           strings.TrimSpace(input.Role),
		Profile:        strings.TrimSpace(input.Profile),
		Executor:       "codex",
		Status:         status,
		StartedAt:      now.Format(time.RFC3339Nano),
		WorkDir:        filepath.Clean(input.WorkDir),
		ManifestPath:   manifestPath,
		PromptPath:     promptPath,
		EventLogPath:   eventLogPath,
		StderrLogPath:  stderrLogPath,
		ResultPath:     resultPath,
		Command:        command,
		CommandSummary: shellQuote(command) + " < " + promptPath + " > " + eventLogPath + " 2> " + stderrLogPath,
		PID:            input.PID,
		PublicSafe:     false,
		PrivateStorage: true,
		SafeSummary:    strings.TrimSpace(input.SafeSummary),
	}
	report := RunStartReport{
		SchemaVersion: RunStartReportVersion,
		DryRun:        input.DryRun,
		Exec:          input.Exec,
		Manifest:      manifest,
		PromptSummary: input.PromptSummary,
		NextStep:      fmt.Sprintf("gira run status --id %s", runID),
	}
	if input.DryRun {
		return report, nil
	}
	if len(input.Prompt) > 0 {
		if err := os.WriteFile(promptPath, input.Prompt, 0o600); err != nil {
			return RunStartReport{}, fmt.Errorf("write run prompt: %w", err)
		}
	}
	if err := WriteRunManifest(manifest); err != nil {
		return RunStartReport{}, err
	}
	return report, nil
}

func allocateRunID(root string, initialRunID string, repo RepoRef, ticket int, now time.Time, customRunID bool) (string, error) {
	if err := os.MkdirAll(filepath.Join(root, "runs"), 0o700); err != nil {
		return "", fmt.Errorf("create run root: %w", err)
	}
	runID := initialRunID
	for attempt := 0; attempt < 5; attempt++ {
		runDir := filepath.Join(root, "runs", runID)
		if err := os.Mkdir(runDir, 0o700); err == nil {
			return runID, nil
		} else if !os.IsExist(err) {
			return "", fmt.Errorf("create run directory: %w", err)
		}
		if customRunID {
			return "", fmt.Errorf("run id %q already exists", runID)
		}
		runID = generateRunID(now, repo, ticket)
	}
	return "", fmt.Errorf("could not allocate unique run id")
}

func DefaultCodexRunCommand(workDir string, resultPath string) []string {
	args := []string{"codex", "exec", "--json"}
	if strings.TrimSpace(workDir) != "" {
		args = append(args, "-C", filepath.Clean(workDir))
	}
	if strings.TrimSpace(resultPath) != "" {
		args = append(args, "-o", filepath.Clean(resultPath))
	}
	return append(args, "-")
}

func WriteRunManifest(manifest RunManifest) error {
	if strings.TrimSpace(manifest.ManifestPath) == "" {
		return fmt.Errorf("manifest path is required")
	}
	if err := os.MkdirAll(filepath.Dir(manifest.ManifestPath), 0o700); err != nil {
		return fmt.Errorf("create run manifest directory: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(manifest.ManifestPath, data, 0o600); err != nil {
		return fmt.Errorf("write run manifest: %w", err)
	}
	return nil
}

func BuildRunStatusReport(input RunSelectInput) (RunStatusReport, error) {
	manifest, matches, err := selectRunManifest(input)
	if err != nil {
		return RunStatusReport{}, err
	}
	next := "run is ready for collection"
	if manifest != nil {
		refreshed, changed := RefreshRunManifest(*manifest, time.Now().UTC())
		if changed {
			if err := WriteRunManifest(refreshed); err != nil {
				return RunStatusReport{}, err
			}
			for i := range matches {
				if matches[i].RunID == refreshed.RunID {
					matches[i] = refreshed
					break
				}
			}
			manifest = &refreshed
		}
		next = fmt.Sprintf("gira run collect --id %s", manifest.RunID)
	}
	return RunStatusReport{SchemaVersion: RunStatusReportVersion, Manifest: manifest, Matches: matches, NextStep: next}, nil
}

func RefreshRunManifest(manifest RunManifest, now time.Time) (RunManifest, bool) {
	if manifest.Status != "running" {
		return manifest, false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	refreshed := manifest
	refreshed.LastObservedAt = now.Format(time.RFC3339)
	if fileHasContent(manifest.ResultPath) {
		refreshed.Status = "completed"
		refreshed.FinishedAt = refreshed.LastObservedAt
		return refreshed, true
	}
	if manifest.PID > 0 && !processRunning(manifest.PID) {
		refreshed.Status = "exited"
		refreshed.FinishedAt = refreshed.LastObservedAt
		return refreshed, true
	}
	return refreshed, refreshed.LastObservedAt != manifest.LastObservedAt
}

func FormatRunStart(report RunStartReport) string {
	m := report.Manifest
	mode := "prepared"
	if report.DryRun {
		mode = "dry-run"
	} else if report.Exec {
		mode = "running"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "run start: %s status=%s repo=%s ticket=#%d\n", m.RunID, mode, m.Repo, m.Ticket)
	fmt.Fprintf(&b, "manifest: %s\n", m.ManifestPath)
	fmt.Fprintf(&b, "prompt: %s\n", m.PromptPath)
	fmt.Fprintf(&b, "events: %s\n", m.EventLogPath)
	fmt.Fprintf(&b, "stderr: %s\n", m.StderrLogPath)
	fmt.Fprintf(&b, "result: %s\n", m.ResultPath)
	if m.PID > 0 {
		fmt.Fprintf(&b, "pid: %d\n", m.PID)
	}
	if report.PromptSummary != nil {
		summary := report.PromptSummary
		fmt.Fprintf(&b, "handoff: schema=%s role=%s readiness=%s next=%s\n", valueOrUnknown(summary.SchemaVersion), valueOrUnknown(summary.Role), valueOrUnknown(summary.Readiness), valueOrUnknown(summary.NextAction))
		if len(summary.IncludedContext) > 0 {
			fmt.Fprintf(&b, "context included: %s\n", strings.Join(summary.IncludedContext, ", "))
		}
		if len(summary.GuidancePointers) > 0 {
			fmt.Fprintf(&b, "policy pointers: %s\n", strings.Join(summary.GuidancePointers, ", "))
		}
		if summary.ExtraContextCount > 0 {
			fmt.Fprintf(&b, "extra context notes: %d\n", summary.ExtraContextCount)
		}
		if strings.TrimSpace(summary.PrivateStorageNotice) != "" {
			fmt.Fprintf(&b, "storage: %s\n", summary.PrivateStorageNotice)
		}
	}
	fmt.Fprintf(&b, "command: %s\n", m.CommandSummary)
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func SummarizeTicketHandoffPrompt(handoff TicketHandoffReport) RunPromptSummary {
	included := []string{
		"ticket metadata",
		"ticket body",
		"goal/scope/acceptance",
		"expected evidence",
		"branch policy",
		"linked PR state",
		"next Gira action",
		"public-safe/private-storage flags",
	}
	if strings.TrimSpace(handoff.WorkOrder.ExpectedDelivery) != "" {
		included = append(included, "expected delivery")
	}
	if strings.TrimSpace(handoff.WorkOrder.ReviewGuidance) != "" {
		included = append(included, "review guidance")
	}
	if handoff.RolePacket != nil {
		included = append(included, "role packet")
	}
	if len(handoff.OperatorContext) > 0 {
		included = append(included, "operator extra notes")
	}
	guidance := []string{}
	for _, item := range handoff.Guidance {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			continue
		}
		status := strings.TrimSpace(item.Status)
		if status != "" {
			path += " (" + status + ")"
		}
		guidance = append(guidance, path)
	}
	notice := strings.TrimSpace(handoff.StorageNotice)
	if notice == "" {
		notice = "prompt is written to private local Gira state; it is not public-safe by default"
	}
	return RunPromptSummary{
		SchemaVersion:        handoff.SchemaVersion,
		Role:                 handoff.Role,
		Profile:              handoff.Profile,
		Readiness:            handoff.Readiness.Readiness,
		NextAction:           handoff.NextAction,
		NextSafeCommand:      handoff.NextSafeCommand,
		IncludedContext:      included,
		GuidancePointers:     guidance,
		ExtraContextCount:    len(handoff.OperatorContext),
		PublicSafe:           handoff.PublicSafe,
		PrivateStorage:       handoff.PrivateStorage,
		PrivateStorageNotice: notice,
	}
}

func FormatRunStatus(report RunStatusReport) string {
	if report.Manifest == nil {
		return "run status: no matching local runs\n"
	}
	m := report.Manifest
	var b strings.Builder
	fmt.Fprintf(&b, "run status: %s status=%s repo=%s ticket=#%d\n", m.RunID, m.Status, m.Repo, m.Ticket)
	if m.PID > 0 {
		fmt.Fprintf(&b, "pid: %d\n", m.PID)
	}
	fmt.Fprintf(&b, "manifest: %s\n", m.ManifestPath)
	fmt.Fprintf(&b, "result: %s\n", m.ResultPath)
	if strings.TrimSpace(m.SafeSummary) != "" {
		fmt.Fprintf(&b, "summary: %s\n", m.SafeSummary)
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func ReadRunResult(manifest RunManifest) (string, error) {
	if strings.TrimSpace(manifest.ResultPath) == "" {
		return "", fmt.Errorf("run result path is missing")
	}
	data, err := os.ReadFile(manifest.ResultPath)
	if err != nil {
		return "", fmt.Errorf("read run result: %w", err)
	}
	return string(data), nil
}

func selectRunManifest(input RunSelectInput) (*RunManifest, []RunManifest, error) {
	root, err := runStateRoot(input.StateRoot)
	if err != nil {
		return nil, nil, err
	}
	runsRoot := filepath.Join(root, "runs")
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read run root: %w", err)
	}
	var matches []RunManifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(runsRoot, entry.Name(), "manifest.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var manifest RunManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}
		if strings.TrimSpace(input.RunID) != "" && manifest.RunID != strings.TrimSpace(input.RunID) {
			continue
		}
		if input.Ticket > 0 && manifest.Ticket != input.Ticket {
			continue
		}
		if repoRefIsSet(input.Repo) && !strings.EqualFold(manifest.Repo, input.Repo.FullName()) {
			continue
		}
		matches = append(matches, manifest)
	}
	sortRunManifests(matches)
	if len(matches) == 0 {
		return nil, nil, nil
	}
	selected := matches[0]
	if strings.TrimSpace(input.RunID) != "" {
		return &selected, matches, nil
	}
	if input.Latest || input.Ticket > 0 || repoRefIsSet(input.Repo) {
		return &selected, matches, nil
	}
	return nil, matches, fmt.Errorf("select a run with --latest, --id, or --ticket")
}

func runStateRoot(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return filepath.Abs(expandUserPath(override))
	}
	root, err := DefaultGlobalConfigRoot()
	if err != nil {
		return "", err
	}
	stateRoot, _, err := DefaultGiraStateRoot(root)
	return stateRoot, err
}

func generateRunID(now time.Time, repo RepoRef, ticket int) string {
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		copy(suffix, []byte{0, 0, 0})
	}
	return fmt.Sprintf("%s-%s-%d-%s", now.UTC().Format("20060102-150405"), runIDSlug(repo.Owner+"-"+repo.Name), ticket, hex.EncodeToString(suffix))
}

func runIDSlug(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "run"
	}
	return slug
}

func isSafeRunID(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func fileHasContent(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Size() > 0
}

func sortRunManifests(matches []RunManifest) {
	sort.Slice(matches, func(i, j int) bool {
		left, leftOK := parseRunTimestamp(matches[i].StartedAt)
		right, rightOK := parseRunTimestamp(matches[j].StartedAt)
		if leftOK && rightOK && !left.Equal(right) {
			return left.After(right)
		}
		if leftOK != rightOK {
			return leftOK
		}
		if matches[i].StartedAt != matches[j].StartedAt {
			return matches[i].StartedAt > matches[j].StartedAt
		}
		return matches[i].RunID > matches[j].RunID
	})
}

func parseRunTimestamp(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func shellQuote(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			quoted = append(quoted, "''")
			continue
		}
		if strings.ContainsAny(arg, " \t\n'\"\\$`") {
			quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}
