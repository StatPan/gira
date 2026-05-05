package gira

import (
	"fmt"
	"strings"
)

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func BuildVersionInfo() VersionInfo {
	return VersionInfo{
		Version: normalizeBuildValue(Version, "dev"),
		Commit:  normalizeBuildValue(Commit, "unknown"),
		Date:    normalizeBuildValue(Date, "unknown"),
	}
}

func FormatVersionInfo(info VersionInfo) string {
	return fmt.Sprintf("gira %s (%s, %s)\n", info.Version, info.Commit, info.Date)
}

func normalizeBuildValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
