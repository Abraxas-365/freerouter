# Tenant IAM and rollout

FreeRouter's customer API manages one authenticated tenant. `*` grants all customer application actions **inside that tenant**. It never bypasses tenant ownership. Legacy `platform:*`/`platform:…` and `admin:…` grants are inert; existing stored values can be removed separately.

## Customer/operator boundary

Customer routes retain tenant BYOK keys, wallets, paid Stripe checkout, spending limits, routing, usage, guardrails, webhooks, and gateway access. Shared provider/model/mapping/fallback catalogs are readable but not customer-writable. Managed provider credentials, manual balance top-ups/adjustments, and all-tenant cache invalidation are not exposed. No operator application or operator identity was added. Provision shared resources through controlled operator tooling outside this customer API.

## Request identity and permissions

`kernel.AuthContext.Actor` replaces `UserID` and `IsAPIKey`. `Actor.UserID()` and `Actor.APIKeyID()` return `(typedID, bool)`; zero and ambiguous actors are invalid. API keys act as themselves, not their associated user or creator. API-key creation, mutation, revocation, deletion, and invitation creation require a user actor. Existing optional API-key `user_id` is an association, not an authenticated identity; there is no new persisted `created_by` field.

Gateway model allowlists and wallet binding remain intact. Gateway usage logs now carry the acting API-key ID independently of the upstream provider-key ID, including streaming requests. Actor itself requires no migration.

JWT requests reload membership, tenant status, active session, credential generation, and current effective user/role scopes. Scope resolution errors fail closed. Removing user or role permissions takes effect on subsequent requests, rather than waiting for token expiry. API keys retain independent grants until revoked or changed. Key creation/scope replacement, direct-user scope replacement, and role creation/update enforce caller scope coverage in services. Role assignment checks caller authority in the service and locks the authorized role version during persistence. Pending invitations pin the role version checked at issuance; a changed role requires a new invitation. A concrete grant cannot delegate a broader wildcard.

## Membership and credential lifecycle

- No `POST /api/v1/users`; joining is invitation-based.
- `PUT /api/v1/users/:id` rejects `status`. Scope replacement also requires `scopes:write`; explicit `[]` clears direct grants.
- `POST /api/v1/users/:id/suspend` requires a reason.
- `POST /api/v1/users/:id/reinstate` restores only a suspended, verified member. Activation belongs to onboarding.
- API-key updates reject `is_active`, including false. Use `POST /api/v1/api-keys/:id/revoke`; revoked keys cannot be reactivated. Create a replacement key instead.
- User suspension advances a persisted credential generation, revokes refresh credentials, and expires sessions atomically in PostgreSQL. Old credentials cannot recover on reinstatement. Tenant suspension rejects both JWTs and API keys.
- New access and refresh credentials are bound to a server session. Logout revokes all of that user's sessions/refresh credentials; subsequent access checks reject the old tokens.

## Invitations and authentication

Signup initiation only sends a `SIGNUP` OTP. It does not create a membership, enable login, attach roles/scopes, or consume the invitation. Signup verification requires `name`, `email`, `code`, `tenant_id`, and `invitation_token`. After email ownership verification, one PostgreSQL transaction locks and validates the invitation/tenant/member, applies membership and grants, updates tenant capacity, and accepts the invitation. Role failures roll everything back. OTP consumption occurs before that transaction; retry with a fresh code after a failed acceptance.

Existing pending/active members can receive invitations to link another authentication method; suspended/inactive members cannot use invitations to reactivate. Returning OAuth login matches the stored provider **and subject**, with tenant selection for ambiguous memberships, and does not need another invitation. Microsoft Graph mail/UPN does not prove ownership: existing linked Microsoft users can return, but new Microsoft invitation linking is blocked pending trustworthy mailbox verification. OTP login/redemption/resend require `OTPEnabled` and eligible membership.

Invitation management includes `POST /api/v1/invitations/:id/resend`. Public token inspection is at `/api/v1/invitations/public/validate?token=…` and `/api/v1/invitations/public/token/:token`; treat URLs containing tokens as secrets. Signup OTP resend requires the invitation token and `purpose: "signup"`. Login uses the separate `LOGIN` OTP purpose.

