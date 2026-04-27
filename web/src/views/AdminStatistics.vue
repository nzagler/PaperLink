<script setup lang="ts">
import { onMounted, ref, computed } from "vue"
import { BarChart3, RefreshCcw } from "lucide-vue-next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

import { getAdminStats, type AdminStats } from "@/lib/admin_api"

type Notice = { type: "success" | "error"; message: string } | null

const notice = ref<Notice>(null)
let noticeTimer: number | undefined
function showNotice(n: Notice) {
  notice.value = n
  if (noticeTimer) window.clearTimeout(noticeTimer)
  if (n) noticeTimer = window.setTimeout(() => (notice.value = null), 3500)
}

const stats = ref<AdminStats | null>(null)
const loading = ref(false)

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatNumber(n: number) {
  return new Intl.NumberFormat().format(n)
}

const avgPagesPerDoc = computed(() => {
  if (!stats.value || stats.value.documentCount === 0) return 0
  return Math.round(stats.value.totalPages / stats.value.documentCount)
})

const avgDocSize = computed(() => {
  if (!stats.value || stats.value.documentCount === 0) return "0 B"
  return formatBytes(stats.value.totalDocSize / stats.value.documentCount)
})

const d4sCoverage = computed(() => {
  if (!stats.value || stats.value.userCount === 0) return 0
  return Math.round((stats.value.d4sAccountCount / stats.value.userCount) * 100)
})

