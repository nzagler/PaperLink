<script setup lang="ts">
import { onMounted, ref, computed } from "vue"
import { BarChart3, RefreshCcw, Users, FileText, HardDrive, FileStack } from "lucide-vue-next"

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
const animatedValues = ref<Record<string, number>>({})

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function animateValue(key: string, target: number, duration = 900) {
  const start = Date.now()
  const from = animatedValues.value[key] ?? 0
  const step = () => {
    const elapsed = Date.now() - start
    const progress = Math.min(elapsed / duration, 1)
    const ease = 1 - Math.pow(1 - progress, 3)
    animatedValues.value[key] = Math.round(from + (target - from) * ease)
    if (progress < 1) requestAnimationFrame(step)
  }
  requestAnimationFrame(step)
}

async function load() {
  loading.value = true
  try {
    stats.value = await getAdminStats()
    if (stats.value) {
      animateValue("userCount", stats.value.userCount)
      animateValue("documentCount", stats.value.documentCount)
      animateValue("totalPages", stats.value.totalPages)
      animateValue("d4sBookCount", stats.value.d4sBookCount)
      animateValue("d4sAccountCount", stats.value.d4sAccountCount)
    }
  } catch (e: any) {
    showNotice({ type: "error", message: e?.message ?? "Failed to load statistics" })
  } finally {
    loading.value = false
  }
}

onMounted(async () => { await load() })

// Derived insight stats shown beneath each KPI
const derived = computed(() => {
  if (!stats.value) return null
  const s = stats.value
  return {
    docsPerUser: s.userCount > 0 ? (s.documentCount / s.userCount).toFixed(1) : "—",
    avgDocSize: s.documentCount > 0 ? formatBytes(s.totalDocSize / s.documentCount) : "—",
    avgPagesPerDoc: s.documentCount > 0 ? (s.totalPages / s.documentCount).toFixed(1) : "—",
    avgDocSizeRaw: s.documentCount > 0 ? s.totalDocSize / s.documentCount : 0,
  }
})

// Content density gauge: avg pages/doc on a 0–100 page scale (capped)
const densityGauge = computed(() => {
  if (!derived.value) return { pct: 0, label: "—" }
  const avg = parseFloat(derived.value.avgPagesPerDoc)
  const MAX = 100
  const pct = Math.min((avg / MAX) * 100, 100)
  return { pct, label: derived.value.avgPagesPerDoc }
})

// Storage arc gauge
const storageTier = computed(() => {
  if (!stats.value) return { label: "Empty", color: "#059669", pct: 0 }
  const gb = stats.value.totalDocSize / 1073741824
  if (gb < 1)  return { label: "Low",      color: "#059669", pct: Math.min(gb * 40, 40) }
  if (gb < 10) return { label: "Moderate", color: "#f59e0b", pct: 40 + ((gb - 1) / 9) * 35 }
  return              { label: "High",     color: "#ef4444", pct: 75 + Math.min(((gb - 10) / 90) * 25, 25) }
})
</script>

