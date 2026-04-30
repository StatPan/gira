package gira

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const AuditSchemaVersion = "v1"

type AuditRecord struct {
	SchemaVersion string `json:"schema_version"`
	TS            string `json:"ts"`
	Actor         string `json:"actor"`
	Command       string `json:"command"`
	PolicyHash    string `json:"policy_hash"`
	Action        string `json:"action"`
	Target        string `json:"target"`
	Result        string `json:"result"`
	Reason        string `json:"reason,omitempty"`
	Permission    string `json:"permission_outcome,omitempty"`
	PrevHash      string `json:"prev_hash,omitempty"`
	Hash          string `json:"hash"`
}

type AuditVerifyReport struct {
	Files       []string `json:"files"`
	Records     int      `json:"records"`
	Valid       bool     `json:"valid"`
	Failure     string   `json:"failure,omitempty"`
	FailureFile string   `json:"failure_file,omitempty"`
	FailureLine int      `json:"failure_line,omitempty"`
}

func NewAuditRecord(command, policyHash, action, target, result, reason, permission string, now time.Time) AuditRecord {
	actor := strings.TrimSpace(os.Getenv("USER"))
	if actor == "" {
		actor = "unknown"
	}
	if policyHash == "" {
		policyHash = "sha256:unknown"
	}
	return AuditRecord{
		SchemaVersion: AuditSchemaVersion,
		TS:            now.UTC().Format(time.RFC3339),
		Actor:         actor,
		Command:       command,
		PolicyHash:    policyHash,
		Action:        action,
		Target:        target,
		Result:        result,
		Reason:        reason,
		Permission:    permission,
	}
}

func AppendAuditRecords(path string, records []AuditRecord) error {
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	unlock, err := acquireAuditLock(path)
	if err != nil {
		return err
	}
	defer unlock()

	prevHash, err := lastAuditHash(path)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for i := range records {
		rec := records[i]
		if err := validateAuditRecord(rec, true); err != nil {
			return err
		}
		rec.PrevHash = prevHash
		rec.Hash = computeAuditHash(rec)
		if err := enc.Encode(rec); err != nil {
			return err
		}
		prevHash = rec.Hash
	}
	if err := writeLastAuditHash(path, prevHash); err != nil {
		return err
	}
	return nil
}

func VerifyAuditLedger(globPath string) AuditVerifyReport {
	report := AuditVerifyReport{Valid: true}
	files, err := filepath.Glob(globPath)
	if err != nil || len(files) == 0 {
		report.Valid = false
		report.Failure = "no_audit_files_found"
		return report
	}
	sort.Strings(files)
	report.Files = files
	prevHash := ""
	for _, file := range files {
		prevHash = ""
		fh, err := os.Open(file)
		if err != nil {
			return failAudit(report, file, 0, fmt.Sprintf("open_failed:%v", err))
		}
		scanner := bufio.NewScanner(fh)
		line := 0
		for scanner.Scan() {
			line++
			report.Records++
			var rec AuditRecord
			if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
				fh.Close()
				return failAudit(report, file, line, "malformed_json")
			}
			if err := validateAuditRecord(rec, false); err != nil {
				fh.Close()
				return failAudit(report, file, line, err.Error())
			}
			if rec.PrevHash != prevHash {
				fh.Close()
				return failAudit(report, file, line, "hash_chain_broken")
			}
			if rec.Hash != computeAuditHash(rec) {
				fh.Close()
				return failAudit(report, file, line, "hash_mismatch")
			}
			prevHash = rec.Hash
		}
		if err := scanner.Err(); err != nil {
			fh.Close()
			return failAudit(report, file, line, fmt.Sprintf("scan_failed:%v", err))
		}
		fh.Close()
	}
	if report.Records == 0 {
		return failAudit(report, "", 0, "no_audit_records")
	}
	return report
}

