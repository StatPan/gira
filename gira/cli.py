from __future__ import annotations

from datetime import date
from typing import Optional

import typer

from gira.config import parse_repo_ref
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
    created_at: Optional[str] = typer.Option(None, "--created-at", help="Override render date for deterministic tests."),
) -> None:
    """Bootstrap a repository into a Gira-managed project workspace."""
    if not dry_run:
        raise typer.BadParameter("Slice 1 only supports --dry-run; real bootstrap comes later")

    repo_ref = parse_repo_ref(repo)
    rendered = render_template_tree(
        template=template,
        repo=repo_ref,
        created_at=created_at or date.today().isoformat(),
    )
    typer.echo(format_dry_run(rendered), nl=False)


if __name__ == "__main__":
    app()
