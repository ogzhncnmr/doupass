# Security Policy

doupass is a security tool; reports are taken seriously.

## Reporting a vulnerability

Please do not open public issues for security problems. Use GitHub's private vulnerability reporting: **Security** tab → **Report a vulnerability**.

Include:

- affected version or tag,
- reproduction steps,
- expected vs actual behavior,
- impact assessment if you have one.

What to expect:

- Acknowledgment within 72 hours.
- Assessment and fix plan within 7 days.
- Credit in the release notes unless you prefer otherwise.

## Scope

In scope:

- policy bypass (pattern matching, path canonicalization, precedence),
- audit log integrity claims,
- hook or proxy enforcement gaps,
- installer corrupting existing configuration,
- release artifact integrity.

Out of scope:

- OS sandbox escapes (explicit non-goal),
- malware detection, prompt-injection detection heuristics,
- vulnerabilities in third-party MCP servers themselves.

## Design commitments

- No telemetry, no network calls, no accounts.
- The audit log is tamper-evident (SHA-256 chain), not tamper-proof; `doupass log verify` detects edits, not truncation.
- Release artifacts ship with checksums and Sigstore signatures.
