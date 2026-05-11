import { apiFetch } from "@/auth/api"

export type AdminStats = {
  userCount: number
  documentCount: number
  totalDocSize: number
  totalPages: number
  d4sBookCount: number
  d4sAccountCount: number
}

export type AdminUser = {
  id: number
  username: string
  isAdmin: boolean
  documentCount: number
  totalSize: number
  totalPages: number
}

type AdminStatsEnvelope = {
  code: number
  data: AdminStats
}

type AdminUsersEnvelope = {
  code: number
  data: AdminUser[]
}

export async function getAdminStats(): Promise<AdminStats> {
  const res = await apiFetch("/api/v1/admin/stats")
  if (!res.ok) {
    const msg = await safeError(res)
    throw new Error(msg)
  }
  const json = (await res.json()) as AdminStatsEnvelope
  return json.data
}

export async function getAdminUsers(): Promise<AdminUser[]> {
  const res = await apiFetch("/api/v1/admin/users")
  if (!res.ok) {
    const msg = await safeError(res)
    throw new Error(msg)
  }
  const json = (await res.json()) as AdminUsersEnvelope
  return json.data
}

export async function updateAdminUserRole(userId: number, isAdmin: boolean): Promise<void> {
  const res = await apiFetch(`/api/v1/admin/users/${userId}/role`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ isAdmin }),
  })
  if (!res.ok) {
    const msg = await safeError(res)
    throw new Error(msg)
  }
}

export async function invalidateAdminUserSessions(userId: number): Promise<void> {
  const res = await apiFetch(`/api/v1/admin/users/${userId}/logout`, {
    method: "POST",
  })
  if (!res.ok) {
    const msg = await safeError(res)
    throw new Error(msg)
  }
}

export async function deleteAdminUser(userId: number): Promise<void> {
  const res = await apiFetch(`/api/v1/admin/users/${userId}`, {
    method: "DELETE",
  })
  if (!res.ok) {
    const msg = await safeError(res)
    throw new Error(msg)
  }
}

async function safeError(res: Response): Promise<string> {
  try {
    const json = (await res.json()) as any
    return json?.error ?? `Request failed (${res.status})`
  } catch {
    return `Request failed (${res.status})`
  }
}