func VerifyAuditLedgerForRepo(globPath string, repo RepoRef) AuditVerifyReport {
	report := AuditVerifyReport{Valid: true}
	files, err := filepath.Glob(globPath)
	if err != nil || len(files) == 0 {
		report.Valid = false
		report.Failure = "no_audit_files_found"
		return report
	}
	wantFile := fmt.Sprintf("%s_%s.jsonl", repo.Owner, repo.Name)
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		if filepath.Base(file) == wantFile {
			filtered = append(filtered, file)
		}
	}
	if len(filtered) == 0 {
		report.Valid = false
		report.Failure = "no_audit_files_found"
		return report
	}
	sort.Strings(filtered)
	report.Files = filtered
	prevHash := ""
	for _, file := range filtered {
		prevHash = ""
		fh, err := os.Open(file)
		if err != nil {
			return failAudit(report, file, 0, fmt.Sprintf("open_failed:%v", err))
		}
		scanner := bufio.NewScanner(fh)
		line := 0
		for scanner.Scan() {
			line++
			report.Records++
			var rec AuditRecord
			if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
				fh.Close()
				return failAudit(report, file, line, "malformed_json")
			}
			if err := validateAuditRecord(rec, false); err != nil {
				fh.Close()
				return failAudit(report, file, line, err.Error())
			}
			if rec.PrevHash != prevHash {
				fh.Close()
				return failAudit(report, file, line, "hash_chain_broken")
			}
			if rec.Hash != computeAuditHash(rec) {
				fh.Close()
				return failAudit(report, file, line, "hash_mismatch")
			}
			prevHash = rec.Hash
		}
		if err := scanner.Err(); err != nil {
			fh.Close()
			return failAudit(report, file, line, fmt.Sprintf("scan_failed:%v", err))
		}
		fh.Close()
	}
	if report.Records == 0 {
		return failAudit(report, "", 0, "no_audit_records")
	}
	return report
}

func failAudit(report AuditVerifyReport, file string, line int, reason string) AuditVerifyReport {
	report.Valid = false
	report.Failure = reason
	report.FailureFile = file
	report.FailureLine = line
	return report
}

func validateAuditRecord(rec AuditRecord, writing bool) error {
	if rec.SchemaVersion != AuditSchemaVersion {
		return fmt.Errorf("invalid_schema_version")
	}
	if _, err := time.Parse(time.RFC3339, rec.TS); err != nil {
		return fmt.Errorf("invalid_ts")
	}
	required := map[string]string{
		"actor":       rec.Actor,
		"command":     rec.Command,
		"policy_hash": rec.PolicyHash,
		"action":      rec.Action,
		"target":      rec.Target,
		"result":      rec.Result,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("missing_%s", key)
		}
	}
	if !strings.HasPrefix(rec.PolicyHash, "sha256:") {
		return fmt.Errorf("invalid_policy_hash")
	}
	if !writing && strings.TrimSpace(rec.Hash) == "" {
		return fmt.Errorf("missing_hash")
	}
	return nil
}

func computeAuditHash(rec AuditRecord) string {
	payload := struct {
		SchemaVersion string `json:"schema_version"`
		TS            string `json:"ts"`
		Actor         string `json:"actor"`
		Command       string `json:"command"`
		PolicyHash    string `json:"policy_hash"`
		Action        string `json:"action"`
		Target        string `json:"target"`
		Result        string `json:"result"`
		Reason        string `json:"reason,omitempty"`
		Permission    string `json:"permission_outcome,omitempty"`
		PrevHash      string `json:"prev_hash,omitempty"`
	}{
		SchemaVersion: rec.SchemaVersion,
		TS:            rec.TS,
		Actor:         rec.Actor,
		Command:       rec.Command,
		PolicyHash:    rec.PolicyHash,
		Action:        rec.Action,
		Target:        rec.Target,
		Result:        rec.Result,
		Reason:        rec.Reason,
		Permission:    rec.Permission,
		PrevHash:      rec.PrevHash,
	}
	base, _ := json.Marshal(payload)
	sum := sha256.Sum256(base)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func lastAuditHash(path string) (string, error) {
	if v, ok, err := readLastAuditHash(path); err != nil {
		return "", err
	} else if ok {
		return v, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	last := ""
	for s.Scan() {
		var rec AuditRecord
		if err := json.Unmarshal(s.Bytes(), &rec); err != nil {
			return "", fmt.Errorf("existing_audit_malformed")
		}
		last = rec.Hash
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	if last != "" {
		if err := writeLastAuditHash(path, last); err != nil {
			return "", err
		}
	}
	return last, nil
}

func readLastAuditHash(path string) (string, bool, error) {
	b, err := os.ReadFile(path + ".lasthash")
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", false, nil
	}
	return v, true, nil
}

func writeLastAuditHash(path, hash string) error {
	if strings.TrimSpace(hash) == "" {
		return nil
	}
	return os.WriteFile(path+".lasthash", []byte(hash+"\n"), 0o644)
}

func acquireAuditLock(path string) (func(), error) {
	lockPath := path + ".lock"
	for i := 0; i < 50; i++ {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if os.IsExist(err) {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("audit_lock_timeout")
}
