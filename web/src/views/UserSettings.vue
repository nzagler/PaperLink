<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { FileText, KeyRound, Link, Shield, ShieldCheck, Unlink, User as UserIcon } from "lucide-vue-next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"

import { currentUser } from "@/auth/user"
import { changePassword, changeUsername, disconnectOidcIdentity, getOidcConfig, saveOidcConfig } from "@/auth/account"

type Notice = { type: "success" | "error"; message: string } | null

const notice = ref<Notice>(null)
let noticeTimer: number | undefined
function showNotice(n: Notice) {
  notice.value = n
  if (noticeTimer) window.clearTimeout(noticeTimer)
  if (n) noticeTimer = window.setTimeout(() => (notice.value = null), 3500)
}

const displayUsername = computed(() => currentUser.value?.username ?? "")

// Change username
const newUsername = ref("")
const usernameSaving = ref(false)

const usernameError = computed(() => {
  const u = newUsername.value.trim()
  if (!u) return ""
  if (u.length < 3) return "Minimum 3 characters."
  if (u === displayUsername.value) return "That’s already your username."
  return ""
})

const usernameCanSave = computed(() => {
  const u = newUsername.value.trim()
  if (!u) return false
  if (u.length < 3) return false
  if (u === displayUsername.value) return false
  return !usernameSaving.value
})

async function onSaveUsername() {
  if (!usernameCanSave.value) return
  usernameSaving.value = true
  try {
    await changeUsername(newUsername.value.trim())
    showNotice({ type: "success", message: "Username updated." })
    newUsername.value = ""
  } catch (e: any) {
    showNotice({ type: "error", message: e?.message ?? "Failed to change username" })
  } finally {
    usernameSaving.value = false
  }
}

// Change password
const oldPassword = ref("")
const newPassword = ref("")
const passwordSaving = ref(false)

const passwordError = computed(() => {
  if (!newPassword.value) return ""
  if (newPassword.value.length < 8) return "Minimum 8 characters."
  return ""
})

const passwordCanSave = computed(() => {
  if (!oldPassword.value || !newPassword.value) return false
  if (newPassword.value.length < 8) return false
  return !passwordSaving.value
})

async function onSavePassword() {
  if (!passwordCanSave.value) return
  passwordSaving.value = true
  try {
    await changePassword(oldPassword.value, newPassword.value)
    showNotice({ type: "success", message: "Password updated." })
    oldPassword.value = ""
    newPassword.value = ""
  } catch (e: any) {
    showNotice({ type: "error", message: e?.message ?? "Failed to change password" })
  } finally {
    passwordSaving.value = false
  }
}

const oidcIssuerUrl = ref("")
const oidcClientId = ref("")
const oidcClientSecret = ref("")
const oidcScopes = ref("openid profile email")
const oidcEnabled = ref(false)
const oidcConfigured = ref(false)
const oidcConnected = ref(false)
const oidcLoading = ref(false)
const oidcSaving = ref(false)
const oidcDisconnecting = ref(false)
const oidcRedirectUri = computed(() => `${window.location.origin}/api/v1/auth/oidc/callback`)

const oidcCanSave = computed(() => {
  return (
    oidcIssuerUrl.value.trim() !== "" &&
    oidcClientId.value.trim() !== "" &&
    (oidcConfigured.value || oidcClientSecret.value.trim() !== "") &&
    !oidcSaving.value
  )
})

async function loadOidcConfig() {
  oidcLoading.value = true
  try {
    const config = await getOidcConfig()
    oidcConfigured.value = config.configured
    oidcConnected.value = config.connected
    oidcIssuerUrl.value = config.issuerUrl
    oidcClientId.value = config.clientId
    oidcScopes.value = config.scopes || "openid profile email"
    oidcEnabled.value = config.enabled
  } catch (e: any) {
    showNotice({ type: "error", message: e?.message ?? "Failed to load OIDC settings" })
  } finally {
    oidcLoading.value = false
  }
}

