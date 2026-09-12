# AGENTS.md

## Project

doupass — a local-first policy engine for AI coding agents. See `README.md`.

## Layout

- `cmd/doupass/` — CLI entrypoint
- `internal/` — engine, proxy, hook, audit packages (added per plan)
- `spec/` — policy format specification (policy-v0)
- `plans/` — construction plan (Turkish; planning artifact only)
- `docs/` — user-facing docs (English)

## Commands

- Build: `go build ./...`
- Vet: `go vet ./...`
- Test: `go test ./...`

On Windows, if `go` is not on PATH in your shell, use `& "C:\Program Files\Go\bin\go.exe"` as a drop-in replacement.

## Conventions

- Code, comments, commits, issues, and docs are English. Only `plans/` may be Turkish.
- No comments unless they explain a non-obvious decision.
- Deterministic behavior first: no LLM calls in the core policy engine.
- Keep `internal/policy` free of MCP or harness dependencies so it stays a pure library.
- Every policy rule must have positive and negative test fixtures.
