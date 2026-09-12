# Wiring doupass into harnesses

doupass enforces policy on two surfaces:

- **MCP proxy** — wraps an MCP server command; every `tools/call` is evaluated before it reaches the server.
- **Hooks** — the harness asks doupass before running a native tool (Bash, Read, Write, Edit).

## Claude Code

Automatic:

```sh
doupass install claude                          # writes ~/.claude/settings.json, with backup
doupass install claude --settings ./settings.json
doupass uninstall claude
```

The installer adds a `PreToolUse` command hook that runs `doupass hook claude`. On `deny` and `ask` the hook returns Claude Code's `permissionDecision` JSON; on `allow` it stays silent so your existing permission flow is unchanged.

Manual (equivalent):

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          { "type": "command", "command": "doupass hook claude" }
        ]
      }
    ]
  }
}
```

MCP servers in Claude Code can also be routed through the proxy:

```sh
claude mcp add filesystem -- doupass proxy --server filesystem -- npx -y @modelcontextprotocol/server-filesystem .
```

## OpenCode

Automatic:

```sh
doupass install opencode                        # finds ./opencode.json[c] or ~/.config/opencode/opencode.json[c]
doupass install opencode --config ./opencode.jsonc
doupass uninstall opencode
```

For every `mcp` entry with `"type": "local"`, the command is rewritten to run through the proxy. Comments, other settings, and `environment` blocks are preserved; a `.doupass.bak` backup is written.

Before:

```json
{
  "mcp": {
    "fs": {
      "type": "local",
      "command": ["npx", "-y", "@modelcontextprotocol/server-filesystem", "."]
    }
  }
}
```

After `doupass install opencode`:

```json
{
  "mcp": {
    "fs": {
      "type": "local",
      "command": ["doupass", "proxy", "--server", "fs", "--", "npx", "-y", "@modelcontextprotocol/server-filesystem", "."]
    }
  }
}
```

## Codex CLI

Codex MCP servers are configured in `~/.codex/config.toml`. Wrap the command manually:

```toml
[mcp_servers.filesystem]
command = "doupass"
args = ["proxy", "--server", "filesystem", "--", "npx", "-y", "@modelcontextprotocol/server-filesystem", "."]
```

Codex native tool approval (sandbox/approval modes) keeps working alongside; hook-based enforcement for Codex's native tools is on the v0.2 roadmap.

## Cursor

Cursor MCP servers are configured in `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "doupass",
      "args": ["proxy", "--server", "filesystem", "--", "npx", "-y", "@modelcontextprotocol/server-filesystem", "."]
    }
  }
}
```

Cursor native tool enforcement is on the v0.2 roadmap.

## Choosing a policy

`doupass proxy` and `doupass hook` look for the policy in this order:

1. `--policy <file>` flag
2. `./doupass.yml`
3. `~/.doupass/doupass.yml`

Start from `rules/starter.yml` (18 rules) or run `doupass init`.
