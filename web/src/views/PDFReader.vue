<template>
  <div class="h-[calc(100vh-2rem)] w-full overflow-hidden">
    <section class="flex h-full min-h-0 flex-row flex-nowrap gap-4">

      <!-- LEFT SIDEBAR -->
      <div class="flex h-full min-h-0 w-60 shrink-0 flex-col gap-6 rounded-2xl border border-neutral-200 bg-white
                  p-4 dark:border-neutral-800 dark:bg-neutral-900">

        <!-- Navigation -->
        <div class="space-y-3">
          <div class="text-xs font-semibold text-neutral-500">Navigation</div>

          <div class="flex items-center justify-between">
            <Button size="icon" variant="outline" @click="goFirst">
              <ChevronsLeft class="h-4 w-4"/>
            </Button>

            <Button size="icon" variant="outline" @click="prevPage">
              <ChevronLeft class="h-4 w-4"/>
            </Button>

            <div class="text-sm font-medium text-center px-2">
              {{ currentPage }} / {{ pageCount }}
            </div>

            <Button size="icon" variant="outline" @click="nextPage">
              <ChevronRight class="h-4 w-4"/>
            </Button>

            <Button size="icon" variant="outline" @click="goLast">
              <ChevronsRight class="h-4 w-4"/>
            </Button>
          </div>

        </div>

        <!-- Thumbnails -->
        <div class="flex min-h-0 flex-1 flex-col space-y-2">
          <div class="text-xs font-semibold text-neutral-500">Pages</div>
          <div
            ref="thumbnailScrollEl"
            class="min-h-0 flex-1 overflow-y-auto pr-1 space-y-2"
            @scroll="onThumbnailScroll"
          >
            <button
              v-for="page in pageCount"
              :key="page"
              class="w-full rounded-lg border p-1 text-left transition-colors"
              :class="currentPage === page
                ? 'border-neutral-900 bg-neutral-100 dark:border-neutral-100 dark:bg-neutral-800'
                : 'border-neutral-200 hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800/70'"
              @click="go(page)"
            >
              <img
                v-if="thumbnails[page - 1]"
                :src="thumbnails[page - 1]"
                :alt="`Page ${page}`"
                class="w-full rounded object-contain"
                loading="lazy"
              >
              <div
                v-else
                class="h-24 w-full rounded bg-neutral-100 dark:bg-neutral-800"
              />
              <div class="mt-1 text-[11px] text-neutral-500">Page {{ page }}</div>
            </button>
          </div>
        </div>

      </div>

      <!-- PDF VIEWER -->
      <div class="flex h-full min-h-0 flex-1 justify-center rounded-2xl border border-neutral-200 bg-white
            dark:border-neutral-800 dark:bg-neutral-900
            overflow-auto">

        <div
          v-if="readerError"
          class="m-4 w-full max-w-2xl rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300"
        >
          {{ readerError }}
        </div>

        <!-- padded wrapper for canvas -->
        <div v-else class="flex justify-center h-full p-4">
          <canvas ref="canvasEl" class="block"></canvas>
        </div>

      </div>


      <!-- RIGHT SIDEBAR PLACEHOLDER -->
      <div class="h-full w-60 shrink-0 rounded-2xl border border-neutral-200 bg-white
                  dark:border-neutral-800 dark:bg-neutral-900 p-4">
        <!-- empty for now -->
      </div>

    </section>
  </div>
</template>

<script setup lang="ts">
import { usePdfReader } from '@/composables/usePdfReader'
import { Button } from "@/components/ui/button"
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from "lucide-vue-next"

const {
  pageCount,
  currentPage,
  canvasEl,
  thumbnailScrollEl,
  thumbnails,
  readerError,
  onThumbnailScroll,
  go,
  goFirst,
  goLast,
  prevPage,
  nextPage,
} = usePdfReader()
</script>
