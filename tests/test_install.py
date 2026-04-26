from __future__ import annotations

import subprocess
from pathlib import Path

import pytest
from typer.testing import CliRunner

from gira.cli import app

runner = CliRunner()


def _git(repo: Path, *args: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["git", "-C", str(repo), *args],
        check=True,
        capture_output=True,
        text=True,
    )


@pytest.fixture
def git_repo(tmp_path: Path) -> Path:
    repo = tmp_path / "target"
    repo.mkdir()
    _git(repo, "init", "-q", "-b", "main")
    _git(repo, "config", "user.email", "test@example.com")
    _git(repo, "config", "user.name", "test")
    (repo / "README.md").write_text("seed\n")
    _git(repo, "add", ".")
    _git(repo, "commit", "-q", "-m", "seed")
    return repo


def _bootstrap_args(repo_path: Path, *extra: str) -> list[str]:
    return [
        "bootstrap",
        "--repo",
        "StatPan/example",
        "--template",
        "default",
        "--path",
        str(repo_path),
        "--created-at",
        "2026-04-26",
        *extra,
    ]


def test_install_creates_files(git_repo: Path):
    result = runner.invoke(app, _bootstrap_args(git_repo))
    assert result.exit_code == 0, result.output
    assert (git_repo / "AGENTS.md").exists()
    assert "created:" in result.output


def test_install_is_idempotent(git_repo: Path):
    first = runner.invoke(app, _bootstrap_args(git_repo))
    assert first.exit_code == 0, first.output

    # capture working tree state after first run
    snapshot = {p: p.read_bytes() for p in git_repo.rglob("*") if p.is_file() and ".git" not in p.parts}

    second = runner.invoke(app, _bootstrap_args(git_repo))
    assert second.exit_code == 0, second.output

    after = {p: p.read_bytes() for p in git_repo.rglob("*") if p.is_file() and ".git" not in p.parts}
    assert snapshot == after
    assert "created:     0" in second.output


def test_existing_different_file_preserved_by_default(git_repo: Path):
    agents = git_repo / "AGENTS.md"
    agents.write_text("custom contents\n")
    _git(git_repo, "add", "AGENTS.md")
    _git(git_repo, "commit", "-q", "-m", "custom")

    result = runner.invoke(app, _bootstrap_args(git_repo))
    # conflicts cause non-zero exit code
    assert result.exit_code == 1
    assert agents.read_text() == "custom contents\n"
    assert "conflict" in result.output


def test_overwrite_replaces_existing(git_repo: Path):
    agents = git_repo / "AGENTS.md"
    agents.write_text("custom contents\n")
    _git(git_repo, "add", "AGENTS.md")
    _git(git_repo, "commit", "-q", "-m", "custom")

    result = runner.invoke(app, _bootstrap_args(git_repo, "--overwrite"))
    assert result.exit_code == 0, result.output
    assert agents.read_text() != "custom contents\n"
    assert "overwritten:" in result.output


def test_default_branch_is_chore_gira_bootstrap(git_repo: Path):
    result = runner.invoke(app, _bootstrap_args(git_repo))
    assert result.exit_code == 0, result.output
    current = _git(git_repo, "rev-parse", "--abbrev-ref", "HEAD").stdout.strip()
    assert current == "chore/gira-bootstrap"


def test_no_branch_keeps_current_branch(git_repo: Path):
    result = runner.invoke(app, _bootstrap_args(git_repo, "--no-branch"))
    assert result.exit_code == 0, result.output
    current = _git(git_repo, "rev-parse", "--abbrev-ref", "HEAD").stdout.strip()
    assert current == "main"


def test_install_requires_git_repo(tmp_path: Path):
    not_a_repo = tmp_path / "plain"
    not_a_repo.mkdir()
    result = runner.invoke(app, _bootstrap_args(not_a_repo))
    assert result.exit_code != 0


def test_non_dry_run_requires_path():
    result = runner.invoke(
        app,
        [
            "bootstrap",
            "--repo",
            "StatPan/example",
            "--template",
            "default",
        ],
    )
    assert result.exit_code != 0
    assert "--path" in result.output or "path" in result.output.lower()
