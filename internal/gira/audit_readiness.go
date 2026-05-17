package gira

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	AuditReadinessStatusOK      = "ok"
	AuditReadinessStatusMissing = "missing"
	AuditReadinessStatusFailed  = "failed"

	AuditReadinessModeDailyOperation = "daily_operation"
	AuditReadinessModeNoOpenWork     = "no_open_work"
)

type AuditReadinessReport struct {
	Repo      string               `json:"repo"`
	Command   string               `json:"command"`
	Mode      string               `json:"mode"`
	Ready     bool                 `json:"ready"`
	CheckedAt string               `json:"checked_at"`
	Doctor    DoctorReport         `json:"doctor"`
	Audit     AuditReadinessHealth `json:"audit"`
	NextStep  string               `json:"next_step"`
}

type AuditReadinessHealth struct {
	Status      string            `json:"status"`
	Detail      string            `json:"detail"`
	Remediation string            `json:"remediation,omitempty"`
	Verify      AuditVerifyReport `json:"verify"`
}

func BuildAuditReadinessReport(repo RepoRef, ledgerPath string, runner CommandRunner, checkedAt time.Time) AuditReadinessReport {
	if strings.TrimSpace(ledgerPath) == "" {
		ledgerPath = ".gira/audit/*.jsonl"
	}
	doctor := BuildDoctorReport(repo.FullName(), runner, checkedAt)
	verify := AuditVerifyReport{}
	if _, err := filepath.Glob(ledgerPath); err != nil {
		verify = AuditVerifyReport{Valid: false, Failure: "invalid_audit_glob"}
	} else {
		verify = VerifyAuditLedgerForRepo(ledgerPath, repo)
	}
	audit := auditReadinessHealth(repo, ledgerPath, verify)
	doctor, mode := auditReadinessDoctorMode(doctor)
	report := AuditReadinessReport{
		Repo:      repo.FullName(),
		Command:   "audit readiness",
		Mode:      mode,
		Ready:     doctor.Ready && audit.Status != AuditReadinessStatusFailed,
		CheckedAt: checkedAt.UTC().Format(time.RFC3339),
		Doctor:    doctor,
		Audit:     audit,
	}
	report.NextStep = auditReadinessNextStep(report, ledgerPath)
	return report
}

func FormatAuditReadinessReport(report AuditReadinessReport) string {
	verdict := "READY"
	if !report.Ready {
		verdict = "NOT READY"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "audit readiness: %s\n", verdict)
	if strings.TrimSpace(report.Repo) != "" {
		fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	}
	if strings.TrimSpace(report.CheckedAt) != "" {
		fmt.Fprintf(&b, "checked_at: %s\n", report.CheckedAt)
	}
	if strings.TrimSpace(report.Mode) != "" {
		fmt.Fprintf(&b, "mode: %s\n", report.Mode)
	}
	fmt.Fprintln(&b, "\nreadiness/doctor checks:")
	for _, check := range report.Doctor.Checks {
		fmt.Fprintf(&b, "- [%s] %s: %s\n", check.Status, check.ID, check.Detail)
		if check.Status == DoctorCheckFail && strings.TrimSpace(check.Remediation) != "" {
			fmt.Fprintf(&b, "  remediation: %s\n", check.Remediation)
		}
	}
	fmt.Fprintln(&b, "\naudit ledger health:")
	fmt.Fprintf(&b, "- [%s] audit_ledger: %s\n", auditReadinessHumanStatus(report.Audit.Status), report.Audit.Detail)
	if strings.TrimSpace(report.Audit.Remediation) != "" {
		fmt.Fprintf(&b, "  remediation: %s\n", report.Audit.Remediation)
	}
	fmt.Fprintf(&b, "\nnext step: %s\n", report.NextStep)
	return b.String()
}

func auditReadinessDoctorMode(doctor DoctorReport) (DoctorReport, string) {
	if doctor.Ready {
		return doctor, AuditReadinessModeDailyOperation
	}
	if !auditReadinessOnlyNoOpenWorkFailed(doctor) {
		return doctor, AuditReadinessModeDailyOperation
	}
	out := doctor
	out.Checks = append([]DoctorCheck(nil), doctor.Checks...)
	for i, check := range out.Checks {
		if auditReadinessNoOpenWorkCheck(check) {
			out.Checks[i].Status = DoctorCheckWarn
			out.Checks[i].Remediation = "repository has no open work; create or sync ready issues when starting the next daily operation cycle"
			break
		}
	}
	out.Ready = true
	return out, AuditReadinessModeNoOpenWork
}

