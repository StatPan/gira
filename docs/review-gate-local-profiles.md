# Local review-gate profiles

`gira review gate --local-exec` executes checks only in a trusted checkout.
It first uses `review.local_checks` from `.gira/config.yaml` (or TOML). The
commands are argument arrays, not shell strings, so the selected program and
arguments are visible in the result.

```yaml
review:
  local_checks:
    - name: ruff
      command: [ruff, check, .]
    - name: mypy
      command: [mypy, src]
    - name: pytest
      command: [pytest]
```

Without an explicit profile, Gira detects a Go checkout from `go.mod`, or a
Python checkout from configured `ruff`, `mypy`, or `pytest` sections in
`pyproject.toml`. It does not combine detections: a mixed-language checkout
must declare `review.local_checks`.

If no safe profile exists, the gate returns `configuration_needed` with the
`local_review_profile_required` blocker. This is a configuration gap, not a
language-specific check failure.
