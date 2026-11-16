#!/usr/bin/env sh
# Usage: scripts/env-set.sh KEY VALUE [--if-empty]
# Sets KEY=VALUE in $ENV_FILE (default .env): rewrites an existing KEY line in
# place, or appends one. With --if-empty, leaves a non-empty value alone.
set -eu
key=$1 value=$2 mode=${3:-}
env_file=${ENV_FILE:-.env}
current=$(sed -n "s/^$key=//p" "$env_file" | tail -n 1)
if [ "$mode" = --if-empty ] && [ -n "$current" ]; then exit 0; fi
tmp="$env_file.tmp.$$"
if grep -q "^$key=" "$env_file"; then
  KEY=$key VALUE=$value awk 'index($0, ENVIRON["KEY"] "=") == 1 { print ENVIRON["KEY"] "=" ENVIRON["VALUE"]; next } { print }' "$env_file" > "$tmp"
else
  cp "$env_file" "$tmp"
  printf '%s=%s\n' "$key" "$value" >> "$tmp"
fi
chmod 600 "$tmp"
mv "$tmp" "$env_file"
