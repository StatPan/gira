package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const cachePruneCommand = "cache prune"
const CachePruneReportSchemaVersion = "cache-prune-report/v1"

type CachePruneOptions struct {
	Root           string
	ActiveVersion  string
	ExecutablePath string
	DryRun         bool
	Apply          bool
}

type CachePruneReport struct {
	SchemaVersion    string             `json:"schema_version,omitempty"`
	Command          string             `json:"command"`
	Root             string             `json:"root"`
	ActiveVersion    string             `json:"active_version"`
	ActiveComparable bool               `json:"active_comparable"`
	DryRun           bool               `json:"dry_run"`
	Apply            bool               `json:"apply"`
	Counts           CachePruneCounts   `json:"counts"`
	Actions          []CachePruneAction `json:"actions"`
	Approval         *ApprovalEvidence  `json:"approval,omitempty"`
}

func EnsureCachePruneReportSchema(report *CachePruneReport) {
	if report != nil && strings.TrimSpace(report.SchemaVersion) == "" {
		report.SchemaVersion = CachePruneReportSchemaVersion
	}
}

type CachePruneCounts struct {
	Planned int `json:"planned"`
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	Errors  int `json:"errors"`
}

type CachePruneAction struct {
	Action  string `json:"action"`
	Status  string `json:"status"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	Reason  string `json:"reason"`
	Error   string `json:"error,omitempty"`
}

func DefaultCachePruneRoot() (string, error) {
	if override := strings.TrimSpace(os.Getenv("GIRA_PYPI_CACHE_DIR")); override != "" {
		return filepath.Abs(expandHome(override))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "gira-cli"), nil
}

func BuildCachePruneReport(options CachePruneOptions) (CachePruneReport, error) {
	if options.DryRun == options.Apply {
		return CachePruneReport{}, fmt.Errorf("exactly one of dry-run or apply is required")
	}
	root := strings.TrimSpace(options.Root)
	var err error
	if root == "" {
		root, err = DefaultCachePruneRoot()
		if err != nil {
			return CachePruneReport{}, fmt.Errorf("resolve cache root: %w", err)
		}
	} else {
		root, err = filepath.Abs(expandHome(root))
		if err != nil {
			return CachePruneReport{}, fmt.Errorf("resolve cache root: %w", err)
		}
	}

	activeVersion := normalizeBuildValue(options.ActiveVersion, "dev")
	_, activeComparable := semverParts(activeVersion)
	report := CachePruneReport{
		SchemaVersion:    CachePruneReportSchemaVersion,
		Command:          cachePruneCommand,
		Root:             root,
		ActiveVersion:    activeVersion,
		ActiveComparable: activeComparable,
		DryRun:           options.DryRun,
		Apply:            options.Apply,
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			if options.DryRun {
				report.Approval = CachePruneApprovalEvidence(report)
			}
			return report, nil
		}
		return CachePruneReport{}, fmt.Errorf("read cache root: %w", err)
	}

	executablePath := normalizeComparablePath(options.ExecutablePath)
	for _, entry := range entries {
		action := planCachePruneEntry(root, entry, activeVersion, activeComparable, executablePath, options)
		if action.Status == "planned" && options.Apply {
			if err := os.RemoveAll(action.Path); err != nil {
				action.Status = "error"
				action.Error = err.Error()
				action.Reason = "remove stale version directory failed"
			} else {
				action.Status = "applied"
				action.Reason = "removed stale version directory"
			}
		}
		report.Actions = append(report.Actions, action)
		switch action.Status {
		case "planned":
			report.Counts.Planned++
		case "applied":
			report.Counts.Applied++
		case "error":
			report.Counts.Errors++
		default:
			report.Counts.Skipped++
		}
	}

	if options.DryRun {
		report.Approval = CachePruneApprovalEvidence(report)
	}
	return report, nil
}

func FormatCachePruneReport(report CachePruneReport) string {
	mode := "dry-run"
	if !report.DryRun {
		mode = "apply"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "cache prune: gira\nroot: %s\nactive: %s\nmode: %s\n", report.Root, report.ActiveVersion, mode)
	fmt.Fprintf(&b, "counts: planned=%d applied=%d skipped=%d errors=%d\n", report.Counts.Planned, report.Counts.Applied, report.Counts.Skipped, report.Counts.Errors)
	for _, action := range report.Actions {
		fmt.Fprintf(&b, "- %s %s: %s", action.Status, action.Action, action.Name)
		if action.Reason != "" {
			fmt.Fprintf(&b, " (%s)", action.Reason)
		}
		if action.Error != "" {
			fmt.Fprintf(&b, ": %s", action.Error)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func planCachePruneEntry(root string, entry os.DirEntry, activeVersion string, activeComparable bool, executablePath string, options CachePruneOptions) CachePruneAction {
	name := entry.Name()
	path := filepath.Join(root, name)
	action := CachePruneAction{Action: "skip", Status: "skipped", Name: name, Path: path}

	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		action.Reason = "entry is outside cache root"
		return action
	}
	if strings.Contains(rel, string(filepath.Separator)) {
		action.Reason = "entry is not a direct child of cache root"
		return action
	}

	info, err := entry.Info()
	if err != nil {
		action.Status = "error"
		action.Error = err.Error()
		action.Reason = "inspect cache entry failed"
		return action
	}
	if info.Mode()&os.ModeSymlink != 0 {
		action.Reason = "symlink entries are never deleted"
		return action
	}
	if !info.IsDir() {
		action.Reason = "files are never deleted"
		return action
	}

	if _, ok := semverParts(name); !ok {
		action.Reason = "entry name is not a stable semver release"
		return action
	}
	action.Version = name

	if executablePath != "" && pathContainsComparablePath(path, executablePath) {
		action.Reason = "directory contains the current executable"
		return action
	}
	if !activeComparable {
		action.Reason = "active version is not a stable comparable release"
		return action
	}
	cmp, ok := compareSemverTags(name, activeVersion)
	if !ok {
		action.Reason = "entry version is not comparable"
		return action
	}
	if cmp == 0 {
		action.Reason = "active version is never deleted"
		return action
	}
	if cmp > 0 {
		action.Reason = "newer versions are never deleted"
		return action
	}

	action.Action = "prune"
	if options.Apply {
		action.Status = "planned"
		action.Reason = "stale version directory"
		return action
	}
	action.Status = "planned"
	action.Reason = "would remove stale version directory"
	return action
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func normalizeComparablePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil && strings.TrimSpace(resolved) != "" {
		abs = resolved
	}
	return filepath.Clean(abs)
}

func pathContainsComparablePath(parent string, child string) bool {
	parent = normalizeComparablePath(parent)
	child = normalizeComparablePath(child)
	if parent == "" || child == "" {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel))
}
