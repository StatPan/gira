package gira

import "strings"

type statusCategory string

const (
	statusCategoryToDo       statusCategory = "to_do"
	statusCategoryInProgress statusCategory = "in_progress"
	statusCategoryDone       statusCategory = "done"
)

type statusDefinition struct {
	name, label        string
	category           statusCategory
	priority           int
	color, description string
}

// The order also preserves the existing sync label order; backlog is not synced.
var defaultStatuses = []statusDefinition{
	{"Backlog", "status:backlog", statusCategoryToDo, 1, "", ""},
	{"Ready", "status:ready", statusCategoryToDo, 2, "C2E0C6", "Ready to start."},
	{"In progress", "status:in-progress", statusCategoryInProgress, 4, "1D76DB", "Work has started on a branch or active implementation."},
	{"In review", "status:in-review", statusCategoryInProgress, 5, "7057FF", "A linked PR or review surface is active."},
	{"Blocked", "status:blocked", statusCategoryInProgress, 3, "E99695", "Blocked by an external dependency or decision."},
	{"Done", "status:done", statusCategoryDone, 6, "0E8A16", "Work is completed with merged, closed, or accepted completion evidence."},
}

func statusByLabel(label string) (statusDefinition, bool) {
	normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(label)), "_", "-")
	for _, status := range defaultStatuses {
		if status.label == normalized {
			return status, true
		}
	}
	return statusDefinition{}, false
}

func statusToLabel(name string) (string, bool) {
	for _, status := range defaultStatuses {
		if status.name == strings.TrimSpace(name) {
			return status.label, true
		}
	}
	return "", false
}

type derivedStatus struct {
	managed string
	raw     string
}

// deriveStatus preserves raw label spelling for existing reports while using
// the managed priority when more than one status is present.
func deriveStatus(labels []string) derivedStatus {
	result := derivedStatus{raw: "open"}
	foundRaw := false
	best := 0
	count := 0
	bestRaw := ""
	for _, label := range labels {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(label)), "status:") {
			count++
		}
		if !foundRaw && strings.HasPrefix(label, "status:") {
			result.raw = strings.TrimPrefix(label, "status:")
			foundRaw = true
		}
		status, ok := statusByLabel(label)
		if ok && status.priority > best {
			best = status.priority
			result.managed = status.name
			bestRaw = strings.TrimPrefix(status.label, "status:")
			if strings.HasPrefix(label, "status:") {
				bestRaw = strings.TrimPrefix(label, "status:")
			}
		}
	}
	if count > 1 && best > 0 {
		result.raw = bestRaw
	}
	return result
}
