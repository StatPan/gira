package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type WorkspaceStatusOptions struct {
	Repos          []RepoRef
	Limit          int
	ActiveOnly     bool
	MaxConcurrency int
	CacheTTL       time.Duration
	Refresh        bool
	CacheRoot      string
}

type WorkspaceRateLimitClient interface {
	FetchRateLimit() (WorkspaceRateLimit, error)
}

type workspaceStatusCache struct {
	enabled bool
	root    string
	ttl     time.Duration
	now     time.Time
	hits    int
	misses  int
	writes  int
	stale   int
	mu      sync.Mutex
}

type workspaceStatusResult struct {
	index   int
	repo    RepoRef
	summary StatusSummary
	err     error
}

func DefaultGiraCacheRoot() (string, error) {
	if root, err := DefaultGlobalConfigRoot(); err == nil {
		if cfg, err := LoadGlobalConfig(root); err == nil && strings.TrimSpace(cfg.Paths.CacheRoot) != "" {
			return expandUserPath(cfg.Paths.CacheRoot), nil
		}
	}
	root, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return filepath.Join(root, "gira"), nil
}

func selectWorkspaceStatusRepos(all []RepoRef, options WorkspaceStatusOptions) ([]RepoRef, error) {
	selected := append([]RepoRef(nil), all...)
	if len(options.Repos) > 0 {
		selected = selected[:0]
		for _, repo := range options.Repos {
			if !workspaceContainsRepo(all, repo) {
				return nil, fmt.Errorf("%s is not in workspace.repos", repo.FullName())
			}
			if !workspaceContainsRepo(selected, repo) {
				selected = append(selected, repo)
			}
		}
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return strings.ToLower(selected[i].FullName()) < strings.ToLower(selected[j].FullName())
	})
	if options.Limit > 0 && options.Limit < len(selected) {
		selected = selected[:options.Limit]
	}
	return selected, nil
}

func newWorkspaceStatusCache(options WorkspaceStatusOptions, now time.Time) (*workspaceStatusCache, error) {
	if options.CacheTTL <= 0 {
		return &workspaceStatusCache{now: now}, nil
	}
	root := strings.TrimSpace(options.CacheRoot)
	if root == "" {
		resolved, err := DefaultGiraCacheRoot()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(resolved, "workspace-status")
	}
	return &workspaceStatusCache{enabled: true, root: root, ttl: options.CacheTTL, now: now}, nil
}

func (c *workspaceStatusCache) summary() WorkspaceStatusCache {
	if c == nil || !c.enabled {
		return WorkspaceStatusCache{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return WorkspaceStatusCache{
		Enabled:    true,
		Root:       c.root,
		TTLSeconds: int(c.ttl.Seconds()),
		Hits:       c.hits,
		Misses:     c.misses,
		Writes:     c.writes,
		Stale:      c.stale,
	}
}

func fetchWorkspaceStatusSummaries(repos []RepoRef, client WorkspaceClient, now time.Time, staleDays int, options WorkspaceStatusOptions, cache *workspaceStatusCache) ([]StatusSummary, error) {
	if len(repos) == 0 {
		return nil, nil
	}
	maxConcurrency := options.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 4
	}
	if maxConcurrency > len(repos) {
		maxConcurrency = len(repos)
	}
	results := make([]workspaceStatusResult, len(repos))
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	for i, repo := range repos {
		i, repo := i, repo
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			summary, err := fetchWorkspaceStatusSummary(repo, client, now, staleDays, options, cache)
			results[i] = workspaceStatusResult{index: i, repo: repo, summary: summary, err: err}
		}()
	}
	wg.Wait()
	summaries := make([]StatusSummary, 0, len(results))
	for _, result := range results {
		if result.err != nil {
			return nil, fmt.Errorf("%s: %w", result.repo.FullName(), result.err)
		}
		summaries = append(summaries, result.summary)
	}
	return summaries, nil
}

func fetchWorkspaceStatusSummary(repo RepoRef, client WorkspaceClient, now time.Time, staleDays int, options WorkspaceStatusOptions, cache *workspaceStatusCache) (StatusSummary, error) {
	if cache != nil && cache.enabled && !options.Refresh {
		if summary, ok := cache.read(repo); ok {
			return summary, nil
		}
	}
	summary, err := client.FetchStatus(repo, now, staleDays)
	if err != nil {
		return StatusSummary{}, err
	}
	if cache != nil && cache.enabled {
		cache.write(repo, summary)
	}
	return summary, nil
}

func (c *workspaceStatusCache) read(repo RepoRef) (StatusSummary, bool) {
	path := c.path(repo)
	info, err := os.Stat(path)
	if err != nil {
		c.addMiss()
		return StatusSummary{}, false
	}
	if c.now.Sub(info.ModTime()) > c.ttl {
		c.addStale()
		return StatusSummary{}, false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		c.addMiss()
		return StatusSummary{}, false
	}
	var summary StatusSummary
	if err := json.Unmarshal(content, &summary); err != nil {
		c.addMiss()
		return StatusSummary{}, false
	}
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
	return summary, true
}

func (c *workspaceStatusCache) write(repo RepoRef, summary StatusSummary) {
	path := c.path(repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	content, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err == nil {
		c.mu.Lock()
		c.writes++
		c.mu.Unlock()
	}
}

func (c *workspaceStatusCache) path(repo RepoRef) string {
	return filepath.Join(c.root, repo.Owner, repo.Name+".json")
}

func (c *workspaceStatusCache) addMiss() {
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
}

func (c *workspaceStatusCache) addStale() {
	c.mu.Lock()
	c.stale++
	c.mu.Unlock()
}

func estimateWorkspaceStatusRequests(config WorkspaceConfigResolved, repos []RepoRef) int {
	estimate := len(repos) * 2
	if !workspaceContainsRepo(config.Repos, config.InboxRepo) {
		estimate++
	}
	return estimate
}

func addWorkspaceStatusSummary(report *WorkspaceReport, summary StatusSummary, repoView WorkspaceRepo) {
	report.Repos = append(report.Repos, repoView)
	report.Counts.RepoOpen += repoView.Open
	report.Counts.Ready += repoView.Ready
	report.Counts.InProgress += repoView.InProgress
	report.Counts.Blocked += repoView.Blocked
	report.Counts.Stale += repoView.Stale
	for _, issue := range summary.Issues.Open {
		report.Backlog = append(report.Backlog, workspaceBacklogFromIssue(summary.Repo, issue))
	}
}

func workspaceRepoIsActive(repo WorkspaceRepo) bool {
	return repo.Open > 0 || repo.Ready > 0 || repo.InProgress > 0 || repo.Blocked > 0 || repo.Stale > 0 || strings.TrimSpace(repo.ActiveMilestone) != ""
}

func actionableGitHubStatusError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "secondary rate") || strings.Contains(lower, "retry-after") || strings.Contains(lower, "http 429") || strings.Contains(lower, "http 403") {
		return fmt.Errorf("GitHub API rate limit while reading status: retry after the reset window, use --cache-ttl, narrow with --repo or --limit, and avoid --refresh for background reads; detail: %w", err)
	}
	return err
}

func expandUserPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(trimmed, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	return trimmed
}
