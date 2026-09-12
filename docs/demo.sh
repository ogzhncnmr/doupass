#!/usr/bin/env sh
set -eu

# doupass demo script (for recordings). Requires doupass on PATH.
demo_dir="$(mktemp -d)"
cd "$demo_dir"

say() {
  printf '\n\033[1;36m$ %s\033[0m\n' "$1"
}

say "doupass version"
doupass version

say "doupass init"
doupass init

say "doupass policy lint doupass.yml"
doupass policy lint doupass.yml

say "doupass policy test doupass.yml --tool Read --arg file_path=~/.ssh/id_rsa"
doupass policy test doupass.yml --tool Read --arg file_path=~/.ssh/id_rsa

say 'doupass policy test doupass.yml --surface hook --tool Bash --arg "command=npm install x"'
doupass policy test doupass.yml --surface hook --tool Bash --arg "command=npm install x"

say "echo PreToolUse | doupass hook claude"
printf '%s' '{"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"/home/x/.ssh/id_rsa"}}' \
  | doupass hook claude --policy doupass.yml

say "doupass log tail"
doupass log tail

say "doupass log verify"
doupass log verify

printf '\nDemo directory: %s\n' "$demo_dir"
