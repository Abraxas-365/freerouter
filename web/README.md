# FreeRouter Console

The operator web console for [FreeRouter](../README.md) — a React + TypeScript
single-page app for managing providers, models, provider keys, rate limits,
guardrails, webhooks, and usage analytics.

The console reads the server's auth mode at startup (`GET /api/v1/config`,
from `FREEROUTER_AUTH`) and signs in accordingly:

| Mode | Sign-in |
| --- | --- |
| `none` | None — the server only answers on localhost |
| `token` | The server's `FREEROUTER_ADMIN_TOKEN`, exchanged for an HttpOnly, same-origin session cookie (`POST /api/v1/session`) |
| `iam` | IAMKit email + password against IAMKit's `/identity/v1` API; the bearer token is sent to the FreeRouter API. The IAMKit URL and boundary IDs come from the server |

## Development

```bash
npm install
npm run dev   # http://localhost:5173 — proxies /api and /v1 to http://localhost:3000
```

No `.env` is needed for a server on `localhost:3000`.

## Configuration

| Variable | Description |
| --- | --- |
| `VITE_API_URL` | FreeRouter API base URL. Default empty: same origin (the dev proxy, or the console served under the API's host). `token` mode needs same origin — its session cookie isn't sent cross-site |
| `VITE_IAMKIT_*` | Fallback IAMKit URL/boundary for a server that doesn't serve them in `/api/v1/config` |

## Scripts

- `npm run dev` — start the Vite dev server
- `npm run build` — type-check and build for production
- `npm run lint` — run oxlint
- `npm run preview` — preview a production build
