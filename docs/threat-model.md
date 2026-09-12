# Threat Model (draft)

Status: draft for v0.1. This document is intentionally explicit about what doupass does and does not defend against.

## What doupass is

A local policy engine that sits between an AI coding agent and the tools it invokes. It evaluates deterministic rules against tool calls, returns allow / deny / ask decisions, and records an audit trail.

## Trust boundaries

- The policy file is trusted input; the agent and its prompts are untrusted.
- MCP servers are semi-trusted; a compromised or malicious server must not bypass policy.
- The audit log is local evidence: tamper-evident (hash chain), not tamper-proof.

## Threats addressed (v0.1)

| Threat | Surface | Mitigation |
|---|---|---|
| Prompt injection leading to credential reads (~/.ssh, ~/.aws, .env) | MCP + hook | deny rules with canonicalized path matching |
| Destructive commands (rm -rf, force push, DROP) | hook | deny / ask starter rules |
| Exfiltration via tool calls (curl -d @file, base64 piping) | hook | ask / deny starter rules |
| Unknown or malicious MCP server | MCP proxy | default `unknown_server: deny` |
| TTY-less environment silently allowing | prompt channel | fail-closed `ask_fallback: deny` with timeout |

## Non-goals (v0.1)

- OS-level sandboxing or preventing sandbox escape.
- Malware detection, model alignment, or prompt-injection detection heuristics.
- Cross-call data-flow tracking (taint): an allowed read can still be exfiltrated by an allowed write; starter egress rules only partially mitigate this.
- Multi-user or multi-machine policy management (v0.2+).

## Proxy-specific risks

- Downstream server hangs: enforced timeouts.
- Oversized payloads: message size cap (default 10 MB), denial on exceed.
- Malformed messages: rejected without crashing the proxy.
