# Finish review policy

`finish_review_policy` is a repository contract in `.gira/config.yaml`.

```yaml
finish_review_policy: required # or: none
```

New repo-scope contracts created by `gira workspace init` use `required`. A
solo-maintainer repository may explicitly choose `none`. In an existing
contract that does not declare this value, `gira ticket finish` fails closed with
`review_policy_not_configured`; add an explicit value before retrying. A
required policy accepts only an approving GitHub review recorded for the
current PR head. This keeps a review from an earlier head from authorizing a
later change.
