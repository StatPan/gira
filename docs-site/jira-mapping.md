# Jira To GitHub Mapping

Gira keeps Jira-style language while storing canonical state in GitHub.

| Jira | GitHub | Gira behavior |
| --- | --- | --- |
| Workspace | Account plus configured repos | Optional inbox and repo backlog visibility. |
| Project | Repository | The default execution space. |
| Epic | Top-level issue with `type:epic` | `gira epic status` and `gira epic finish`. |
| Story, task, bug | Issue | Main executable work packet. |
| Sprint or release phase | Milestone | Planning and rollover boundary. |
| Status | Labels plus PR evidence | Cross-checked against branch, PR, review, and checks. |
| Done | Merged PR plus closed issue | Completion is proven by GitHub evidence. |
