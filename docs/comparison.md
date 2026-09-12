# doupass vs other agent-security tools

Star counts verified via the GitHub API on **2026-09-12**. This is intentionally honest positioning; corrections are welcome via PR.

## Landscape

| Project | Stars | Primary focus |
|---|---|---|
| [agentgateway/agentgateway](https://github.com/agentgateway/agentgateway) | 4,814 | Agentic proxy for MCP/A2A (Kubernetes-era infrastructure) |
| [IBM/mcp-context-forge](https://github.com/IBM/mcp-context-forge) | 4,464 | AI gateway/registry with centralized guardrails and management |
| [nolabs-ai/nono](https://github.com/nolabs-ai/nono) | 4,054 | Secure multiplexed execution paths for agents |
| [stacklok/toolhive](https://github.com/stacklok/toolhive) | 2,162 | Enterprise platform for running MCP servers |
| [luckyPipewrench/pipelock](https://github.com/luckyPipewrench/pipelock) | 841 | Agent firewall: HTTP/MCP/A2A scanning, egress, signed receipts |
| [afshinm/zerobox](https://github.com/afshinm/zerobox) | 716 | Cross-platform process sandboxing (Codex runtime) |
| [emiliaprotocol/emilia-protocol](https://github.com/emiliaprotocol/emilia-protocol) | 649 | Authority control plane with verifiable mandates |
| [hashgraph-online/hol-guard](https://github.com/hashgraph-online/hol-guard) | 599 | Runtime "antivirus" for agent tools, skills, and plugins |
| [secureagentics/Adrian](https://github.com/secureagentics/Adrian) | 560 | Runtime monitoring and policy-drift detection |
| [TheLunarCompany/lunar](https://github.com/TheLunarCompany/lunar) | 493 | Agent-native MCP gateway for governance |
| [Agent-Threat-Rule/agent-threat-rules](https://github.com/Agent-Threat-Rule/agent-threat-rules) | 389 | Detection-rule standard (Sigma for agents) |
| [getagentseal/agentseal](https://github.com/getagentseal/agentseal) | 371 | Point-in-time scanning of skills/MCP configs |
| [MCP-Defender/MCP-Defender](https://github.com/MCP-Defender/MCP-Defender) | 257 | Desktop MCP traffic scanner/blocker |
| [node9-ai/node9-proxy](https://github.com/node9-ai/node9-proxy) | 211 | Deterministic "sudo" governance for agents |
| Harness-native permissions (Claude Code allow/deny + sandbox, Codex approval modes) | — | Built-in per-harness permission systems |

Most of these are excellent at what they do. doupass deliberately occupies a narrow gap.

## What doupass does differently

1. **One portable policy across surfaces.** Harness-native permissions cannot be shared or versioned across tools. doupass evaluates the same YAML policy for MCP tool calls (proxy) and Claude Code native tools (hook), with identical semantics.
2. **An open, testable format.** `spec/policy-v0.md` ships with a JSON Schema and a conformance suite; rules from `rules/starter.yml` are verified by positive and negative fixtures in CI.
3. **Local-first, zero infrastructure.** One static binary, no account, no server, no telemetry. Works on a laptop; no Kubernetes anywhere in the happy path.
4. **Deterministic and auditable.** No model calls in the decision path; every decision lands in a hash-chained JSONL log with sensitive-argument masking.

## Where doupass is weaker (use the others)

| Need | Better fit |
|---|---|
| Enterprise SSO/RBAC, fleet management, K8s deployment | mcp-context-forge, toolhive, lunar |
| Network egress scanning, DLP, signed mediator receipts | pipelock |
| Strong OS-level process sandboxing | zerobox, nono |
| Heuristic threat detection (prompt injection, tool poisoning) | hol-guard, Adrian, agent-threat-rules |
| One-time environment audit before trusting it | agentseal, MCP-Defender |

## Which should you use?

- You want one policy file that travels with your repo and covers both MCP and Claude Code native tools → doupass.
- You run agents for a whole company and need centralized control planes → the enterprise gateways above.
- You need defense in depth → combine: zerobox/pipelock for isolation and egress, doupass for deterministic per-call policy, agent-threat-rules for detection content.
