from __future__ import annotations

import json
from datetime import date
from pathlib import Path
from typing import Optional

import typer

from gira.config import parse_repo_ref
from gira.github_status import build_status_summary, format_status_text
from gira.github_sync import GhClient, GhError, apply_sync_plan, build_sync_plan, format_sync_plan
from gira.install import DEFAULT_BRANCH, format_summary, install_templates
from gira.templates import format_dry_run, render_template_tree

app = typer.Typer(help="Gira: GitHub-native project OS bootstrapper.")


@app.callback()
def main() -> None:
    """Gira: GitHub-native project OS bootstrapper."""


@app.command()
def bootstrap(
    repo: str = typer.Option(..., "--repo", help="Target GitHub repo in OWNER/REPO format."),
    template: str = typer.Option("default", "--template", help="Template name to render."),
    dry_run: bool = typer.Option(False, "--dry-run", help="Render without writing files or calling GitHub."),
    path: Optional[Path] = typer.Option(None, "--path", help="Local target git repo path (required for non-dry-run)."),
    overwrite: bool = typer.Option(False, "--overwrite", help="Overwrite existing files that differ."),
    branch: str = typer.Option(DEFAULT_BRANCH, "--branch", help="Branch to create/checkout before install."),
    no_branch: bool = typer.Option(False, "--no-branch", help="Skip branch creation/checkout."),
    created_at: Optional[str] = typer.Option(None, "--created-at", help="Override render date for deterministic tests."),
) -> None:
    """Bootstrap a repository into a Gira-managed project workspace."""
    repo_ref = parse_repo_ref(repo)
    rendered = render_template_tree(
        template=template,
        repo=repo_ref,
        created_at=created_at or date.today().isoformat(),
    )

    if dry_run:
        typer.echo(format_dry_run(rendered), nl=False)
        return

    if path is None:
        raise typer.BadParameter("--path is required when not running --dry-run")

    try:
        result = install_templates(
            target_path=path,
            rendered=rendered,
            overwrite=overwrite,
            branch=None if no_branch else branch,
        )
    except ValueError as exc:
        raise typer.BadParameter(str(exc), param_hint="--path") from exc
    typer.echo(format_summary(result), nl=False)
    if result.conflicts:
        raise typer.Exit(code=1)


@app.command()
def sync(
    repo: str = typer.Option(..., "--repo", help="Target GitHub repo in OWNER/REPO format."),
    dry_run: bool = typer.Option(False, "--dry-run", help="Plan sync without creating or updating GitHub metadata."),
) -> None:
    """Sync Gira labels, milestones, and bootstrap issues through gh."""
    repo_ref = parse_repo_ref(repo)
    client = GhClient(repo_ref)
    try:
        plan = build_sync_plan(client)
        typer.echo(format_sync_plan(plan, dry_run=dry_run), nl=False)
        if not dry_run:
            apply_sync_plan(client, plan)
            typer.echo("sync complete")
    except GhError as exc:
        raise typer.BadParameter(str(exc), param_hint="--repo") from exc


@app.command()
def status(
    repo: str = typer.Option(..., "--repo", help="Target GitHub repo in OWNER/REPO format."),
    json_output: bool = typer.Option(False, "--json", help="Emit stable JSON for automation."),
    stale_days: int = typer.Option(14, "--stale-days", min=1, help="Days since update before open issues count as stale."),
) -> None:
    """Show a compact read-only status summary from GitHub issues and milestones."""
    repo_ref = parse_repo_ref(repo)
    client = GhClient(repo_ref)
    try:
        summary = build_status_summary(client, stale_days=stale_days)
    except GhError as exc:
        raise typer.BadParameter(str(exc), param_hint="--repo") from exc

    if json_output:
        typer.echo(json.dumps(summary, sort_keys=True, indent=2))
    else:
        typer.echo(format_status_text(summary), nl=False)


if __name__ == "__main__":
    app()
