# Contributing

Thanks for wanting to help. doupass is small on purpose; keep changes focused.

## Before you start

- For anything larger than a bug fix, open an issue first so we can agree on scope.
- House rules: deterministic decisions, no LLM calls in the engine, English code and docs (only `plans/` is Turkish), and tests for every behavior change.

## Development

```sh
go build ./...
go vet ./...
go test ./...
go test ./... -cover
```

## Adding a starter rule

1. Add the rule to `rules/starter.yml`.
2. Add at least one positive and one negative case to `rules/cases.yml`.
3. CI runs every case; a rule without fixtures will be rejected in review.

## Pull requests

- One concern per PR; reference the issue it closes.
- Paste the output of the commands above into the PR description.
- Policy format changes must update `spec/policy-v0.md` and `spec/policy-v0.schema.json` together.
- Run `doupass policy lint rules/starter.yml` before submitting rule changes.
