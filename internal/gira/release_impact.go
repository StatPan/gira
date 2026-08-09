package gira

import (
	"fmt"
	"strings"
)

const (
	ReleaseImpactBlockStart = "<!-- gira:release-impact:start -->"
	ReleaseImpactBlockEnd   = "<!-- gira:release-impact:end -->"

	ReleaseImpactUserFacing = "user-facing"
	ReleaseImpactInternal   = "internal"
	ReleaseImpactExempt     = "exempt"
)

// TicketReleaseImpact is a low-cardinality declaration made with the ticket,
// then copied to its PR so local status, CI, and release preparation share the
// same source of truth.
type TicketReleaseImpact struct {
	Declared          bool   `json:"declared"`
	Impact            string `json:"impact,omitempty"`
	Reason            string `json:"reason,omitempty"`
	ChangelogRequired bool   `json:"changelog_required"`
	Source            string `json:"source,omitempty"`
}

func ParseTicketReleaseImpact(body string) TicketReleaseImpact {
	block := extractReleaseImpactBlock(body)
	if block == "" {
		return TicketReleaseImpact{Source: "missing"}
	}
	report := TicketReleaseImpact{Declared: true, Source: "ticket_body"}
	for _, line := range strings.Split(block, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "impact":
			report.Impact = strings.ToLower(strings.TrimSpace(value))
		case "reason":
			report.Reason = strings.TrimSpace(value)
		}
	}
	report.ChangelogRequired = report.Impact == ReleaseImpactUserFacing
	return report
}

func RenderTicketReleaseImpact(impact string, reason string) (string, error) {
	impact = strings.ToLower(strings.TrimSpace(impact))
	reason = strings.TrimSpace(reason)
	if !validTicketReleaseImpact(impact) {
		return "", fmt.Errorf("--release-impact must be one of user-facing, internal, or exempt")
	}
	if impact == ReleaseImpactExempt && reason == "" {
		return "", fmt.Errorf("--release-impact-reason is required when --release-impact=exempt")
	}
	if reason == "" && impact == ReleaseImpactInternal {
		reason = "Internal-only change."
	}
	var b strings.Builder
	b.WriteString(ReleaseImpactBlockStart)
	b.WriteString("\nimpact: ")
	b.WriteString(impact)
	b.WriteString("\nreason: ")
	b.WriteString(reason)
	b.WriteString("\n")
	b.WriteString(ReleaseImpactBlockEnd)
	return b.String(), nil
}

func ticketReleaseImpactForNewTicket(ticketType string, impact string, reason string) (TicketReleaseImpact, string, error) {
	impact = strings.ToLower(strings.TrimSpace(impact))
	if impact == "" {
		if strings.EqualFold(strings.TrimSpace(ticketType), "story") {
			impact = ReleaseImpactUserFacing
		} else {
			return TicketReleaseImpact{Source: "not_declared"}, "", nil
		}
	}
	block, err := RenderTicketReleaseImpact(impact, reason)
	if err != nil {
		return TicketReleaseImpact{}, "", err
	}
	report := ParseTicketReleaseImpact(block)
	return report, block, nil
}

func validTicketReleaseImpact(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ReleaseImpactUserFacing, ReleaseImpactInternal, ReleaseImpactExempt:
		return true
	default:
		return false
	}
}

func extractReleaseImpactBlock(body string) string {
	start := strings.Index(body, ReleaseImpactBlockStart)
	if start < 0 {
		return ""
	}
	start += len(ReleaseImpactBlockStart)
	end := strings.Index(body[start:], ReleaseImpactBlockEnd)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(body[start : start+end])
}

func appendTicketReleaseImpact(body string, block string) string {
	if strings.TrimSpace(block) == "" {
		return body
	}
	body = strings.TrimSpace(body)
	if extractReleaseImpactBlock(body) != "" {
		return body
	}
	if body == "" {
		return block
	}
	return body + "\n\n" + block
}

func releaseImpactPRBody(issueNumber int, issueBody string) string {
	body := fmt.Sprintf("Closes #%d", issueNumber)
	impact := ParseTicketReleaseImpact(issueBody)
	if !impact.Declared {
		return body
	}
	block, err := RenderTicketReleaseImpact(impact.Impact, impact.Reason)
	if err != nil {
		return body
	}
	return body + "\n\n" + block
}
