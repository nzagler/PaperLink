<script setup lang="ts">
import { onMounted, ref } from "vue"
import { Button } from "@/components/ui/button"
import { Shield } from "lucide-vue-next"

const emit = defineEmits<{
  (e: "started"): void
  (e: "error", message: string): void
  (e: "status"): void
}>()

const OIDC_START_URL = "/api/v1/auth/oidc/start"
const oidcEnabled = ref(false)
const loading = ref(true)

async function loadStatus() {
  loading.value = true
  try {
    const res = await fetch("/api/v1/auth/oidc/status", { credentials: "include" })
    const body = await res.json()
    oidcEnabled.value = Boolean(body?.data?.configured && body?.data?.enabled)
  } catch {
    oidcEnabled.value = false
  } finally {
    loading.value = false
  }
}

function continueWithOidc() {
  emit("status")
  if (!oidcEnabled.value) {
    emit("error", "OIDC is not configured yet.")
    return
  }
  emit("started")
  try {
    window.location.assign(OIDC_START_URL)
  } catch {
    emit("error", "Could not start external login.")
  }
}

onMounted(loadStatus)
</script>

<template>
  <div class="space-y-4">
    <Button
        type="button"
        class="w-full bg-neutral-100 text-neutral-900 hover:bg-white"
        :disabled="loading || !oidcEnabled"
        @click="continueWithOidc"
    >
      <Shield class="mr-2 size-4" />
      {{ loading ? 'Checking OIDC...' : 'Continue with OIDC' }}
    </Button>

    <p v-if="oidcEnabled" class="text-xs text-neutral-500">
      You will be redirected to your identity provider and returned to Paperlink after signing in.
    </p>
    <p v-else class="text-xs text-neutral-500">
      OIDC must be configured in user settings before external login is available.
    </p>
  </div>
</template>
