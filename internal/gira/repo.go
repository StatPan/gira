package gira

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type RepoRef struct {
	Owner string
	Name  string
}

type RepoContextOptions struct {
	RepoValue  string
	ConfigRoot string
	WorkDir    string
	Runner     CommandRunner
}

type ResolvedRepoContext struct {
	Repo           RepoRef
	Source         string
	Detail         string
	GlobalRepo     *GlobalRepoRegistryEntry
	GlobalRepoPath string
}

func (r RepoRef) FullName() string {
	return r.Owner + "/" + r.Name
}

func ParseRepoRef(value string) (RepoRef, error) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return RepoRef{}, fmt.Errorf("repo must be in OWNER/REPO format")
	}
	return RepoRef{Owner: parts[0], Name: parts[1]}, nil
}

func ResolveRepoContext(repoValue string, runner CommandRunner) (RepoRef, error) {
	ctx, err := ResolveRepoContextDetails(RepoContextOptions{RepoValue: repoValue, Runner: runner})
	if err != nil {
		return RepoRef{}, err
	}
	return ctx.Repo, nil
}

func ResolveRepoContextDetails(options RepoContextOptions) (ResolvedRepoContext, error) {
	runner := options.Runner
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	workDir, err := normalizeRepoContextWorkDir(options.WorkDir)
	if err != nil {
		return ResolvedRepoContext{}, err
	}
	repoValue := strings.TrimSpace(options.RepoValue)
	if repoValue != "" {
		repo, err := ParseRepoRef(repoValue)
		if err == nil {
			return ResolvedRepoContext{Repo: repo, Source: "explicit", Detail: "--repo OWNER/REPO"}, nil
		}
		if ctx, ok, err := repoContextFromGlobalAlias(options.ConfigRoot, repoValue); err != nil {
			return ResolvedRepoContext{}, err
		} else if ok {
			return ctx, nil
		}
		return ResolvedRepoContext{}, fmt.Errorf("repo context unavailable: --repo must be OWNER/REPO or a registered global repo alias")
	}
	if repo, ok, err := repoContextFromGitOrigin(workDir, runner); err != nil {
		return ResolvedRepoContext{}, err
	} else if ok {
		ctx := ResolvedRepoContext{Repo: repo, Source: "git_origin", Detail: "git remote get-url origin"}
		if entry, path, ok, err := loadOptionalGlobalRepoRegistryEntry(options.ConfigRoot, repo); err != nil {
			return ResolvedRepoContext{}, err
		} else if ok {
			ctx.GlobalRepo = &entry
			ctx.GlobalRepoPath = path
		}
		return ctx, nil
	}
	if ctx, ok, err := repoContextFromGlobalPath(options.ConfigRoot, workDir); err != nil {
		return ResolvedRepoContext{}, err
	} else if ok {
		return ctx, nil
	}
	if repo, ok, err := repoContextFromConfig(DefaultInitConfigPath(workDir)); err != nil {
		return ResolvedRepoContext{}, err
	} else if ok {
		return ResolvedRepoContext{Repo: repo, Source: "repo_config", Detail: DefaultInitConfigPath(workDir)}, nil
	}
	tomlPath := filepath.Join(workDir, ".gira", "config.toml")
	if repo, ok, err := repoContextFromConfig(tomlPath); err != nil {
		return ResolvedRepoContext{}, err
	} else if ok {
		return ResolvedRepoContext{Repo: repo, Source: "repo_config", Detail: tomlPath}, nil
	}
	return ResolvedRepoContext{}, fmt.Errorf("repo context unavailable: pass --repo OWNER/REPO, register a global repo alias/path, set repo in .gira/config.yaml, or run from a git checkout with a GitHub origin remote")
}

