import { nextTick, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { Canvas as FabricCanvas, Textbox } from 'fabric'
import {
  annotationTools,
  cloneAnnotation,
  createTextboxAnnotation,
  fromCollabAnnotation,
  toCollabAnnotation,
  type AnnotationTool,
  type PDFAnnotation,
} from '@/lib/pdf_annotations'
import type { CollabServerMessage, CollabStatus } from '@/lib/pdf_collab'

type OverlayOptions = {
  currentPage: Ref<number>
  pdfCanvasEl: Ref<HTMLCanvasElement | null>
  pageRenderVersion: Ref<number>
  collabStatus: Ref<CollabStatus>
  subscribeCollabMessages: (listener: (message: CollabServerMessage) => void) => () => void
  requestPageAnnotations: (page: number) => boolean
  createAnnotation: (annotation: ReturnType<typeof toCollabAnnotation>) => boolean
  updateAnnotation: (annotation: ReturnType<typeof toCollabAnnotation>) => boolean
  moveAnnotation: (annotation: ReturnType<typeof toCollabAnnotation>) => boolean
  deleteAnnotation: (annotationId: number) => boolean
}

type FabricTextboxWithId = Textbox & {
  annotationId?: number
  pendingCreate?: boolean
}

export function usePdfAnnotationOverlay({
  currentPage,
  pdfCanvasEl,
  pageRenderVersion,
  collabStatus,
  subscribeCollabMessages,
  requestPageAnnotations,
  createAnnotation,
  updateAnnotation,
  moveAnnotation,
  deleteAnnotation,
}: OverlayOptions) {
  const annotationHostEl = ref<HTMLDivElement | null>(null)
  const activeTool = ref<AnnotationTool>('select')
  const annotationCount = ref(0)
  const overlayReady = ref(false)
  const selectedAnnotationId = ref<number | null>(null)

  const annotationsByPage = new Map<number, Map<number, PDFAnnotation>>()
  const annotationPageIndex = new Map<number, number>()
  const renderedTextboxes = new Map<number, FabricTextboxWithId>()
  let fabricCanvas: FabricCanvas | null = null
  let resizeObserver: ResizeObserver | null = null
  let loadToken = 0
  let isHydrating = false
  let resizeFrame = 0
  let unsubscribeCollabMessages: (() => void) | null = null

  function getOverlaySize() {
    const canvas = pdfCanvasEl.value
    if (!canvas) return { width: 0, height: 0 }

    const rect = canvas.getBoundingClientRect()
    const width = Math.round(rect.width || canvas.clientWidth || canvas.width)
    const height = Math.round(rect.height || canvas.clientHeight || canvas.height)

    return {
      width,
      height,
    }
  }

  function syncOverlaySize() {
    if (!fabricCanvas) return false

    const { width, height } = getOverlaySize()
    if (!width || !height) return false

    if (annotationHostEl.value) {
      annotationHostEl.value.style.width = `${width}px`
      annotationHostEl.value.style.height = `${height}px`
    }

    fabricCanvas.setDimensions({ width, height })
    fabricCanvas.requestRenderAll()
    return true
  }

  function getPageAnnotationMap(page: number) {
    let pageMap = annotationsByPage.get(page)
    if (!pageMap) {
      pageMap = new Map<number, PDFAnnotation>()
      annotationsByPage.set(page, pageMap)
    }
    return pageMap
  }

  function getCachedAnnotation(annotationID: number) {
    const page = annotationPageIndex.get(annotationID)
    if (page === undefined) {
      return null
    }

    return annotationsByPage.get(page)?.get(annotationID) ?? null
  }

  function listPageAnnotations(page: number) {
    const pageMap = annotationsByPage.get(page)
    if (!pageMap) {
      return []
    }
    return Array.from(pageMap.values()).sort((a, b) => a.id - b.id)
  }

  function replacePageAnnotations(page: number, annotations: PDFAnnotation[]) {
    const existingPageMap = annotationsByPage.get(page)
    if (existingPageMap) {
      for (const annotationID of existingPageMap.keys()) {
        annotationPageIndex.delete(annotationID)
      }
    }

    const nextPageMap = new Map<number, PDFAnnotation>()
    for (const annotation of annotations) {
      nextPageMap.set(annotation.id, cloneAnnotation(annotation))
      annotationPageIndex.set(annotation.id, page)
    }

    annotationsByPage.set(page, nextPageMap)
  }

  function upsertCachedAnnotation(annotation: PDFAnnotation) {
    const previousPage = annotationPageIndex.get(annotation.id)
    if (previousPage !== undefined && previousPage !== annotation.page) {
      const previousPageMap = annotationsByPage.get(previousPage)
      previousPageMap?.delete(annotation.id)
      if (previousPageMap && previousPageMap.size === 0) {
        annotationsByPage.delete(previousPage)
      }
    }

    getPageAnnotationMap(annotation.page).set(annotation.id, cloneAnnotation(annotation))
    annotationPageIndex.set(annotation.id, annotation.page)
  }

  function removeCachedAnnotation(annotationID: number) {
    const page = annotationPageIndex.get(annotationID)
    if (page === undefined) {
      return false
    }

    const pageMap = annotationsByPage.get(page)
    pageMap?.delete(annotationID)
    if (pageMap && pageMap.size === 0) {
      annotationsByPage.delete(page)
    }
    annotationPageIndex.delete(annotationID)
    return page === currentPage.value
  }

  function createTextboxObject(annotation: PDFAnnotation, width: number, height: number) {
    const textbox = new Textbox(annotation.text, {
      left: annotation.positionX * width,
      top: annotation.positionY * height,
      width: annotation.width * width,
      fontSize: Math.max(12, annotation.fontSize * height),
      fill: annotation.fill,
      angle: annotation.angle,
      editable: true,
      borderColor: '#0f172a',
      cornerColor: '#0f172a',
      cornerStrokeColor: '#ffffff',
      transparentCorners: false,
      objectCaching: true,
    }) as FabricTextboxWithId

    textbox.annotationId = annotation.id
    return textbox
  }

  function hasFabricTextbox(annotationID: number) {
    return renderedTextboxes.has(annotationID)
  }

  function findFabricTextbox(annotationID: number) {
    return renderedTextboxes.get(annotationID) ?? null
  }

  function findPendingTextbox() {
    if (!fabricCanvas) {
      return null
    }

    for (const object of fabricCanvas.getObjects()) {
      if (object.type !== 'textbox') {
        continue
      }

      const textbox = object as FabricTextboxWithId
      if (textbox.pendingCreate) {
        return textbox
      }
    }

    return null
  }

  function applyAnnotationToTextbox(textbox: FabricTextboxWithId, annotation: PDFAnnotation) {
    return applyAnnotationToTextboxWithSize(textbox, annotation, getOverlaySize())
  }

  function applyAnnotationToTextboxWithSize(
    textbox: FabricTextboxWithId,
    annotation: PDFAnnotation,
    size: { width: number; height: number },
  ) {
    const { width, height } = size
    if (!width || !height) {
      return false
    }

    textbox.annotationId = annotation.id
    textbox.pendingCreate = false
    textbox.set({
      text: annotation.text,
      left: annotation.positionX * width,
      top: annotation.positionY * height,
      width: annotation.width * width,
      fontSize: Math.max(12, annotation.fontSize * height),
      fill: annotation.fill,
      angle: annotation.angle,
      scaleX: 1,
      scaleY: 1,
    })
    textbox.setCoords()
    return true
  }

  function syncRenderedAnnotations(page: number) {
    if (!fabricCanvas) return

    const size = getOverlaySize()
    if (!size.width || !size.height) return

    const annotations = listPageAnnotations(page)
    const nextIDs = new Set(annotations.map((annotation) => annotation.id))

    for (const [annotationID, textbox] of Array.from(renderedTextboxes.entries())) {
      if (!nextIDs.has(annotationID)) {
        renderedTextboxes.delete(annotationID)
        fabricCanvas.remove(textbox)
      }
    }

    for (const annotation of annotations) {
      const existingTextbox = renderedTextboxes.get(annotation.id)
      if (existingTextbox) {
        applyAnnotationToTextboxWithSize(existingTextbox, annotation, size)
        continue
      }

      const textbox = createTextboxObject(annotation, size.width, size.height)
      renderedTextboxes.set(annotation.id, textbox)
      fabricCanvas.add(textbox)
    }

    annotationCount.value = annotations.length
    fabricCanvas.requestRenderAll()
  }

  function serializeTextbox(textbox: FabricTextboxWithId, width: number, height: number): PDFAnnotation {
    return createTextboxAnnotation({
      id: textbox.annotationId ?? 0,
      page: currentPage.value,
      text: textbox.text ?? '',
      positionX: (textbox.left ?? 0) / width,
      positionY: (textbox.top ?? 0) / height,
      width: ((textbox.width ?? 0) * (textbox.scaleX ?? 1)) / width,
      fontSize: (textbox.fontSize ?? 16) / height,
      fill: typeof textbox.fill === 'string' ? textbox.fill : '#0f172a',
      angle: textbox.angle ?? 0,
    })
  }

  async function reloadAnnotations(page: number) {
    if (!fabricCanvas) return

    if (!syncOverlaySize()) {
      annotationCount.value = 0
      return
    }

    const localToken = ++loadToken
    const { width, height } = getOverlaySize()
    if (!width || !height) return

    isHydrating = true

    try {
      if (localToken !== loadToken || !fabricCanvas) return
      void width
      void height
      syncRenderedAnnotations(page)
    } finally {
      isHydrating = false
    }
  }

  function requestCurrentPageAnnotations() {
    if (collabStatus.value !== 'connected') {
      return
    }

    requestPageAnnotations(currentPage.value)
  }

  function setActiveTool(tool: AnnotationTool) {
    activeTool.value = tool
    if (!fabricCanvas) return

    fabricCanvas.selection = true
    fabricCanvas.skipTargetFind = false
    fabricCanvas.requestRenderAll()
  }

  function syncSelectionState() {
    if (!fabricCanvas) {
      selectedAnnotationId.value = null
      return
    }

    const activeObject = fabricCanvas.getActiveObject()
    if (!activeObject || activeObject.type !== 'textbox') {
      selectedAnnotationId.value = null
      return
    }

    const annotationID = (activeObject as FabricTextboxWithId).annotationId
    selectedAnnotationId.value = typeof annotationID === 'number' ? annotationID : null
  }

  async function addTextbox() {
    if (!fabricCanvas || collabStatus.value !== 'connected') return
    if (!syncOverlaySize()) return

    const { width, height } = getOverlaySize()
    const textbox = new Textbox('Text', {
      left: width * 0.16,
      top: height * 0.14,
      width: width * 0.3,
      fontSize: Math.max(18, height * 0.032),
      fill: '#111827',
      editable: true,
      borderColor: '#0f172a',
      cornerColor: '#0f172a',
      cornerStrokeColor: '#ffffff',
      transparentCorners: false,
      objectCaching: true,
    }) as FabricTextboxWithId
    textbox.pendingCreate = true

    fabricCanvas.add(textbox)
    fabricCanvas.setActiveObject(textbox)
    annotationCount.value = fabricCanvas.getObjects().length
    fabricCanvas.requestRenderAll()
    textbox.enterEditing()
    textbox.selectAll()
    setActiveTool('select')

    createAnnotation(toCollabAnnotation(createTextboxAnnotation({
      id: 0,
      page: currentPage.value,
      text: 'Text',
      positionX: 0.16,
      positionY: 0.14,
      width: 0.3,
      fontSize: 0.032,
      fill: '#111827',
    })))
  }

  function persistTextbox(object: FabricTextboxWithId, mode: 'update' | 'move' = 'update') {
    if (!fabricCanvas || isHydrating || collabStatus.value !== 'connected') return
    if (!object.annotationId || object.pendingCreate) return

    const { width, height } = getOverlaySize()
    if (!width || !height) return

    const annotation = serializeTextbox(object, width, height)
    upsertCachedAnnotation(annotation)
    annotationCount.value = fabricCanvas.getObjects().length

    if (mode === 'move') {
      moveAnnotation(toCollabAnnotation(annotation))
      return
    }

    updateAnnotation(toCollabAnnotation(annotation))
  }

  function removeSelectedAnnotation() {
    if (!fabricCanvas) {
      return
    }

    const activeObject = fabricCanvas.getActiveObject()
    if (!activeObject || activeObject.type !== 'textbox') {
      return
    }

    fabricCanvas.remove(activeObject)
    fabricCanvas.discardActiveObject()
    selectedAnnotationId.value = null
    fabricCanvas.requestRenderAll()
  }

  function handleServerMessage(message: CollabServerMessage) {
    switch (message.type) {
      case 'annotations:page': {
        if (typeof message.page !== 'number' || !Array.isArray(message.annotations)) {
          return
        }

        const annotations = message.annotations
          .map((annotation) => fromCollabAnnotation(annotation))
          .filter((annotation): annotation is PDFAnnotation => annotation !== null)

        replacePageAnnotations(message.page, annotations)
        if (message.page === currentPage.value) {
          syncRenderedAnnotations(message.page)
        }
        return
      }

      case 'annotation:created':
      case 'annotation:updated':
      case 'annotation:moved': {
        if (!message.annotation) {
          return
        }

        const annotation = fromCollabAnnotation(message.annotation)
        if (!annotation) {
          return
        }

        const previousPage = annotationPageIndex.get(annotation.id)
        upsertCachedAnnotation(annotation)
        const pendingTextbox = message.type === 'annotation:created' ? findPendingTextbox() : null
        if (pendingTextbox) {
          applyAnnotationToTextbox(pendingTextbox, annotation)
          fabricCanvas?.requestRenderAll()
          void nextTick(() => {
            persistTextbox(pendingTextbox)
          })
          return
        }

        if (previousPage === currentPage.value && annotation.page !== currentPage.value) {
          syncRenderedAnnotations(currentPage.value)
          return
        }

        if (annotation.page === currentPage.value) {
          const textbox = findFabricTextbox(annotation.id)
          if (textbox) {
            const didApply = applyAnnotationToTextbox(textbox, annotation)
            if (didApply) {
              annotationCount.value = fabricCanvas?.getObjects().length ?? annotationCount.value
              fabricCanvas?.requestRenderAll()
              return
            }
          }

          if (!hasFabricTextbox(annotation.id)) {
            syncRenderedAnnotations(currentPage.value)
          }
        }
        return
      }

      case 'annotation:deleted': {
        if (typeof message.annotationId !== 'number') {
          return
        }

        if (removeCachedAnnotation(message.annotationId)) {
          const existingTextbox = renderedTextboxes.get(message.annotationId)
          if (existingTextbox && fabricCanvas) {
            renderedTextboxes.delete(message.annotationId)
            fabricCanvas.remove(existingTextbox)
            annotationCount.value = fabricCanvas.getObjects().length
            fabricCanvas.requestRenderAll()
          } else {
            syncRenderedAnnotations(currentPage.value)
          }
        }
        if (selectedAnnotationId.value === message.annotationId) {
          selectedAnnotationId.value = null
        }
      }
    }
  }

  function scheduleResizeSync() {
    if (resizeFrame) cancelAnimationFrame(resizeFrame)
    resizeFrame = requestAnimationFrame(() => {
      resizeFrame = 0
      if (!syncOverlaySize()) return
      syncRenderedAnnotations(currentPage.value)
    })
  }

  function mountFabricCanvas() {
    const host = annotationHostEl.value
    if (!host || fabricCanvas) return

    const canvasEl = document.createElement('canvas')
    canvasEl.className = 'block h-full w-full'
    host.replaceChildren(canvasEl)

    fabricCanvas = new FabricCanvas(canvasEl, {
      preserveObjectStacking: true,
      renderOnAddRemove: false,
      selection: true,
      containerClass: 'paperlink-annotation-overlay',
    })

    const wrapperEl = (fabricCanvas as unknown as { wrapperEl?: HTMLDivElement }).wrapperEl
    if (wrapperEl) {
      wrapperEl.style.position = 'absolute'
      wrapperEl.style.inset = '0'
      wrapperEl.style.width = '100%'
      wrapperEl.style.height = '100%'
      wrapperEl.style.zIndex = '20'
    }

    const lowerCanvasEl = (fabricCanvas as unknown as { lowerCanvasEl?: HTMLCanvasElement }).lowerCanvasEl
    if (lowerCanvasEl) {
      lowerCanvasEl.style.position = 'absolute'
      lowerCanvasEl.style.inset = '0'
      lowerCanvasEl.style.width = '100%'
      lowerCanvasEl.style.height = '100%'
      lowerCanvasEl.style.zIndex = '20'
    }

    const upperCanvasEl = (fabricCanvas as unknown as { upperCanvasEl?: HTMLCanvasElement }).upperCanvasEl
    if (upperCanvasEl) {
      upperCanvasEl.style.position = 'absolute'
      upperCanvasEl.style.inset = '0'
      upperCanvasEl.style.width = '100%'
      upperCanvasEl.style.height = '100%'
      upperCanvasEl.style.zIndex = '21'
    }

    fabricCanvas.on('object:modified', (event) => {
      if (!event.target || event.target.type !== 'textbox') return
      const textbox = event.target as FabricTextboxWithId
      const previous = typeof textbox.annotationId === 'number' ? getCachedAnnotation(textbox.annotationId) : null
      const { width, height } = getOverlaySize()
      if (!width || !height) return

      const next = serializeTextbox(textbox, width, height)
      const isMoveOnly = previous !== null
        && previous.text === next.text
        && previous.width === next.width
        && previous.fontSize === next.fontSize
        && previous.fill === next.fill
        && previous.angle === next.angle
        && (previous.positionX !== next.positionX || previous.positionY !== next.positionY || previous.page !== next.page)

      persistTextbox(textbox, isMoveOnly ? 'move' : 'update')
      syncSelectionState()
    })
    fabricCanvas.on('object:removed', (event) => {
      if (isHydrating || !event.target || event.target.type !== 'textbox') return
      const annotationID = (event.target as FabricTextboxWithId).annotationId
      if (typeof annotationID !== 'number') return

      annotationCount.value = fabricCanvas?.getObjects().length ?? 0
      renderedTextboxes.delete(annotationID)
      removeCachedAnnotation(annotationID)
      deleteAnnotation(annotationID)
      syncSelectionState()
    })
    fabricCanvas.on('text:changed', (event) => {
      if (!event.target || event.target.type !== 'textbox') return
      persistTextbox(event.target as FabricTextboxWithId)
    })
    fabricCanvas.on('selection:created', syncSelectionState)
    fabricCanvas.on('selection:updated', syncSelectionState)
    fabricCanvas.on('selection:cleared', syncSelectionState)

    overlayReady.value = true
    setActiveTool('select')
    syncOverlaySize()
    void reloadAnnotations(currentPage.value)
    requestCurrentPageAnnotations()
  }

  onMounted(async () => {
    await nextTick()
    mountFabricCanvas()
    unsubscribeCollabMessages = subscribeCollabMessages(handleServerMessage)

    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => {
        scheduleResizeSync()
      })

      if (pdfCanvasEl.value) {
        resizeObserver.observe(pdfCanvasEl.value)
      }
    }
  })

  watch(pdfCanvasEl, (canvas, prevCanvas) => {
    if (prevCanvas && resizeObserver) {
      resizeObserver.unobserve(prevCanvas)
    }
    if (canvas && resizeObserver) {
      resizeObserver.observe(canvas)
    }
    mountFabricCanvas()
    scheduleResizeSync()
  })

  watch(currentPage, (page) => {
    void reloadAnnotations(page)
    requestPageAnnotations(page)
  })

  watch(collabStatus, (status) => {
    if (status === 'connected') {
      requestCurrentPageAnnotations()
    }
  })

  watch(pageRenderVersion, () => {
    scheduleResizeSync()
  })

  onBeforeUnmount(() => {
    if (resizeFrame) cancelAnimationFrame(resizeFrame)
    resizeObserver?.disconnect()
    resizeObserver = null
    unsubscribeCollabMessages?.()
    unsubscribeCollabMessages = null
    overlayReady.value = false

    if (fabricCanvas) {
      fabricCanvas.dispose()
      fabricCanvas = null
    }
  })

  return {
    annotationHostEl,
    annotationCount,
    annotationTools,
    activeTool,
    overlayReady,
    selectedAnnotationId,
    setActiveTool,
    addTextbox,
    removeSelectedAnnotation,
  }
}
