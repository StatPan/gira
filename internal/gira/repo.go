package gira

import (
	"fmt"
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
