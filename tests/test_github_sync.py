from __future__ import annotations

from typer.testing import CliRunner

import gira.cli as cli
from gira.github_sync import (
    BOOTSTRAP_LABEL,
    BootstrapIssueDef,
    BootstrapIssuePlan,
    ExistingIssue,
    ExistingLabel,
    ExistingMilestone,
    LabelDef,
    LabelPlan,
    MilestoneDef,
    MilestonePlan,
    SyncPlan,
    plan_bootstrap_issues,
    plan_labels,
    plan_milestones,
)


runner = CliRunner()


def test_labels_are_planned_by_name():
    desired = [
        LabelDef("agent:worker", "BFDADC", "Ready for a worker."),
        LabelDef("agent:human", "FBCA04", "Human owned."),
    ]
    existing = [ExistingLabel("agent:worker", "BFDADC", "Ready for a worker.")]

    plan = plan_labels(desired, existing)

    assert [item.action for item in plan] == ["skip", "create"]
    assert plan[1].desired.name == "agent:human"


def test_label_update_detected_when_color_or_description_differ():
    desired = [LabelDef("agent:worker", "BFDADC", "Ready for a worker.")]
    existing = [ExistingLabel("agent:worker", "000000", "Old description.")]

    plan = plan_labels(desired, existing)

    assert len(plan) == 1
    assert plan[0].action == "update"


def test_milestones_are_planned_by_title():
    desired = [
        MilestoneDef("MVP", "CLI-first scope."),
        MilestoneDef("Beta", "Hardening."),
    ]
    existing = [ExistingMilestone(number=10, title="MVP", description="CLI-first scope.")]

    plan = plan_milestones(desired, existing)

    assert [item.action for item in plan] == ["skip", "create"]
    assert plan[1].desired.title == "Beta"


def test_milestone_update_detected_when_metadata_differs():
    desired = [MilestoneDef("MVP", "New description.")]
    existing = [ExistingMilestone(number=10, title="MVP", description="Old description.")]

    plan = plan_milestones(desired, existing)

    assert len(plan) == 1
    assert plan[0].action == "update"
    assert plan[0].existing is existing[0]


def test_bootstrap_issues_deduplicated_by_title_and_bootstrap_label():
    desired = [BootstrapIssueDef("[Task] Slice 3", "body", (BOOTSTRAP_LABEL, "agent:worker"))]
    existing = [ExistingIssue(number=4, title="[Task] Slice 3", labels=(BOOTSTRAP_LABEL, "agent:worker"))]

    plan = plan_bootstrap_issues(desired, existing)

    assert len(plan) == 1
    assert plan[0].action == "skip"
    assert plan[0].existing is existing[0]


def test_extra_labels_and_issues_are_preserved_not_deleted():
    label_plan = plan_labels(
        [LabelDef("agent:worker", "BFDADC", "Ready for a worker.")],
        [
            ExistingLabel("agent:worker", "BFDADC", "Ready for a worker."),
            ExistingLabel("custom", "000000", "Keep me."),
        ],
    )
    issue_plan = plan_bootstrap_issues(
        [BootstrapIssueDef("[Task] Slice 3", "body", (BOOTSTRAP_LABEL,))],
        [ExistingIssue(number=99, title="User issue", labels=("custom",))],
    )

    assert [item.action for item in label_plan] == ["skip"]
    assert [item.action for item in issue_plan] == ["create"]


def test_same_title_without_bootstrap_label_is_not_deduplicated():
    desired = [BootstrapIssueDef("[Task] Slice 3", "body", (BOOTSTRAP_LABEL,))]
    existing = [ExistingIssue(number=4, title="[Task] Slice 3", labels=("agent:worker",))]

    plan = plan_bootstrap_issues(desired, existing)

    assert len(plan) == 1
    assert plan[0].action == "create"


def test_cli_sync_dry_run_outputs_plan_without_applying(monkeypatch):
    sync_plan = SyncPlan(
        labels=[
            LabelPlan("create", LabelDef("agent:worker", "BFDADC", "Ready for a worker."))
        ],
        milestones=[
            MilestonePlan("skip", MilestoneDef("MVP", "CLI-first scope."))
        ],
        bootstrap_issues=[
            BootstrapIssuePlan(
                "create",
                BootstrapIssueDef("[Task] Slice 3", "body", (BOOTSTRAP_LABEL,)),
            )
        ],
    )

    class FakeClient:
        def __init__(self, repo):
            self.repo = repo

    def fail_apply(*args, **kwargs):
        raise AssertionError("dry-run must not apply sync plan")

    monkeypatch.setattr(cli, "GhClient", FakeClient)
    monkeypatch.setattr(cli, "build_sync_plan", lambda client: sync_plan)
    monkeypatch.setattr(cli, "apply_sync_plan", fail_apply)

    result = runner.invoke(cli.app, ["sync", "--repo", "StatPan/example", "--dry-run"])

    assert result.exit_code == 0, result.output
    assert "sync plan:" in result.output
    assert "labels:" in result.output
    assert "would create" in result.output
    assert "issue: [Task] Slice 3" in result.output