async function onSaveOidcConfig() {
  if (!oidcCanSave.value) return
  oidcSaving.value = true
  try {
    const config = await saveOidcConfig({
      issuerUrl: oidcIssuerUrl.value.trim(),
      clientId: oidcClientId.value.trim(),
      clientSecret: oidcClientSecret.value.trim(),
      scopes: oidcScopes.value.trim() || "openid profile email",
      enabled: oidcEnabled.value,
    })
    oidcConfigured.value = config.configured
    oidcConnected.value = config.connected
    oidcIssuerUrl.value = config.issuerUrl
    oidcClientId.value = config.clientId
    oidcScopes.value = config.scopes
    oidcEnabled.value = config.enabled
    oidcClientSecret.value = ""
    showNotice({ type: "success", message: "OIDC provider saved." })
  } catch (e: any) {
    showNotice({ type: "error", message: e?.message ?? "Failed to save OIDC provider" })
  } finally {
    oidcSaving.value = false
  }
}

function connectOidc() {
  window.location.assign("/api/v1/auth/oidc/start?mode=link")
}

async function disconnectOidc() {
  oidcDisconnecting.value = true
  try {
    await disconnectOidcIdentity()
    oidcConnected.value = false
    showNotice({ type: "success", message: "OIDC account disconnected." })
  } catch (e: any) {
    showNotice({ type: "error", message: e?.message ?? "Failed to disconnect OIDC account" })
  } finally {
    oidcDisconnecting.value = false
  }
}

onMounted(() => {
  const params = new URLSearchParams(window.location.search)
  if (params.get("oidc") === "connected") {
    showNotice({ type: "success", message: "OIDC account connected." })
    window.history.replaceState({}, "", "/settings")
  }
  loadOidcConfig()
})
</script>