<template>
  <div class="mx-auto max-w-6xl px-4 lg:px-6 py-5 lg:py-7 space-y-4">
    <!-- Header -->
    <section class="rounded-2xl border border-neutral-200 bg-white shadow-sm shadow-neutral-200/70 overflow-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:shadow-none">
      <div class="px-4 sm:px-6 py-4 bg-gradient-to-r from-neutral-50 via-white to-emerald-50/70 dark:from-neutral-900 dark:via-neutral-900 dark:to-emerald-900/30">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-3">
            <div class="inline-flex h-10 w-10 items-center justify-center rounded-2xl bg-emerald-600/10 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200">
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
            :class="notice.type === 'success'
            ? 'border-emerald-600/30 bg-emerald-600/10 text-emerald-900 dark:text-emerald-200 dark:bg-emerald-500/10'
            : 'border-red-600/30 bg-red-600/10 text-red-900 dark:text-red-200 dark:bg-red-500/10'"
        >
          {{ notice.message }}
        </div>
      </div>
    </section>

    <div v-if="loading" class="text-sm text-neutral-600 dark:text-neutral-300 px-1">Loading…</div>

    <div
        v-else-if="!stats"
        class="rounded-xl border border-dashed border-neutral-300 bg-neutral-50 p-4 text-sm text-neutral-600 dark:border-neutral-700 dark:bg-neutral-900/40 dark:text-neutral-300"
    >
      No statistics available.
    </div>

    <template v-else>
      <!-- KPI row — clean numbers with derived insights -->
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">

        <!-- Users -->
        <Card class="border border-neutral-200 dark:border-neutral-800">
          <CardContent class="p-4">
            <div class="flex items-start justify-between">
              <div>
                <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">Users</p>
                <p class="mt-1 text-3xl font-semibold tabular-nums">{{ animatedValues.userCount ?? stats.userCount }}</p>
              </div>
              <div class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-600/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300 shrink-0">
                <Users class="h-4 w-4" />
              </div>
            </div>
            <div class="mt-4 rounded-lg bg-neutral-50 dark:bg-neutral-800/50 px-3 py-2 flex items-center justify-between">
              <span class="text-[11px] text-neutral-500 dark:text-neutral-400">Avg. docs per user</span>
              <span class="text-[11px] font-semibold text-neutral-800 dark:text-neutral-100 tabular-nums">
                {{ derived?.docsPerUser }}
              </span>
            </div>
          </CardContent>
        </Card>

        <!-- Documents -->
        <Card class="border border-neutral-200 dark:border-neutral-800">
          <CardContent class="p-4">
            <div class="flex items-start justify-between">
              <div>
                <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">Documents</p>
                <p class="mt-1 text-3xl font-semibold tabular-nums">{{ animatedValues.documentCount ?? stats.documentCount }}</p>
              </div>
              <div class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-blue-600/10 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300 shrink-0">
                <FileText class="h-4 w-4" />
              </div>
            </div>
            <div class="mt-4 rounded-lg bg-neutral-50 dark:bg-neutral-800/50 px-3 py-2 flex items-center justify-between">
              <span class="text-[11px] text-neutral-500 dark:text-neutral-400">Avg. size per document</span>
              <span class="text-[11px] font-semibold text-neutral-800 dark:text-neutral-100 tabular-nums">
                {{ derived?.avgDocSize }}
              </span>
            </div>
          </CardContent>
        </Card>

        <!-- Total Pages -->
        <Card class="border border-neutral-200 dark:border-neutral-800">
          <CardContent class="p-4">
            <div class="flex items-start justify-between">
              <div>
                <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">Total pages</p>
                <p class="mt-1 text-3xl font-semibold tabular-nums">{{ animatedValues.totalPages ?? stats.totalPages }}</p>
              </div>
              <div class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-violet-600/10 text-violet-700 dark:bg-violet-500/15 dark:text-violet-300 shrink-0">
                <FileStack class="h-4 w-4" />
              </div>
            </div>
            <div class="mt-4 rounded-lg bg-neutral-50 dark:bg-neutral-800/50 px-3 py-2 flex items-center justify-between">
              <span class="text-[11px] text-neutral-500 dark:text-neutral-400">Avg. pages per document</span>
              <span class="text-[11px] font-semibold text-neutral-800 dark:text-neutral-100 tabular-nums">
                {{ derived?.avgPagesPerDoc }}
              </span>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Second row -->
      <div class="grid gap-3 lg:grid-cols-3">

        <!-- Storage arc gauge -->
        <Card class="border border-neutral-200 dark:border-neutral-800">
          <CardHeader class="pb-2">
            <div class="flex items-center gap-2">
              <div class="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-amber-600/10 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">
                <HardDrive class="h-3.5 w-3.5" />
              </div>
              <CardTitle class="text-sm">Storage used</CardTitle>
            </div>
            <CardDescription class="text-[11px]">Total document storage</CardDescription>
          </CardHeader>
          <CardContent class="pt-0">
            <div class="flex flex-col items-center py-2">
              <svg viewBox="0 0 100 60" class="w-40" aria-hidden="true">
                <path d="M 10 55 A 40 40 0 0 1 90 55" fill="none" stroke="currentColor" stroke-width="8" stroke-linecap="round" class="text-neutral-200 dark:text-neutral-700" />
                <path
                    d="M 10 55 A 40 40 0 0 1 90 55"
                    fill="none"
                    :stroke="storageTier.color"
                    stroke-width="8"
                    stroke-linecap="round"
                    stroke-dasharray="125.66"
                    :stroke-dashoffset="125.66 - (storageTier.pct / 100) * 125.66"
                    style="transition: stroke-dashoffset 1s cubic-bezier(0.34,1.56,0.64,1)"
                />
              </svg>
              <p class="text-2xl font-semibold -mt-2 tabular-nums">{{ formatBytes(stats.totalDocSize) }}</p>
              <span
                  class="mt-1 inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium"
                  :style="{ backgroundColor: storageTier.color + '22', color: storageTier.color }"
              >
                {{ storageTier.label }} usage
              </span>
            </div>
            <!-- avg per doc — useful context for the gauge -->
            <div class="mt-2 rounded-lg bg-neutral-50 dark:bg-neutral-800/50 px-3 py-2 flex items-center justify-between">
              <span class="text-[11px] text-neutral-500 dark:text-neutral-400">Avg. per document</span>
              <span class="text-[11px] font-semibold text-neutral-800 dark:text-neutral-100 tabular-nums">
                {{ derived?.avgDocSize }}
              </span>
            </div>
          </CardContent>
        </Card>

        <!-- Content density gauge (replaces misleading docs vs pages bars) -->
        <Card class="border border-neutral-200 dark:border-neutral-800">
          <CardHeader class="pb-2">
            <div class="flex items-center gap-2">
              <div class="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-violet-600/10 text-violet-700 dark:bg-violet-500/15 dark:text-violet-300">
                <BarChart3 class="h-3.5 w-3.5" />
              </div>
              <CardTitle class="text-sm">Content density</CardTitle>
            </div>
            <CardDescription class="text-[11px]">Average pages per document</CardDescription>
          </CardHeader>
          <CardContent class="pt-2">
            <!-- Big number -->
            <div class="flex items-baseline gap-1.5 mb-4">
              <span class="text-4xl font-semibold tabular-nums text-neutral-900 dark:text-neutral-50">
                {{ derived?.avgPagesPerDoc }}
              </span>
              <span class="text-sm text-neutral-500 dark:text-neutral-400">pages / doc</span>
            </div>

            <!-- Gauge bar scaled to 100 pages -->
            <div>
              <div class="h-2.5 rounded-full bg-neutral-100 dark:bg-neutral-800 overflow-hidden">
                <div
                    class="h-full rounded-full bg-violet-500 transition-all duration-[1100ms] ease-[cubic-bezier(0.34,1.56,0.64,1)]"
                    :style="{ width: densityGauge.pct + '%' }"
                />
              </div>
              <div class="flex justify-between mt-1">
                <span class="text-[10px] text-neutral-400">0</span>
                <span class="text-[10px] text-neutral-400">50</span>
                <span class="text-[10px] text-neutral-400">100 pages</span>
              </div>
            </div>

            <!-- Supporting totals -->
            <div class="mt-4 grid grid-cols-2 gap-2">
              <div class="rounded-lg bg-neutral-50 dark:bg-neutral-800/50 px-3 py-2">
                <p class="text-[10px] text-neutral-500 dark:text-neutral-400 uppercase tracking-[0.12em]">Documents</p>
                <p class="text-sm font-semibold tabular-nums">{{ stats.documentCount.toLocaleString() }}</p>
              </div>
              <div class="rounded-lg bg-neutral-50 dark:bg-neutral-800/50 px-3 py-2">
                <p class="text-[10px] text-neutral-500 dark:text-neutral-400 uppercase tracking-[0.12em]">Total pages</p>
                <p class="text-sm font-semibold tabular-nums">{{ stats.totalPages.toLocaleString() }}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <CardContent class="pt-2 space-y-4">
          <!-- The insight that actually matters -->
          <div>
            <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">Books per account</p>
            <p class="mt-1 text-3xl font-semibold tabular-nums">
              {{ stats.d4sAccountCount > 0 ? (stats.d4sBookCount / stats.d4sAccountCount).toFixed(1) : '—' }}
            </p>
          </div>

          <Separator />

          <!-- Raw counts as supporting context -->
          <div class="grid grid-cols-2 gap-2">
            <div class="rounded-lg bg-neutral-50 dark:bg-neutral-800/50 px-3 py-2">
              <p class="text-[10px] uppercase tracking-[0.12em] text-neutral-500 dark:text-neutral-400">Books</p>
              <p class="text-sm font-semibold tabular-nums">{{ animatedValues.d4sBookCount ?? stats.d4sBookCount }}</p>
            </div>
            <div class="rounded-lg bg-neutral-50 dark:bg-neutral-800/50 px-3 py-2">
              <p class="text-[10px] uppercase tracking-[0.12em] text-neutral-500 dark:text-neutral-400">Accounts</p>
              <p class="text-sm font-semibold tabular-nums">{{ animatedValues.d4sAccountCount ?? stats.d4sAccountCount }}</p>
            </div>
          </div>
        </CardContent>

      </div>
    </template>
  </div>
</template>