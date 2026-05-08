import { setCurrentUser } from "@/auth/user"
import { apiFetch } from "@/auth/api"

export async function changeUsername(username: string): Promise<void> {
  const res = await apiFetch("/api/v1/auth/username", {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username }),
  })
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    throw new Error(body?.error || body?.message || "Failed to change username")
  }
  const updatedUsername = body?.data?.username ?? username
  setCurrentUser({ username: updatedUsername })
}

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  const res = await apiFetch("/api/v1/auth/password", {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ oldPassword, newPassword }),
  })
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    throw new Error(body?.error || body?.message || "Failed to change password")
  }
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
