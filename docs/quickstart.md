# Quickstart (5 minutes)

## 1. Install

```sh
go install github.com/ogzhncnmr/doupass/cmd/doupass@latest   # Go 1.27+
doupass version
```

## 2. Create a policy

```sh
doupass init            # writes ./doupass.yml (starter, 4 rules)
```

Or copy the full starter set with 18 rules:

```sh
cp rules/starter.yml doupass.yml
```

## 3. Dry-run decisions

```sh
doupass policy test doupass.yml
doupass policy test doupass.yml --tool Read --arg file_path=~/.ssh/id_rsa
doupass policy test doupass.yml --surface hook --tool Bash --arg "command=npm install x"
```

## 4. Guard Claude Code's native tools

```sh
doupass install claude     # adds a PreToolUse hook, backs up settings.json
```

Claude Code now asks doupass before every matching tool call. `deny` and `ask` decisions use Claude Code's native UI; `allow` changes nothing (your existing permission flow still applies).

Try it: ask the agent to read `~/.ssh/id_rsa`. Expected: denied with "SSH credentials are off-limits".

Remove with `doupass uninstall claude`.

## 5. Guard an MCP server

```sh
doupass proxy --server fs -- npx -y @modelcontextprotocol/server-filesystem .
```

Point your harness at this command instead of the raw server (see `docs/wiring.md` for OpenCode automation and Codex/Cursor snippets). Every `tools/call` is evaluated before it reaches the server.

## 6. Review the audit trail

```sh
doupass log tail
doupass log verify     # recomputes the SHA-256 hash chain
```

## Troubleshooting

- `no policy file found` — you are not in the directory with `doupass.yml` and there is no `~/.doupass/doupass.yml`; pass `--policy`.
- Decisions from different directories differ — `${WORKSPACE}` is the current directory when the engine loads; run from your project root or use absolute patterns.
- An `ask` never prompts — there is no terminal (GUI harness). The decision falls back to `ask_fallback` (default `deny`); on Claude Code, `ask` is delegated to the harness UI instead.
- Something is denied that should not be — dry-run with `doupass policy test` and adjust the rule. Denies are absolute in v0; v0.2 adds rule priorities/exceptions.
