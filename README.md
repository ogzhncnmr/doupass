# doupass

A local-first policy engine for AI coding agents. One portable policy file governs tool calls across harnesses: MCP tool calls through a proxy, harness-native tools (Bash, Read, Write, Edit) through hooks.

> Status: early development (pre-v0.1). The policy format and CLI are being designed in the open.

## Why

- Agent permissions today live in per-harness settings that cannot be shared, versioned, or audited together.
- Existing agent firewalls are enterprise/Kubernetes-heavy or point-in-time scanners.
- doupass gives you one policy, a local audit trail, and a small binary that runs on your machine.

## Planned v0.1 scope

- MCP stdio proxy with deterministic allow / deny / ask decisions
- Claude Code PreToolUse hook adapter sharing the same engine
- CLI: `doupass init | proxy | hook | policy test | log tail | log verify`
- 15+ starter rules and 3 example policies
- Signed release binaries for Windows / macOS / Linux

## Build

```sh
go build ./...
go vet ./...
go test ./...
```

The policy format spec (`spec/policy-v0.md`) lands next; see `plans/` for the construction plan.

## License

Apache-2.0
