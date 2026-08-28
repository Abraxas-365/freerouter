import { createContext, useContext, type ReactNode } from "react"
import type { ApiPort } from "./ports"
import { realApi } from "./real-adapter"
import { mockApi } from "./mock-adapter"

const ApiContext = createContext<ApiPort | null>(null)

const USE_MOCK = import.meta.env.VITE_USE_MOCK === "true"

export function ApiProvider({ children, mock }: { children: ReactNode; mock?: boolean }) {
  const api = (mock ?? USE_MOCK) ? mockApi : realApi

  return <ApiContext.Provider value={api}>{children}</ApiContext.Provider>
}

export function useApi(): ApiPort {
  const ctx = useContext(ApiContext)
  if (!ctx) throw new Error("useApi must be used within ApiProvider")
  return ctx
}