IAM email delivery is wired to the existing SES notification adapter. Configure `NOTIFX_PROVIDER=ses` and the existing sender/region/AWS credentials. Console notification mode **fails closed for IAM secrets**, rather than logging codes or invitation tokens. Tests can inject recording notifiers. Delivery failure can leave a pending invitation; use resend. No delivery outbox or public first-tenant bootstrap is included.

## Deployment (breaking authentication change)

1. Back up the database and stop old application instances. Do not mix old and new binaries: old binaries do not enforce credential generations/session binding.
2. Apply `migrations/018_tenant_iam_credentials.up.sql` and then `migrations/019_iam_concurrent_mutations.up.sql` once, after all earlier migrations, using your migration runner or a single transaction with `ON_ERROR_STOP`.
3. Deploy backend and dashboard together; configure IAM email delivery before inviting users or using OTP.
4. Require all users to log in again. Legacy access tokens lack mandatory session binding; legacy refresh credentials are revoked by the migration. Existing API keys remain usable only for active tenants and customer routes.
5. Verify tenant isolation, current role grants, invitation delivery/acceptance, wallet-bound gateway calls, and Stripe checkout in your deployment.

Migration 018 adds user/refresh credential versions, the suspension trigger, `SIGNUP`/`LOGIN` purposes, and a **VARCHAR(255)** session foreign key matching FreeRouter's actual schema. It does not alter old migration numbering (including the two existing `003` files). No automated down migration is provided: removing these controls can restore old credentials. Use a reviewed rollback plan rather than restoring an old binary against live traffic.

## Validation and limits

Unit regressions cover Actor exclusivity/serialization, tenant-local wildcards, API-key grant coverage and tenant eligibility, JWT session/version/current-scope checks, and returning Microsoft identity behavior. Container integration tests apply migrations to disposable PostgreSQL and Redis, exercise gateway flows, invitation acceptance/rollback/replay, capacity accounting, and suspension/reinstatement. Production databases are not migrated by tests.

The dashboard's existing API contracts were adjusted for reinstatement, invitation public paths/resend, and current tenant lookup. Catalog editing and key reactivation controls were removed. This does not add a new login or team-management screen; this checkout's existing dashboard did not contain those screens. Browser OAuth/OTP flows and real SES delivery still require deployment testing.

## Additional security review

Migration 019 adds optimistic versions to tenant, user, role, API-key, and invitation rows. Loaded entities may only update the version read; concurrent mutations fail rather than restoring old grants or recreating deleted records. Reload before retrying a mutation. API-key revocation is a dedicated monotonic update; usage telemetry does not contend with security versions. Login no longer saves an entire user entity just to record last login.

OTP attempt increments and consumption are atomic, and consumed challenges cannot reopen. Passwordless endpoints have a per-process, per-IP limit of 20 requests/minute; configure trusted proxy handling and an edge/shared rate limit for multi-instance deployments. Issuance cooldown also applies to exhausted/consumed challenges. Database failures reject gateway limit checks; RPM count-and-admit is atomic in Redis.

Webhook delivery permits only HTTP(S), rejects private/special-use destinations at connection time, pins the resolved address, ignores proxy environment variables, and does not follow redirects. Deploy an outbound network policy as defense in depth. Private/internal webhook receivers are intentionally unsupported.

Global `/metrics` is no longer registered on the customer listener. Metric instrumentation remains; exposing it later requires a private operator surface, not tenant wildcard authority.

Existing pending role invitations have no authorized version and must be revoked/reissued after migration 019. Role changes invalidate outstanding pinned invitations conservatively, including metadata-only role changes.

### Outstanding billing decision — not security-cleared for release

Gateway prechecks now stop execution on denial and dependency failure. They are **not** atomic budget reservations. Costs are determined and debited after upstream execution; simultaneous requests may all pass a balance/cap check. The existing tenant-balance nonnegative constraint can reject an actual-cost debit, while wallet debits clamp the balance to zero. Debit errors are logged, without a durable settlement retry. Choosing strict prepaid reservations versus recorded overdraft/postpaid settlement changes product/accounting policy and remains a release blocker pending approval. Do not interpret passing IAM tests as clearance of this billing exposure.
