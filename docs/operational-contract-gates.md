# Operational Contract Gates

Operational contract gates catch impossible lifecycle state before Gira renders reports, queues work, or asks an agent to continue.

The first gate covers `ticket-status/v1` through `ValidateWorkStatusContract`. It validates the normalized state Gira already produces from GitHub issue, PR, check, review, and finish evidence.

## Scope

The gate checks state combinations, not prose quality or GitHub availability.

Examples of blocked combinations:

- `checks_status=passed` with no check evidence for an available PR;
- `next_action=done` while the issue is still open;
- finish actions without a linked PR or closing-reference evidence;
- failed or pending check summaries without matching check evidence or blockers;
- mismatched PR numbers across `pr_number`, `pull_request`, and `pr_readiness`;
- unknown lifecycle values for `next_action`, `checks_status`, `review_status`, or check state.

## Relationship To Other Gates

This gate is narrower than dogfood smoke testing. Dogfood proves that real CLI flows still work against GitHub. Operational contracts prove that Gira does not accept internally inconsistent lifecycle state as valid input.

This gate is also narrower than release testing. Release tests cover packaging, command behavior, and distribution readiness. Operational contracts protect the meaning of shared JSON surfaces such as `ticket-status/v1`, `pr-readiness/v1`, `finish-readiness/v1`, and `workspace-queues/v1`.

## Local Command

Run the focused gate with:

```sh
sh scripts/check-operational-contract.sh
```

The script runs the operational contract tests and the agent workflow benchmark fixtures together, because benchmark fixtures are the smallest local examples of user-visible lifecycle decisions.