func repoContextFromConfig(path string) (RepoRef, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RepoRef{}, false, nil
		}
		return RepoRef{}, false, fmt.Errorf("read repo context config %q: %w", path, err)
	}
	var cfg struct {
		Repo string `yaml:"repo" toml:"repo"`
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".toml":
		if err := toml.Unmarshal(content, &cfg); err != nil {
			return RepoRef{}, false, fmt.Errorf("parse repo context config %q: %w", path, err)
		}
	default:
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			return RepoRef{}, false, fmt.Errorf("parse repo context config %q: %w", path, err)
		}
	}
	if strings.TrimSpace(cfg.Repo) == "" {
		return RepoRef{}, false, nil
	}
	repo, err := ParseRepoRef(cfg.Repo)
	if err != nil {
		return RepoRef{}, false, fmt.Errorf("invalid repo context config %q: repo must be in OWNER/REPO format", path)
	}
	return repo, true, nil
}

func repoContextFromGitOrigin(workDir string, runner CommandRunner) (RepoRef, bool, error) {
	var output []byte
	var err error
	if workDir == "." {
		output, err = runner.Run("git", "remote", "get-url", "origin")
	} else {
		output, err = runner.Run("git", "-C", workDir, "remote", "get-url", "origin")
	}
	if err != nil {
		return RepoRef{}, false, nil
	}
	repo, err := ParseGitHubRemoteRepo(strings.TrimSpace(string(output)))
	if err != nil {
		return RepoRef{}, false, fmt.Errorf("repo context unavailable: origin remote is not a GitHub OWNER/REPO URL; pass --repo OWNER/REPO or set repo in .gira/config.yaml")
	}
	return repo, true, nil
}

func repoContextFromGlobalAlias(configRoot string, alias string) (ResolvedRepoContext, bool, error) {
	entries, err := loadGlobalRepoRegistryEntries(configRoot)
	if err != nil {
		return ResolvedRepoContext{}, false, err
	}
	needle := strings.TrimSpace(alias)
	var matches []ResolvedRepoContext
	for _, candidate := range entries {
		for _, value := range candidate.Entry.Aliases {
			if strings.EqualFold(strings.TrimSpace(value), needle) {
				matches = append(matches, candidate.Context("global_alias", candidate.Path))
			}
		}
	}
	if len(matches) > 1 {
		return ResolvedRepoContext{}, false, fmt.Errorf("repo context unavailable: global repo alias %q matches multiple repos", alias)
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	return ResolvedRepoContext{}, false, nil
}

func repoContextFromGlobalPath(configRoot string, workDir string) (ResolvedRepoContext, bool, error) {
	entries, err := loadGlobalRepoRegistryEntries(configRoot)
	if err != nil {
		return ResolvedRepoContext{}, false, err
	}
	workPath, err := filepath.Abs(workDir)
	if err != nil {
		return ResolvedRepoContext{}, false, fmt.Errorf("resolve repo context path %q: %w", workDir, err)
	}
	workPath = filepath.Clean(workPath)
	var match ResolvedRepoContext
	bestLength := -1
	duplicateBest := false
	for _, candidate := range entries {
		base, ok := repoContextPathMatchBase(candidate.Entry.Path, workPath)
		if !ok {
			continue
		}
		if len(base) > bestLength {
			match = candidate.Context("global_path", candidate.Path)
			bestLength = len(base)
			duplicateBest = false
			continue
		}
		if len(base) == bestLength {
			duplicateBest = true
		}
	}
	if duplicateBest {
		return ResolvedRepoContext{}, false, fmt.Errorf("repo context unavailable: global repo paths match multiple repos")
	}
	if bestLength >= 0 {
		return match, true, nil
	}
	return ResolvedRepoContext{}, false, nil
}

type globalRepoRegistryCandidate struct {
	Repo  RepoRef
	Entry GlobalRepoRegistryEntry
	Path  string
}

func (c globalRepoRegistryCandidate) Context(source string, detail string) ResolvedRepoContext {
	entry := c.Entry
	return ResolvedRepoContext{
		Repo:           c.Repo,
		Source:         source,
		Detail:         detail,
		GlobalRepo:     &entry,
		GlobalRepoPath: c.Path,
	}
}

func loadOptionalGlobalRepoRegistryEntry(configRoot string, repo RepoRef) (GlobalRepoRegistryEntry, string, bool, error) {
	path, err := GlobalRepoRegistryPath(configRoot, repo)
	if err != nil {
		return GlobalRepoRegistryEntry{}, "", false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return GlobalRepoRegistryEntry{}, path, false, nil
		}
		return GlobalRepoRegistryEntry{}, path, false, fmt.Errorf("inspect global repo registry %q: %w", path, err)
	}
	entry, err := LoadGlobalRepoRegistryEntry(configRoot, repo)
	if err != nil {
		return GlobalRepoRegistryEntry{}, path, false, err
	}
	return entry, path, true, nil
}

