# Policy format validation kit (S7.5)

Purpose: before the policy format freezes, verify that developers who did not design it can read the spec and write a correct rule. This is the weakest assumption in the project plan, and it requires **5 external participants**.

Status: kit ready — awaiting 5 participants. Track outcomes in a GitHub issue.

## Protocol

- Audience: developers who use a coding agent (any) but have not seen doupass before.
- Time: ~15 minutes, no help from the author, spec only (`spec/policy-v0.md`) plus `rules/starter.yml` as an example.
- Record: task success (yes/no), errors, verbatim confusion quotes.

## Task A — write a rule (5 min)

Given the spec, write a rule that:

1. denies any tool call whose arguments reference a path under `/secrets`, and
2. leaves `/work` readable.

Answer (for the facilitator): add the rule below to `rules/starter.yml` and confirm with `doupass policy test`:

```yaml
  - id: block-secrets-dir
    match:
      args:
        "*": "**/secrets/**"
    action: deny
    reason: "Secrets directory is off-limits"
```

Success = the format, glob behavior, and `action` semantics were understood without help.

## Task B — predict decisions (5 min)

Given `rules/starter.yml` with `WORKSPACE=/work` and `HOME=/home/dev`, predict the action for each call:

| # | Call | Expected |
|---|---|---|
| 1 | hook `Read` `file_path=/home/dev/.ssh/id_rsa` | deny (`block-ssh-credentials`) |
| 2 | hook `Bash` `command="cat /home/dev/.ssh/id_rsa"` | deny (the raw value matches too) |
| 3 | hook `Bash` `command="npm install left-pad"` | ask (`ask-package-install`) |
| 4 | hook `Read` `file_path=/tmp/notes.txt` | allow (default) |
| 5 | hook `Read` `file_path=/work/app/.env` | deny (`block-env-files`; deny beats `allow-workspace`) |

Score: 5/5 or 4/5 = pass (the pattern in #2 may surprise; that is a documented behavior, not a trick).

## Task C — explain a deny (3 min)

Why does the pattern `**/.ssh/**` in an `args: {"*": ...}` rule catch `cat ~/.ssh/id_rsa` although the argument is a shell command, not a path?

Expected explanation: `*` patterns match any characters (including `/`), so the raw command string matches; path-like values are additionally canonicalized (variable/`~` expansion, symlink resolution) before matching, which closes the obvious bypasses.

## Result template

```
Participant 1: A pass / B 4/5 / C pass — notes:
Participant 2: ...
Success threshold: >= 3 of 5 pass A and B.
```

If the threshold is not met, the fallback documented in `plans/doupass-plan.md` applies: narrow the positioning to "security layer for Claude Code" and revisit the format.
