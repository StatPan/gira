# MCP Authentication Interface

## Status

Research decision for #746.

## Sources

- [MCP Authorization tutorial](https://modelcontextprotocol.io/docs/tutorials/security/authorization)
- [MCP Authorization specification](https://modelcontextprotocol.io/specification/draft/basic/authorization)
- [GitHub MCP server](https://github.com/github/github-mcp-server)
- [GitHub MCP Enterprise configuration](https://docs.github.com/en/copilot/how-tos/provide-context/use-mcp-in-your-ide/enterprise-configuration)
- [CLI and MCP Workflow Parity](cli-mcp-workflow-parity.md)
- [Hosted MCP Service Boundary](hosted-mcp-service-boundary.md)

## Problem

`gira mcp serve` currently behaves like a local CLI wrapper. It calls `gira ... --json`, and most Gira GitHub access flows eventually rely on the local `gh` authentication context.

That is safe for local CLI parity, but it is incomplete as an MCP interface. MCP users commonly expect credentials to be supplied explicitly to the server process by environment variables, secrets managers, or remote authorization flows. Hosted or remote MCP also has a different authorization model from local stdio MCP.

Gira needs an explicit auth interface so agents know which credential source is used and so future hosted MCP does not accidentally inherit local CLI assumptions.

## Decision

Gira should support three distinct MCP auth modes:

| Mode | Scope | Credential source | Status |
| --- | --- | --- | --- |
| `local-gh` | Local stdio MCP on the user's machine. | Existing `gh` CLI auth context. | Supported now and remains default. |
| `env-token` | Local stdio MCP launched by an MCP client or secrets manager. | Environment variable supplied to the `gira mcp serve` process. | Accepted as the next bounded implementation. |
| `hosted-oauth` | Future remote/hosted MCP. | OAuth or GitHub App based authorization. | Deferred; requires hosted MCP auth policy and implementation. |

The v1 local MCP server should keep `local-gh` as default and add `env-token` as an explicit optional mode. It should not store credentials in Gira config.

## Recommended Local Interface

Add support for these environment variables:

| Variable | Purpose |
| --- | --- |
| `GIRA_MCP_GITHUB_TOKEN` | Preferred token for Gira MCP GitHub access. |
| `GITHUB_TOKEN` | Compatibility fallback used by many GitHub tools and CI systems. |
| `GH_TOKEN` | Compatibility fallback recognized by GitHub CLI. |
| `GITHUB_HOST` | Optional GitHub Enterprise host, matching GitHub MCP conventions where useful. |

Precedence should be:

```text
GIRA_MCP_GITHUB_TOKEN
  -> GITHUB_TOKEN
  -> GH_TOKEN
  -> existing gh auth context
```

The first three modes should be implemented by passing the selected token to child CLI/`gh` calls through the process environment, not by writing it to disk.

## Why Keep `gh` Auth Default

`gh` auth reuse remains the best default for local CLI parity:

- it matches how existing Gira commands behave;
- it avoids Gira storing credentials;
- it respects the user's current GitHub account and host setup;
- it keeps local MCP equivalent to local CLI behavior.

However, `gh` auth should not be the only supported interface because many MCP hosts configure servers by command plus environment variables, and remote/containerized MCP servers may not have an interactive `gh auth login` context.

## Why Add Env Token Mode

Env-token mode is accepted for local stdio MCP because it is the common MCP deployment pattern for local servers launched by a host application or a secrets manager.

Rules:

- Never print token values.
- Never write token values to Gira config, cache, receipts, logs, or issue comments.
- Do not add token flags such as `--token`; prefer process environment to avoid shell history leaks.
- Treat env-token auth as equivalent to the caller's GitHub authority, not as a Gira identity.
- Continue to return deterministic MCP tool errors when auth is missing or insufficient.

## Hosted And Remote MCP

The official MCP authorization model is transport-level authorization for HTTP-based transports. Hosted MCP should not reuse the local stdio env-token design as its long-term auth model.

Hosted Gira MCP should prefer one of these later designs:

- GitHub App installation tokens with least-privilege repo scopes;
- OAuth authorization tied to the MCP host/client and user consent;
- short-lived delegated credentials with explicit tenant isolation.

Hosted MCP must remain read-only by default under the current hosted boundary. Hosted mutation requires a separate ADR.

## Failure Behavior

When no usable auth is available, Gira MCP should fail closed:

- tool result should be an MCP error result, not a panic;
- error should say GitHub authentication is unavailable or insufficient;
- error should recommend one of: `gh auth status`, `gh auth login`, or setting `GIRA_MCP_GITHUB_TOKEN` in the MCP client environment;
- error must not include secret values;
- unsupported hosted or OAuth modes should return explicit unsupported-mode errors.

## Doctor Impact

Future implementation should add an MCP auth diagnostic surface, either:

```bash
gira mcp doctor --repo OWNER/REPO --json
```

or a section inside existing:

```bash
gira doctor --repo OWNER/REPO --json
```

The diagnostic should report:

- selected auth mode without exposing token values;
- whether `gh auth status` succeeds when using local-gh mode;
- whether an env token variable is present when env-token mode is selected;
- GitHub host selection;
- next safe setup step.

## Follow-up

Accepted implementation follow-up: add local env-token support and MCP auth diagnostics.

Implementation successor: #747.

## Implemented local commands

`gira mcp serve` now selects local authentication using the documented precedence and passes the selected token only through child process environment.

`gira mcp doctor --repo OWNER/REPO --json` reports the selected auth mode, token variable presence without values, GitHub host, and next setup step.

Example MCP client config with an env token:

```toml
[mcp_servers.gira]
command = "gira"
args = ["mcp", "serve"]
env = { GIRA_MCP_GITHUB_TOKEN = "..." }
```
