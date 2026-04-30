package gira

import (
	"testing"
	"time"
)

func TestValidateWorkerHandoffPayload(t *testing.T) {
	payload := WorkerHandoffPayload{
		SchemaVersion:       WorkerHandoffSchemaVersion,
		Goal:                "Implement worker claim protocol",
		Context:             "Issue #72",
		AcceptanceCriteria:  []string{"claims are exclusive"},
		VerificationCommands: []string{"go test ./internal/gira"},
		RollbackNotes:       "revert worker state files",
	}
	if err := ValidateWorkerHandoffPayload(payload); err != nil {
		t.Fatalf("expected valid payload, got %v", err)
	}
}

func TestValidateWorkerHandoffPayloadRejectsMissingFields(t *testing.T) {
	payload := WorkerHandoffPayload{SchemaVersion: WorkerHandoffSchemaVersion}
	if err := ValidateWorkerHandoffPayload(payload); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestIsLeaseActive(t *testing.T) {
	now := time.Now().UTC()
	active := WorkerClaim{LeaseUntilUTC: now.Add(5 * time.Minute)}
	expired := WorkerClaim{LeaseUntilUTC: now.Add(-5 * time.Minute)}

	if !IsLeaseActive(now, active) {
		t.Fatalf("expected active lease")
	}
	if IsLeaseActive(now, expired) {
		t.Fatalf("expected expired lease")
	}
}
