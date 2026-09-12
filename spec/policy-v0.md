# doupass Policy Format — v0

Status: draft for doupass v0.1 · License: CC0 1.0 (see `LICENSE` in this directory)

## 1. Purpose

A single, small, portable policy file that describes what an AI coding agent may do. One policy governs every enforcement surface doupass supports:

- `hook` — harness-native tools (for example Claude Code's `Bash`, `Read`, `Write`, `Edit`).
- `mcp` — tool calls crossing an MCP (Model Context Protocol) server connection.

The format is intentionally deterministic: no model calls, no heuristics in the decision path. Given the same policy and the same call, the decision is always identical.

## 2. File

- Encoding: UTF-8.
- Syntax: YAML (a JSON document with the same shape is also valid).
- Conventional filename: `doupass.yml`.
- Default lookup order (tooling): `./doupass.yml`, then `~/.doupass/doupass.yml`.

Top level:

| Field | Required | Description |
|---|---|---|
| `version` | yes | Format version. For this document: `"0.1"`. |
| `name` | no | Human-readable policy name. |
| `rules` | yes | Ordered list of rules (may be empty). |
| `defaults` | yes | Fallback behavior. `defaults.action` is required. |
| `audit` | no | Audit log configuration. |

## 3. Rules

```yaml
rules:
  - id: block-ssh-credentials
    match:
      args:
        "*": "**/.ssh/**"
    action: deny
    reason: "SSH credentials are off-limits"
```

| Field | Required | Description |
|---|---|---|
| `id` | yes | Unique, stable identifier. `^[a-z0-9][a-z0-9-]{0,63}$`. |
| `match` | yes | Predicate (section 4). Must not be empty. |
| `action` | yes | `allow`, `deny`, `ask`, or `log`. |
| `reason` | no | Shown to the agent and written to the audit log. |

## 4. Matching

A rule matches a call when **every** present field in `match` matches:

| Field | Matches against | Pattern language |
|---|---|---|
| `surface` | `hook` or `mcp` | exact: `hook`, `mcp`, or `any` |
| `tool` | tool name | glob or regex |
| `server` | MCP server name | glob or regex |
| `args` | call arguments | see below |

`args` is a map from **argument name** to a **value pattern**:

- An exact argument name (for example `command`) applies the pattern to that argument's value. If the value is a list, the pattern matches when any element matches.
- The key `"*"` applies the pattern to **every** argument value; it matches when any argument value matches.
- Multiple entries are ANDed: each entry must be satisfied.
- Argument names and values are compared case-sensitively, except for path handling (below).

### 4.1 Pattern language

By default a pattern is a **glob**:

- `*` — any sequence of characters, including `/`.
- `**` — same as `*`; allowed for readability in path patterns.
- `?` — exactly one character.
- `\` — escapes the next character (use `\\` for a literal backslash).

A pattern starting with `re:` is a **regular expression** instead (Go RE2 syntax). It must match the entire string; doupass anchors it automatically.

### 4.2 Path arguments

To prevent trivial bypasses, path-like argument values are also matched in canonical form. Before matching, for each argument value doupass:

1. expands `${HOME}` and `${WORKSPACE}` variables and a leading `~`;
2. canonicalizes when the value looks like a path: starts with `/`, `~`, `./`, `../`, `.\\`, `..\\`, or a drive letter such as `C:\`;
3. makes relative paths absolute against the workspace root;
4. applies `filepath.Clean`, resolves symlinks when the path exists, converts separators to `/`;
5. case-folds the value and the path patterns on Windows.

The pattern is tested against both the raw value and the canonical form; a match on either is a match. Variables are also expanded in call argument values that contain `${...}` before matching.

Path pattern example: `**/.ssh/**` matches `/home/dev/.ssh/id_rsa`, `cat ~/.ssh/key`, and a path reached through a symlink that resolves into `/.ssh/`.

## 5. Decision semantics

1. If the policy contains at least one rule with `match.server`, and the call's `server` matches none of those rules, the decision is `defaults.unknown_server` (default `deny`). If the policy has no server-scoped rules, this step is skipped.
2. Otherwise, all matching rules contribute. The decision is the highest-precedence action among them:
   `deny` > `ask` > `allow` > `log`.
   Rule order in the file does **not** affect the action.
3. The reported rule is the first matching rule (in file order) with the winning action. Its `reason` is used.
4. If no rule matches, the decision is `defaults.action`.

`log` never changes what the agent is allowed to do; it records the call.

Version 0 has no exception mechanism (an `allow` rule cannot carve a hole out of a `deny` rule). This is intentional: denies are absolute.

## 6. Ask semantics

`ask` is a decision to request interactive confirmation from the operator:

- With a terminal available, doupass prompts on the controlling terminal (`/dev/tty` on Unix, `CONIN$`/`CONOUT$` on Windows) and waits up to `defaults.ask_timeout_seconds` (default 60).
- Without a terminal (GUI harness, daemon), or on timeout, the decision falls back to `defaults.ask_fallback` (`allow` or `deny`, default `deny`).
- On hook surfaces the prompt is delegated to the harness where the harness supports it, so the operator sees a native confirmation UI.

## 7. Audit

When `audit.path` is set, every decision is appended as one JSON object per line:

```json
{"seq":1,"time":"2026-09-12T18:00:00Z","surface":"hook","tool":"Bash",
 "args":{"command":"npm install"},"action":"ask","rule":"ask-npm-install",
 "prev_hash":"0000...","hash":"9f86..."}
```

- Argument values are truncated (256 chars) and values under sensitive keys (`password`, `token`, `secret`, anything ending in `key`) are replaced with `"***"`.
- With `audit.hash_chain: true`, each entry's `hash` is the SHA-256 of the canonical JSON of the entry (with `hash` empty) and includes the previous entry's hash, making the log tamper-evident.
- The log is local evidence only: it is tamper-evident, not tamper-proof.

## 8. Versioning

- The `version` field is required; a tool must refuse policies whose major version it does not implement.
- Unknown fields are rejected (strict parsing) to catch typos early.
- Backward-compatible additions will reuse version `0.x`; breaking changes move to `1.0`.

## 9. Complete example

See `examples/default-dev.yml`, `examples/locked-down.yml`, and `examples/red-team.yml`. Behavior fixtures live in `fixtures/conformance.yml`.
