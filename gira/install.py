from __future__ import annotations

import subprocess
from dataclasses import dataclass, field
from pathlib import Path
from typing import Iterable

from gira.templates import RenderedTemplate

DEFAULT_BRANCH = "chore/gira-bootstrap"


@dataclass
class InstallResult:
    created: list[str] = field(default_factory=list)
    skipped: list[str] = field(default_factory=list)
    conflicts: list[str] = field(default_factory=list)
    overwritten: list[str] = field(default_factory=list)
    branch: str | None = None

    @property
    def changed(self) -> bool:
        return bool(self.created or self.overwritten)


def is_git_repo(path: Path) -> bool:
    return (path / ".git").exists()


def ensure_branch(path: Path, branch: str) -> None:
    """Create branch if missing and check it out. Idempotent."""
    existing = subprocess.run(
        ["git", "-C", str(path), "rev-parse", "--verify", "--quiet", f"refs/heads/{branch}"],
        capture_output=True,
        text=True,
    )
    if existing.returncode == 0:
        subprocess.run(
            ["git", "-C", str(path), "checkout", branch],
            check=True,
            capture_output=True,
        )
    else:
        subprocess.run(
            ["git", "-C", str(path), "checkout", "-b", branch],
            check=True,
            capture_output=True,
        )


def install_templates(
    target_path: Path,
    rendered: Iterable[RenderedTemplate],
    overwrite: bool = False,
    branch: str | None = DEFAULT_BRANCH,
) -> InstallResult:
    """Install rendered templates into a local git repo idempotently."""
    target_path = target_path.resolve()
    if not target_path.is_dir():
        raise ValueError(f"target path is not a directory: {target_path}")
    if not is_git_repo(target_path):
        raise ValueError(f"target path is not a git repository: {target_path}")

    if branch is not None:
        ensure_branch(target_path, branch)

    result = InstallResult(branch=branch)
    for item in rendered:
        rel = Path(item.path)
        if rel.is_absolute() or ".." in rel.parts:
            raise ValueError(f"unsafe template path: {item.path}")

        dest = target_path / rel
        new_bytes = item.content.encode("utf-8")

        if dest.exists():
            existing = dest.read_bytes()
            if existing == new_bytes:
                result.skipped.append(item.path)
                continue
            if not overwrite:
                result.conflicts.append(item.path)
                continue
            dest.write_bytes(new_bytes)
            result.overwritten.append(item.path)
        else:
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_bytes(new_bytes)
            result.created.append(item.path)

    return result


def format_summary(result: InstallResult) -> str:
    lines = []
    if result.branch:
        lines.append(f"branch: {result.branch}")
    lines.append(f"created:     {len(result.created)}")
    lines.append(f"skipped:     {len(result.skipped)}")
    lines.append(f"overwritten: {len(result.overwritten)}")
    lines.append(f"conflicts:   {len(result.conflicts)}")
    for path in result.conflicts:
        lines.append(f"  conflict: {path}")
    return "\n".join(lines) + "\n"