func loadGlobalRepoRegistryEntries(configRoot string) ([]globalRepoRegistryCandidate, error) {
	root, err := GlobalReposRoot(configRoot)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect global repo registry root %q: %w", root, err)
	}
	var entries []globalRepoRegistryCandidate
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".yaml") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		repoValue := filepath.ToSlash(strings.TrimSuffix(rel, filepath.Ext(rel)))
		repo, err := ParseRepoRef(repoValue)
		if err != nil {
			return fmt.Errorf("invalid global repo registry path %q: expected OWNER/REPO.yaml", path)
		}
		entry, err := LoadGlobalRepoRegistryEntry(configRoot, repo)
		if err != nil {
			return err
		}
		entries = append(entries, globalRepoRegistryCandidate{Repo: repo, Entry: entry, Path: path})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func repoContextPathMatchBase(registeredPath string, workPath string) (string, bool) {
	base, ok := normalizeRepoContextRegistryPath(registeredPath)
	if !ok {
		return "", false
	}
	if samePath(base, workPath) {
		return base, true
	}
	rel, err := filepath.Rel(base, workPath)
	if err != nil {
		return "", false
	}
	if rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return base, true
	}
	return "", false
}

func normalizeRepoContextRegistryPath(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsRune(trimmed, 0) {
		return "", false
	}
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		if trimmed == "~" {
			trimmed = home
		} else {
			trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", false
	}
	return filepath.Clean(abs), true
}

func normalizeRepoContextWorkDir(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ".", nil
	}
	if strings.ContainsRune(trimmed, 0) {
		return "", fmt.Errorf("repo context workdir must not contain NUL bytes")
	}
	return filepath.Clean(trimmed), nil
}

func samePath(a string, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func ParseGitHubRemoteRepo(remoteURL string) (RepoRef, error) {
	value := strings.TrimSpace(remoteURL)
	if value == "" {
		return RepoRef{}, fmt.Errorf("remote URL is empty")
	}

	if !strings.Contains(value, "://") {
		if repo, ok := parseSCPStyleGitHubRemote(value); ok {
			return repo, nil
		}
		return RepoRef{}, fmt.Errorf("remote URL is not a GitHub URL")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return RepoRef{}, fmt.Errorf("parse remote URL: %w", err)
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") {
		return RepoRef{}, fmt.Errorf("remote URL host is not github.com")
	}
	return repoFromRemotePath(parsed.Path)
}

func parseSCPStyleGitHubRemote(value string) (RepoRef, bool) {
	hostAndPath := strings.SplitN(value, ":", 2)
	if len(hostAndPath) != 2 {
		return RepoRef{}, false
	}
	host := hostAndPath[0]
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if !strings.EqualFold(host, "github.com") {
		return RepoRef{}, false
	}
	repo, err := repoFromRemotePath(hostAndPath[1])
	if err != nil {
		return RepoRef{}, false
	}
	return repo, true
}

func repoFromRemotePath(remotePath string) (RepoRef, error) {
	pathValue := strings.Trim(strings.TrimSpace(remotePath), "/")
	if strings.HasSuffix(pathValue, ".git") {
		pathValue = strings.TrimSuffix(pathValue, ".git")
	}
	return ParseRepoRef(pathValue)
}
