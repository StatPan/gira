package gira

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const DefaultBranch = "chore/gira-bootstrap"

type InstallResult struct {
	Created     []string
	Skipped     []string
	Conflicts   []string
	Overwritten []string
	Branch      string
}

func (r InstallResult) Changed() bool {
	return len(r.Created) > 0 || len(r.Overwritten) > 0
}

func InstallTemplates(targetPath string, rendered []RenderedTemplate, overwrite bool, branch string) (InstallResult, error) {
	resolved, err := resolveInstallRoot(targetPath)
	if err != nil {
		return InstallResult{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return InstallResult{}, fmt.Errorf("target path is not a directory: %s", resolved)
		}
		return InstallResult{}, err
	}
	if !info.IsDir() {
		return InstallResult{}, fmt.Errorf("target path is not a directory: %s", resolved)
	}
	gitInfo, err := os.Lstat(filepath.Join(resolved, ".git"))
	if err != nil {
		if os.IsNotExist(err) {
			return InstallResult{}, fmt.Errorf("target path is not a git repository: %s", resolved)
		}
		return InstallResult{}, err
	}
	if gitInfo.Mode()&os.ModeSymlink != 0 {
		return InstallResult{}, fmt.Errorf("target path is not a git repository: .git is a symlink: %s", resolved)
	}

	if branch != "" {
		if err := ensureBranch(resolved, branch); err != nil {
			return InstallResult{}, err
		}
	}

	result := InstallResult{Branch: branch}
	for _, item := range rendered {
		if !isSafeTemplatePath(item.Path) {
			return InstallResult{}, fmt.Errorf("unsafe template path: %s", item.Path)
		}

		dest := filepath.Join(resolved, filepath.FromSlash(item.Path))
		newBytes := []byte(item.Content)
		if err := ensureInstallParent(resolved, filepath.Dir(dest)); err != nil {
			return InstallResult{}, err
		}
		info, err := os.Lstat(dest)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return InstallResult{}, fmt.Errorf("unsafe template destination: symlink: %s", item.Path)
		}
		if err == nil && info.IsDir() {
			return InstallResult{}, fmt.Errorf("template destination is a directory: %s", item.Path)
		}
		if err != nil && !os.IsNotExist(err) {
			return InstallResult{}, err
		}
		existing, err := os.ReadFile(dest)
		if err == nil {
			if bytes.Equal(existing, newBytes) {
				result.Skipped = append(result.Skipped, item.Path)
				continue
			}
			if !overwrite {
				result.Conflicts = append(result.Conflicts, item.Path)
				continue
			}
			if err := atomicInstallWrite(dest, newBytes, info.Mode().Perm()); err != nil {
				return InstallResult{}, err
			}
			result.Overwritten = append(result.Overwritten, item.Path)
			continue
		}
		if !os.IsNotExist(err) {
			return InstallResult{}, err
		}
		if err := atomicInstallWrite(dest, newBytes, 0o644); err != nil {
			return InstallResult{}, err
		}
		result.Created = append(result.Created, item.Path)
	}

	return result, nil
}

func resolveInstallRoot(targetPath string) (string, error) {
	resolved, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("target path is not a directory: %s", resolved)
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("target path is not a directory: %s", resolved)
	}
	resolved, err = filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve target path symlinks %q: %w", targetPath, err)
	}
	return filepath.Abs(resolved)
}

func ensureInstallParent(root string, parent string) error {
	root = filepath.Clean(root)
	parent = filepath.Clean(parent)
	rel, err := filepath.Rel(root, parent)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe template parent: %s", parent)
	}
	if rel == "." {
		return nil
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
				return fmt.Errorf("create template parent %q: %w", current, err)
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return fmt.Errorf("inspect template parent %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe template parent: symlink: %s", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("template parent is not a directory: %s", current)
		}
	}
	return nil
}

func atomicInstallWrite(dest string, content []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(dest), ".gira-install-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, dest)
}

func FormatInstallSummary(result InstallResult) string {
	var b strings.Builder
	if result.Branch != "" {
		fmt.Fprintf(&b, "branch: %s\n", result.Branch)
	}
	fmt.Fprintf(&b, "created:     %d\n", len(result.Created))
	fmt.Fprintf(&b, "skipped:     %d\n", len(result.Skipped))
	fmt.Fprintf(&b, "overwritten: %d\n", len(result.Overwritten))
	fmt.Fprintf(&b, "conflicts:   %d\n", len(result.Conflicts))
	for _, path := range result.Conflicts {
		fmt.Fprintf(&b, "  conflict: %s\n", path)
	}
	return b.String()
}

func FormatBootstrapInstallSummary(result InstallResult, repo RepoRef) string {
	summary := FormatInstallSummary(result)
	if len(result.Conflicts) == 0 {
		return summary
	}
	var b strings.Builder
	b.WriteString(summary)
	b.WriteString("next:\n")
	b.WriteString("- generated non-conflicting files are still in the worktree\n")
	fmt.Fprintf(&b, "- bind this bootstrap work to a ticket: %s\n", bootstrapContinuationTicketCommand(repo, result))
	b.WriteString("- resolve the listed conflicts, then run: gira ticket pr --apply --draft\n")
	return b.String()
}

func bootstrapContinuationTicketCommand(repo RepoRef, result InstallResult) string {
	notes := "Continue bootstrap"
	if strings.TrimSpace(result.Branch) != "" {
		notes += " from " + result.Branch
	}
	if len(result.Conflicts) > 0 {
		notes += "; resolve conflicts: " + strings.Join(result.Conflicts, ", ")
	}
	return fmt.Sprintf(
		"gira ticket new --repo %s --title %q --type task --notes %q --apply --start",
		repo.FullName(),
		"Adopt Gira bootstrap files",
		notes,
	)
}

func ensureBranch(path string, branch string) error {
	if _, err := runGit(path, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		_, err = runGit(path, "checkout", branch)
		return err
	}
	_, err := runGit(path, "checkout", "-b", branch)
	return err
}

func runGit(path string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"-C", path}, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("%s: %w", detail, err)
		}
		return nil, err
	}
	return output, nil
}

func isSafeTemplatePath(path string) bool {
	if path == "" || filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return false
	}
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return false
		}
	}
	return true
}
