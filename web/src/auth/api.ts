import { accessToken, setAccessToken } from './auth'
import { refreshAccessToken } from './refresh'
import { setCurrentUser } from './user'

let isRefreshing = false
let refreshPromise: Promise<void> | null = null

export async function apiFetch(input: RequestInfo, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  if (accessToken.value) {
    headers.set('Authorization', `Bearer ${accessToken.value}`)
  }

  const response = await fetch(input, {
    ...init,
    headers,
    credentials: 'include',
  })

  // 403 means forbidden, not necessarily unauthenticated.
  if (response.status === 403) {
    return response
  }

  if (response.status !== 401) {
    return response
  }

  if (!isRefreshing) {
    isRefreshing = true
    refreshPromise = refreshAccessToken().finally(() => {
      isRefreshing = false
    })
  }

  try {
    await refreshPromise
  } catch {
    setAccessToken(null)
    setCurrentUser(null)
    throw new Error('Session expired')
  }

  const retryHeaders = new Headers(init.headers)
  if (accessToken.value) {
    retryHeaders.set('Authorization', `Bearer ${accessToken.value}`)
  }

  return fetch(input, {
    ...init,
    headers: retryHeaders,
    credentials: 'include',
  })
}
