import { createContext, useContext, type ReactNode } from "react"
import type { ApiPort } from "./ports"
import { realApi } from "./real/adapter"

const ApiContext = createContext<ApiPort | null>(null)

export function ApiProvider({ children }: { children: ReactNode }) {
  return <ApiContext.Provider value={realApi}>{children}</ApiContext.Provider>
}

export function useApi(): ApiPort {
  const ctx = useContext(ApiContext)
  if (!ctx) throw new Error("useApi must be used within ApiProvider")
  return ctx
}
