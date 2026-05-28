<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue"
import { Loader2, MoreVertical, RefreshCcw, LibraryBig, Trash2 } from "lucide-vue-next"
import { apiFetch } from "@/auth/api"

import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

import { deleteD4SBook, listD4SBooks, takeD4SBook, type Digi4SchoolBook } from "@/lib/d4s_api"

type Notice = { type: "success" | "error"; message: string } | null

// Shared notice (simple in-page toast replace)
const notice = ref<Notice>(null)
let noticeTimer: number | undefined
function showNotice(n: Notice) {
  notice.value = n
  if (noticeTimer) window.clearTimeout(noticeTimer)
  if (n) noticeTimer = window.setTimeout(() => (notice.value = null), 3500)
}

// Library state
const isLoadingBooks = ref(false)
const books = ref<Digi4SchoolBook[]>([])
const bookSearch = ref("")
const takingBookIds = ref(new Set<number>())
const deletingBookIds = ref(new Set<number>())
const deleteDialogOpen = ref(false)
const deleteTarget = ref<Digi4SchoolBook | null>(null)
const deleteError = ref<string | null>(null)
const bookThumbnails = ref<Record<number, string>>({})

const filteredBooks = computed(() => {
  const q = bookSearch.value.trim().toLowerCase()
  if (!q) return books.value
  return books.value.filter((b) => (b.bookName ?? "").toLowerCase().includes(q))
})

function revokeThumbnailUrls() {
  for (const url of Object.values(bookThumbnails.value)) {
    URL.revokeObjectURL(url)
  }
}

async function fetchFirstThumbnail(id: number): Promise<string | null> {
  const res = await apiFetch(`/api/v1/d4s/thumbnail/${id}`)
  if (!res.ok) return null
  const blob = await res.blob()
  return URL.createObjectURL(blob)
}

async function loadBookThumbnails(nextBooks: Digi4SchoolBook[]) {
  const nextThumbnails: Record<number, string> = {}
  const workers = Math.min(6, nextBooks.length)
  let cursor = 0

  async function worker() {
    while (cursor < nextBooks.length) {
      const i = cursor++
      const book = nextBooks[i]
      if (!book) continue
      const url = await fetchFirstThumbnail(book.id).catch(() => null)
      if (url) nextThumbnails[book.id] = url
    }
  }

  await Promise.all(Array.from({ length: workers }, () => worker()))
  revokeThumbnailUrls()
  bookThumbnails.value = nextThumbnails
}

async function loadBooks() {
  isLoadingBooks.value = true
  try {
    const nextBooks = await listD4SBooks()
    books.value = nextBooks
    await loadBookThumbnails(nextBooks)
  } catch (e: any) {
    showNotice({ type: "error", message: e?.message ?? "Failed to load books" })
  } finally {
    isLoadingBooks.value = false
  }
}

async function onTakeBook(id: number) {
  takingBookIds.value.add(id)
  try {
    await takeD4SBook(id)
    showNotice({ type: "success", message: "Book added to your library." })
  } catch (e: any) {
    showNotice({ type: "error", message: e?.message ?? "Failed to take book" })
  } finally {
    takingBookIds.value.delete(id)
  }
}

function promptDeleteBook(book: Digi4SchoolBook) {
  deleteTarget.value = book
  deleteError.value = null
  deleteDialogOpen.value = true
}

function onDeleteDialogChange(open: boolean) {
  deleteDialogOpen.value = open
  if (!open) {
    deleteTarget.value = null
    deleteError.value = null
  }
}

async function confirmDeleteBook() {
  const book = deleteTarget.value
  if (!book) return
  deletingBookIds.value.add(book.id)
  deleteError.value = null
  try {
    await deleteD4SBook(book.id)
    books.value = books.value.filter((item) => item.id !== book.id)
    if (bookThumbnails.value[book.id]) {
      URL.revokeObjectURL(bookThumbnails.value[book.id])
      const next = { ...bookThumbnails.value }
      delete next[book.id]
      bookThumbnails.value = next
    }
    showNotice({ type: "success", message: "Book deleted." })
    deleteDialogOpen.value = false
    deleteTarget.value = null
  } catch (e: any) {
    deleteError.value = e?.message ?? "Failed to delete book"
  } finally {
    deletingBookIds.value.delete(book.id)
  }
}

onMounted(async () => {
  await loadBooks()
})

