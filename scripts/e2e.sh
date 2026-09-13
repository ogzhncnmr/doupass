#!/usr/bin/env bash
# End-to-end smoke test: builds doupass, then exercises policy decisions, the
# Claude hook, a real MCP server behind the proxy, setup wiring, the audit log,
# and doctor — all inside a sandboxed HOME so the developer's own configs and
# audit log are never touched.
set -u

cd "$(dirname "$0")/.."
DOUPASS=${DOUPASS:-./doupass.exe}
GO=${GO:-go}
PASS=0
FAIL=0

ok()   { PASS=$((PASS+1)); echo "  ok  - $1"; }
bad()  { FAIL=$((FAIL+1)); echo "  FAIL - $1"; }
assert_contains() { # desc haystack needle
  case "$2" in *"$3"*) ok "$1" ;; *) bad "$1 (missing: $3)" ;; esac
}
assert_not_contains() {
  case "$2" in *"$3"*) bad "$1 (unexpected: $3)" ;; *) ok "$1" ;; esac
}

echo "== build =="
$GO build -o "$DOUPASS" ./cmd/doupass || { echo "build failed"; exit 1; }
case "$DOUPASS" in /*) ;; *) DOUPASS="$(pwd)/${DOUPASS#./}" ;; esac
ok "go build"

SB=$(mktemp -d)
trap 'rm -rf "$SB"' EXIT
export HOME="$SB"
export USERPROFILE="$SB"
APPDATA="$SB/AppData"
export AppData="$APPDATA"
export XDG_CONFIG_HOME="$SB/xdg"
mkdir -p "$APPDATA/Code/User/globalStorage/saoudrizwan.claude-dev/settings" "$SB/work"
WORK=$(cd "$SB/work" && pwd)
# Run everything from the sandbox HOME so a doupass.yml in the developer's
# checkout can never stand in for the policy the test is building.
cd "$SB"

echo "== sandbox fixtures =="
printf '{}' > "$SB/.claude.settings" 2>/dev/null || true
mkdir -p "$SB/.claude" "$SB/.cursor" "$SB/.config/opencode" "$SB/.codex"
printf '{}' > "$SB/.claude/settings.json"
printf '{"mcpServers":{"fs":{"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","."]}}}' > "$SB/.cursor/mcp.json"
printf '// cursor-style config\n{"mcp": {"fs": {"type": "local", "command": ["npx", "-y", "server-fs", "."]}}}\n' > "$SB/.config/opencode/opencode.jsonc"
printf '[mcp_servers.fs]\ncommand = "npx"\nargs = ["-y", "server-fs", "."]\n' > "$SB/.codex/config.toml"
printf '{"mcpServers":{"fs":{"command":"npx","args":["-y","server-fs","."]}}}' > "$APPDATA/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json"
ok "fake agent configs created"

echo "== 1. init: ready-made policy, zero YAML writing =="
OUT=$("$DOUPASS" init --dir "$WORK" 2>&1)
assert_contains "init writes starter policy" "$OUT" "wrote"
POLICY="$WORK/doupass.yml"
OUT=$("$DOUPASS" init --dir "$WORK" 2>&1 || true)
assert_contains "second init refuses to overwrite" "$OUT" "already exists"
OUT=$("$DOUPASS" init --preset starter --force --dir "$WORK" 2>&1)
assert_contains "policy updates in place with --force" "$OUT" "wrote"

echo "== 2. policy engine: real Windows-path protections =="
decide() { "$DOUPASS" decide --policy "$POLICY" --input "$SB/call.json" 2>&1; }
mkcall() { printf '%s' "$1" > "$SB/call.json"; }

mkcall '{"surface":"hook","tool":"Read","args":{"file_path":"C:\\Users\\dev\\.ssh\\id_rsa"}}'
OUT=$(decide)
assert_contains "windows ssh path denied" "$OUT" '"action":"deny"'
assert_contains "windows ssh rule" "$OUT" 'block-ssh-credentials'

mkcall '{"surface":"hook","tool":"Read","args":{"file_path":".env"}}'
OUT=$(decide)
assert_contains "bare .env denied (root anchoring)" "$OUT" '"action":"deny"'

mkcall '{"surface":"hook","tool":"Read","args":{"file_path":".kube/config"}}'
OUT=$(decide)
assert_contains "relative .kube/config denied" "$OUT" '"action":"deny"'

mkcall '{"surface":"hook","tool":"Read","args":{"file_path":"C:\\work\\src\\main.go"}}'
OUT=$(decide)
assert_contains "workspace file allowed" "$OUT" '"action":"allow"'

mkcall '{"surface":"hook","tool":"Bash","args":{"command":"npm run build"}}'
OUT=$(decide)
assert_contains "npm run allowed" "$OUT" '"action":"allow"'

echo "== 3. claude hook: PreToolUse adapter =="
HOOKIN='{"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"C:\\Users\\dev\\.ssh\\id_rsa"}}'
OUT=$(printf '%s' "$HOOKIN" | "$DOUPASS" hook claude --policy "$POLICY" 2>&1)
assert_contains "hook denies ssh read" "$OUT" '"permissionDecision":"deny"'

HOOKIN='{"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"C:\\work\\main.go"}}'
OUT=$(printf '%s' "$HOOKIN" | "$DOUPASS" hook claude --policy "$POLICY" 2>&1)
assert_not_contains "hook stays silent on allow" "$OUT" "permissionDecision"

echo "== 4. proxy: real node MCP server behind the engine =="
cat > "$SB/mini-mcp.js" <<'JS'
let buf = "";
process.stdin.on("data", (d) => {
  buf += d;
  let i;
  while ((i = buf.indexOf("\n")) >= 0) {
    const line = buf.slice(0, i).trim();
    buf = buf.slice(i + 1);
    if (!line) continue;
    let msg;
    try { msg = JSON.parse(line); } catch { continue; }
    if (msg.method === "initialize") {
      send({ jsonrpc: "2.0", id: msg.id, result: { protocolVersion: "2024-11-05", capabilities: {}, serverInfo: { name: "mini-fs", version: "1.0.0" } } });
    } else if (msg.method === "tools/call") {
      send({ jsonrpc: "2.0", id: msg.id, result: { content: [{ type: "text", text: "ran " + msg.params.name }] } });
    } else if (msg.id !== undefined) {
      send({ jsonrpc: "2.0", id: msg.id, result: {} });
    }
  }
});
function send(m) { process.stdout.write(JSON.stringify(m) + "\n"); }
JS

cat > "$SB/proxy-in.jsonl" <<'JSONL'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"C:\\Users\\dev\\.ssh\\id_rsa"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"C:\\work\\main.go"}}}
JSONL

OUT=$(cat "$SB/proxy-in.jsonl" | "$DOUPASS" proxy --server fs --policy "$POLICY" -- node "$SB/mini-mcp.js" 2>"$SB/proxy.err" || true)
assert_contains "initialize forwarded to real server" "$OUT" "mini-fs"
assert_contains "allowed call got real result" "$OUT" "ran read_file"
assert_contains "denied call returned policy error" "$OUT" '-32001'
assert_not_contains "denied call never reached server" "$OUT" '"id":2,"result"'

echo "== 5. setup: one command wires every installed tool =="
OUT=$("$DOUPASS" setup 2>&1)
assert_contains "auto-created starter policy" "$OUT" "starter policy"
assert_contains "claude hook installed" "$OUT" "hook installed"
assert_contains "cursor wrapped" "$OUT" "wrapped 1 MCP server"
assert_contains "opencode wrapped" "$OUT" "opencode"
assert_contains "codex wrapped" "$OUT" "codex"
assert_contains "cline wrapped" "$OUT" "cline"

grep -q "doupass" "$SB/.cursor/mcp.json" && ok "cursor config wrapped on disk" || bad "cursor config not wrapped on disk"
grep -q "doupass" "$SB/.config/opencode/opencode.jsonc" && ok "opencode config wrapped on disk (comments intact)" || bad "opencode not wrapped"
grep -q "cursor-style config" "$SB/.config/opencode/opencode.jsonc" && ok "opencode comment preserved" || bad "opencode comment lost"
grep -q "doupass" "$SB/.codex/config.toml" && ok "codex config wrapped on disk" || bad "codex not wrapped"
grep -q "doupass" "$APPDATA/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json" && ok "cline config wrapped on disk" || bad "cline not wrapped"
grep -q "doupass hook" "$SB/.claude/settings.json" && ok "claude hook registered on disk" || bad "claude hook missing"

echo "== 6. setup is idempotent =="
OUT=$("$DOUPASS" setup 2>&1)
assert_contains "second setup: hook already installed" "$OUT" "hook already installed"
assert_contains "second setup: codex nothing to wrap" "$OUT" "no local MCP servers"

echo "== 7. audit log: recorded and tamper-evident =="
OUT=$("$DOUPASS" log tail --n 5 2>&1)
assert_contains "log tail shows decisions" "$OUT" "deny"
"$DOUPASS" log verify >/dev/null 2>&1 && ok "hash chain verifies" || bad "hash chain broken before tamper"
printf '\n{"fake":true}\n' >> "$SB/.doupass/audit.jsonl"
"$DOUPASS" log verify >/dev/null 2>&1 && bad "tampering went undetected" || ok "tampering detected"

echo "== 8. uninstall restores original configs =="
"$DOUPASS" uninstall claude --settings "$SB/.claude/settings.json" >/dev/null 2>&1 && \
  ! grep -q "doupass" "$SB/.claude/settings.json" && ok "claude hook removed" || bad "claude uninstall failed"
"$DOUPASS" uninstall codex --config "$SB/.codex/config.toml" >/dev/null 2>&1 && \
  ! grep -q "doupass" "$SB/.codex/config.toml" && ok "codex unwrapped" || bad "codex uninstall failed"

echo
echo "== doctor (fresh HOME, nothing wired) =="
OUT=$("$DOUPASS" doctor 2>&1 || true)
assert_contains "doctor reports policy" "$OUT" "policy"

echo
echo "================================"
echo "PASS: $PASS  FAIL: $FAIL"
[ "$FAIL" -eq 0 ] || exit 1
