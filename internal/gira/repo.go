package gira

import (
	"fmt"
	"net/url"
	"strings"
)

type RepoRef struct {
	Owner string
	Name  string
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
	if strings.TrimSpace(repoValue) != "" {
		return ParseRepoRef(repoValue)
	}
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	output, err := runner.Run("git", "remote", "get-url", "origin")
	if err != nil {
		return RepoRef{}, fmt.Errorf("repo context unavailable: pass --repo OWNER/REPO or run from a git checkout with a GitHub origin remote")
	}
	repo, err := ParseGitHubRemoteRepo(strings.TrimSpace(string(output)))
	if err != nil {
		return RepoRef{}, fmt.Errorf("repo context unavailable: origin remote is not a GitHub OWNER/REPO URL; pass --repo OWNER/REPO")
	}
	return repo, nil
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