onBeforeUnmount(() => {
  revokeThumbnailUrls()
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
              <LibraryBig class="h-5 w-5" />
            </div>
            <div>
              <h1 class="text-lg font-semibold tracking-tight">Digi4School</h1>
              <p class="text-xs text-neutral-500 dark:text-neutral-400">
                Your synced Digi4School library.
              </p>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <Button
              variant="outline"
              class="rounded-full"
              :disabled="isLoadingBooks"
              @click="loadBooks"
            >
              <RefreshCcw class="h-4 w-4" />
              Refresh
            </Button>
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

    <!-- Library -->
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="w-full sm:w-80">
          <Input v-model="bookSearch" placeholder="Search books by title…" class="rounded-full" />
        </div>
        <div class="text-xs text-neutral-500 dark:text-neutral-400">
          <span v-if="!isLoadingBooks">
            {{ filteredBooks.length }} book<span v-if="filteredBooks.length !== 1">s</span>
          </span>
          <span v-else>Loading…</span>
        </div>
      </div>

      <div v-if="isLoadingBooks" class="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <Card v-for="i in 8" :key="i" class="overflow-hidden">
          <div class="aspect-[3/4] bg-neutral-200/60 dark:bg-neutral-800/60" />
          <CardContent class="p-3">
            <div class="h-4 w-3/4 bg-neutral-200/70 rounded dark:bg-neutral-800/70" />
            <div class="mt-2 h-3 w-1/2 bg-neutral-200/60 rounded dark:bg-neutral-800/60" />
          </CardContent>
        </Card>
      </div>

      <div v-else class="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <Card
            v-for="book in filteredBooks"
            :key="book.id"
            class="overflow-hidden border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 shadow-sm gap-0 pt-0"
        >
          <div class="relative aspect-[3/4] mt-0">
            <img
              v-if="bookThumbnails[book.id]"
              :src="bookThumbnails[book.id]"
              :alt="book.bookName"
              class="h-full w-full object-cover"
              loading="lazy"
            />
            <div
              v-else
              class="h-full w-full bg-gradient-to-br from-neutral-900 via-neutral-800 to-emerald-900/60 text-white"
            >
              <div class="absolute inset-0 opacity-20 bg-[radial-gradient(circle_at_30%_30%,#34d399,transparent_55%)]" />
              <div class="absolute bottom-0 left-0 right-0 p-3">
                <div class="text-[11px] uppercase tracking-[0.16em] text-emerald-200/90">Digi4School</div>
              </div>
            </div>

            <div class="absolute top-2 right-2">
              <div class="flex gap-2">
                <Button
                  variant="secondary"
                  size="icon-sm"
                  class="rounded-full bg-black/30 text-white hover:bg-black/40"
                  :disabled="takingBookIds.has(book.id)"
                  @click="onTakeBook(book.id)"
                  :aria-label="`Take ${book.bookName}`"
                >
                  <!-- <MoreVertical class="h-4 w-4" /> -->
                </Button>
                <Button
                  variant="secondary"
                  size="icon-sm"
                  class="rounded-full bg-black/30 text-white hover:bg-red-600/90"
                  :disabled="deletingBookIds.has(book.id)"
                  @click="promptDeleteBook(book)"
                  :aria-label="`Delete ${book.bookName}`"
                >
                  <Trash2 class="h-4 w-4" />
                </Button>
              </div>
            </div>
          </div>

          <CardContent class="p-3">
            <div class="space-y-1">
              <p class="text-sm font-semibold leading-snug line-clamp-2 min-h-[2.5em]">
                {{ book.bookName }}
              </p>
              <p class="text-[11px] text-neutral-500 dark:text-neutral-400">
                BookID: {{ book.bookId }}
              </p>
            </div>

            <Separator class="my-3" />

            <Button
              class="w-full rounded-xl bg-emerald-700 text-white hover:bg-emerald-700/90"
              :disabled="takingBookIds.has(book.id)"
              @click="onTakeBook(book.id)"
            >
              <span v-if="!takingBookIds.has(book.id)">Take Book</span>
              <span v-else>Working…</span>
            </Button>
          </CardContent>
        </Card>

        <div
          v-if="!filteredBooks.length"
          class="col-span-full rounded-2xl border border-dashed border-neutral-300 bg-neutral-50 p-8 text-center text-sm text-neutral-600 dark:border-neutral-700 dark:bg-neutral-900/40 dark:text-neutral-300"
        >
          No books found. Ask an admin to run a sync from the Admin page.
        </div>
      </div>
    </div>

    <Dialog :open="deleteDialogOpen" @update:open="onDeleteDialogChange" modal>
      <DialogContent class="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>Delete Digi4School book</DialogTitle>
          <DialogDescription>
            This removes the synced book from PaperLink. Documents already created from it stay available.
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-3 text-sm text-neutral-600 dark:text-neutral-300">
          <p>
            Delete <strong>{{ deleteTarget?.bookName }}</strong>?
          </p>
          <p
            v-if="deleteError"
            class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-200"
          >
            {{ deleteError }}
          </p>
        </div>
        <DialogFooter class="mt-6">
          <DialogClose as-child>
            <Button variant="outline" :disabled="deleteTarget ? deletingBookIds.has(deleteTarget.id) : false">Cancel</Button>
          </DialogClose>
          <Button
            variant="destructive"
            :disabled="!deleteTarget || deletingBookIds.has(deleteTarget.id)"
            @click="confirmDeleteBook"
          >
            <Loader2 v-if="deleteTarget && deletingBookIds.has(deleteTarget.id)" class="mr-2 h-4 w-4 animate-spin" />
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
