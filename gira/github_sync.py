from __future__ import annotations

import json
import subprocess
from dataclasses import dataclass, field
from typing import Any, Iterable, Literal

from gira.config import RepoRef


BOOTSTRAP_LABEL = "gira:bootstrap"


@dataclass(frozen=True)
class LabelDef:
    name: str
    color: str
    description: str


@dataclass(frozen=True)
class MilestoneDef:
    title: str
    description: str
    due_on: str | None = None


@dataclass(frozen=True)
class BootstrapIssueDef:
    title: str
    body: str
    labels: tuple[str, ...]
    milestone: str | None = None


@dataclass(frozen=True)
class ExistingLabel:
    name: str
    color: str
    description: str


@dataclass(frozen=True)
class ExistingMilestone:
    number: int
    title: str
    description: str
    due_on: str | None = None


@dataclass(frozen=True)
class ExistingIssue:
    number: int
    title: str
    labels: tuple[str, ...]


PlanAction = Literal["create", "update", "skip"]


@dataclass(frozen=True)
class LabelPlan:
    action: PlanAction
    desired: LabelDef
    existing: ExistingLabel | None = None


@dataclass(frozen=True)
class MilestonePlan:
    action: PlanAction
    desired: MilestoneDef
    existing: ExistingMilestone | None = None


@dataclass(frozen=True)
class BootstrapIssuePlan:
    action: Literal["create", "skip"]
    desired: BootstrapIssueDef
    existing: ExistingIssue | None = None


@dataclass
class SyncPlan:
    labels: list[LabelPlan] = field(default_factory=list)
    milestones: list[MilestonePlan] = field(default_factory=list)
    bootstrap_issues: list[BootstrapIssuePlan] = field(default_factory=list)

    @property
    def changes(self) -> int:
        return sum(1 for item in self.labels if item.action != "skip") + sum(
            1 for item in self.milestones if item.action != "skip"
        ) + sum(1 for item in self.bootstrap_issues if item.action != "skip")


class GhError(RuntimeError):
    def __init__(self, command: list[str], returncode: int, stderr: str):
        self.command = command
        self.returncode = returncode
        self.stderr = stderr.strip()
        rendered = " ".join(command)
        detail = f": {self.stderr}" if self.stderr else ""
        super().__init__(f"gh command failed ({returncode}): {rendered}{detail}")


