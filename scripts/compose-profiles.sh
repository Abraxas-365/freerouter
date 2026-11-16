#!/usr/bin/env sh
# Prints the docker compose profiles FreeRouter's .env needs: "iam" when the
# server runs with IAMKit (FREEROUTER_AUTH=iam, or unset with
# IAMKIT_ENVIRONMENT_ID configured, mirroring config.defaultAuthMode), else "".
env_file=${ENV_FILE:-.env}
[ -f "$env_file" ] || exit 0
get() { sed -n "s/^$1=//p" "$env_file" | tail -n 1; }
auth=$(get FREEROUTER_AUTH)
if [ "$auth" = iam ] || { [ -z "$auth" ] && [ -n "$(get IAMKIT_ENVIRONMENT_ID)" ]; }; then
  echo iam
fi
