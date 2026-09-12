# FAQ

**Does doupass send any data anywhere?**
No. doupass is a local binary with no network calls, no telemetry, and no account. The only network traffic is what your MCP servers and agent already make.

**Does it slow my agent down?**
No measurable impact. Each tool call is matched against compiled regexes (microseconds); the proxy adds a line-parse per JSON-RPC message. The 10 MB message cap and per-request timeouts protect the proxy from pathological servers.

**Claude Code already has permission rules and a sandbox. Why add doupass?**
Claude Code's rules only apply to Claude Code, are not versioned with your project, and do not cover MCP `tools/call`. doupass gives you one reviewable policy file that also covers MCP calls, plus a portable audit trail. It complements rather than replaces harness permissions: `allow` decisions stay silent and let Claude Code's own flow decide.

**Can't prompt injection just tell the agent to ignore doupass?**
The agent never evaluates the policy. Injection can try to call a tool, but the decision is made deterministically at the tool boundary by doupass, outside the model. Note the limits: doupass denies per policy; it does not detect that an injection happened, and v0.1 has no cross-call taint analysis (see `docs/threat-model.md`).

**What happens when a rule is too strict?**
Dry-run with `doupass policy test <policy> --tool <name> --arg k=v` and adjust. In v0, `deny` is absolute: no rule can carve an exception out of a deny. Rule priorities/exceptions are planned for v0.2. Open an issue if a starter rule is wrong; fixtures make fixes easy.

**Why did `deny` on `**/.ssh/**` block a shell command, not just file reads?**
`*` in patterns matches any character including `/`, so command strings match too; path-like values are additionally canonicalized (including symlink resolution). That is intentional: `cat ~/.ssh/id_rsa` is exactly the bypass we care about.

**Does the audit log contain my secrets?**
Argument values are truncated to 256 characters and values under keys like `password`, `token`, `secret`, or anything ending in `key` are replaced with `***`. Treat the log as sensitive anyway; it is written `0600`.

**Is the audit log tamper-proof?**
Tamper-evident, not tamper-proof. Each entry chains a SHA-256 hash of the previous entry, so edits are detectable with `doupass log verify`; deleting the tail is not detectable in v0.1.

**Which platforms are supported?**
Windows, macOS, and Linux. CI builds and tests on Ubuntu and Windows; interactive `ask` prompts work on a terminal (fallback `deny` when there is no TTY).

**Which harnesses are supported?**
MCP proxying works with any MCP client (Claude Code, OpenCode, Codex, Cursor — see `docs/wiring.md`). Native-tool hooks exist for Claude Code today; Codex/OpenCode/Cursor hooks are on the v0.2 roadmap.

**How is this different from a "gateway"?**
Gateways are servers you deploy for a fleet; doupass is a binary you run locally. If you need SSO, RBAC, and central management, use a gateway (see `docs/comparison.md`).
