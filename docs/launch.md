# Launch kit (S9)

Everything below is copy-paste ready. Posting requires the maintainer's own accounts — that is the only part that cannot be automated.

Release verified: `v0.1.0-rc4` (prerelease) with Sigstore-signed checksums, binaries for Linux/macOS (amd64+arm64) and Windows.

## Demo recording script (60 seconds)

```sh
# 1. Show the problem: a policy file
cat doupass.yml

# 2. Dry-run three decisions
doupass policy test doupass.yml --tool Read --arg file_path=~/.ssh/id_rsa    # deny
doupass policy test doupass.yml --tool Bash --arg "command=npm install x"    # ask
doupass policy test doupass.yml --tool Read --arg file_path=/etc/hosts       # allow

# 3. Wire it into Claude Code
doupass install claude

# 4. Prove it blocks the real thing (in Claude Code): "read ~/.ssh/id_rsa" -> denied

# 5. Show the evidence
doupass log tail
doupass log verify
```

## Show HN

**Title:** Show HN: Doupass – one policy file to govern AI coding agents (MCP + hooks)

**Body:**

> I keep giving coding agents the ability to run shell commands, read files, and call MCP servers, but the permissions live in per-tool settings that can't be shared, versioned, or audited. So I built a small local policy engine:
>
> - One YAML policy, evaluated deterministically (no LLM calls in the decision path).
> - Two enforcement points: an MCP stdio proxy and a Claude Code PreToolUse hook, same engine, same audit trail.
> - allow / deny / ask decisions; `ask` falls back to deny when there's no terminal.
> - Hash-chained local audit log with secret masking (`doupass log verify`).
> - Single Go binary, Apache-2.0, no account, no telemetry.
>
> The policy format has a spec, a JSON Schema, and a conformance suite (33 spec cases + 47 starter-rule cases in CI). It's v0.1 — native-tool hooks only cover Claude Code today, and it does not try to be an enterprise gateway or a sandbox.
>
> Honest limitations in the FAQ; I'd love feedback on the rule format especially.
>
> Repo: https://github.com/ogzhncnmr/doupass

**First comment (post immediately):**

> Some design notes that HN usually asks about:
> - Why deny beats allow regardless of rule order: order-independent precedence is one less footgun for security rules.
> - Path matching canonicalizes (`~`, `${WORKSPACE}`, symlinks, Windows case) and matches both raw and canonical values, so `cat ~/.ssh/id_rsa` is caught by the same rule that blocks reads.
> - `ask` on hook surfaces is delegated to Claude Code's native prompt; only the proxy prompts on a TTY.
> - Non-goals: egress scanning (pipelock), OS sandboxing (zerobox/nono), detection content (agent-threat-rules). I compared honestly in docs/comparison.md.

## Reddit

### r/ClaudeAI

**Title:** I built a local policy layer for Claude Code (PreToolUse hook): block ~/.ssh reads, ask before npm install

**Body:** Short version of the Show HN body, one code block of the policy YAML, one code block of install + the deny working, link to docs/quickstart.md. Ask: "which starter rules would you want?"

### r/mcp

**Title:** A transparent stdio MCP proxy that enforces a YAML policy per `tools/call` (open source, local-first)

**Body:** Focus on `doupass proxy --server fs -- npx ...`; explain deny response shape (JSON-RPC error -32001), unknown-server default deny, and that it works with any MCP client. Ask for feedback on tool-argument matching.

### r/LocalLLaMA

**Title:** Local-first guardrails for coding agents: deterministic policy engine, no cloud, no LLM in the decision path

**Body:** Emphasize: single binary, deterministic regex/glob engine, hash-chained audit log, runs fully offline. Mention Windows/macOS/Linux binaries on the release page.

### r/cybersecurity (or r/netsec) — weekend-friendly angle

**Title:** An open policy format for agent tool calls (spec + conformance suite), and a local reference implementation

**Body:** Lead with the format and the "Sigma-style" angle; ask for adversarial review of the threat model and bypasses. Include the symlink/canonicalization notes.

## Awesome lists / directories

- `punkpeye/awesome-mcp-servers` — likely out of scope (doupass is a client-side guard, not a server). Skip unless a security section exists.
- Any `awesome-mcp-security` / `awesome-ai-agent-security` list — add under "Firewalls / Policy engines" with: `doupass – local-first policy engine for agent tool calls (MCP proxy + Claude Code hook), portable policy format, audited decisions.`
- MCP Registry (`registry.modelcontextprotocol.io`) is for servers; doupass wraps servers, so it does not belong there. Mention as "works with any MCP server" instead.

## dev.to / blog article (EN)

Title: "Your coding agent doesn't need omnipotence: a 15-minute local policy layer"

Outline:
1. The permission sprawl problem (per-harness settings, no audit).
2. The format: a 10-line YAML rule and its semantics (glob, precedence).
3. Wiring it: `install claude` + MCP proxy in one command.
4. The proof: deny `~/.ssh`, ask `npm install`, verify the log.
5. Limits and non-goals; when to use a sandbox or gateway instead.
6. CTA: repo + starter rules + feedback ask.

## Turkish blog/community post

Başlık: "Yapay zeka kod ajanlarına yerel politika katmanı: doupass"

Çerçeve: ajanlara sınırsız yetki verme sorunu, tek YAML ile deny/ask/allow, KVKK/gizlilik açısından verinin makineden çıkmaması, Türkçe kural yazma örnekleri, katkı çağrısı.

## Launch-day checklist

- [ ] Record the 60-second demo (asciinema or GIF).
- [ ] Post Show HN (weekday morning US Eastern works best).
- [ ] Cross-post Reddit within 24h; reply to every early comment.
- [ ] Watch issues; label `good first issue` for rule requests.
- [ ] Track for 7 days: stars, release downloads, issues, `doupass log verify` bug reports.

## Metrics baseline (from GitHub API)

- Stars: 0 at launch (new repo).
- Release downloads: `gh api repos/ogzhncnmr/doupass/releases` and sum `download_count`.
- CI: build/test on Ubuntu + Windows for every push; release workflow on tags.
