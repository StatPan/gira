# Feature Map

Gira supports an optional issue-backed feature map for teams that want a durable
capability view in GitHub.

The model is:

| Surface | Role |
| --- | --- |
| GitHub issue | Canonical feature or capability record. |
| GitHub Project | Visibility view for the map, roadmap, and todo slices. |
| Milestone | Delivery batch for executable work. |
| PR | Implementation evidence. |
| Gira | Read-only checker/compiler. |

Feature records are GitHub issues labeled `type:capability` or `type:feature`,
or issues titled with `Capability:` or `Feature:`.

```markdown
Key: tl
Status: stable

## User Need
## Capability
## Surface
## Docs
## Evidence
```

Work issues can link back with:

```markdown
Related capability: #31
```

Start with the read-only commands:

```bash
gira feature list --repo OWNER/REPO
gira feature check --repo OWNER/REPO
gira feature for 123 --repo OWNER/REPO
```

For daily typing, use the short alias:

```bash
gira feat check
gira feat for 123
```

If no feature records exist, the map is treated as not configured. Normal ticket
lifecycle commands keep working.