class GhClient:
    def __init__(self, repo: RepoRef):
        self.repo = repo

    def run(self, args: list[str]) -> str:
        command = ["gh", *args]
        completed = subprocess.run(
            command,
            capture_output=True,
            text=True,
        )
        if completed.returncode != 0:
            raise GhError(command, completed.returncode, completed.stderr)
        return completed.stdout

    def json(self, args: list[str]) -> Any:
        output = self.run(args)
        return json.loads(output or "null")

    def list_labels(self) -> list[ExistingLabel]:
        rows = self.json(
            [
                "label",
                "list",
                "--repo",
                self.repo.full_name,
                "--json",
                "name,color,description",
                "--limit",
                "1000",
            ]
        )
        return [
            ExistingLabel(
                name=row["name"],
                color=_normalize_color(row.get("color") or ""),
                description=row.get("description") or "",
            )
            for row in rows
        ]

    def create_label(self, label: LabelDef) -> None:
        self.run(
            [
                "label",
                "create",
                label.name,
                "--repo",
                self.repo.full_name,
                "--color",
                label.color,
                "--description",
                label.description,
            ]
        )

    def update_label(self, label: LabelDef) -> None:
        self.run(
            [
                "label",
                "edit",
                label.name,
                "--repo",
                self.repo.full_name,
                "--color",
                label.color,
                "--description",
                label.description,
            ]
        )

    def list_milestones(self) -> list[ExistingMilestone]:
        rows = self.json(
            [
                "api",
                f"repos/{self.repo.full_name}/milestones",
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
        rows = _flatten_pages(rows)
        return [
            ExistingMilestone(
                number=int(row["number"]),
                title=row["title"],
                description=row.get("description") or "",
                due_on=_normalize_due_on(row.get("due_on")),
            )
            for row in rows
        ]

    def create_milestone(self, milestone: MilestoneDef) -> None:
        args = [
            "api",
            f"repos/{self.repo.full_name}/milestones",
            "-X",
            "POST",
            "-f",
            f"title={milestone.title}",
            "-f",
            f"description={milestone.description}",
        ]
        if milestone.due_on:
            args.extend(["-f", f"due_on={milestone.due_on}"])
        self.run(args)

    def update_milestone(self, number: int, milestone: MilestoneDef) -> None:
        args = [
            "api",
            f"repos/{self.repo.full_name}/milestones/{number}",
            "-X",
            "PATCH",
            "-f",
            f"title={milestone.title}",
            "-f",
            f"description={milestone.description}",
        ]
        if milestone.due_on:
            args.extend(["-f", f"due_on={milestone.due_on}"])
        else:
            args.extend(["-F", "due_on=null"])
        self.run(args)

    def list_bootstrap_issues(self) -> list[ExistingIssue]:
        rows = self.json(
            [
                "issue",
                "list",
                "--repo",
                self.repo.full_name,
                "--state",
                "all",
                "--label",
                BOOTSTRAP_LABEL,
                "--json",
                "number,title,labels",
                "--limit",
                "1000",
            ]
        )
        return [
            ExistingIssue(
                number=int(row["number"]),
                title=row["title"],
                labels=tuple(label["name"] for label in row.get("labels", [])),
            )
            for row in rows
        ]

    def create_issue(self, issue: BootstrapIssueDef) -> None:
        args = [
            "issue",
            "create",
            "--repo",
            self.repo.full_name,
            "--title",
            issue.title,
            "--body",
            issue.body,
        ]
        for label in issue.labels:
            args.extend(["--label", label])
        if issue.milestone:
            args.extend(["--milestone", issue.milestone])
        self.run(args)


DESIRED_LABELS: tuple[LabelDef, ...] = (
    LabelDef("gira:bootstrap", "5319E7", "Created or managed by Gira bootstrap metadata sync."),
    LabelDef("type:epic", "0E8A16", "Large outcome that groups related implementation tasks."),
    LabelDef("type:task", "1D76DB", "Concrete implementation task."),
    LabelDef("type:chore", "D4C5F9", "Maintenance or process work."),
    LabelDef("agent:human", "FBCA04", "Owned by a human project lead."),
    LabelDef("agent:worker", "BFDADC", "Ready for an implementation worker."),
    LabelDef("status:ready", "C2E0C6", "Ready to start."),
    LabelDef("status:blocked", "E99695", "Blocked by an external dependency or decision."),
)


DESIRED_MILESTONES: tuple[MilestoneDef, ...] = (
    MilestoneDef("MVP", "CLI-first Gira bootstrapper with templates and GitHub metadata sync."),
    MilestoneDef("Beta", "Broader validation and hardening after the MVP workflow is usable."),
    MilestoneDef("v1", "Stable first release of the GitHub-native project OS workflow."),
)


DESIRED_BOOTSTRAP_ISSUES: tuple[BootstrapIssueDef, ...] = (
    BootstrapIssueDef(
        title="[Epic] Gira MVP",
        body=(
            "## Goal\n"
            "Ship the CLI-first Gira MVP.\n\n"
            "## Scope\n"
            "- local template bootstrap\n"
            "- GitHub label, milestone, and bootstrap issue sync\n"
            "- compact status summary\n"
        ),
        labels=("gira:bootstrap", "type:epic", "agent:human", "status:ready"),
        milestone="MVP",
    ),
    BootstrapIssueDef(
        title="[Task] Slice 1: package skeleton and default template",
        body="## Goal\nCreate the Python package, CLI entrypoint, and default project template.",
        labels=("gira:bootstrap", "type:task", "agent:worker", "status:ready"),
        milestone="MVP",
    ),
    BootstrapIssueDef(
        title="[Task] Slice 2: local bootstrap install",
        body="## Goal\nInstall rendered template files into a local git repository idempotently.",
        labels=("gira:bootstrap", "type:task", "agent:worker", "status:ready"),
        milestone="MVP",
    ),
    BootstrapIssueDef(
        title="[Task] Slice 3: labels/milestones/bootstrap-issues sync",
        body="## Goal\nSync GitHub labels, milestones, and bootstrap issues through the gh CLI.",
        labels=("gira:bootstrap", "type:task", "agent:worker", "status:ready"),
        milestone="MVP",
    ),
    BootstrapIssueDef(
        title="[Task] Slice 4: compact status summary",
        body="## Goal\nShow a compact status summary for a Gira-managed GitHub repository.",
        labels=("gira:bootstrap", "type:task", "agent:worker", "status:ready"),
        milestone="MVP",
    ),
)


def plan_labels(desired: Iterable[LabelDef], existing: Iterable[ExistingLabel]) -> list[LabelPlan]:
    by_name = {item.name: item for item in existing}
    plan: list[LabelPlan] = []
    for label in desired:
        current = by_name.get(label.name)
        if current is None:
            plan.append(LabelPlan("create", label))
        elif _normalize_color(current.color) != _normalize_color(label.color) or current.description != label.description:
            plan.append(LabelPlan("update", label, current))
        else:
            plan.append(LabelPlan("skip", label, current))
    return plan


def plan_milestones(
    desired: Iterable[MilestoneDef],
    existing: Iterable[ExistingMilestone],
) -> list[MilestonePlan]:
    by_title = {item.title: item for item in existing}
    plan: list[MilestonePlan] = []
    for milestone in desired:
        current = by_title.get(milestone.title)
        if current is None:
            plan.append(MilestonePlan("create", milestone))
        elif current.description != milestone.description or current.due_on != milestone.due_on:
            plan.append(MilestonePlan("update", milestone, current))
        else:
            plan.append(MilestonePlan("skip", milestone, current))
    return plan


def plan_bootstrap_issues(
    desired: Iterable[BootstrapIssueDef],
    existing: Iterable[ExistingIssue],
) -> list[BootstrapIssuePlan]:
    existing_keys = {
        issue.title
        for issue in existing
        if BOOTSTRAP_LABEL in issue.labels
    }
    by_title = {issue.title: issue for issue in existing if BOOTSTRAP_LABEL in issue.labels}
    plan: list[BootstrapIssuePlan] = []
    for issue in desired:
        if issue.title in existing_keys:
            plan.append(BootstrapIssuePlan("skip", issue, by_title[issue.title]))
        else:
            plan.append(BootstrapIssuePlan("create", issue))
    return plan


def build_sync_plan(client: GhClient) -> SyncPlan:
    return SyncPlan(
        labels=plan_labels(DESIRED_LABELS, client.list_labels()),
        milestones=plan_milestones(DESIRED_MILESTONES, client.list_milestones()),
        bootstrap_issues=plan_bootstrap_issues(DESIRED_BOOTSTRAP_ISSUES, client.list_bootstrap_issues()),
    )


def apply_sync_plan(client: GhClient, plan: SyncPlan) -> None:
    for item in plan.labels:
        if item.action == "create":
            client.create_label(item.desired)
        elif item.action == "update":
            client.update_label(item.desired)

    for item in plan.milestones:
        if item.action == "create":
            client.create_milestone(item.desired)
        elif item.action == "update":
            if item.existing is None:
                raise ValueError(f"missing existing milestone for update: {item.desired.title}")
            client.update_milestone(item.existing.number, item.desired)

    for item in plan.bootstrap_issues:
        if item.action == "create":
            client.create_issue(item.desired)


def format_sync_plan(plan: SyncPlan, *, dry_run: bool) -> str:
    prefix = "would " if dry_run else ""
    lines = [
        "sync plan:",
        f"labels:           {_count_action(plan.labels, 'create')} {prefix}create, {_count_action(plan.labels, 'update')} {prefix}update, {_count_action(plan.labels, 'skip')} skip",
        f"milestones:       {_count_action(plan.milestones, 'create')} {prefix}create, {_count_action(plan.milestones, 'update')} {prefix}update, {_count_action(plan.milestones, 'skip')} skip",
        f"bootstrap issues: {_count_action(plan.bootstrap_issues, 'create')} {prefix}create, {_count_action(plan.bootstrap_issues, 'skip')} skip",
    ]
    lines.extend(_format_items("label", plan.labels))
    lines.extend(_format_items("milestone", plan.milestones))
    lines.extend(_format_items("issue", plan.bootstrap_issues))
    return "\n".join(lines) + "\n"


def _format_items(kind: str, items: Iterable[LabelPlan | MilestonePlan | BootstrapIssuePlan]) -> list[str]:
    lines: list[str] = []
    for item in items:
        if item.action == "skip":
            continue
        name = item.desired.name if isinstance(item.desired, LabelDef) else item.desired.title
        lines.append(f"  {item.action} {kind}: {name}")
    return lines


def _count_action(items: Iterable[Any], action: str) -> int:
    return sum(1 for item in items if item.action == action)


def _normalize_color(value: str) -> str:
    return value.strip().lstrip("#").upper()


def _normalize_due_on(value: str | None) -> str | None:
    if not value:
        return None
    return value


def _flatten_pages(value: Any) -> list[Any]:
    if not isinstance(value, list):
        return []
    if value and all(isinstance(item, list) for item in value):
        return [row for page in value for row in page]
    return value
