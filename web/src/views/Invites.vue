<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Check, FileText, Loader2, X } from 'lucide-vue-next'
import { apiFetch } from '@/auth/api'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

type DocumentInvite = {
  documentId: number
  documentUuid: string
  documentName: string
  owner: string
  role: 'VIEWER' | 'EDITOR'
  updatedAt: string
}

const router = useRouter()
const invites = ref<DocumentInvite[]>([])
const loading = ref(false)
const actionLoading = ref<string | null>(null)
const error = ref<string | null>(null)

function formatDate(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleDateString()
}

async function loadInvites() {
  loading.value = true
  error.value = null
  try {
    const res = await apiFetch('/api/v1/document/invites')
    const json = await res.json().catch(() => null)
    if (!res.ok) {
      throw new Error(json?.error || 'Failed to load invites.')
    }
    invites.value = Array.isArray(json?.data) ? json.data : []
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load invites.'
  } finally {
    loading.value = false
  }
}

async function respond(invite: DocumentInvite, action: 'accept' | 'decline') {
  actionLoading.value = `${invite.documentUuid}:${action}`
  error.value = null
  try {
    const res = await apiFetch(`/api/v1/document/${invite.documentUuid}/invite/${action}`, {
      method: 'POST',
    })
    const json = await res.json().catch(() => null)
    if (!res.ok) {
      throw new Error(json?.error || `Failed to ${action} invite.`)
    }
    invites.value = invites.value.filter((item) => item.documentUuid !== invite.documentUuid)
    if (action === 'accept') {
      await router.push(`/pdf/${invite.documentUuid}`)
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : `Failed to ${action} invite.`
  } finally {
    actionLoading.value = null
  }
}

onMounted(() => {
  void loadInvites()
})
</script>

<template>
  <div class="mx-auto max-w-5xl px-4 lg:px-6 py-5 lg:py-7 space-y-4">
    <header class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold tracking-tight">Invites</h1>
        <p class="text-xs text-neutral-500 dark:text-neutral-400">Review document sharing invitations.</p>
      </div>
      <Button variant="outline" size="sm" :disabled="loading" @click="loadInvites">
        <Loader2 v-if="loading" class="mr-2 h-4 w-4 animate-spin" />
        Refresh
      </Button>
    </header>

    <p v-if="error" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-200">
      {{ error }}
    </p>

    <div v-if="loading" class="py-10 text-center text-sm text-neutral-500 dark:text-neutral-400">
      Loading...
    </div>

    <Card v-else-if="!invites.length" class="border-dashed">
      <CardContent class="flex items-center gap-3 p-5">
        <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-neutral-900 text-neutral-50 dark:bg-neutral-200 dark:text-neutral-900">
          <FileText class="h-4 w-4" />
        </div>
        <div>
          <p class="text-sm font-medium">No pending invites</p>
          <p class="text-xs text-neutral-500 dark:text-neutral-400">Accepted documents appear in Home and Search.</p>
        </div>
      </CardContent>
    </Card>

    <div v-else class="space-y-3">
      <Card v-for="invite in invites" :key="invite.documentUuid">
        <CardHeader class="pb-3">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <CardTitle class="truncate text-sm">{{ invite.documentName }}</CardTitle>
              <CardDescription class="text-xs">
                From {{ invite.owner }} · {{ formatDate(invite.updatedAt) }}
              </CardDescription>
            </div>
            <Badge variant="outline">{{ invite.role }}</Badge>
          </div>
        </CardHeader>
        <CardContent class="flex justify-end gap-2 pt-0">
          <Button
              variant="outline"
              size="sm"
              :disabled="actionLoading !== null"
              @click="respond(invite, 'decline')"
          >
            <X class="mr-2 h-4 w-4" />
            Decline
          </Button>
          <Button
              size="sm"
              :disabled="actionLoading !== null"
              @click="respond(invite, 'accept')"
          >
            <Loader2 v-if="actionLoading === `${invite.documentUuid}:accept`" class="mr-2 h-4 w-4 animate-spin" />
            <Check v-else class="mr-2 h-4 w-4" />
            Accept
          </Button>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