<template>
  <div class="mx-auto max-w-4xl px-4 lg:px-6 py-5 lg:py-7 space-y-4">
    <section
      class="rounded-2xl border border-neutral-200 bg-white shadow-sm shadow-neutral-200/70 overflow-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:shadow-none"
    >
      <div
        class="px-4 sm:px-6 py-4 bg-gradient-to-r from-neutral-50 via-white to-emerald-50/70 dark:from-neutral-900 dark:via-neutral-900 dark:to-emerald-900/30"
      >
        <div class="flex items-center gap-3">
          <div
            class="inline-flex h-10 w-10 items-center justify-center rounded-2xl bg-emerald-600/10 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200"
          >
            <Shield class="h-5 w-5" />
          </div>
          <div class="min-w-0">
            <h1 class="text-lg font-semibold tracking-tight">User settings</h1>
            <p class="text-xs text-neutral-500 dark:text-neutral-400 truncate">
              Signed in as
              <span class="font-medium text-neutral-900 dark:text-neutral-50">{{ displayUsername || '—' }}</span>
            </p>
          </div>
        </div>

        <div
          v-if="notice"
          class="mt-4 rounded-xl border px-4 py-3 text-sm"
          :class="
            notice.type === 'success'
              ? 'border-emerald-600/30 bg-emerald-600/10 text-emerald-900 dark:text-emerald-200 dark:bg-emerald-500/10'
              : 'border-red-600/30 bg-red-600/10 text-red-900 dark:text-red-200 dark:bg-red-500/10'
          "
        >
          {{ notice.message }}
        </div>
      </div>
    </section>

    <Card class="border border-neutral-200 dark:border-neutral-800">
      <CardHeader>
        <div class="flex items-center gap-2">
          <span
            class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-emerald-700/10 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200"
          >
            <UserIcon class="h-4 w-4" />
          </span>
          <div>
            <CardTitle class="text-sm">Account settings</CardTitle>
            <CardDescription class="text-[11px]">Change your username and password.</CardDescription>
          </div>
        </div>
      </CardHeader>

      <CardContent class="space-y-4">
        <!-- Username -->
        <div class="space-y-2">
          <p class="text-xs font-medium text-neutral-700 dark:text-neutral-200">Change username</p>
          <div class="flex flex-col sm:flex-row gap-2">
            <Input v-model="newUsername" placeholder="New username" class="flex-1" />
            <Button
              class="rounded-xl bg-emerald-700 text-white hover:bg-emerald-700/90"
              :disabled="!usernameCanSave"
              @click="onSaveUsername"
            >
              {{ usernameSaving ? 'Saving…' : 'Save' }}
            </Button>
          </div>
          <p v-if="usernameError" class="text-[11px] text-red-600 dark:text-red-400">{{ usernameError }}</p>
          <p v-else class="text-[11px] text-neutral-500 dark:text-neutral-400">Minimum 3 characters.</p>
        </div>

        <Separator />

        <!-- Password -->
        <div class="space-y-2">
          <p class="text-xs font-medium text-neutral-700 dark:text-neutral-200">Change password</p>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <Input v-model="oldPassword" type="password" placeholder="Current password" />
            <Input v-model="newPassword" type="password" placeholder="New password" />
          </div>

          <div class="flex items-center justify-between gap-3">
            <p v-if="passwordError" class="text-[11px] text-red-600 dark:text-red-400">{{ passwordError }}</p>
            <p v-else class="text-[11px] text-neutral-500 dark:text-neutral-400">Minimum 8 characters.</p>

            <Button
              class="rounded-xl bg-emerald-700 text-white hover:bg-emerald-700/90"
              :disabled="!passwordCanSave"
              @click="onSavePassword"
            >
              <KeyRound class="h-4 w-4" />
              {{ passwordSaving ? 'Saving…' : 'Update' }}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="border border-neutral-200 dark:border-neutral-800">
      <CardHeader>
        <div class="flex items-center gap-2">
          <span
            class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-emerald-700/10 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200"
          >
            <ShieldCheck class="h-4 w-4" />
          </span>
          <div>
            <CardTitle class="text-sm">OpenID Connect</CardTitle>
            <CardDescription class="text-[11px]">Configure an OIDC provider and link it to this account.</CardDescription>
          </div>
        </div>
      </CardHeader>

      <CardContent class="space-y-4">
        <div
          class="rounded-xl border px-4 py-3 text-xs"
          :class="
            oidcConnected
              ? 'border-emerald-600/30 bg-emerald-600/10 text-emerald-900 dark:text-emerald-200 dark:bg-emerald-500/10'
              : 'border-neutral-200 bg-neutral-50 text-neutral-600 dark:border-neutral-800 dark:bg-neutral-900/40 dark:text-neutral-300'
          "
        >
          {{ oidcConnected ? 'This account is linked to OIDC.' : 'Save a provider, then connect this account.' }}
        </div>

        <label class="flex items-center gap-2 text-xs text-neutral-700 dark:text-neutral-200">
          <input v-model="oidcEnabled" type="checkbox" class="h-4 w-4 rounded border-neutral-300 accent-emerald-700" />
          Enable OIDC
        </label>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <Input v-model="oidcIssuerUrl" placeholder="Issuer URL, e.g. https://id.example.com" />
          <Input v-model="oidcClientId" placeholder="Client ID" />
          <Input v-model="oidcClientSecret" placeholder="Client secret" />
          <Input v-model="oidcScopes" placeholder="Scopes" />
        </div>

        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <p class="text-[11px] text-neutral-500 dark:text-neutral-400">
            Redirect URI: <span class="font-mono">{{ oidcRedirectUri }}</span>
            <span v-if="oidcConfigured">. Leave the client secret blank to keep the existing value.</span>
          </p>
          <div class="flex gap-2">
            <Button
              variant="outline"
              class="rounded-xl"
              :disabled="!oidcConfigured || oidcLoading"
              @click="connectOidc"
            >
              <Link class="h-4 w-4" />
              Connect
            </Button>
            <Button
              v-if="oidcConnected"
              variant="outline"
              class="rounded-xl"
              :disabled="oidcDisconnecting"
              @click="disconnectOidc"
            >
              <Unlink class="h-4 w-4" />
              Disconnect
            </Button>
            <Button
              class="rounded-xl bg-emerald-700 text-white hover:bg-emerald-700/90"
              :disabled="!oidcCanSave"
              @click="onSaveOidcConfig"
            >
              {{ oidcSaving ? 'Saving...' : 'Save OIDC' }}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="border border-neutral-200 dark:border-neutral-800">
      <CardHeader>
        <div class="flex items-center gap-2">
          <span
            class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-emerald-700/10 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200"
          >
            <FileText class="h-4 w-4" />
          </span>
          <div>
            <CardTitle class="text-sm">PDF reader settings</CardTitle>
            <CardDescription class="text-[11px]">Coming soon.</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div
          class="rounded-xl border border-dashed border-neutral-300 bg-neutral-50 p-4 text-sm text-neutral-600 dark:border-neutral-700 dark:bg-neutral-900/40 dark:text-neutral-300"
        >
          Coming soon.
        </div>
      </CardContent>
    </Card>
  </div>
</template>

<style scoped>

</style>
