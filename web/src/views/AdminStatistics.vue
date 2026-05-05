<script setup lang="ts">
import { onMounted, ref, computed } from "vue"
import { BarChart3, RefreshCcw, Users, FileText, HardDrive, BookOpen, FileStack } from "lucide-vue-next"

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
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
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

onMounted(async () => {
  await load()
})

// --- Donut chart for D4S breakdown ---
const donutSegments = computed(() => {
  if (!stats.value) return []
  const books = stats.value.d4sBookCount
  const accs = stats.value.d4sAccountCount
  const total = books + accs
  if (total === 0) return []
  const r = 36
  const circumference = 2 * Math.PI * r

  const booksDash = (books / total) * circumference
  const accsDash = (accs / total) * circumference

  return [
    { dash: booksDash, offset: 0, color: "#059669", label: "Books", value: books },
    { dash: accsDash, offset: -booksDash, color: "#6ee7b7", label: "Accounts", value: accs },
  ]
})

// --- Bar chart for docs vs pages ratio ---
const barData = computed(() => {
  if (!stats.value) return null
  const docs = stats.value.documentCount
  const pages = stats.value.totalPages
  const pagesPerDoc = docs > 0 ? (pages / docs).toFixed(1) : "0"
  const maxBar = Math.max(docs, pages, 1)
  return {
    docs,
    pages,
    pagesPerDoc,
    docsWidth: (docs / maxBar) * 100,
    pagesWidth: (pages / maxBar) * 100,
  }
})

// --- Storage gauge ---
const storageTier = computed(() => {
  if (!stats.value) return { label: "Empty", color: "#059669", pct: 0 }
  const gb = stats.value.totalDocSize / 1073741824
  if (gb < 1) return { label: "Low", color: "#059669", pct: Math.min(gb * 40, 40) }
  if (gb < 10) return { label: "Moderate", color: "#f59e0b", pct: 40 + ((gb - 1) / 9) * 35 }
  return { label: "High", color: "#ef4444", pct: 75 + Math.min(((gb - 10) / 90) * 25, 25) }
})
</script>

