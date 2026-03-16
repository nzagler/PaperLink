import { setAccessToken } from './auth'
import { setCurrentUser } from './user'
import { apiFetch } from './api'
import { clearAdminCache } from '@/lib/admin'

const LOGOUT_ENDPOINT = '/api/v1/auth/logout'

export async function logout() {
  // Immediately clear local auth state so navigation is blocked right away.
  setAccessToken(null)
  setCurrentUser(null)
  clearAdminCache()

  // Best-effort server logout to clear refresh cookie.
  // Use apiFetch so it matches the same origin/proxy setup.
  try {
    await apiFetch(LOGOUT_ENDPOINT, { method: 'POST' })
  } catch {
  }
}
