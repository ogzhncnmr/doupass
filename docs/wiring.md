# Wiring doupass into harnesses

doupass enforces policy on two surfaces:

- **MCP proxy** — wraps an MCP server command; every `tools/call` is evaluated before it reaches the server.
- **Hooks** — the harness asks doupass before running a native tool (Bash, Read, Write, Edit).

The fastest way to wire everything that is installed on your machine:

```sh
doupass setup --dry-run    # show what would change
doupass setup              # apply; backups are written next to every config file
```

`setup` detects Claude Code, opencode, Codex CLI, Cursor, Windsurf, Kiro, Cline, and Roo Code, and wraps their MCP servers. Native-tool hooks are available for Claude Code and opencode today; everything else gets MCP proxying. If no policy exists yet, `setup` first writes the starter policy to `~/.doupass/doupass.yml`, so one command leaves the machine protected.

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

Automatic (MCP proxy):

```sh
doupass install opencode                        # finds ./opencode.json[c] or ~/.config/opencode/opencode.json[c]
doupass install opencode --config ./opencode.jsonc
doupass uninstall opencode
```

Automatic (native tools plugin):

```sh
doupass install opencode --plugin               # writes .opencode/plugin/doupass.js
doupass uninstall opencode --plugin
```

The plugin hooks `tool.execute.before` and asks doupass before every native tool call. Restart opencode after installing. It normalizes tool names (`bash` → `Bash`, `read` → `Read`, `filePath` → `file_path`) so the same policy works across Claude Code and opencode. A `deny` blocks the call with the rule reason; `ask` is fail-closed (blocked with an explanation) because plugins cannot prompt interactively; if doupass itself fails, the plugin fails closed too.

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

Automatic:

```sh
doupass install codex                      # edits ~/.codex/config.toml, with backup
doupass install codex --config ./config.toml
doupass uninstall codex
```

The installer rewrites each local `[mcp_servers.<name>]` entry so its command runs through the proxy:

```toml
[mcp_servers.filesystem]
command = "doupass"
args = ["proxy", "--server", "filesystem", "--", "npx", "-y", "@modelcontextprotocol/server-filesystem", "."]
```

Remote (url-based) servers are left untouched — they never run a local command doupass could wrap. Codex does not expose an external pre-tool command hook, so doupass enforces MCP traffic only; native-tool approval stays with Codex's own sandbox and approval modes. If a future Codex version adds hooks, an adapter can reuse `doupass decide`.

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

Cursor does not expose an external pre-tool command hook, so doupass enforces MCP traffic only.

## Cursor, Windsurf, Kiro, Cline, Roo Code (mcpServers-style clients)

`doupass setup` handles the known locations automatically. For anything else — or a custom path — use the generic installer, which works on any config with an `mcpServers` object:

```sh
doupass install generic --config "~/.cursor/mcp.json"
doupass install generic --config "%APPDATA%/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json"
doupass uninstall generic --config "~/.cursor/mcp.json"
```

Locations `setup` checks:

| Tool | Path |
|---|---|
| Codex CLI | `~/.codex/config.toml` |
| Cursor | `~/.cursor/mcp.json` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` |
| Kiro | `~/.kiro/settings/mcp.json` |
| Cline (VS Code/Cursor/Windsurf) | `<config>/<App>/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` |
| Roo Code (VS Code/Cursor/Windsurf) | `<config>/<App>/User/globalStorage/rooveterinaryinc.roo-cline/settings/mcp_settings.json` |

Comments are preserved, `env` blocks are untouched, and a `.doupass.bak` backup is written before every change. For Cline/Roo/Cursor/Kiro, MCP traffic is enforced; their native file/shell tools are not intercepted yet — watch [issue #3](https://github.com/ogzhncnmr/doupass/issues/3).

## Custom integrations

Any harness or tool that can run a command can integrate through `doupass decide`: it reads one call as JSON on stdin and prints the decision as JSON, writing the audit entry on the way.

```sh
echo '{"surface":"hook","tool":"Bash","args":{"command":"npm install x"}}' | doupass decide --policy doupass.yml
# {"action":"ask","rule":"ask-package-install","reason":"Package installs run lifecycle scripts"}
```

## Choosing a policy

`doupass proxy` and `doupass hook` look for the policy in this order:

1. `--policy <file>` flag
2. `./doupass.yml`
3. `~/.doupass/doupass.yml`

Start from `rules/starter.yml` (18 rules) or run `doupass init`.
