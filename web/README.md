# FreeRouter Console

The operator web console for [FreeRouter](../README.md) — a React + TypeScript
single-page app for managing providers, models, provider keys, rate limits,
guardrails, webhooks, and usage analytics.

FreeRouter has no built-in user/organization management. All authentication
is delegated to an [IAMKit](https://github.com/Abraxas-365/iamkit) sidecar:
this console signs in against IAMKit's `/identity/v1` API directly to obtain
a bearer token, then sends that token to the FreeRouter API.

## Development

```bash
npm install
cp .env.example .env   # fill in the IAMKit boundary IDs, see below
npm run dev
```

## Configuration

| Variable | Description |
| --- | --- |
| `VITE_API_URL` | Base URL of the FreeRouter API (default `http://localhost:3000`) |
| `VITE_IAMKIT_URL` | Base URL of the IAMKit sidecar (default `http://localhost:8080`) |
| `VITE_IAMKIT_ENVIRONMENT_ID` | Must match `IAMKIT_ENVIRONMENT_ID` on the server |
| `VITE_IAMKIT_ORGANIZATION_ID` | Organization the signing-in user belongs to |
| `VITE_IAMKIT_APPLICATION_ID` | Must match `IAMKIT_APPLICATION_ID` on the server |
| `VITE_IAMKIT_RESOURCE_ID` | Must match `IAMKIT_RESOURCE_ID` on the server |
| `VITE_IAMKIT_AUDIENCE` | Must match `IAMKIT_AUDIENCE` on the server |

See the server's `.env.example` and IAMKit's `docs/guides/password-login.md`
for how these IDs are provisioned.

## Scripts

- `npm run dev` — start the Vite dev server
- `npm run build` — type-check and build for production
- `npm run lint` — run oxlint
- `npm run preview` — preview a production build
