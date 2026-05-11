# Jira To GitHub Mapping

Gira keeps Jira-style language while storing canonical state in GitHub.

| Jira | GitHub | Gira behavior |
| --- | --- | --- |
| Workspace | Account plus configured repos | Optional inbox and repo backlog visibility. |
| Project | Repository plus optional repo-linked Project board | Repo issues are canonical; user/org Projects can show them as board views. |
| Epic | Top-level issue with `type:epic` | `gira epic status` and `gira epic finish`. |
| Story, task, bug | Issue | Main executable work packet. |
| Sprint or release phase | Milestone | Planning and rollover boundary. |
| Status | Labels plus PR evidence | Cross-checked against branch, PR, review, and checks. |
| Done | Merged PR plus closed issue | Completion is proven by GitHub evidence. |

Project-only items are intake, portfolio, or visibility context until they are routed or lowered to repository issues.

Jira-primary provider mode is the exception to the default status ownership rule: Jira owns planning and status, while GitHub still owns execution evidence. See [Jira-Primary Provider Mode](/jira-primary-provider).
