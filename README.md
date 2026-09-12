# doupass

A local-first policy engine for AI coding agents. One portable policy file governs tool calls across harnesses: MCP tool calls through a proxy, harness-native tools (Bash, Read, Write, Edit) through hooks.

Status: v0.1 in development. The core engine, MCP proxy, Claude Code hook, audit log, and CLI are working and tested. Packaging and docs land next; see `plans/` for the construction plan.

## Why

- Agent permissions today live in per-harness settings that cannot be shared, versioned, or audited together.
- Existing agent firewalls are enterprise/Kubernetes-heavy or point-in-time scanners.
- doupass gives you one policy, a local audit trail, and a small binary that runs on your machine.

## Quickstart

```sh
go install github.com/ogzhncnmr/doupass/cmd/doupass@latest   # Go 1.27+

doupass init                                                  # writes ./doupass.yml
doupass policy test doupass.yml --tool Read --arg file_path=~/.ssh/id_rsa
doupass install claude                                        # registers the PreToolUse hook
doupass proxy --server fs -- npx -y @modelcontextprotocol/server-filesystem .
doupass log tail
doupass log verify                                            # hash-chain tamper check
```

Prebuilt release binaries arrive with the v0.1 tag.

## Policy example

```yaml
version: "0.1"
defaults:
  action: allow
  ask_fallback: deny
rules:
  - id: block-ssh-credentials
    match:
      args:
        "*": "**/.ssh/**"
    action: deny
    reason: "SSH credentials are off-limits"
  - id: ask-npm-install
    match:
      surface: hook
      tool: Bash
      args:
        command: "*npm install*"
    action: ask
```

The full format is specified in [`spec/policy-v0.md`](spec/policy-v0.md) with a JSON Schema and a conformance suite.

## What works today

- `doupass policy test` — validate a policy; evaluate a single call.
- `doupass proxy` — transparent MCP stdio proxy with allow/deny/ask enforcement and fail-closed ask fallback.
- `doupass hook claude` — Claude Code `PreToolUse` adapter emitting `permissionDecision` JSON; `allow` stays silent so the harness keeps its own permission flow.
- `doupass install|uninstall claude` — idempotent `settings.json` merging with backups.
- `doupass log tail|verify` — JSONL audit log with a SHA-256 hash chain and sensitive-argument masking.

## Non-goals (v0.1)

OS-level sandboxing, malware detection, prompt-injection heuristics, and cross-call taint tracking. Other harnesses' native tools (Codex, OpenCode, Cursor) are not enforced yet. Details in [`docs/threat-model.md`](docs/threat-model.md).

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

## License

Apache-2.0. The policy specification (`spec/`) is CC0.
