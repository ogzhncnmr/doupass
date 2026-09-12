# doupass

A local-first policy engine for AI coding agents. One portable policy file governs tool calls across harnesses: MCP tool calls through a proxy, harness-native tools (Bash, Read, Write, Edit) through hooks.

Status: v0.1-rc. The core engine, MCP proxy, Claude Code hook, audit log, CLI, and docs are working and tested; release engineering is in progress. See `plans/` for the construction plan.

## Why

- Agent permissions today live in per-harness settings that cannot be shared, versioned, or audited together.
- Existing agent firewalls are enterprise/Kubernetes-heavy or point-in-time scanners.
- doupass gives you one policy, a local audit trail, and a small binary that runs on your machine.

## Quickstart

```sh
go install github.com/ogzhncnmr/doupass/cmd/doupass@latest   # Go 1.27+

doupass init                                                  # starter preset (18 rules)
doupass init --preset locked-down                             # or: minimal, red-team
doupass policy lint doupass.yml                               # catch blanket rules and missing reasons
doupass policy test doupass.yml --tool Read --arg file_path=~/.ssh/id_rsa
doupass setup --dry-run                                       # see which installed tools would be wired
doupass setup                                                 # wire Claude Code, opencode, Codex, Cursor, Windsurf, Kiro, Cline, Roo Code
doupass install claude                                        # registers the PreToolUse hook
doupass proxy --server fs -- npx -y @modelcontextprotocol/server-filesystem .
doupass log tail
doupass log verify                                            # hash-chain tamper check
```

Run `doupass` with no arguments in a terminal for an interactive menu of the most useful actions (status, setup, policy, audit); piped or non-interactive runs get a plain-text guide instead.

Prebuilt binaries (Linux/macOS/Windows, amd64+arm64) are on the [releases page](https://github.com/ogzhncnmr/doupass/releases); checksums are Sigstore-signed, see `packaging/README.md` for verification commands.

## Playground

Try the format without installing anything — decisions run locally in WebAssembly, nothing leaves the page: **https://ogzhncnmr.github.io/doupass/**

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

- `doupass setup` — detect installed agent tools and wire them in one command (dry-run supported; writes the starter policy first if none exists).
- `doupass install generic --config <file>` — wrap MCP servers in any `mcpServers`-style JSON config.
- `doupass install codex` — wrap local MCP servers in the Codex CLI `config.toml`.
- `doupass policy test` — validate a policy; evaluate a single call.
- `doupass policy lint` — detect duplicate rules, blanket patterns, and missing reasons.
- `doupass init --preset` — starter, minimal, locked-down, and red-team policies.
- `doupass proxy` — transparent MCP stdio proxy with allow/deny/ask enforcement and fail-closed ask fallback.
- `doupass hook claude` — Claude Code `PreToolUse` adapter emitting `permissionDecision` JSON; `allow` stays silent so the harness keeps its own permission flow, and any doupass failure (unreadable input, missing policy) denies out loud instead of failing open.
- `doupass decide` — generic JSON-in / decision-out endpoint for custom integrations.
- `doupass install opencode --plugin` — native-tool enforcement for opencode via its plugin API.
- `doupass install|uninstall claude` — idempotent `settings.json` merging with backups.
- `doupass log tail|verify` — JSONL audit log with a SHA-256 hash chain and sensitive-argument masking.

## Docs

- [Quickstart](docs/quickstart.md)
- [Wiring harnesses](docs/wiring.md)
- [Comparison with other agent-security tools](docs/comparison.md)
- [FAQ](docs/faq.md)
- [Threat model](docs/threat-model.md)
- [Policy format specification](spec/policy-v0.md)

## Performance

The decision path is allocation-free, so it can sit on every tool call:

```
BenchmarkDecide-12    1301170    1785 ns/op    0 B/op    0 allocs/op
```

Run it yourself: `go test ./internal/policy -bench=BenchmarkDecide -benchtime=2s -run=^$`

## Non-goals (v0.1)

OS-level sandboxing, malware detection, prompt-injection heuristics, and cross-call taint tracking. Native tools of other harnesses (Codex, Cursor) are not intercepted yet — Codex, Cursor & co. are covered on the MCP surface, while Claude Code and opencode also enforce their native tools. Remote (URL-based) MCP servers are out of scope. Details in [`docs/threat-model.md`](docs/threat-model.md).

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

## License

Apache-2.0. The policy specification (`spec/`) is CC0.
