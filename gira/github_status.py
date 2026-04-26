from __future__ import annotations

from datetime import datetime, timedelta, timezone
from typing import Any, Protocol

from gira.config import RepoRef


class JsonClient(Protocol):
    repo: RepoRef

    def json(self, args: list[str]) -> Any: ...


def build_status_summary(
    client: JsonClient,
    *,
    fetched_at: datetime | None = None,
    stale_days: int = 14,
) -> dict[str, Any]:
    """Fetch GitHub issues and milestones and return a stable status payload."""
    now = _as_utc(fetched_at or datetime.now(timezone.utc))
    milestones = fetch_milestones(client)
    issues = fetch_issues(client)
    return summarize_status(
        repo=client.repo.full_name,
        milestones=milestones,
        issues=issues,
        fetched_at=now,
        stale_days=stale_days,
    )


def fetch_milestones(client: JsonClient) -> list[dict[str, Any]]:
    rows = client.json(
        [
            "api",
            f"repos/{client.repo.full_name}/milestones",
            "--paginate",
            "--slurp",
            "-X",
            "GET",
            "-f",
            "state=all",
            "-f",
            "per_page=100",
        ]
    )
    return [_normalize_milestone(row) for row in _flatten_pages(rows)]


def fetch_issues(client: JsonClient) -> list[dict[str, Any]]:
    rows = client.json(
        [
            "api",
            f"repos/{client.repo.full_name}/issues",
            "--paginate",
            "--slurp",
            "-X",
            "GET",
            "-f",
            "state=all",
            "-f",
            "per_page=100",
        ]
    )
    return [
        _normalize_issue(row)
        for row in _flatten_pages(rows)
        if "pull_request" not in row
    ]


def summarize_status(
    *,
    repo: str,
    milestones: list[dict[str, Any]],
    issues: list[dict[str, Any]],
    fetched_at: datetime,
    stale_days: int = 14,
) -> dict[str, Any]:
    now = _as_utc(fetched_at)
    stale_cutoff = now - timedelta(days=stale_days)
    open_issues = [issue for issue in issues if issue["state"] == "open"]
    closed_issues = [issue for issue in issues if issue["state"] == "closed"]
    stale_open_issues = [
        issue
        for issue in open_issues
        if _parse_github_datetime(issue["updated_at"]) <= stale_cutoff
    ]
    blocked_open_issues = [
        issue
        for issue in open_issues
        if "status:blocked" in issue["labels"]
    ]

    milestone_rows = [
        _summarize_milestone(milestone)
        for milestone in sorted(milestones, key=lambda item: int(item["number"]))
    ]
    issue_rows = [_issue_summary(issue) for issue in sorted(open_issues, key=lambda item: int(item["number"]))]
    stale_rows = [
        _issue_summary(issue)
        for issue in sorted(
            stale_open_issues,
            key=lambda item: (_parse_github_datetime(item["updated_at"]), int(item["number"])),
        )
    ]
    blocked_rows = [
        _issue_summary(issue)
        for issue in sorted(blocked_open_issues, key=lambda item: int(item["number"]))
    ]

    return {
        "repo": repo,
        "fetched_at": _format_github_datetime(now),
        "stale_days": stale_days,
        "counts": {
            "issues": {
                "total": len(issues),
                "open": len(open_issues),
                "closed": len(closed_issues),
                "stale_open": len(stale_rows),
                "blocked_open": len(blocked_rows),
            },
            "milestones": {
                "total": len(milestone_rows),
                "open": sum(1 for milestone in milestone_rows if milestone["state"] == "open"),
                "closed": sum(1 for milestone in milestone_rows if milestone["state"] == "closed"),
            },
        },
        "milestones": milestone_rows,
        "issues": {
            "open": issue_rows,
            "stale_open": stale_rows,
            "blocked_open": blocked_rows,
        },
    }


