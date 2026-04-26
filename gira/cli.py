from __future__ import annotations

from datetime import date
from pathlib import Path
from typing import Optional

import typer

from gira.config import parse_repo_ref
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

    result = install_templates(
        target_path=path,
        rendered=rendered,
        overwrite=overwrite,
        branch=None if no_branch else branch,
    )
    typer.echo(format_summary(result), nl=False)
    if result.conflicts:
        raise typer.Exit(code=1)


if __name__ == "__main__":
    app()
