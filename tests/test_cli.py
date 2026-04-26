from typer.testing import CliRunner

from gira.cli import app

runner = CliRunner()


def test_help_output():
    result = runner.invoke(app, ["--help"])
    assert result.exit_code == 0
    assert "GitHub-native project OS bootstrapper" in result.output


def test_bootstrap_dry_run_is_deterministic():
    args = [
        "bootstrap",
        "--repo",
        "StatPan/example",
        "--template",
        "default",
        "--dry-run",
        "--created-at",
        "2026-04-26",
    ]
    first = runner.invoke(app, args)
    second = runner.invoke(app, args)

    assert first.exit_code == 0
    assert second.exit_code == 0
    assert first.output == second.output
    assert "--- AGENTS.md" in first.output


def test_bootstrap_dry_run_substitutes_template_variables():
    result = runner.invoke(
        app,
        [
            "bootstrap",
            "--repo",
            "StatPan/example",
            "--template",
            "default",
            "--dry-run",
            "--created-at",
            "2026-04-26",
        ],
    )

    assert result.exit_code == 0
    assert "StatPan/example" in result.output
    assert "example" in result.output
    assert "2026-04-26" in result.output
