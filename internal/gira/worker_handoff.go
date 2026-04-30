package gira

import (
	"fmt"
	"strings"
	"time"
)

const WorkerHandoffSchemaVersion = "v1"

type WorkerClaim struct {
	Repo          string    `json:"repo"`
	IssueNumber   int       `json:"issue_number"`
	Worker        string    `json:"worker"`
	LeaseUntilUTC time.Time `json:"lease_until_utc"`
	Version       string    `json:"version"`
}

type WorkerHandoffPayload struct {
	SchemaVersion       string   `json:"schema_version"`
	Goal                string   `json:"goal"`
	Context             string   `json:"context"`
	AcceptanceCriteria   []string `json:"acceptance_criteria"`
	VerificationCommands []string `json:"verification_commands"`
	RollbackNotes        string   `json:"rollback_notes"`
}

func ValidateWorkerHandoffPayload(payload WorkerHandoffPayload) error {
	if payload.SchemaVersion != WorkerHandoffSchemaVersion {
		return fmt.Errorf("invalid handoff schema version")
	}
	if strings.TrimSpace(payload.Goal) == "" {
		return fmt.Errorf("missing goal")
	}
	if strings.TrimSpace(payload.Context) == "" {
		return fmt.Errorf("missing context")
	}
	if len(payload.AcceptanceCriteria) == 0 {
		return fmt.Errorf("missing acceptance criteria")
	}
	if len(payload.VerificationCommands) == 0 {
		return fmt.Errorf("missing verification commands")
	}
	if strings.TrimSpace(payload.RollbackNotes) == "" {
		return fmt.Errorf("missing rollback notes")
	}
	return nil
}

func IsLeaseActive(now time.Time, claim WorkerClaim) bool {
	return claim.LeaseUntilUTC.After(now.UTC())
}
