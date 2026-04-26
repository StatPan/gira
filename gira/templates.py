from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

from jinja2 import Environment, FileSystemLoader, StrictUndefined

from gira.config import RepoRef


@dataclass(frozen=True)
class RenderedTemplate:
    path: str
    content: str


_TEMPLATE_ROOT = Path(__file__).resolve().parent.parent / "templates"


def render_template_tree(template: str, repo: RepoRef, created_at: str) -> list[RenderedTemplate]:
    """Render every file in a Gira template tree in deterministic path order."""
    if Path(template).name != template:
        raise ValueError(f"invalid template name: {template}")

    source = _TEMPLATE_ROOT / template
    if not source.is_dir():
        raise ValueError(f"unknown template: {template}")

    env = Environment(
        loader=FileSystemLoader(str(source)),
        undefined=StrictUndefined,
        autoescape=False,
        keep_trailing_newline=True,
    )
    context = {
        "repo_owner": repo.owner,
        "repo_name": repo.name,
        "repo_full_name": repo.full_name,
        "created_at": created_at,
    }

    rendered: list[RenderedTemplate] = []
    for path in _iter_template_files(source):
        rel = path.relative_to(source).as_posix()
        out_rel = rel[:-3] if rel.endswith(".j2") else rel
        content = env.get_template(rel).render(context)
        rendered.append(RenderedTemplate(path=out_rel, content=content))
    return rendered


def format_dry_run(rendered: Iterable[RenderedTemplate]) -> str:
    """Return a stable, human-readable dry-run rendering."""
    lines: list[str] = []
    for item in rendered:
        lines.append(f"--- {item.path}")
        lines.append(item.content.rstrip())
        lines.append("")
    return "\n".join(lines).rstrip() + "\n"


def _iter_template_files(source: Path) -> list[Path]:
    return sorted(path for path in source.rglob("*") if path.is_file())
