import { setCurrentUser } from "@/auth/user"
import { apiFetch } from "@/auth/api"

/**
 * Frontend-only stubs for account settings.
 *
 * When the backend is ready, replace the bodies with apiFetch calls, e.g.
 *   PATCH /api/v1/auth/username  { username }
 *   PATCH /api/v1/auth/password  { oldPassword, newPassword }
 */

export async function changeUsername(username: string): Promise<void> {
  // Simulate latency
  await new Promise((r) => setTimeout(r, 350))

  // TODO (backend): validate uniqueness, enforce rules, return updated username
  setCurrentUser({ username })
}

export async function changePassword(_oldPassword: string, _newPassword: string): Promise<void> {
  // Simulate latency
  await new Promise((r) => setTimeout(r, 350))

  // TODO (backend): verify old password, update hash, return ok
}

export type OidcConfig = {
  configured: boolean
  connected: boolean
  issuerUrl: string
  clientId: string
  scopes: string
  enabled: boolean
}

export type SaveOidcConfigRequest = {
  issuerUrl: string
  clientId: string
  clientSecret: string
  scopes: string
  enabled: boolean
}

async function readApiData<T>(res: Response): Promise<T> {
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    throw new Error(body?.error || body?.message || "Request failed")
  }
  return body.data as T
}

export async function getOidcConfig(): Promise<OidcConfig> {
  const res = await apiFetch("/api/v1/auth/oidc/config")
  return readApiData<OidcConfig>(res)
}

export async function saveOidcConfig(config: SaveOidcConfigRequest): Promise<OidcConfig> {
  const res = await apiFetch("/api/v1/auth/oidc/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(config),
  })
  return readApiData<OidcConfig>(res)
}

export async function disconnectOidcIdentity(): Promise<void> {
  const res = await apiFetch("/api/v1/auth/oidc/identity", {
    method: "DELETE",
  })
  await readApiData<{ ok: boolean }>(res)
}
