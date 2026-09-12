# Community rule packs

Packs are complete policies you can adopt with a single flag:

```sh
doupass install ...                 # any integration
doupass policy lint rules/packs/*.yml --strict
doupass policy test rules/packs/ci-safe.yml --tool Bash --arg "command=go test ./..."
```

## Adding a pack

1. One `.yml` file per pack, ASCII filename, `name:` matching the filename.
2. Must pass `doupass policy lint --strict` (no duplicates, no unexplained denies, no blanket rules).
3. Must pass schema validation and load without errors; CI enforces this.
4. Add a short comment-free description to this README table.

| Pack | Use case |
|---|---|
| `ci-safe.yml` | Unattended CI agents: read/test/build only, no network or writes |
