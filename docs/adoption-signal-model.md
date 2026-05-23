# Adoption Signal Model

Gira should measure early adoption as distribution and engagement signals, not
as active-user truth. Package downloads, release asset pulls, repository
traffic, and docs visits are all useful, but none of them prove a person used
Gira successfully.

This model answers one bounded question:

> Is there repeatable evidence that people can find, install, and engage with
> Gira, and which channel appears to carry that signal?

## Decision

Do not add default-on CLI telemetry.

Do not claim active users from package-manager downloads.

Do not implement `gira stats adoption` yet. The first owner is a docs-only
operator note plus an optional release checklist snapshot. A command becomes
worth implementing only after at least two release cycles produce manual
snapshots with the same fields and the reporting format stabilizes.

## Signal Tiers

| Tier | Signal | Source | Interpretation | Risk |
| --- | --- | --- | --- | --- |
| Distribution reach | npm downloads for `@statpan/gira` | `https://api.npmjs.org/downloads/*/@statpan/gira` | Wrapper package was downloaded from npm. Useful for JavaScript-channel reach. | Includes automated installs, CI, mirrors, repeated upgrades, and wrapper fetches that may not mean Gira ran. |
| Distribution reach | PyPI downloads for `gira-cli` | PyPIStats / PyPI BigQuery download data | Python wrapper was downloaded. Useful for uv, pipx, and pip channel reach. | Includes automated installs and CI; PyPIStats is rate-limited and retained for a bounded history window. |
| Binary pull signal | GitHub Release asset `download_count` | GitHub Releases REST API | Native release archive or checksum was downloaded. Closest public proxy for actual binary acquisition. | Installers may download checksums separately; package wrappers may pull assets; repeated downloads inflate counts. |
| Repo engagement | stars, forks, watchers, issues, PRs | GitHub repo API and issue/PR APIs | Lightweight public interest and contribution signal. | Very early projects can be used without stars; low counts do not prove no usage. |
| Repo traffic | views, clones, referrers, paths | GitHub traffic API | Short-window anonymous repo discovery and clone signal. | GitHub exposes only recent aggregate traffic and clone spikes can be bot/scraper-heavy. |
| Docs interest | docs visits and top pages | Privacy-safe docs analytics, if explicitly enabled | Product education and onboarding interest. | GitHub Pages does not provide a full product analytics surface; adding client analytics is a product/privacy decision. |
| Release health | publish channel success and smoke checks | GitHub Actions release workflow plus manual smoke tests | Whether each channel is healthy enough to trust signal from it. | A channel with failed publishing should not be interpreted as low adoption. |

## Current Baseline Snapshot

Snapshot date: 2026-05-23.

Commands used:

```bash
curl -fsSL 'https://api.npmjs.org/downloads/point/last-week/@statpan/gira'
curl -fsSL 'https://api.npmjs.org/downloads/range/2026-05-06:2026-05-23/@statpan/gira'
curl -fsSL 'https://pypistats.org/api/packages/gira-cli/recent'
gh api repos/StatPan/gira/releases --paginate
gh api repos/StatPan/gira/traffic/views
gh api repos/StatPan/gira/traffic/clones
gh api repos/StatPan/gira/traffic/popular/referrers
gh api repos/StatPan/gira
```

Observed values:

| Signal | Value |
| --- | --- |
| npm `@statpan/gira` last week | 361 downloads, 2026-05-15 through 2026-05-21 |
| npm `@statpan/gira` range | 3,231 downloads, 2026-05-06 through 2026-05-23 |
| PyPIStats `gira-cli` recent | last_day 11, last_week 410, last_month 2,821 |
| GitHub repo traffic views | 29 total views, 7 unique visitors, 14-day window |
| GitHub repo traffic clones | 5,496 total clones, 2,100 unique cloners, 14-day window |
| GitHub repo referrers | `github.com` only in top referrers sample, 18 views, 3 uniques |
| GitHub repo public counters | 1 star, 0 forks, 1 watcher, 0 subscribers, 4 open issues |

The clone count is much larger than views and should be treated as likely
automation-heavy until correlated with issues, PRs, docs visits, release asset
pulls, or package downloads.

## Data Sources And Limits

### npm

Use the public npm downloads endpoint for the wrapper package:

```text
https://api.npmjs.org/downloads/point/last-week/@statpan/gira
https://api.npmjs.org/downloads/range/YYYY-MM-DD:YYYY-MM-DD/@statpan/gira
```

Store the raw `start`, `end`, `package`, and `downloads` values. npm registry
API surfaces can return `429` when rate limited, so cache snapshots and avoid
polling more than once per day.

Interpret npm downloads as channel reach, not Gira active usage. The npm wrapper
installs or invokes the Go-built binary; downloads can include CI, reinstall,
cache-miss, and bot behavior.

### PyPI

Use PyPIStats for lightweight manual snapshots:

```text
https://pypistats.org/api/packages/gira-cli/recent
```

