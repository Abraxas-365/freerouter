/**
 * Runtime configuration served by the FreeRouter server at
 * `GET /api/v1/config` (public). It tells the console which auth mode the
 * server runs in (FREEROUTER_AUTH) and, in iam mode, the IAMKit boundary to
 * sign in against — so one console build works with or without IAMKit.
 */

export type AuthMode = "none" | "token" | "iam"

export interface IAMKitConfig {
  url: string
  environment_id: string
  organization_id: string
  application_id: string
  resource_id: string
  audience: string
}

export interface RuntimeConfig {
  auth: AuthMode
  iamkit?: IAMKitConfig
}

export const API_BASE = (import.meta.env.VITE_API_URL as string | undefined) ?? ""

/** Build-time IAMKit values (VITE_IAMKIT_*): fallback for older servers. */
function envIAMKit(): IAMKitConfig {
  const env = import.meta.env
  return {
    url: (env.VITE_IAMKIT_URL as string | undefined) ?? "http://localhost:8080",
    environment_id: (env.VITE_IAMKIT_ENVIRONMENT_ID as string | undefined) ?? "",
    organization_id: (env.VITE_IAMKIT_ORGANIZATION_ID as string | undefined) ?? "",
    application_id: (env.VITE_IAMKIT_APPLICATION_ID as string | undefined) ?? "",
    resource_id: (env.VITE_IAMKIT_RESOURCE_ID as string | undefined) ?? "",
    audience: (env.VITE_IAMKIT_AUDIENCE as string | undefined) ?? "",
  }
}

let current: RuntimeConfig | null = null

/** Fetches the server's config once; later calls reuse it. */
export async function loadRuntimeConfig(): Promise<RuntimeConfig> {
  if (current) return current
  current = withIAMKitFallback(await fetchConfig())
  return current
}

/**
 * GET /api/v1/config. A 404 is a server that predates the auth modes (iam
 * only). A network error is retried with backoff first — a server
 * restarting under `make dev` must not pin a none/token console to iam.
 */
async function fetchConfig(): Promise<RuntimeConfig> {
  for (let attempt = 0; ; attempt++) {
    try {
      const res = await fetch(`${API_BASE}/api/v1/config`)
      if (res.status === 404) return { auth: "iam" }
      if (res.ok) return (await res.json()) as RuntimeConfig
    } catch {
      // network error: retry below
    }
    if (attempt === 3) return { auth: "iam" }
    await new Promise((r) => setTimeout(r, 500 * 2 ** attempt))
  }
}

function withIAMKitFallback(cfg: RuntimeConfig): RuntimeConfig {
  if (cfg.auth === "iam") {
    const fallback = envIAMKit()
    const fromServer = cfg.iamkit ?? fallback
    // A field the server leaves empty falls back to the build-time value.
    cfg.iamkit = Object.fromEntries(
      Object.entries(fromServer).map(([k, v]) => [k, v || fallback[k as keyof IAMKitConfig]]),
    ) as unknown as IAMKitConfig
  }
  return cfg
}

/** The loaded config; only valid after loadRuntimeConfig resolved. */
export function runtimeConfig(): RuntimeConfig {
  if (!current) throw new Error("runtime config not loaded")
  return current
}