func auditReadinessOnlyNoOpenWorkFailed(doctor DoctorReport) bool {
	found := false
	for _, check := range doctor.Checks {
		if check.Status != DoctorCheckFail {
			continue
		}
		if !auditReadinessNoOpenWorkCheck(check) {
			return false
		}
		found = true
	}
	return found
}

func auditReadinessNoOpenWorkCheck(check DoctorCheck) bool {
	return check.ID == "onboard_readiness" && strings.TrimSpace(check.Detail) == "open issues=0"
}

func auditReadinessHealth(repo RepoRef, ledgerPath string, verify AuditVerifyReport) AuditReadinessHealth {
	if verify.Valid {
		return AuditReadinessHealth{
			Status: AuditReadinessStatusOK,
			Detail: fmt.Sprintf("verified %d records across %d file(s)", verify.Records, len(verify.Files)),
			Verify: verify,
		}
	}
	if verify.Failure == "no_audit_files_found" || verify.Failure == "no_audit_records" {
		detail := fmt.Sprintf("no audit ledger found for %s at %s", repo.FullName(), ledgerPath)
		if verify.Failure == "no_audit_records" {
			detail = fmt.Sprintf("audit ledger for %s at %s has no records yet", repo.FullName(), ledgerPath)
		}
		return AuditReadinessHealth{
			Status:      AuditReadinessStatusMissing,
			Detail:      detail,
			Remediation: "run a Gira mutation command when ready, then rerun `gira audit readiness`",
			Verify:      verify,
		}
	}
	detail := verify.Failure
	if strings.TrimSpace(detail) == "" {
		detail = "audit ledger verification failed"
	}
	if verify.FailureFile != "" {
		detail = fmt.Sprintf("%s in %s", detail, verify.FailureFile)
	}
	if verify.FailureLine > 0 {
		detail = fmt.Sprintf("%s line %d", detail, verify.FailureLine)
	}
	return AuditReadinessHealth{
		Status:      AuditReadinessStatusFailed,
		Detail:      detail,
		Remediation: fmt.Sprintf("fix the audit ledger, then run `gira audit verify --repo %s --path %s`", repo.FullName(), ledgerPath),
		Verify:      verify,
	}
}

func auditReadinessNextStep(report AuditReadinessReport, ledgerPath string) string {
	repoFlag := ""
	if strings.TrimSpace(report.Repo) != "" {
		repoFlag = " --repo " + report.Repo
	}
	if report.Audit.Status == AuditReadinessStatusFailed {
		return fmt.Sprintf("fix audit ledger corruption, then run `gira audit verify%s --path %s`", repoFlag, ledgerPath)
	}
	if report.Mode == AuditReadinessModeNoOpenWork {
		return fmt.Sprintf("no open work; run `gira status%s` for completion evidence or create a ready ticket when starting the next cycle", repoFlag)
	}
	if !report.Doctor.Ready {
		for _, check := range report.Doctor.Checks {
			if check.Status == DoctorCheckFail {
				if strings.TrimSpace(check.Remediation) != "" {
					return fmt.Sprintf("fix %s: %s; then run `gira audit readiness%s --path %s`", check.ID, check.Remediation, repoFlag, ledgerPath)
				}
				return fmt.Sprintf("fix %s, then run `gira audit readiness%s --path %s`", check.ID, repoFlag, ledgerPath)
			}
		}
		return fmt.Sprintf("fix readiness checks, then run `gira audit readiness%s --path %s`", repoFlag, ledgerPath)
	}
	if report.Audit.Status == AuditReadinessStatusMissing {
		return fmt.Sprintf("run `gira status%s`, then create or start work with `gira ticket new ... --apply --start`", repoFlag)
	}
	return fmt.Sprintf("gira status%s", repoFlag)
}

func auditReadinessHumanStatus(status string) string {
	switch status {
	case AuditReadinessStatusOK:
		return "pass"
	case AuditReadinessStatusMissing:
		return "warn"
	default:
		return "fail"
	}
}