def format_status_text(summary: dict[str, Any]) -> str:
    issue_counts = summary["counts"]["issues"]
    milestone_counts = summary["counts"]["milestones"]
    lines = [
        f"status: {summary['repo']}",
        (
            "issues: "
            f"{issue_counts['open']} open, {issue_counts['closed']} closed, "
            f"{issue_counts['total']} total; {issue_counts['stale_open']} stale open "
            f"({summary['stale_days']}d); {issue_counts['blocked_open']} blocked"
        ),
        (
            "milestones: "
            f"{milestone_counts['open']} open, {milestone_counts['closed']} closed, "
            f"{milestone_counts['total']} total"
        ),
    ]

    if summary["milestones"]:
        lines.append("milestone progress:")
        for milestone in summary["milestones"]:
            lines.append(
                "  "
                f"{milestone['title']}: {milestone['open_issues']} open / "
                f"{milestone['closed_issues']} closed ({milestone['progress_percent']}%)"
            )
    else:
        lines.append("milestone progress: none")

    lines.extend(_format_issue_section("stale open issues", summary["issues"]["stale_open"]))
    lines.extend(_format_issue_section("blocked issues", summary["issues"]["blocked_open"]))
    lines.extend(_format_issue_section("open issues", summary["issues"]["open"], limit=8))
    return "\n".join(lines) + "\n"


def _normalize_milestone(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "number": int(row["number"]),
        "title": row["title"],
        "state": row["state"],
        "description": row.get("description") or "",
        "due_on": row.get("due_on"),
        "open_issues": int(row.get("open_issues") or 0),
        "closed_issues": int(row.get("closed_issues") or 0),
    }


def _normalize_issue(row: dict[str, Any]) -> dict[str, Any]:
    milestone = row.get("milestone")
    return {
        "number": int(row["number"]),
        "title": row["title"],
        "state": row["state"],
        "labels": tuple(sorted(label["name"] for label in row.get("labels", []))),
        "milestone": milestone["title"] if milestone else None,
        "created_at": row["created_at"],
        "updated_at": row["updated_at"],
        "closed_at": row.get("closed_at"),
        "url": row.get("html_url") or row.get("url") or "",
    }


def _summarize_milestone(milestone: dict[str, Any]) -> dict[str, Any]:
    open_count = int(milestone["open_issues"])
    closed_count = int(milestone["closed_issues"])
    total = open_count + closed_count
    progress = round((closed_count / total) * 100) if total else 0
    return {
        "number": milestone["number"],
        "title": milestone["title"],
        "state": milestone["state"],
        "open_issues": open_count,
        "closed_issues": closed_count,
        "total_issues": total,
        "progress_percent": progress,
        "due_on": milestone["due_on"],
        "description": milestone["description"],
    }


def _issue_summary(issue: dict[str, Any]) -> dict[str, Any]:
    return {
        "number": issue["number"],
        "title": issue["title"],
        "state": issue["state"],
        "labels": list(issue["labels"]),
        "milestone": issue["milestone"],
        "updated_at": issue["updated_at"],
        "url": issue["url"],
    }


def _format_issue_section(title: str, issues: list[dict[str, Any]], *, limit: int | None = None) -> list[str]:
    if not issues:
        return [f"{title}: none"]
    rendered = [f"{title}: {len(issues)}"]
    visible = issues[:limit] if limit is not None else issues
    for issue in visible:
        milestone = f" [{issue['milestone']}]" if issue["milestone"] else ""
        rendered.append(f"  #{issue['number']} {issue['title']}{milestone} updated {issue['updated_at']}")
    if limit is not None and len(issues) > limit:
        rendered.append(f"  ... {len(issues) - limit} more")
    return rendered


def _flatten_pages(value: Any) -> list[Any]:
    if not isinstance(value, list):
        return []
    if value and all(isinstance(item, list) for item in value):
        return [row for page in value for row in page]
    return value


def _parse_github_datetime(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(timezone.utc)


def _as_utc(value: datetime) -> datetime:
    if value.tzinfo is None:
        return value.replace(tzinfo=timezone.utc)
    return value.astimezone(timezone.utc)


def _format_github_datetime(value: datetime) -> str:
    return _as_utc(value).replace(microsecond=0).isoformat().replace("+00:00", "Z")