<template>
  <div class="mx-auto max-w-6xl px-4 lg:px-6 py-5 lg:py-7 space-y-4">
    <!-- Header -->
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

    <!-- Loading -->
    <div v-if="loading" class="text-sm text-neutral-600 dark:text-neutral-300 px-1">Loading…</div>

    <!-- No data -->
    <div
        v-else-if="!stats"
        class="rounded-xl border border-dashed border-neutral-300 bg-neutral-50 p-4 text-sm text-neutral-600 dark:border-neutral-700 dark:bg-neutral-900/40 dark:text-neutral-300"
    >
      No statistics available.
    </div>

    <template v-else>
      <!-- Top KPI row -->
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <!-- Users -->
        <Card class="border border-neutral-200 dark:border-neutral-800 overflow-hidden">
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
            <!-- Mini user bar visual -->
            <div class="mt-4 flex gap-1 items-end h-8">
              <div
                  v-for="n in 12"
                  :key="n"
                  class="flex-1 rounded-sm bg-emerald-500/20 dark:bg-emerald-500/15"
                  :style="{ height: `${20 + Math.sin(n * 1.7 + (stats.userCount % 5)) * 12 + (n === 12 ? 12 : 0)}px`, backgroundColor: n === 12 ? 'rgb(16 185 129 / 0.7)' : undefined }"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Documents -->
        <Card class="border border-neutral-200 dark:border-neutral-800 overflow-hidden">
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
            <div class="mt-4 flex gap-1 items-end h-8">
              <div
                  v-for="n in 12"
                  :key="n"
                  class="flex-1 rounded-sm"
                  :style="{
                  height: `${18 + Math.abs(Math.cos(n * 2.1)) * 14 + (n === 12 ? 8 : 0)}px`,
                  backgroundColor: n === 12 ? 'rgb(59 130 246 / 0.7)' : 'rgb(59 130 246 / 0.15)'
                }"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Total Pages -->
        <Card class="border border-neutral-200 dark:border-neutral-800 overflow-hidden">
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
            <div class="mt-4 flex gap-1 items-end h-8">
              <div
                  v-for="n in 12"
                  :key="n"
                  class="flex-1 rounded-sm"
                  :style="{
                  height: `${14 + Math.abs(Math.sin(n * 0.9 + 1)) * 18 + (n === 12 ? 6 : 0)}px`,
                  backgroundColor: n === 12 ? 'rgb(139 92 246 / 0.7)' : 'rgb(139 92 246 / 0.15)'
                }"
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Second row: Storage gauge + Docs/Pages bar + D4S donut -->
      <div class="grid gap-3 lg:grid-cols-3">

        <!-- Storage card with arc gauge -->
        <Card class="border border-neutral-200 dark:border-neutral-800">
          <CardHeader class="pb-2">
            <div class="flex items-center gap-2">
              <div class="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-amber-600/10 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">
                <HardDrive class="h-3.5 w-3.5" />
              </div>
              <CardTitle class="text-sm">Storage Used</CardTitle>
            </div>
            <CardDescription class="text-[11px]">Total document storage</CardDescription>
          </CardHeader>
          <CardContent class="pt-0">
            <div class="flex flex-col items-center py-2">
              <!-- SVG arc gauge -->
              <svg viewBox="0 0 100 60" class="w-40" aria-hidden="true">
                <!-- Background arc -->
                <path
                    d="M 10 55 A 40 40 0 0 1 90 55"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="8"
                    stroke-linecap="round"
                    class="text-neutral-200 dark:text-neutral-700"
                />
                <!-- Value arc -->
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
          </CardContent>
        </Card>

        <!-- Docs vs Pages horizontal bar chart -->
        <Card class="border border-neutral-200 dark:border-neutral-800">
          <CardHeader class="pb-2">
            <div class="flex items-center gap-2">
              <div class="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-blue-600/10 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300">
                <BarChart3 class="h-3.5 w-3.5" />
              </div>
              <CardTitle class="text-sm">Docs &amp; Pages</CardTitle>
            </div>
            <CardDescription class="text-[11px]">Avg. {{ barData?.pagesPerDoc }} pages per document</CardDescription>
          </CardHeader>
          <CardContent class="pt-2 space-y-4">
            <div v-if="barData">
              <!-- Documents bar -->
              <div>
                <div class="flex justify-between mb-1.5">
                  <span class="text-[11px] text-neutral-500 dark:text-neutral-400 uppercase tracking-[0.12em]">Documents</span>
                  <span class="text-[11px] font-semibold tabular-nums">{{ barData.docs }}</span>
                </div>
                <div class="h-2.5 rounded-full bg-neutral-100 dark:bg-neutral-800 overflow-hidden">
                  <div
                      class="h-full rounded-full bg-blue-500"
                      :style="{ width: barData.docsWidth + '%', transition: 'width 1s cubic-bezier(0.34,1.56,0.64,1)' }"
                  />
                </div>
              </div>

              <!-- Pages bar -->
              <div>
                <div class="flex justify-between mb-1.5">
                  <span class="text-[11px] text-neutral-500 dark:text-neutral-400 uppercase tracking-[0.12em]">Pages</span>
                  <span class="text-[11px] font-semibold tabular-nums">{{ barData.pages }}</span>
                </div>
                <div class="h-2.5 rounded-full bg-neutral-100 dark:bg-neutral-800 overflow-hidden">
                  <div
                      class="h-full rounded-full bg-violet-500"
                      :style="{ width: barData.pagesWidth + '%', transition: 'width 1.1s cubic-bezier(0.34,1.56,0.64,1) 0.1s' }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- D4S donut -->
        <Card class="border border-neutral-200 dark:border-neutral-800">
          <CardHeader class="pb-2">
            <div class="flex items-center gap-2">
              <div class="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-emerald-600/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300">
                <BookOpen class="h-3.5 w-3.5" />
              </div>
              <CardTitle class="text-sm">D4S Overview</CardTitle>
            </div>
            <CardDescription class="text-[11px]">Books vs. linked accounts</CardDescription>
          </CardHeader>
          <CardContent class="pt-0">
            <div class="flex items-center gap-4 py-1">
              <!-- Donut SVG -->
              <svg viewBox="0 0 88 88" class="w-24 h-24 shrink-0" aria-hidden="true">
                <circle cx="44" cy="44" r="36" fill="none" stroke="currentColor" stroke-width="12" class="text-neutral-100 dark:text-neutral-800" />
                <template v-if="donutSegments.length">
                  <circle
                      v-for="seg in donutSegments"
                      :key="seg.label"
                      cx="44" cy="44" r="36"
                      fill="none"
                      :stroke="seg.color"
                      stroke-width="12"
                      stroke-linecap="butt"
                      :stroke-dasharray="`${seg.dash} ${2 * Math.PI * 36 - seg.dash}`"
                      :stroke-dashoffset="seg.offset"
                      style="transform: rotate(-90deg); transform-origin: 44px 44px; transition: stroke-dasharray 1s ease"
                  />
                </template>
                <!-- Center text -->
                <text x="44" y="40" text-anchor="middle" class="fill-neutral-800 dark:fill-neutral-100" font-size="11" font-weight="600">
                  {{ stats.d4sBookCount + stats.d4sAccountCount }}
                </text>
                <text x="44" y="52" text-anchor="middle" class="fill-neutral-400" font-size="7">total</text>
              </svg>

              <!-- Legend -->
              <div class="space-y-2.5 text-xs">
                <div class="flex items-center gap-2">
                  <span class="inline-block h-2.5 w-2.5 rounded-sm shrink-0" style="background:#059669" />
                  <span class="text-neutral-600 dark:text-neutral-300">Books</span>
                  <span class="ml-auto font-semibold tabular-nums">{{ animatedValues.d4sBookCount ?? stats.d4sBookCount }}</span>
                </div>
                <div class="flex items-center gap-2">
                  <span class="inline-block h-2.5 w-2.5 rounded-sm shrink-0" style="background:#6ee7b7" />
                  <span class="text-neutral-600 dark:text-neutral-300">Accounts</span>
                  <span class="ml-auto font-semibold tabular-nums">{{ animatedValues.d4sAccountCount ?? stats.d4sAccountCount }}</span>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

      </div>
    </template>
  </div>
</template>