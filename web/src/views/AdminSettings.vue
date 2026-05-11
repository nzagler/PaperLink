<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { Loader2, LogOut, Search, Shield, Trash2, Users } from "lucide-vue-next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  deleteAdminUser,
  getAdminUsers,
  invalidateAdminUserSessions,
  type AdminUser,
  updateAdminUserRole,
} from "@/lib/admin_api"
import { currentUser } from "@/auth/user"

const users = ref<AdminUser[]>([])
const loading = ref(false)
const actionLoading = ref<string | null>(null)
const error = ref<string | null>(null)
const query = ref("")

const filteredUsers = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return users.value
  return users.value.filter((user) => user.username.toLowerCase().includes(q))
})

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  let value = bytes
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx++
  }
  const decimals = value >= 10 || idx === 0 ? 0 : 1
  return `${value.toFixed(decimals)} ${units[idx]}`
}

async function loadUsers() {
  loading.value = true
  error.value = null
  try {
    users.value = await getAdminUsers()
  } catch (err) {
    error.value = err instanceof Error ? err.message : "Failed to load users."
  } finally {
    loading.value = false
  }
}

async function changeRole(user: AdminUser, value: string) {
  const nextIsAdmin = value === "admin"
  if (nextIsAdmin === user.isAdmin) return

  actionLoading.value = `role:${user.id}`
  error.value = null
  try {
    await updateAdminUserRole(user.id, nextIsAdmin)
    user.isAdmin = nextIsAdmin
  } catch (err) {
    error.value = err instanceof Error ? err.message : "Failed to update role."
  } finally {
    actionLoading.value = null
  }
}

function onRoleChange(user: AdminUser, event: Event) {
  const target = event.target
  if (!(target instanceof HTMLSelectElement)) return
  void changeRole(user, target.value)
}

async function logoutUser(user: AdminUser) {
  actionLoading.value = `logout:${user.id}`
  error.value = null
  try {
    await invalidateAdminUserSessions(user.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : "Failed to end sessions."
  } finally {
    actionLoading.value = null
  }
}

async function removeUser(user: AdminUser) {
  const ok = window.confirm(`Delete ${user.username}? Shared documents will be transferred to the first user they were shared with.`)
  if (!ok) return

  actionLoading.value = `delete:${user.id}`
  error.value = null
  try {
    await deleteAdminUser(user.id)
    users.value = users.value.filter((item) => item.id !== user.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : "Failed to delete user."
  } finally {
    actionLoading.value = null
  }
}

onMounted(() => {
  void loadUsers()
})
</script>

<template>
  <div class="mx-auto max-w-6xl px-4 lg:px-6 py-5 lg:py-7 space-y-4">
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
          <div>
            <h1 class="text-lg font-semibold tracking-tight">Admin Settings</h1>
            <p class="text-xs text-neutral-500 dark:text-neutral-400">User management and access controls.</p>
          </div>
        </div>
      </div>
    </section>

    <Card class="border border-neutral-200 dark:border-neutral-800">
      <CardHeader class="gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-center gap-2">
          <span class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-emerald-700/10 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200">
            <Users class="h-4 w-4" />
          </span>
          <div>
            <CardTitle class="text-sm">Users</CardTitle>
            <CardDescription class="text-[11px]">Manage roles, sessions, and accounts.</CardDescription>
          </div>
        </div>
        <Button variant="outline" size="sm" :disabled="loading" @click="loadUsers">
          <Loader2 v-if="loading" class="mr-2 h-4 w-4 animate-spin" />
          Refresh
        </Button>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="relative max-w-sm">
          <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-neutral-400" />
          <Input v-model="query" class="pl-9" placeholder="Search users" />
        </div>

        <p v-if="error" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-200">
          {{ error }}
        </p>

        <div v-if="loading" class="py-8 text-center text-sm text-neutral-500 dark:text-neutral-400">
          Loading...
        </div>

        <div v-else class="overflow-x-auto rounded-lg border border-neutral-200 dark:border-neutral-800">
          <table class="w-full min-w-[720px] text-sm">
            <thead class="bg-neutral-50 text-xs text-neutral-500 dark:bg-neutral-900/60 dark:text-neutral-400">
              <tr>
                <th class="px-3 py-2 text-left font-medium">User</th>
                <th class="px-3 py-2 text-left font-medium">Role</th>
                <th class="px-3 py-2 text-right font-medium">Docs</th>
                <th class="px-3 py-2 text-right font-medium">Pages</th>
                <th class="px-3 py-2 text-right font-medium">Storage</th>
                <th class="px-3 py-2 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-neutral-200 dark:divide-neutral-800">
              <tr v-for="user in filteredUsers" :key="user.id" class="bg-white dark:bg-neutral-950">
                <td class="px-3 py-2">
                  <div class="font-medium text-neutral-900 dark:text-neutral-50">
                    {{ user.username }}
                    <span v-if="currentUser?.username === user.username" class="ml-1 text-xs text-neutral-400">(you)</span>
                  </div>
                </td>
                <td class="px-3 py-2">
                  <select
                    class="h-9 rounded-md border border-neutral-300 bg-white px-2 text-sm dark:border-neutral-700 dark:bg-neutral-900"
                    :value="user.isAdmin ? 'admin' : 'user'"
                    :disabled="actionLoading !== null"
                    @change="onRoleChange(user, $event)"
                  >
                    <option value="user">User</option>
                    <option value="admin">Admin</option>
                  </select>
                </td>
                <td class="px-3 py-2 text-right tabular-nums">{{ user.documentCount }}</td>
                <td class="px-3 py-2 text-right tabular-nums">{{ user.totalPages }}</td>
                <td class="px-3 py-2 text-right tabular-nums">{{ formatBytes(user.totalSize) }}</td>
                <td class="px-3 py-2">
                  <div class="flex justify-end gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      :disabled="actionLoading !== null"
                      @click="logoutUser(user)"
                    >
                      <Loader2 v-if="actionLoading === `logout:${user.id}`" class="mr-2 h-4 w-4 animate-spin" />
                      <LogOut v-else class="mr-2 h-4 w-4" />
                      Log out
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      :disabled="actionLoading !== null || currentUser?.username === user.username"
                      @click="removeUser(user)"
                    >
                      <Loader2 v-if="actionLoading === `delete:${user.id}`" class="mr-2 h-4 w-4 animate-spin" />
                      <Trash2 v-else class="mr-2 h-4 w-4" />
                      Delete
                    </Button>
                  </div>
                </td>
              </tr>
              <tr v-if="filteredUsers.length === 0">
                <td colspan="6" class="px-3 py-8 text-center text-sm text-neutral-500 dark:text-neutral-400">
                  No users found.
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
