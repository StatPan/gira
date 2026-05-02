package gira

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const DefaultBlockerReportCooldown = 6 * time.Hour

type ExecutionLoopState struct {
	ActiveIssue         int                          `json:"active_issue"`
	LastClosedIssue     int                          `json:"last_closed_issue"`
	LastMergedPR        int                          `json:"last_merged_pr"`
	LastBlockerByIssue  map[int]ExecutionBlockerNote `json:"last_blocker_by_issue,omitempty"`
	ConsecutiveFailures int                          `json:"consecutive_failures"`
}

type ExecutionBlockerNote struct {
	Reason      string    `json:"reason"`
	ReportedAt  time.Time `json:"reported_at"`
	RepeatCount int       `json:"repeat_count"`
	IssueNumber int       `json:"issue_number"`
	Suppressed  int       `json:"suppressed"`
}

type PostMergeResult struct {
	VerifiedClosed bool `json:"verified_closed"`
	NextIssue      int  `json:"next_issue"`
}

type LoopRunSummary struct {
	Issue      int    `json:"issue"`
	Action     string `json:"action"`
	Result     string `json:"result"`
	Blocker    string `json:"blocker,omitempty"`
	NextIssue  int    `json:"next_issue,omitempty"`
	RetryCount int    `json:"retry_count"`
}

func SelectQueueHead(openIssues []int, state ExecutionLoopState) int {
	if len(openIssues) == 0 {
		return 0
	}
	copyIssues := append([]int(nil), openIssues...)
	sort.Ints(copyIssues)
	if state.ActiveIssue > 0 {
		for _, issue := range copyIssues {
			if issue == state.ActiveIssue {
				return state.ActiveIssue
			}
		}
	}
	return copyIssues[0]
}

func RecordBlocker(state *ExecutionLoopState, issue int, reason string, now time.Time, cooldown time.Duration) bool {
	if state.LastBlockerByIssue == nil {
		state.LastBlockerByIssue = map[int]ExecutionBlockerNote{}
	}
	if cooldown <= 0 {
		cooldown = DefaultBlockerReportCooldown
	}
	reason = strings.TrimSpace(reason)
	note, ok := state.LastBlockerByIssue[issue]
	if !ok {
		state.LastBlockerByIssue[issue] = ExecutionBlockerNote{Reason: reason, ReportedAt: now.UTC(), RepeatCount: 1, IssueNumber: issue}
		return true
	}
	note.RepeatCount++
	if note.Reason == reason && now.UTC().Sub(note.ReportedAt) < cooldown {
		note.Suppressed++
		state.LastBlockerByIssue[issue] = note
		return false
	}
	note.Reason = reason
	note.ReportedAt = now.UTC()
	state.LastBlockerByIssue[issue] = note
	return true
}

func VerifyPostMergeClosureAndNext(mergedIssue int, isIssueStillOpen bool, openIssues []int, state *ExecutionLoopState) PostMergeResult {
	result := PostMergeResult{VerifiedClosed: !isIssueStillOpen}
	if !isIssueStillOpen && mergedIssue > 0 {
		state.LastClosedIssue = mergedIssue
	}
	state.ActiveIssue = SelectQueueHead(openIssues, ExecutionLoopState{})
	result.NextIssue = state.ActiveIssue
	return result
}

func FormatOperationalReport(summary LoopRunSummary) string {
	blocker := summary.Blocker
	if strings.TrimSpace(blocker) == "" {
		blocker = "none"
	}
	line1 := fmt.Sprintf("issue=%d action=%s result=%s", summary.Issue, strings.TrimSpace(summary.Action), strings.TrimSpace(summary.Result))
	line2 := fmt.Sprintf("next_issue=%d retry_count=%d", summary.NextIssue, summary.RetryCount)
	line3 := fmt.Sprintf("blocker=%s", blocker)
	return line1 + "\n" + line2 + "\n" + line3
}
