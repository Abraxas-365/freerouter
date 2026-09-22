import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import type { Paginated } from "@/api/types"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Follows a paginated endpoint's `page.total`/`offset` until every item has
 * been fetched. The server clamps `limit` to 100 (see `internal/query`), so
 * a single request cannot be relied on to return "all" records.
 */
export async function fetchAllPages<T>(
  fetchPage: (params: { limit: number; offset: number }) => Promise<Paginated<T>>,
  pageSize = 100,
): Promise<T[]> {
  const items: T[] = []
  let offset = 0
  for (;;) {
    const res = await fetchPage({ limit: pageSize, offset })
    items.push(...res.items)
    offset += res.items.length
    if (res.items.length === 0 || offset >= res.page.total) break
  }
  return items
}
