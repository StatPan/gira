from __future__ import annotations

import json
from datetime import datetime, timezone

from typer.testing import CliRunner

import gira.cli as cli
from gira.github_status import format_status_text, summarize_status


runner = CliRunner()
NOW = datetime(2026, 4, 26, 12, 0, tzinfo=timezone.utc)


def _issue(
    number: int,
    *,
    title: str | None = None,
    state: str = "open",
    updated_at: str = "2026-04-25T12:00:00Z",
    labels: tuple[str, ...] = (),
    milestone: str | None = "MVP",
) -> dict:
    return {
        "number": number,
        "title": title or f"Issue {number}",
        "state": state,
        "labels": labels,
        "milestone": milestone,
        "created_at": "2026-04-01T12:00:00Z",
        "updated_at": updated_at,
        "closed_at": "2026-04-20T12:00:00Z" if state == "closed" else None,
        "url": f"https://github.com/StatPan/gira/issues/{number}",
    }


def _milestone(
    number: int,
    title: str,
    *,
    open_issues: int,
    closed_issues: int,
    state: str = "open",
) -> dict:
    return {
        "number": number,
        "title": title,
        "state": state,
        "description": "",
        "due_on": None,
        "open_issues": open_issues,
        "closed_issues": closed_issues,
    }


def test_status_summary_counts_open_and_closed_issues():
    summary = summarize_status(
        repo="StatPan/gira",
        milestones=[],
        issues=[_issue(1), _issue(2, state="closed")],
        fetched_at=NOW,
        stale_days=14,
    )

    assert summary["counts"]["issues"]["open"] == 1
    assert summary["counts"]["issues"]["closed"] == 1
    assert summary["counts"]["issues"]["total"] == 2


def test_milestone_open_closed_counts_are_represented():
    summary = summarize_status(
        repo="StatPan/gira",
        milestones=[_milestone(1, "MVP", open_issues=2, closed_issues=3)],
        issues=[],
        fetched_at=NOW,
    )

    assert summary["milestones"] == [
        {
            "number": 1,
            "title": "MVP",
            "state": "open",
            "open_issues": 2,
            "closed_issues": 3,
            "total_issues": 5,
            "progress_percent": 60,
            "due_on": None,
            "description": "",
        }
    ]


def test_json_output_is_parseable_and_stable_for_hermes(monkeypatch):
    summary = summarize_status(
        repo="StatPan/gira",
        milestones=[_milestone(1, "MVP", open_issues=1, closed_issues=1)],
        issues=[_issue(1), _issue(2, state="closed")],
        fetched_at=NOW,
    )

    class FakeClient:
        def __init__(self, repo):
            self.repo = repo

    monkeypatch.setattr(cli, "GhClient", FakeClient)
    monkeypatch.setattr(cli, "build_status_summary", lambda client, stale_days: summary)

    result = runner.invoke(cli.app, ["status", "--repo", "StatPan/gira", "--json"])

    assert result.exit_code == 0, result.output
    payload = json.loads(result.output)
    assert list(payload.keys()) == ["counts", "fetched_at", "issues", "milestones", "repo", "stale_days"]
    assert payload["repo"] == "StatPan/gira"
    assert payload["counts"]["issues"]["total"] == 2


def test_text_output_is_concise_and_includes_expected_sections():
    summary = summarize_status(
        repo="StatPan/gira",
        milestones=[_milestone(1, "MVP", open_issues=1, closed_issues=4)],
        issues=[
            _issue(5, title="[Task] Slice 4", updated_at="2026-04-01T12:00:00Z"),
            _issue(6, labels=("status:blocked",), updated_at="2026-04-25T12:00:00Z"),
        ],
        fetched_at=NOW,
        stale_days=14,
    )

    text = format_status_text(summary)

    assert "status: StatPan/gira" in text
    assert "issues:" in text
    assert "milestone progress:" in text
    assert "stale open issues:" in text
    assert "blocked issues:" in text
    assert "open issues:" in text
    assert len(text.splitlines()) <= 14


def test_large_issue_list_over_100_is_counted_and_limited_in_text():
    issues = [_issue(number) for number in range(1, 151)]

    summary = summarize_status(
        repo="StatPan/gira",
        milestones=[],
        issues=issues,
        fetched_at=NOW,
    )
    text = format_status_text(summary)

    assert summary["counts"]["issues"]["open"] == 150
    assert len(summary["issues"]["open"]) == 150
    assert "... 142 more" in text
