import { computed, onBeforeUnmount, onMounted, reactive, ref, shallowRef, watch, markRaw } from 'vue'
import { useRoute } from 'vue-router'
import * as pdfjsLib from 'pdfjs-dist/build/pdf.mjs'
import pdfWorker from 'pdfjs-dist/build/pdf.worker.mjs?worker'
import { apiFetch } from '@/auth/api'
import { accessToken } from '@/auth/auth'

pdfjsLib.GlobalWorkerOptions.workerPort = new pdfWorker()

export function usePdfReader() {
  const route = useRoute()
  const pdfID = computed(() => String(route.params.id ?? ''))

  const pageCount = ref(0)
  const currentPage = ref(1)
  const canvasEl = ref<HTMLCanvasElement | null>(null)
  const thumbnailScrollEl = ref<HTMLDivElement | null>(null)
  const thumbnails = ref<(string | null)[]>([])
  const THUMB_BATCH_SIZE = 50
  const thumbnailBatchLocks = reactive<Record<string, boolean>>({})
  const thumbnailBatchCache = reactive<Record<string, boolean>>({})
  const readerError = ref<string | null>(null)

  let keydownHandler: ((e: KeyboardEvent) => void) | null = null
  let currentRenderTask: { cancel: () => void; promise: Promise<void> } | null = null
  const pdfDocument = shallowRef<pdfjsLib.PDFDocumentProxy | null>(null)
  let initToken = 0
  let renderToken = 0

  function clearThumbnailBatchState() {
    for (const k of Object.keys(thumbnailBatchLocks)) delete thumbnailBatchLocks[k]
    for (const k of Object.keys(thumbnailBatchCache)) delete thumbnailBatchCache[k]
  }

  function clearThumbnails() {
    thumbnails.value.forEach((url) => {
      if (url) URL.revokeObjectURL(url)
    })
    thumbnails.value = []
  }

  function disposeCurrentPdf() {
    renderToken++
    if (currentRenderTask) {
      try {
        currentRenderTask.cancel()
      } catch {
      }
      currentRenderTask = null
    }
    if (pdfDocument.value) {
      void pdfDocument.value.destroy()
      pdfDocument.value = null
    }
    clearThumbnails()
    clearThumbnailBatchState()
  }

  async function loadDocument() {
    const res = await apiFetch(`/api/v1/document/get/${pdfID.value}`)
    if (!res.ok) {
      throw new Error('Failed to load document metadata')
    }
    const doc = await res.json().catch(() => null)
    pageCount.value = Number(doc?.file?.pages ?? 0)
    thumbnails.value = Array.from({ length: pageCount.value }, () => null)
  }

  async function loadPDFDocument() {
    const headers: Record<string, string> = {}
    if (accessToken.value) {
      headers.Authorization = `Bearer ${accessToken.value}`
    }

    const task = pdfjsLib.getDocument({
      url: `/api/v1/pdf/${pdfID.value}`,
      httpHeaders: headers,
      withCredentials: true,
      rangeChunkSize: 512 * 1024,
      disableAutoFetch: true,
      disableStream: true,
    })

    pdfDocument.value = markRaw(await task.promise)

    if (pdfDocument.value && pageCount.value !== pdfDocument.value.numPages) {
      pageCount.value = pdfDocument.value.numPages
      thumbnails.value = Array.from({ length: pageCount.value }, () => null)
    }
  }

  async function fetchThumbnailBatch(startIndex: number) {
    const localPdfID = pdfID.value
    if (pageCount.value === 0 || startIndex >= pageCount.value) return
    const start = Math.max(0, startIndex)
    const end = Math.min(pageCount.value - 1, start + THUMB_BATCH_SIZE - 1)
    const key = `${start}-${end}`
    if (thumbnailBatchCache[key] || thumbnailBatchLocks[key]) return
    thumbnailBatchLocks[key] = true

    try {
      const res = await apiFetch(`/api/v1/pdf/thumbnails/${localPdfID}/${start}-${end}`)
      if (!res.ok) return

      const buf = await res.arrayBuffer()
      const bytes = new Uint8Array(buf)
      const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)

      let offset = 0
      let pageIndex = start

      while (offset + 8 <= bytes.length && pageIndex <= end) {
        const size = Number(dv.getBigUint64(offset, true))
        offset += 8
        if (size <= 0 || offset + size > bytes.length) break

        const pngBytes = bytes.slice(offset, offset + size)
        offset += size

        if (localPdfID !== pdfID.value) break
        const url = URL.createObjectURL(new Blob([pngBytes], { type: 'image/png' }))
        if (thumbnails.value[pageIndex]) {
          URL.revokeObjectURL(thumbnails.value[pageIndex]!)
        }
        thumbnails.value[pageIndex] = url
        pageIndex++
      }

      thumbnailBatchCache[key] = true
    } finally {
      thumbnailBatchLocks[key] = false
    }
  }

  function ensureThumbnailBatchForPage(page: number) {
    const idx = Math.max(0, page - 1)
    const batchStart = Math.floor(idx / THUMB_BATCH_SIZE) * THUMB_BATCH_SIZE
    void fetchThumbnailBatch(batchStart)
  }

  function ensureThumbnailBatchesForViewport() {
    const el = thumbnailScrollEl.value
    if (!el) return

    const itemHeight = 128
    const firstVisiblePage = Math.max(1, Math.floor(el.scrollTop / itemHeight) + 1)
    const lastVisiblePage = Math.min(
      pageCount.value,
      Math.ceil((el.scrollTop + el.clientHeight) / itemHeight),
    )

    ensureThumbnailBatchForPage(firstVisiblePage)
    ensureThumbnailBatchForPage(lastVisiblePage)
  }

  function onThumbnailScroll() {
    ensureThumbnailBatchesForViewport()
  }

  function ensureSurrounding(n: number) {
    const pdf = pdfDocument.value
    if (!pdf) return

    const preloadBefore = Math.max(1, n - 1)
    const preloadAfter = Math.min(pageCount.value, n + 1)
    for (let i = preloadBefore; i <= preloadAfter; i++) {
      void pdf.getPage(i).catch(() => {
      })
    }
  }

  async function renderPage(n: number) {
    const token = ++renderToken
    const pdf = pdfDocument.value
    if (!pdf) return
    const canvas = canvasEl.value
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    if (token !== renderToken) return

    if (currentRenderTask) {
      try {
        currentRenderTask.cancel()
      } catch {
      }
      currentRenderTask = null
    }

    const page = await pdf.getPage(n)
    if (token !== renderToken) return
    const viewport = page.getViewport({ scale: 1.5 })

    canvas.width = viewport.width
    canvas.height = viewport.height

    const renderTask = page.render({ canvasContext: ctx, viewport })
    currentRenderTask = renderTask as { cancel: () => void; promise: Promise<void> }
    await renderTask.promise
    if (token !== renderToken) return

    ensureSurrounding(n)
  }

  function go(n: number) {
    if (pageCount.value === 0) return
    n = Math.min(pageCount.value, Math.max(1, n))
    currentPage.value = n
    ensureThumbnailBatchForPage(n)
    void renderPage(n)
  }

  const goFirst = () => go(1)
  const goLast = () => go(pageCount.value)
  const prevPage = () => go(Math.max(currentPage.value - 1, 1))
  const nextPage = () => go(Math.min(currentPage.value + 1, pageCount.value))

  async function initializeReader() {
    const token = ++initToken
    readerError.value = null
    currentPage.value = 1
    pageCount.value = 0
    disposeCurrentPdf()

    try {
      await loadDocument()
      await loadPDFDocument()
      if (token !== initToken) return
      if (pageCount.value === 0) {
        readerError.value = 'This document has no pages.'
        return
      }
      await fetchThumbnailBatch(0)
      ensureThumbnailBatchesForViewport()
      await renderPage(1)
    } catch (err) {
      if (token !== initToken) return
      console.error('Failed to initialize PDF reader', err)
      readerError.value = 'Failed to load this PDF.'
    }
  }

  onMounted(async () => {
    await initializeReader()

    keydownHandler = (e: KeyboardEvent) => {
      if (e.key === 'ArrowRight' || e.key === 'PageDown') nextPage()
      if (e.key === 'ArrowLeft' || e.key === 'PageUp') prevPage()
    }
    window.addEventListener('keydown', keydownHandler)
  })

  watch(
    () => route.params.id,
    async (next, prev) => {
      if (String(next ?? '') === String(prev ?? '')) return
      await initializeReader()
    },
  )

  onBeforeUnmount(() => {
    initToken++
    if (keydownHandler) {
      window.removeEventListener('keydown', keydownHandler)
      keydownHandler = null
    }
    disposeCurrentPdf()
  })

  return {
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
  }
}
