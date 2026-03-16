import { apiFetch } from '@/auth/api'

let cachedIsAdmin: boolean | null = null

export async function checkIsAdmin(): Promise<boolean> {
  if (cachedIsAdmin !== null) return cachedIsAdmin

  try {
    const res = await apiFetch('/api/v1/auth/hasAdmin')
    if (!res.ok) return false
    cachedIsAdmin = true
    return true
  } catch {
    return false
  }
}

export function clearAdminCache() {
  cachedIsAdmin = null
}
