# Adoption Signals

Gira measures early adoption as repeatable public signals, not as active-user
truth. Downloads, release asset pulls, repo traffic, and docs visits can show
whether people can find and install Gira, but they do not prove successful use.

## Current Decision

Do not add default-on CLI telemetry.

Do not claim active users from download counts.

Do not add `gira stats adoption` yet. For now, use a docs-only operator note
and optional release checklist snapshot. A command should wait until the same
manual report shape has been reused across release cycles.

## Signals

| Signal | Source | Meaning | Limit |
| --- | --- | --- | --- |
| npm downloads | `@statpan/gira` public npm downloads endpoint | npm/bun wrapper reach | Can include CI, bots, reinstalls, and cache misses. |
| PyPI downloads | PyPIStats or PyPI BigQuery for `gira-cli` | uv, pipx, and pip wrapper reach | Can include CI and automated installs. |
| Release assets | GitHub Release asset `download_count` | Native binary pull proxy | Checksums and archives should be counted separately. |
| Repo engagement | GitHub stars, forks, watchers, issues, PRs | Public project interest | Low counts do not prove no usage. |
| Repo traffic | GitHub views, clones, referrers, paths | Short-window discovery | GitHub traffic is a 14-day aggregate and clone spikes may be automation-heavy. |
| Docs traffic | Privacy-safe aggregate docs analytics, if enabled | Onboarding interest | Not enabled by default; do not add invasive tracking. |

## Snapshot Commands

```bash
curl -fsSL 'https://api.npmjs.org/downloads/point/last-week/@statpan/gira'
curl -fsSL 'https://api.npmjs.org/downloads/range/YYYY-MM-DD:YYYY-MM-DD/@statpan/gira'
curl -fsSL 'https://pypistats.org/api/packages/gira-cli/recent'
gh api repos/StatPan/gira/releases --paginate
gh api repos/StatPan/gira/traffic/views
gh api repos/StatPan/gira/traffic/clones
gh api repos/StatPan/gira/traffic/popular/referrers
gh api repos/StatPan/gira
```

Capture snapshots before a release announcement, one week after a release, and
monthly while the project is early.

## Interpretation Rules

- Treat npm and PyPI downloads as channel reach.
- Treat GitHub Release asset downloads as the best public proxy for native
  binary acquisition.
- Treat GitHub stars, forks, issues, and PRs as engagement, not usage.
- Treat GitHub traffic as short-window discovery only.
- Treat docs traffic as unknown until privacy-safe aggregate measurement is
  deliberately enabled.
- Never store IP addresses, user agents, or user-level events for this report.

## Related Docs

The full internal model is in `docs/adoption-signal-model.md`. Distribution
details are in [Distribution](/distribution), and workflow health remains
separate in [Closure Funnel Stats](/closure-funnel-stats).
