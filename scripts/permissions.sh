#!/usr/bin/env bash
# Single source of truth for the IAMKit permissions FreeRouter provisions.
# Sourced by bootstrap-iamkit.sh; keep FREEROUTER_PERMISSIONS in sync with
# internal/server/permissions.go (ValidPermissions).

# Permissions of the "FreeRouter API" resource (prefix freerouter).
FREEROUTER_PERMISSIONS='[
  "freerouter:gateway:invoke",
  "freerouter:gateway:write",
  "freerouter:metrics:read",
  "freerouter:providers:read",
  "freerouter:providers:write",
  "freerouter:provider-keys:read",
  "freerouter:provider-keys:write",
  "freerouter:usage:read",
  "freerouter:usage:write",
  "freerouter:rate-limits:read",
  "freerouter:rate-limits:write",
  "freerouter:routing:read",
  "freerouter:routing:write",
  "freerouter:guardrails:read",
  "freerouter:guardrails:write",
  "freerouter:webhooks:read",
  "freerouter:webhooks:write",
  "freerouter:service-accounts:read",
  "freerouter:service-accounts:write",
  "freerouter:users:read",
  "freerouter:users:write",
  "freerouter:roles:read",
  "freerouter:roles:write"
]'

# Permissions of FreeRouter's backend service account on the environment's
# built-in IAM resource (prefix iam): exactly what /api/v1/access and
# /api/v1/service-accounts call. Never a management key (ik_mgmt_).
BACKEND_IAM_PERMISSIONS='[
  "iam:users:read",
  "iam:users:write",
  "iam:roles:read",
  "iam:roles:write",
  "iam:service-accounts:read",
  "iam:service-accounts:write",
  "iam:apps:read"
]'
