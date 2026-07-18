# V3 PM Harness Release Readiness

## Release decision

The V3 PM harness is CLI-first and model-independent. `pm-bootstrap/v1` binds a
caller to the same policy, source schemas, canonical fingerprints, authority
evidence, and receipt sequence whether the caller is a human terminal operator
or an AI host using MCP. MCP remains an adapter over those commands and does not
own a second state machine.

## Compatibility and rollback

- The new bootstrap and conformance schemas are additive. Existing PM, Goal,
  ticket, and generic `gira_cli` contracts remain valid.
- Goal-only compile now uses that Goal body as the intent source when no inline
  or file intent is supplied. Explicit intent still takes precedence.
- Focused MCP PM tools are read-only or dry-run-only. Existing apply paths and
  their approval plans are unchanged.
- Rollback is operationally safe: stop calling the focused tools and bootstrap,
  then use the prior CLI commands. The new surfaces persist no session database,
  credentials, transcripts, or canonical report copies.

## Conformance and dogfood evidence

Run:

```bash
gira pm conformance --json
gira mcp doctor --repo StatPan/gira --json
gira pm bootstrap --repo StatPan/gira --ticket 857 --role human --json
gira pm bootstrap --repo StatPan/gira --ticket 857 --role ai --authority issue:read --json
```

The built-in suite contains one complete human CLI run and two AI host/model
configurations over the same stages and receipts. The bounded weak configuration
records context loss, premature delivery, generic human escalation, unsupported
claims, and authority overreach; every attempted unsafe transition is contained.
`protocol_compliant` and `semantic_quality` are separate fields, so a limited
model is never advertised as having equivalent PM judgment.

Repository dogfood on 2026-07-19 produced three compliant runs (one human, two
AI configurations), five safely contained weak-host failures, and zero unsafe
mutations. MCP doctor reported all six focused PM tools present with current
policy/protocol versions. The first #857 bootstrap correctly stopped on two
legacy Goal compile gaps; after adding explicit Problem and Success Conditions,
compile errors fell to zero and the human bootstrap emitted all ten stages in
3,881 of 6,000 characters. Its next replan remained unapplied because explicit
`plan:write` authority was absent.

## Privacy and telemetry boundary

Permitted evidence is contract version, stage/receipt presence, source refs,
fingerprints, contained failure modes, and aggregate conformance counts. Gira
must not capture secrets, private conversation transcripts, individual worker
rankings, or token-spend productivity scores. Conformance is a protocol safety
benchmark, not employee surveillance or a model-cost leaderboard.

## Release checklist

- [x] Human CLI can bootstrap, compile, observe, preview replan, validate, and
  report without MCP or an LLM.
- [x] Focused MCP tools invoke the same CLI/domain contracts and expose no apply
  shortcut.
- [x] Work-graph and replan apply reject missing or stale fingerprints; authority
  remains explicit in bootstrap and approval/capability receipts.
- [x] Built-in human and two-configuration AI conformance runs record zero unsafe
  mutations and keep semantic quality separate.
- [x] MCP doctor reports policy/schema parity and conformance evidence.
- [x] Full Go tests, race-focused harness tests, generated documentation checks,
  and docs-site build are required before merge.
- [x] Dogfood commands, compatibility behavior, privacy limits, and rollback are
  documented here.
