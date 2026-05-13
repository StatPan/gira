## Related Ticket

Closes #<issue-number>

## Summary

## Gira Lifecycle

- [ ] Work started from a repo issue with `gira ticket start <id> --apply`
- [ ] Branch belongs to one ticket and the PR title/body match that scope
- [ ] PR body includes `Closes #N`, `Fixes #N`, or `Resolves #N`
- [ ] Finish will use `gira ticket finish <id> --apply` after review and checks are ready

## Verification

Commands or checks run:

```text
go test ./...
```

## Production Readiness

- [ ] Tests or manual verification are recorded above
- [ ] Schema, data, or migration impact is described, or not applicable
- [ ] Rollout and rollback plan is described, or not applicable
- [ ] Observability impact is described, or not applicable
- [ ] Security, permissions, or secret impact is described, or not applicable
- [ ] Documentation or runbook updates are included, or not applicable

## Notes
