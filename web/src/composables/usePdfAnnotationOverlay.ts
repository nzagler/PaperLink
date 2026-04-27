import { nextTick, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { Canvas as FabricCanvas, Path as FabricPath, PencilBrush, Textbox } from 'fabric'
import {
  annotationTools,
  cloneAnnotation,
  createCanvasAnnotation,
  createTextboxAnnotation,
  fromCollabAnnotation,
  toCollabAnnotation,
  type AnnotationTool,
  type CanvasPathCommand,
  type PDFAnnotation,
  type PDFCanvasAnnotation,
} from '@/lib/pdf_annotations'
import type { CollabAnnotationLock, CollabServerMessage, CollabStatus } from '@/lib/pdf_collab'

type OverlayOptions = {
  currentPage: Ref<number>
  pdfCanvasEl: Ref<HTMLCanvasElement | null>
  pageRenderVersion: Ref<number>
  collabStatus: Ref<CollabStatus>
  collabClientId: Ref<string | null>
  subscribeCollabMessages: (listener: (message: CollabServerMessage) => void) => () => void
  requestPageAnnotations: (page: number) => boolean
  createAnnotation: (annotation: ReturnType<typeof toCollabAnnotation>) => boolean
  updateAnnotation: (annotation: ReturnType<typeof toCollabAnnotation>) => boolean
  moveAnnotation: (annotation: ReturnType<typeof toCollabAnnotation>) => boolean
  deleteAnnotation: (annotationId: number) => boolean
  lockAnnotation: (annotationId: number) => boolean
  unlockAnnotation: (annotationId: number) => boolean
}

type FabricAnnotationMeta = {
  annotationId?: number
  pendingCreate?: boolean
}

type FabricTextboxWithId = Textbox & FabricAnnotationMeta
type FabricPathWithId = FabricPath & FabricAnnotationMeta
type FabricAnnotationObject = FabricTextboxWithId | FabricPathWithId
type LockedAnnotationSummary = {
  annotationId: number
  username: string
  isMine: boolean
}

const DEFAULT_TEXTBOX_FILL = '#111827'
const DEFAULT_TEXTBOX_FONT_SIZE = 0.032
const DEFAULT_DRAW_COLOR = '#ef4444'
const DEFAULT_DRAW_STROKE_WIDTH = 0.004

export function usePdfAnnotationOverlay({
  currentPage,
  pdfCanvasEl,
  pageRenderVersion,
  collabStatus,
  collabClientId,
  subscribeCollabMessages,
  requestPageAnnotations,
  createAnnotation,
  updateAnnotation,
  moveAnnotation,
  deleteAnnotation,
  lockAnnotation,
  unlockAnnotation,
}: OverlayOptions) {
  const annotationHostEl = ref<HTMLDivElement | null>(null)
  const activeTool = ref<AnnotationTool>('select')
  const annotationCount = ref(0)
  const overlayReady = ref(false)
  const selectedAnnotationId = ref<number | null>(null)
  const selectedAnnotationType = ref<PDFAnnotation['type'] | null>(null)
  const textboxFill = ref(DEFAULT_TEXTBOX_FILL)
  const textboxFontSize = ref(18)
  const canvasStroke = ref(DEFAULT_DRAW_COLOR)
  const canvasStrokeWidth = ref(2)
  const lockedAnnotations = ref<LockedAnnotationSummary[]>([])

  const annotationsByPage = new Map<number, Map<number, PDFAnnotation>>()
  const annotationPageIndex = new Map<number, number>()
  const annotationLocks = new Map<number, CollabAnnotationLock>()
  const renderedObjects = new Map<number, FabricAnnotationObject>()
  let textboxDefaultFill = DEFAULT_TEXTBOX_FILL
  let textboxDefaultFontSize = DEFAULT_TEXTBOX_FONT_SIZE
  let canvasDefaultStroke = DEFAULT_DRAW_COLOR
  let canvasDefaultStrokeWidth = DEFAULT_DRAW_STROKE_WIDTH
  let fabricCanvas: FabricCanvas | null = null
  let lockLayerEl: HTMLDivElement | null = null
  let resizeObserver: ResizeObserver | null = null
  let loadToken = 0
  let isHydrating = false
  let resizeFrame = 0
  let suppressedRemoveEvents = 0
  let suppressedSelectionLockSync = 0
  let activeSelectionLockId: number | null = null
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

  function getOverlayHeight() {
    return getOverlaySize().height || 560
  }

  function syncBrushStyle() {
    if (!fabricCanvas?.freeDrawingBrush) {
      return
    }

    fabricCanvas.freeDrawingBrush.color = canvasDefaultStroke
    fabricCanvas.freeDrawingBrush.width = Math.max(1.5, getOverlayHeight() * canvasDefaultStrokeWidth)
  }

  function syncOverlaySize() {
    if (!fabricCanvas) return false

    const { width, height } = getOverlaySize()
    if (!width || !height) return false

    if (annotationHostEl.value) {
      annotationHostEl.value.style.width = `${width}px`
      annotationHostEl.value.style.height = `${height}px`
    }
    if (lockLayerEl) {
      lockLayerEl.style.width = `${width}px`
      lockLayerEl.style.height = `${height}px`
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

  function isOwnLock(lock: CollabAnnotationLock | null | undefined) {
    return !!lock && !!collabClientId.value && lock.ownerClientId === collabClientId.value
  }

  function getAnnotationLock(annotationId: number) {
    return annotationLocks.get(annotationId) ?? null
  }

  function isAnnotationLockedByOther(annotationId: number) {
    const lock = getAnnotationLock(annotationId)
    return !!lock && !isOwnLock(lock)
  }

  function syncLockedAnnotationSummary() {
    const page = currentPage.value
    const nextLockedAnnotations = Array.from(annotationLocks.values())
      .filter((lock) => annotationPageIndex.get(lock.annotationId) === page)
      .sort((a, b) => a.annotationId - b.annotationId)
      .map((lock) => ({
        annotationId: lock.annotationId,
        username: lock.user.username,
        isMine: isOwnLock(lock),
      }))

    lockedAnnotations.value = nextLockedAnnotations
  }

  function createLockOverlay(lock: CollabAnnotationLock, object: FabricAnnotationObject) {
    const overlay = document.createElement('div')
    const badge = document.createElement('div')
    const mine = isOwnLock(lock)
    const bounds = object.getBoundingRect()
    const width = Math.max(18, Math.round(bounds.width))
    const height = Math.max(18, Math.round(bounds.height))
    const top = Math.max(0, Math.round(bounds.top))
    const left = Math.max(0, Math.round(bounds.left))
    const badgeTop = top > 26 ? top - 24 : top + 4

    overlay.className = mine
      ? 'pointer-events-none absolute rounded-md border border-emerald-500/90 bg-emerald-500/8 shadow-[0_0_0_1px_rgba(16,185,129,0.16)]'
      : 'pointer-events-none absolute rounded-md border border-amber-500/90 bg-amber-500/10 shadow-[0_0_0_1px_rgba(245,158,11,0.16)]'
    overlay.style.left = `${left}px`
    overlay.style.top = `${top}px`
    overlay.style.width = `${width}px`
    overlay.style.height = `${height}px`

    badge.className = mine
      ? 'pointer-events-none absolute max-w-[12rem] truncate rounded-full bg-emerald-600 px-2 py-1 text-[10px] font-semibold text-white shadow-sm'
      : 'pointer-events-none absolute max-w-[12rem] truncate rounded-full bg-amber-500 px-2 py-1 text-[10px] font-semibold text-neutral-950 shadow-sm'
    badge.style.left = `${left}px`
    badge.style.top = `${badgeTop}px`
    badge.textContent = mine ? `${lock.user.username} (You)` : lock.user.username

    return [overlay, badge]
  }

  function syncLockOverlays() {
    if (!lockLayerEl) {
      syncLockedAnnotationSummary()
      return
    }

    lockLayerEl.replaceChildren()
    syncLockedAnnotationSummary()

    const pageLocks = lockedAnnotations.value
      .map((summary) => getAnnotationLock(summary.annotationId))
      .filter((lock): lock is CollabAnnotationLock => lock !== null)

    for (const lock of pageLocks) {
      const object = renderedObjects.get(lock.annotationId)
      if (!object) {
        continue
      }

      object.setCoords()
      const [overlay, badge] = createLockOverlay(lock, object)
      lockLayerEl.append(overlay, badge)
    }
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
    syncLockedAnnotationSummary()
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
    syncLockedAnnotationSummary()
  }

  function removeCachedAnnotation(annotationID: number) {
    const page = annotationPageIndex.get(annotationID)
    if (page === undefined) {
      annotationLocks.delete(annotationID)
      syncLockedAnnotationSummary()
      return false
    }

    const pageMap = annotationsByPage.get(page)
    pageMap?.delete(annotationID)
    if (pageMap && pageMap.size === 0) {
      annotationsByPage.delete(page)
    }
    annotationPageIndex.delete(annotationID)
    annotationLocks.delete(annotationID)
    syncLockedAnnotationSummary()
    return page === currentPage.value
  }

  function isTextboxObject(object: unknown): object is FabricTextboxWithId {
    return !!object && typeof object === 'object' && (object as { type?: string }).type === 'textbox'
  }

  function isPathObject(object: unknown): object is FabricPathWithId {
    return !!object && typeof object === 'object' && (object as { type?: string }).type === 'path'
  }

  function createTextboxObject(annotation: PDFAnnotation, width: number, height: number) {
    if (annotation.type !== 'TEXTBOX') {
      return null
    }

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

  function scaleCanvasPath(path: CanvasPathCommand[], width: number, height: number) {
    return path.map((command) => {
      const scaled: CanvasPathCommand = [command[0]]
      for (let index = 1; index < command.length; index += 1) {
        const dimension = index % 2 === 1 ? width : height
        scaled.push(command[index] * dimension)
      }
      return scaled
    })
  }

  function normalizeCanvasPath(path: CanvasPathCommand[], width: number, height: number) {
    return path.map((command) => {
      const normalized: CanvasPathCommand = [command[0]]
      for (let index = 1; index < command.length; index += 1) {
        const dimension = index % 2 === 1 ? width : height
        normalized.push(dimension ? command[index] / dimension : 0)
      }
      return normalized
    })
  }

  function applyPathAppearance(path: FabricPathWithId, annotation?: PDFCanvasAnnotation) {
    const strokeWidth = annotation?.strokeWidth ?? canvasDefaultStrokeWidth
    const { height } = getOverlaySize()

    path.set({
      fill: null,
      stroke: annotation?.stroke ?? path.stroke ?? canvasDefaultStroke,
      strokeWidth: Math.max(1.5, strokeWidth * height),
      borderColor: '#0f172a',
      cornerColor: '#0f172a',
      cornerStrokeColor: '#ffffff',
      transparentCorners: false,
      objectCaching: true,
      hasControls: false,
      lockScalingX: true,
      lockScalingY: true,
      lockRotation: true,
    })
  }

  function createCanvasObject(annotation: PDFAnnotation, width: number, height: number) {
    if (annotation.type !== 'CANVAS') {
      return null
    }

    const path = new FabricPath(scaleCanvasPath(annotation.path, width, height), {
      left: annotation.positionX * width,
      top: annotation.positionY * height,
    }) as FabricPathWithId

    path.annotationId = annotation.id
    applyPathAppearance(path, annotation)
    path.setCoords()
    return path
  }

  function syncObjectInteractivity(object: FabricAnnotationObject) {
    const drawingMode = activeTool.value === 'draw'
    const annotationId = object.annotationId
    const lockedByOther = typeof annotationId === 'number' && isAnnotationLockedByOther(annotationId)

    if (isPathObject(object) && object.pendingCreate) {
      object.set({
        selectable: false,
        evented: false,
      })
      return
    }

    object.set({
      selectable: !drawingMode && !lockedByOther,
      evented: !drawingMode && !lockedByOther,
    })

    if (isTextboxObject(object)) {
      object.set({
        editable: !lockedByOther,
      })
    }
  }

  function findFabricObject(annotationID: number) {
    return renderedObjects.get(annotationID) ?? null
  }

  function hasFabricObject(annotationID: number) {
    return renderedObjects.has(annotationID)
  }

  function findPendingTextbox() {
    if (!fabricCanvas) {
      return null
    }

    for (const object of fabricCanvas.getObjects()) {
      if (!isTextboxObject(object) || !object.pendingCreate) {
        continue
      }

      return object
    }

    return null
  }

  function areCanvasPathsEqual(a: CanvasPathCommand[], b: CanvasPathCommand[]) {
    if (a.length !== b.length) {
      return false
    }

    return a.every((command, index) => {
      const other = b[index]
      if (!other || command.length !== other.length) {
        return false
      }

      return command.every((value, commandIndex) => {
        const nextValue = other[commandIndex]
        if (typeof value === 'string' || typeof nextValue === 'string') {
          return value === nextValue
        }

        return Math.abs(value - nextValue) < 0.0001
      })
    })
  }

  function findPendingCanvasPath(annotation: PDFCanvasAnnotation) {
    if (!fabricCanvas) {
      return null
    }

    const { width, height } = getOverlaySize()
    if (!width || !height) {
      return null
    }

    for (const object of fabricCanvas.getObjects()) {
      if (!isPathObject(object) || !object.pendingCreate) {
        continue
      }

      const pending = serializeCanvasPath(object, width, height)
      if (
        areCanvasPathsEqual(pending.path, annotation.path)
        && pending.stroke === annotation.stroke
        && Math.abs(pending.strokeWidth - annotation.strokeWidth) < 0.0001
        && Math.abs(pending.positionX - annotation.positionX) < 0.0001
        && Math.abs(pending.positionY - annotation.positionY) < 0.0001
      ) {
        return object
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
    if (annotation.type !== 'TEXTBOX') {
      return false
    }

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
    syncObjectInteractivity(textbox)
    textbox.setCoords()
    return true
  }

  function applyAnnotationToPath(path: FabricPathWithId, annotation: PDFCanvasAnnotation) {
    path.annotationId = annotation.id
    path.pendingCreate = false
    applyPathAppearance(path, annotation)
    syncObjectInteractivity(path)
    path.setCoords()
    return true
  }

  function createObjectForAnnotation(annotation: PDFAnnotation, width: number, height: number) {
    const object = annotation.type === 'TEXTBOX'
      ? createTextboxObject(annotation, width, height)
      : createCanvasObject(annotation, width, height)

    if (!object) {
      return null
    }

    syncObjectInteractivity(object)
    return object
  }

  function withSuppressedRemoveEvents(callback: () => void) {
    suppressedRemoveEvents += 1
    try {
      callback()
    } finally {
      suppressedRemoveEvents -= 1
    }
  }

  function removeObjectInternally(annotationID: number, object: FabricAnnotationObject) {
    if (!fabricCanvas) {
      return
    }

    withSuppressedRemoveEvents(() => {
      renderedObjects.delete(annotationID)
      fabricCanvas?.remove(object)
    })
  }

  function syncRenderedAnnotations(page: number) {
    if (!fabricCanvas) return

    const size = getOverlaySize()
    if (!size.width || !size.height) return

    const annotations = listPageAnnotations(page)
    const nextIDs = new Set(annotations.map((annotation) => annotation.id))

    for (const [annotationID, object] of Array.from(renderedObjects.entries())) {
      if (!nextIDs.has(annotationID)) {
        removeObjectInternally(annotationID, object)
      }
    }

    for (const annotation of annotations) {
      const existingObject = renderedObjects.get(annotation.id)
      if (existingObject && annotation.type === 'TEXTBOX' && isTextboxObject(existingObject)) {
        applyAnnotationToTextboxWithSize(existingObject, annotation, size)
        continue
      }

      if (existingObject) {
        removeObjectInternally(annotation.id, existingObject)
      }

      const object = createObjectForAnnotation(annotation, size.width, size.height)
      if (!object) {
        continue
      }

      renderedObjects.set(annotation.id, object)
      fabricCanvas.add(object)
    }

    annotationCount.value = annotations.length
    syncLockOverlays()
    fabricCanvas.requestRenderAll()
  }

    function serializeTextbox(textbox: FabricTextboxWithId, width: number, height: number): PDFAnnotation {
        const cached = typeof textbox.annotationId === 'number'
            ? getCachedAnnotation(textbox.annotationId)
            : null

    return createTextboxAnnotation({
      id: textbox.annotationId ?? 0,
      page: cached?.page ?? currentPage.value,
      text: textbox.text ?? '',
      positionX: (textbox.left ?? 0) / width,
      positionY: (textbox.top ?? 0) / height,
      width: ((textbox.width ?? 0) * (textbox.scaleX ?? 1)) / width,
      fontSize: (textbox.fontSize ?? 16) / height,
      fill: typeof textbox.fill === 'string' ? textbox.fill : DEFAULT_TEXTBOX_FILL,
      angle: textbox.angle ?? 0,
    })
  }

    function serializeCanvasPath(path: FabricPathWithId, width: number, height: number) {
        const cached = typeof path.annotationId === 'number'
            ? getCachedAnnotation(path.annotationId)
            : null

    return createCanvasAnnotation({
      id: path.annotationId ?? 0,
      page: cached?.page ?? currentPage.value,
      positionX: (path.left ?? 0) / width,
      positionY: (path.top ?? 0) / height,
      path: normalizeCanvasPath(path.path as CanvasPathCommand[], width, height),
      stroke: typeof path.stroke === 'string' ? path.stroke : canvasDefaultStroke,
      strokeWidth: (path.strokeWidth ?? 1.5) / height,
    })
  }

  function syncStyleControls() {
    if (!fabricCanvas) {
      selectedAnnotationType.value = null
      textboxFill.value = textboxDefaultFill
      textboxFontSize.value = Math.max(12, getOverlayHeight() * textboxDefaultFontSize)
      canvasStroke.value = canvasDefaultStroke
      canvasStrokeWidth.value = Math.max(1.5, getOverlayHeight() * canvasDefaultStrokeWidth)
      return
    }

    const activeObject = fabricCanvas.getActiveObject()
    if (activeObject && isTextboxObject(activeObject)) {
      selectedAnnotationType.value = 'TEXTBOX'
      textboxFill.value = typeof activeObject.fill === 'string' ? activeObject.fill : textboxDefaultFill
      textboxFontSize.value = Math.max(12, activeObject.fontSize ?? getOverlayHeight() * textboxDefaultFontSize)
      return
    }

    if (activeObject && isPathObject(activeObject)) {
      selectedAnnotationType.value = 'CANVAS'
      canvasStroke.value = typeof activeObject.stroke === 'string' ? activeObject.stroke : canvasDefaultStroke
      canvasStrokeWidth.value = Math.max(1.5, activeObject.strokeWidth ?? getOverlayHeight() * canvasDefaultStrokeWidth)
      return
    }

    selectedAnnotationType.value = null
    textboxFill.value = textboxDefaultFill
    textboxFontSize.value = Math.max(12, getOverlayHeight() * textboxDefaultFontSize)
    canvasStroke.value = canvasDefaultStroke
    canvasStrokeWidth.value = Math.max(1.5, getOverlayHeight() * canvasDefaultStrokeWidth)
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

  function withSuppressedSelectionLockSync(callback: () => void) {
    suppressedSelectionLockSync += 1
    try {
      callback()
    } finally {
      suppressedSelectionLockSync -= 1
    }
  }

  function releaseSelectionLock(annotationId: number | null = activeSelectionLockId) {
    if (annotationId === null) {
      return
    }
    unlockAnnotation(annotationId)
    if (activeSelectionLockId === annotationId) {
      activeSelectionLockId = null
    }
  }

  function syncSelectionLock(nextAnnotationId: number | null, activeObject: FabricAnnotationObject | null = null) {
    if (suppressedSelectionLockSync > 0) {
      return
    }

    if (activeSelectionLockId !== null && activeSelectionLockId !== nextAnnotationId) {
      releaseSelectionLock(activeSelectionLockId)
    }

    if (nextAnnotationId === null) {
      return
    }

    const lock = getAnnotationLock(nextAnnotationId)
    if (lock && !isOwnLock(lock)) {
      if (fabricCanvas && activeObject) {
        withSuppressedSelectionLockSync(() => {
          fabricCanvas?.discardActiveObject()
        })
        fabricCanvas.requestRenderAll()
      }
      return
    }

    if (activeSelectionLockId === nextAnnotationId || isOwnLock(lock)) {
      activeSelectionLockId = nextAnnotationId
      return
    }

    if (lockAnnotation(nextAnnotationId)) {
      activeSelectionLockId = nextAnnotationId
    }
  }

  function setActiveTool(tool: AnnotationTool) {
    activeTool.value = tool
    if (!fabricCanvas) return

    const drawingMode = tool === 'draw'
    fabricCanvas.isDrawingMode = drawingMode
    fabricCanvas.selection = !drawingMode
    fabricCanvas.skipTargetFind = drawingMode

    if (drawingMode) {
      fabricCanvas.discardActiveObject()
      selectedAnnotationId.value = null
    }

    for (const object of renderedObjects.values()) {
      syncObjectInteractivity(object)
    }

    fabricCanvas.requestRenderAll()
    syncStyleControls()
  }

  function syncSelectionState() {
    if (!fabricCanvas) {
      syncSelectionLock(null)
      selectedAnnotationId.value = null
      selectedAnnotationType.value = null
      syncStyleControls()
      return
    }

    const activeObject = fabricCanvas.getActiveObject()
    if (!activeObject || (!isTextboxObject(activeObject) && !isPathObject(activeObject))) {
      syncSelectionLock(null)
      selectedAnnotationId.value = null
      selectedAnnotationType.value = null
      syncStyleControls()
      return
    }

    const annotationID = activeObject.annotationId
    const nextAnnotationId = typeof annotationID === 'number' ? annotationID : null
    syncSelectionLock(nextAnnotationId, activeObject)
    if (nextAnnotationId !== null && isAnnotationLockedByOther(nextAnnotationId)) {
      selectedAnnotationId.value = null
      selectedAnnotationType.value = null
      syncStyleControls()
      return
    }
    selectedAnnotationId.value = typeof annotationID === 'number' ? annotationID : null
    selectedAnnotationType.value = isTextboxObject(activeObject) ? 'TEXTBOX' : 'CANVAS'
    syncStyleControls()
  }

  async function addTextbox() {
    if (!fabricCanvas || collabStatus.value !== 'connected') return
    if (!syncOverlaySize()) return

    const { width, height } = getOverlaySize()
    const textbox = new Textbox('Text', {
      left: width * 0.16,
      top: height * 0.14,
      width: width * 0.3,
      fontSize: Math.max(12, height * textboxDefaultFontSize),
      fill: textboxDefaultFill,
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
      fontSize: textboxDefaultFontSize,
      fill: textboxDefaultFill,
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
    syncLockOverlays()

    if (mode === 'move') {
      moveAnnotation(toCollabAnnotation(annotation))
      return
    }

    updateAnnotation(toCollabAnnotation(annotation))
  }

  function persistCanvasObject(object: FabricPathWithId, mode: 'update' | 'move' = 'update') {
    if (!fabricCanvas || isHydrating || collabStatus.value !== 'connected') return
    if (!object.annotationId || object.pendingCreate) return

    const { width, height } = getOverlaySize()
    if (!width || !height) return

    const annotation = serializeCanvasPath(object, width, height)
    upsertCachedAnnotation(annotation)
    annotationCount.value = fabricCanvas.getObjects().length
    syncLockOverlays()

    if (mode === 'move') {
      moveAnnotation(toCollabAnnotation(annotation))
      return
    }

    updateAnnotation(toCollabAnnotation(annotation))
  }

  function setTextboxFill(fill: string) {
    textboxDefaultFill = fill
    textboxFill.value = fill

    if (!fabricCanvas) {
      return
    }

    const activeObject = fabricCanvas.getActiveObject()
    if (!activeObject || !isTextboxObject(activeObject)) {
      return
    }

    activeObject.set({ fill })
    activeObject.setCoords()
    fabricCanvas.requestRenderAll()
    persistTextbox(activeObject)
    syncStyleControls()
  }

  function setTextboxFontSize(fontSizePx: number) {
    const nextFontSize = Math.max(12, fontSizePx)
    textboxFontSize.value = nextFontSize
    textboxDefaultFontSize = nextFontSize / getOverlayHeight()

    if (!fabricCanvas) {
      return
    }

    const activeObject = fabricCanvas.getActiveObject()
    if (!activeObject || !isTextboxObject(activeObject)) {
      return
    }

    activeObject.set({ fontSize: nextFontSize })
    activeObject.setCoords()
    fabricCanvas.requestRenderAll()
    persistTextbox(activeObject)
    syncStyleControls()
  }

  function setCanvasStroke(stroke: string) {
    canvasDefaultStroke = stroke
    canvasStroke.value = stroke
    syncBrushStyle()

    if (!fabricCanvas) {
      return
    }

    const activeObject = fabricCanvas.getActiveObject()
    if (!activeObject || !isPathObject(activeObject)) {
      return
    }

    activeObject.set({ stroke })
    activeObject.setCoords()
    fabricCanvas.requestRenderAll()
    persistCanvasObject(activeObject)
    syncStyleControls()
  }

  function setCanvasStrokeWidth(strokeWidthPx: number) {
    const nextStrokeWidth = Math.max(1.5, strokeWidthPx)
    canvasStrokeWidth.value = nextStrokeWidth
    canvasDefaultStrokeWidth = nextStrokeWidth / getOverlayHeight()
    syncBrushStyle()

    if (!fabricCanvas) {
      return
    }

    const activeObject = fabricCanvas.getActiveObject()
    if (!activeObject || !isPathObject(activeObject)) {
      return
    }

    activeObject.set({ strokeWidth: nextStrokeWidth })
    activeObject.setCoords()
    fabricCanvas.requestRenderAll()
    persistCanvasObject(activeObject)
    syncStyleControls()
  }

  function removeSelectedAnnotation() {
    if (!fabricCanvas) {
      return
    }

    const activeObject = fabricCanvas.getActiveObject()
    if (!activeObject || (!isTextboxObject(activeObject) && !isPathObject(activeObject))) {
      return
    }

    const annotationId = activeObject.annotationId
    if (typeof annotationId !== 'number') {
      return
    }

    withSuppressedSelectionLockSync(() => {
      withSuppressedRemoveEvents(() => {
        fabricCanvas?.remove(activeObject)
        fabricCanvas?.discardActiveObject()
      })
    })
    renderedObjects.delete(annotationId)
    removeCachedAnnotation(annotationId)
    selectedAnnotationId.value = null
    selectedAnnotationType.value = null
    if (activeSelectionLockId === annotationId) {
      activeSelectionLockId = null
    }
    annotationCount.value = fabricCanvas.getObjects().length
    fabricCanvas.requestRenderAll()
    syncLockOverlays()
    deleteAnnotation(annotationId)
    syncStyleControls()
  }

  function handleServerMessage(message: CollabServerMessage) {
    switch (message.type) {
      case 'room_state': {
        annotationLocks.clear()
        if (Array.isArray(message.annotationLocks)) {
          for (const lock of message.annotationLocks) {
            annotationLocks.set(lock.annotationId, lock)
          }
        }
        syncRenderedAnnotations(currentPage.value)
        return
      }

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

      case 'annotation:locked': {
        if (!message.annotationLock) {
          return
        }

        annotationLocks.set(message.annotationLock.annotationId, message.annotationLock)
        const object = renderedObjects.get(message.annotationLock.annotationId)
        if (object) {
          syncObjectInteractivity(object)
        }

        if (!isOwnLock(message.annotationLock) && selectedAnnotationId.value === message.annotationLock.annotationId && fabricCanvas) {
          withSuppressedSelectionLockSync(() => {
            fabricCanvas.discardActiveObject()
          })
          selectedAnnotationId.value = null
          selectedAnnotationType.value = null
        }

        syncLockOverlays()
        fabricCanvas?.requestRenderAll()
        syncStyleControls()
        return
      }

      case 'annotation:unlocked': {
        if (!message.annotationLock) {
          return
        }

        annotationLocks.delete(message.annotationLock.annotationId)
        if (activeSelectionLockId === message.annotationLock.annotationId && message.annotationLock.ownerClientId === collabClientId.value) {
          activeSelectionLockId = null
        }

        const object = renderedObjects.get(message.annotationLock.annotationId)
        if (object) {
          syncObjectInteractivity(object)
        }

        syncLockOverlays()
        fabricCanvas?.requestRenderAll()
        syncStyleControls()
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

        if (message.type === 'annotation:created') {
          if (annotation.type === 'TEXTBOX') {
            const pendingTextbox = findPendingTextbox()
            if (pendingTextbox) {
              upsertCachedAnnotation(annotation)
              applyAnnotationToTextbox(pendingTextbox, annotation)
              fabricCanvas?.requestRenderAll()
              syncLockOverlays()
              syncSelectionState()
              void nextTick(() => {
                persistTextbox(pendingTextbox)
              })
              return
            }
          } else {
            const pendingPath = findPendingCanvasPath(annotation)
            if (pendingPath) {
              applyAnnotationToPath(pendingPath, annotation)
              renderedObjects.set(annotation.id, pendingPath)
              annotationCount.value = fabricCanvas?.getObjects().length ?? annotationCount.value
              fabricCanvas?.requestRenderAll()
              syncLockOverlays()
              return
            }
          }
        }

        if (previousPage === currentPage.value && annotation.page !== currentPage.value) {
          syncRenderedAnnotations(currentPage.value)
          return
        }

        if (annotation.page === currentPage.value) {
          const object = findFabricObject(annotation.id)
          if (object && annotation.type === 'TEXTBOX' && isTextboxObject(object)) {
            const didApply = applyAnnotationToTextbox(object, annotation)
            if (didApply) {
              annotationCount.value = fabricCanvas?.getObjects().length ?? annotationCount.value
              syncLockOverlays()
              fabricCanvas?.requestRenderAll()
              return
            }
          }

          if (!hasFabricObject(annotation.id) || annotation.type === 'CANVAS') {
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
          const existingObject = renderedObjects.get(message.annotationId)
          if (existingObject && fabricCanvas) {
            removeObjectInternally(message.annotationId, existingObject)
            annotationCount.value = fabricCanvas.getObjects().length
            fabricCanvas.requestRenderAll()
          } else {
            syncRenderedAnnotations(currentPage.value)
          }
        }
        if (activeSelectionLockId === message.annotationId) {
          activeSelectionLockId = null
        }
        if (selectedAnnotationId.value === message.annotationId) {
          selectedAnnotationId.value = null
          selectedAnnotationType.value = null
        }
        syncLockOverlays()
        syncStyleControls()
        return
      }

      case 'error': {
        if (
          typeof message.annotationId === 'number'
          && activeSelectionLockId === message.annotationId
          && typeof message.error === 'string'
          && message.error.includes('locked')
          && fabricCanvas
        ) {
          activeSelectionLockId = null
          withSuppressedSelectionLockSync(() => {
            fabricCanvas.discardActiveObject()
          })
          selectedAnnotationId.value = null
          selectedAnnotationType.value = null
          syncStyleControls()
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
    lockLayerEl = document.createElement('div')
    lockLayerEl.className = 'pointer-events-none absolute inset-0 z-30'
    host.replaceChildren(canvasEl, lockLayerEl)

    fabricCanvas = new FabricCanvas(canvasEl, {
      preserveObjectStacking: true,
      renderOnAddRemove: false,
      selection: true,
      containerClass: 'paperlink-annotation-overlay',
    })

    const brush = new PencilBrush(fabricCanvas)
    brush.color = DEFAULT_DRAW_COLOR
    brush.width = Math.max(1.5, getOverlaySize().height * DEFAULT_DRAW_STROKE_WIDTH)
    fabricCanvas.freeDrawingBrush = brush

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
      if (!event.target) return

      if (isTextboxObject(event.target)) {
        const previous = typeof event.target.annotationId === 'number'
          ? getCachedAnnotation(event.target.annotationId)
          : null
        const { width, height } = getOverlaySize()
        if (!width || !height) return

        const next = serializeTextbox(event.target, width, height)
        const isMoveOnly = previous !== null
          && previous.type === 'TEXTBOX'
          && previous.text === next.text
          && previous.width === next.width
          && previous.fontSize === next.fontSize
          && previous.fill === next.fill
          && previous.angle === next.angle
          && (previous.positionX !== next.positionX || previous.positionY !== next.positionY || previous.page !== next.page)

        persistTextbox(event.target, isMoveOnly ? 'move' : 'update')
        syncSelectionState()
        return
      }

      if (isPathObject(event.target)) {
        persistCanvasObject(event.target, 'move')
        syncSelectionState()
      }
    })
    fabricCanvas.on('object:removed', (event) => {
      if (isHydrating || suppressedRemoveEvents > 0 || !event.target || (!isTextboxObject(event.target) && !isPathObject(event.target))) return
      const annotationID = event.target.annotationId
      if (typeof annotationID !== 'number') {
        annotationCount.value = fabricCanvas?.getObjects().length ?? 0
        syncSelectionState()
        return
      }

      annotationCount.value = fabricCanvas?.getObjects().length ?? 0
      renderedObjects.delete(annotationID)
      removeCachedAnnotation(annotationID)
      deleteAnnotation(annotationID)
      syncSelectionState()
    })
    fabricCanvas.on('text:changed', (event) => {
      if (!event.target || !isTextboxObject(event.target)) return
      persistTextbox(event.target)
    })
    fabricCanvas.on('path:created', (event) => {
      if (!event.path || !isPathObject(event.path) || collabStatus.value !== 'connected') {
        return
      }

      const { width, height } = getOverlaySize()
      if (!width || !height) {
        return
      }

      event.path.pendingCreate = true
      applyPathAppearance(event.path)
      syncObjectInteractivity(event.path)
      annotationCount.value = fabricCanvas?.getObjects().length ?? annotationCount.value
      fabricCanvas?.requestRenderAll()

      createAnnotation(toCollabAnnotation(serializeCanvasPath(event.path, width, height)))
    })
    fabricCanvas.on('selection:created', syncSelectionState)
    fabricCanvas.on('selection:updated', syncSelectionState)
    fabricCanvas.on('selection:cleared', syncSelectionState)

    overlayReady.value = true
    setActiveTool('select')
    syncBrushStyle()
    syncOverlaySize()
    void reloadAnnotations(currentPage.value)
    requestCurrentPageAnnotations()
    syncLockOverlays()
  }

  onMounted(async () => {
    await nextTick()
    mountFabricCanvas()
    unsubscribeCollabMessages = subscribeCollabMessages(handleServerMessage)

    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => {
        syncBrushStyle()
        syncStyleControls()
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
    syncStyleControls()
    scheduleResizeSync()
  })

  watch(currentPage, (page) => {
    if (activeSelectionLockId !== null) {
      releaseSelectionLock(activeSelectionLockId)
    }
    if (fabricCanvas) {
      withSuppressedSelectionLockSync(() => {
        fabricCanvas?.discardActiveObject()
      })
    }
    selectedAnnotationId.value = null
    selectedAnnotationType.value = null
    void reloadAnnotations(page)
    requestPageAnnotations(page)
    syncStyleControls()
  })

  watch(collabStatus, (status) => {
    if (status === 'connected') {
      requestCurrentPageAnnotations()
      return
    }

    activeSelectionLockId = null
    annotationLocks.clear()
    syncLockOverlays()
    if (fabricCanvas) {
      for (const object of renderedObjects.values()) {
        syncObjectInteractivity(object)
      }
      fabricCanvas.requestRenderAll()
    }
  })

  watch(pageRenderVersion, () => {
    syncBrushStyle()
    syncStyleControls()
    scheduleResizeSync()
  })

  onBeforeUnmount(() => {
    if (resizeFrame) cancelAnimationFrame(resizeFrame)
    resizeObserver?.disconnect()
    resizeObserver = null
    unsubscribeCollabMessages?.()
    unsubscribeCollabMessages = null
    overlayReady.value = false
    releaseSelectionLock(activeSelectionLockId)
    lockLayerEl = null

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
    lockedAnnotations,
    selectedAnnotationId,
    selectedAnnotationType,
    textboxFill,
    textboxFontSize,
    canvasStroke,
    canvasStrokeWidth,
    setActiveTool,
    setTextboxFill,
    setTextboxFontSize,
    setCanvasStroke,
    setCanvasStrokeWidth,
    addTextbox,
    removeSelectedAnnotation,
  }
}