async function load() {
  loading.value = true
  try {
    stats.value = await getAdminStats()
  } catch (e: any) {
    showNotice({ type: "error", message: e?.message ?? "Failed to load statistics" })
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await load()
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
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-3">
            <div
                class="inline-flex h-10 w-10 items-center justify-center rounded-2xl bg-emerald-600/10 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200"
            >
              <BarChart3 class="h-5 w-5" />
            </div>
            <div>
              <h1 class="text-lg font-semibold tracking-tight">Statistics</h1>
              <p class="text-xs text-neutral-500 dark:text-neutral-400">Live overview of your instance.</p>
            </div>
          </div>

          <Button variant="outline" class="rounded-full" :disabled="loading" @click="load">
            <RefreshCcw class="h-4 w-4" :class="loading ? 'animate-spin' : ''" />
            Refresh
          </Button>
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

    <div v-if="loading" class="flex items-center justify-center py-20">
      <div class="flex items-center gap-3 text-sm text-neutral-500 dark:text-neutral-400">
        <RefreshCcw class="h-4 w-4 animate-spin" />
        Loading statistics…
      </div>
    </div>

    <div
        v-else-if="!stats"
        class="rounded-2xl border border-dashed border-neutral-300 bg-neutral-50 p-8 text-sm text-neutral-500 text-center dark:border-neutral-700 dark:bg-neutral-900/40 dark:text-neutral-400"
    >
      No statistics available.
    </div>

    <template v-else>
      <!-- Primary metric cards -->
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-2xl border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">Users</p>
          <p class="mt-1 text-3xl font-semibold tabular-nums">{{ formatNumber(stats.userCount) }}</p>
          <p class="mt-1 text-[11px] text-neutral-400 dark:text-neutral-500">{{ d4sCoverage }}% have D4S accounts</p>
        </div>

        <div class="rounded-2xl border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">Documents</p>
          <p class="mt-1 text-3xl font-semibold tabular-nums">{{ formatNumber(stats.documentCount) }}</p>
          <p class="mt-1 text-[11px] text-neutral-400 dark:text-neutral-500">{{ formatNumber(avgPagesPerDoc) }} pages on avg</p>
        </div>

        <div class="rounded-2xl border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">Total storage</p>
          <p class="mt-1 text-3xl font-semibold tabular-nums">{{ formatBytes(stats.totalDocSize) }}</p>
          <p class="mt-1 text-[11px] text-neutral-400 dark:text-neutral-500">{{ avgDocSize }} per document</p>
        </div>

        <div class="rounded-2xl border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">Total pages</p>
          <p class="mt-1 text-3xl font-semibold tabular-nums">{{ formatNumber(stats.totalPages) }}</p>
          <p class="mt-1 text-[11px] text-neutral-400 dark:text-neutral-500">across all documents</p>
        </div>
      </div>

      <!-- Charts row -->
      <div class="grid gap-3 lg:grid-cols-2">
        <!-- Distribution bar chart -->
        <Card class="border border-neutral-200 dark:border-neutral-800">
          <CardHeader class="pb-2">
            <CardTitle class="text-sm">Instance overview</CardTitle>
            <CardDescription class="text-[11px]">Key counts at a glance.</CardDescription>
          </CardHeader>
          <CardContent>
            <div class="space-y-3">
              <div
                  v-for="item in [
                  { label: 'Users', value: stats.userCount, color: 'bg-emerald-500 dark:bg-emerald-400' },
                  { label: 'Documents', value: stats.documentCount, color: 'bg-blue-500 dark:bg-blue-400' },
                  { label: 'Total pages', value: stats.totalPages, color: 'bg-violet-500 dark:bg-violet-400' },
                  { label: 'D4S books', value: stats.d4sBookCount, color: 'bg-amber-500 dark:bg-amber-400' },
                  { label: 'D4S accounts', value: stats.d4sAccountCount, color: 'bg-rose-500 dark:bg-rose-400' },
                ]"
                  :key="item.label"
                  class="space-y-1"
              >
                <div class="flex items-center justify-between text-xs">
                  <span class="text-neutral-600 dark:text-neutral-300">{{ item.label }}</span>
                  <span class="tabular-nums font-medium text-neutral-900 dark:text-neutral-100">{{ formatNumber(item.value) }}</span>
                </div>
                <div class="h-2 w-full overflow-hidden rounded-full bg-neutral-100 dark:bg-neutral-800">
                  <div
                      class="h-full rounded-full transition-all duration-700"
                      :class="item.color"
                      :style="{
                      width: Math.max(...[stats.userCount, stats.documentCount, stats.totalPages, stats.d4sBookCount, stats.d4sAccountCount]) > 0
                        ? `${(item.value / Math.max(stats.userCount, stats.documentCount, stats.totalPages, stats.d4sBookCount, stats.d4sAccountCount)) * 100}%`
                        : '0%'
                    }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- D4S breakdown + storage card -->
        <div class="space-y-3">
          <!-- D4S ring chart -->
          <Card class="border border-neutral-200 dark:border-neutral-800">
            <CardHeader class="pb-2">
              <CardTitle class="text-sm">D4S integration</CardTitle>
              <CardDescription class="text-[11px]">Books and account coverage.</CardDescription>
            </CardHeader>
            <CardContent>
              <div class="flex items-center gap-6">
                <!-- SVG donut -->
                <div class="shrink-0">
                  <svg width="80" height="80" viewBox="0 0 80 80">
                    <circle
                        cx="40" cy="40" r="28"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="12"
                        class="text-neutral-100 dark:text-neutral-800"
                    />
                    <circle
                        cx="40" cy="40" r="28"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="12"
                        stroke-dasharray="175.93"
                        :stroke-dashoffset="175.93 * (1 - d4sCoverage / 100)"
                        stroke-linecap="round"
                        transform="rotate(-90 40 40)"
                        class="text-emerald-500 dark:text-emerald-400 transition-all duration-700"
                    />
                    <text x="40" y="44" text-anchor="middle" font-size="14" font-weight="500" fill="currentColor" class="text-neutral-900 dark:text-neutral-100">
                      {{ d4sCoverage }}%
                    </text>
                  </svg>
                </div>
                <div class="flex-1 space-y-3">
                  <div>
                    <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">D4S accounts</p>
                    <p class="text-xl font-semibold tabular-nums">{{ formatNumber(stats.d4sAccountCount) }}</p>
                    <p class="text-[11px] text-neutral-400 dark:text-neutral-500">of {{ formatNumber(stats.userCount) }} total users</p>
                  </div>
                  <div>
                    <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">D4S books</p>
                    <p class="text-xl font-semibold tabular-nums">{{ formatNumber(stats.d4sBookCount) }}</p>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <!-- Storage card -->
          <Card class="border border-neutral-200 dark:border-neutral-800">
            <CardHeader class="pb-2">
              <CardTitle class="text-sm">Storage breakdown</CardTitle>
              <CardDescription class="text-[11px]">Total vs. per-document averages.</CardDescription>
            </CardHeader>
            <CardContent>
              <div class="grid grid-cols-3 gap-3">
                <div class="rounded-xl bg-neutral-50 p-3 dark:bg-neutral-800/60">
                  <p class="text-[10px] uppercase tracking-[0.14em] text-neutral-500 dark:text-neutral-400">Total</p>
                  <p class="mt-0.5 text-base font-semibold tabular-nums">{{ formatBytes(stats.totalDocSize) }}</p>
                </div>
                <div class="rounded-xl bg-neutral-50 p-3 dark:bg-neutral-800/60">
                  <p class="text-[10px] uppercase tracking-[0.14em] text-neutral-500 dark:text-neutral-400">Avg / doc</p>
                  <p class="mt-0.5 text-base font-semibold tabular-nums">{{ avgDocSize }}</p>
                </div>
                <div class="rounded-xl bg-neutral-50 p-3 dark:bg-neutral-800/60">
                  <p class="text-[10px] uppercase tracking-[0.14em] text-neutral-500 dark:text-neutral-400">Avg pages</p>
                  <p class="mt-0.5 text-base font-semibold tabular-nums">{{ formatNumber(avgPagesPerDoc) }}</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </template>
  </div>
</template>