PyPIStats documents IP-based rate limiting, daily data updates, mirror
exclusion, and a 180-day retention window. For long-range or repeated reporting,
query the official PyPI downloads BigQuery dataset instead of scraping broad
history from PyPIStats.

Interpret PyPI downloads as wrapper-channel reach for uv, pipx, and pip. They
still include CI and repeated installs.

### GitHub Releases

Use the GitHub Releases REST API and keep asset-level counts:

```bash
gh api repos/StatPan/gira/releases --paginate
```

Each release asset includes `download_count`. Keep checksums separate from
platform archives because installers may fetch both. Roll up by tag and asset
family:

- `checksums.txt`
- `linux_amd64`
- `linux_arm64`
- `darwin_amd64`
- `darwin_arm64`
- `windows_amd64`

Native asset downloads are the strongest public install proxy, but they still
do not prove a successful first run.

### GitHub Repository Signals

Use public repo counters for slow-moving engagement:

```bash
gh api repos/StatPan/gira
```

Track stars, forks, watchers, subscribers, open issues, closed issues, external
issues, and external PRs. Keep this separate from workflow health metrics such
as Closure Funnel stats. Adoption asks whether people are finding and trying
Gira; Closure Funnel asks whether GitHub work is converging.

### GitHub Traffic

Use the GitHub traffic API only as a short-window discovery signal:

```bash
gh api repos/StatPan/gira/traffic/views
gh api repos/StatPan/gira/traffic/clones
gh api repos/StatPan/gira/traffic/popular/referrers
gh api repos/StatPan/gira/traffic/popular/paths
```

GitHub documents the traffic graph as a 14-day aggregate for views, full clones,
referrers, and popular content. Access requires appropriate repository
permissions. Do not treat clone spikes as adoption without corroborating
signals.

### Docs Traffic

GitHub Pages hosts the static docs site, but Gira should not assume that GitHub
repository traffic is the same thing as docs-site traffic for
`https://gira.statpan.com`.

Privacy-safe docs measurement options, in order:

1. No client analytics. Use release snapshots, package downloads, repo traffic,
   and explicit user feedback.
2. Aggregate, cookie-free analytics on the custom domain with a public privacy
   note and no user-identifying tracking.
3. Server-side CDN analytics if the domain is fronted by a provider that exposes
   aggregate page metrics.

Do not add Google Analytics or user-identifying telemetry by default.

## Report Shape

The manual snapshot should fit this shape:

```json
{
  "schema_version": "adoption-signal/v1",
  "generated_at": "2026-05-23T00:00:00Z",
  "window": {
    "npm": "last-week and explicit date range",
    "pypi": "recent",
    "github_traffic": "last 14 days",
    "release_assets": "all visible releases"
  },
  "channels": {
    "npm": {
      "package": "@statpan/gira",
      "downloads_last_week": 361,
      "downloads_range": 3231,
      "interpretation": "channel_reach"
    },
    "pypi": {
      "package": "gira-cli",
      "downloads_last_week": 410,
      "downloads_last_month": 2821,
      "interpretation": "channel_reach"
    },
    "github_releases": {
      "asset_downloads_by_tag": [],
      "interpretation": "binary_pull_proxy"
    },
    "github_repo": {
      "stars": 1,
      "forks": 0,
      "views_14d": 29,
      "unique_visitors_14d": 7,
      "clones_14d": 5496,
      "unique_cloners_14d": 2100,
      "interpretation": "engagement_and_discovery"
    },
    "docs": {
      "measurement": "not_enabled",
      "interpretation": "unknown"
    }
  },
  "confidence": "early_signal_only",
  "warnings": [
    "downloads_are_not_active_users",
    "github_clone_signal_may_be_bot_heavy",
    "docs_traffic_not_measured"
  ]
}
```

## Operating Rhythm

Capture the snapshot:

- before a release announcement.
- one week after the release.
- monthly while Gira is still early.

Store the result as a release note appendix, a maintainer issue comment, or a
private operator note. Do not store IP addresses, user agents, or per-user
events.

## Follow-Up Policy

Create a `gira stats adoption --json` implementation issue only if all of these
become true:

- the manual report shape is reused for at least two release cycles.
- the maintainer wants one command instead of copy/paste API calls.
- the command can remain read-only and privacy-safe.
- rate-limit and cache behavior are specified before implementation.

Until then, this document and the release checklist are enough.

## Source References

- npm package downloads endpoint:
  `https://api.npmjs.org/downloads/point/last-week/@statpan/gira`
- PyPIStats API: `https://pypistats.org/api/`
- PyPI API and datasets: `https://docs.pypi.org/api/`
- GitHub Releases REST API:
  `https://docs.github.com/en/rest/releases/releases`
- GitHub repository traffic docs:
  `https://docs.github.com/en/repositories/viewing-activity-and-data-for-your-repository/viewing-traffic-to-a-repository`
- GitHub repository traffic REST API:
  `https://docs.github.com/en/rest/metrics/traffic`
- GitHub Pages overview:
  `https://docs.github.com/en/pages/getting-started-with-github-pages/about-github-pages`
