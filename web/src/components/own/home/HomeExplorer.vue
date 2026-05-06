<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  Folder,
  FileText,
  ChevronRight,
  ArrowLeft,
  ArrowRight,
  Home as HomeIcon,
  BarChart3,
  Loader2,
  Trash2,
  PencilLine,
} from 'lucide-vue-next'
import {
  Breadcrumb,
  BreadcrumbList,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import {
  TooltipProvider,
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from '@/components/ui/tooltip'
import CardWithoutBorder from '@/components/own/CardWithoutBorder.vue'
import type { Item } from '@/dto/item'
import { apiFetch } from '@/auth/api'
import { useRouter } from 'vue-router'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogClose,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'

type ApiFileNode = { id: string; name: string; size: number }
type ApiDirNode = { id: number; name: string; files: ApiFileNode[]; directories: ApiDirNode[] }
const router = useRouter()
function mapDirNodeToItems(node: ApiDirNode): Item[] {
  const dirs: Item[] = (node.directories ?? []).map(d => ({
    id: String(d.id),
    name: d.name,
    type: 'folder',
    children: mapDirNodeToItems(d),
  }))

  const files: Item[] = (node.files ?? []).map(f => ({
    id: f.id,
    name: f.name.endsWith('.pdf') ? f.name : `${f.name}.pdf`,
    type: 'file',
    size: f.size,
  }))

  return [...dirs, ...files]
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx++
  }
  const decimals = value >= 10 || idx === 0 ? 0 : 1
  return `${value.toFixed(decimals)} ${units[idx]}`
}

const tree = ref<Item[]>([])
const pathIds = ref<string[]>([])
const history = ref<string[][]>([[]])
const historyIndex = ref(0)

const loading = ref(false)
const loadError = ref<string | null>(null)

async function loadTree() {
  loading.value = true
  loadError.value = null
  try {
    const res = await apiFetch('/api/v1/structure/tree', { method: 'GET' })
    const json = await res.json().catch(() => null)

    if (!res.ok || !json?.data) {
      loadError.value = json?.message || 'Failed to load documents.'
      tree.value = []
      pathIds.value = []
      history.value = [[]]
      historyIndex.value = 0
      return
    }

    const root = json.data as ApiDirNode
    tree.value = mapDirNodeToItems(root)
    const nextPath = normalizePath(pathIds.value)
    pathIds.value = nextPath
    history.value = [nextPath]
    historyIndex.value = 0
  } catch {
    loadError.value = 'Failed to load documents.'
    tree.value = []
    pathIds.value = []
    history.value = [[]]
    historyIndex.value = 0
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadTree()
})

const currentItems = computed(() => {
  const last = currentPathNodes.value[currentPathNodes.value.length - 1]
  if (!last) return tree.value
  return last.children ?? []
})

const currentFolderLabel = computed(() => {
  const last = currentPathNodes.value[currentPathNodes.value.length - 1]
  return last ? last.name : 'All documents'
})

const currentPathNodes = computed(() => resolvePath(pathIds.value))
const currentFolder = computed(() => currentPathNodes.value[currentPathNodes.value.length - 1] ?? null)

const libraryStats = computed(() => {
  const result = { folders: 0, files: 0, bytes: 0 }

  const traverse = (items: Item[]) => {
    for (const item of items) {
      if (item.type === 'folder') {
        result.folders++
        if (item.children) traverse(item.children)
      } else {
        result.files++
        if (typeof (item as any).size === 'number') {
          result.bytes += (item as any).size
        }
      }
    }
  }

  traverse(tree.value)
  return result
})

const currentLevelStats = computed(() => {
  let folders = 0
  let files = 0
  let bytes = 0

  for (const item of currentItems.value) {
    if (item.type === 'folder') {
      folders++
    } else {
      files++
      if (typeof (item as any).size === 'number') {
        bytes += (item as any).size
      }
    }
  }

  return { folders, files, bytes }
})

function resolvePath(ids: string[]): Item[] {
  const resolved: Item[] = []
  let level = tree.value

  for (const id of ids) {
    const next = level.find((item) => item.type === 'folder' && String(item.id) === String(id))
    if (!next || next.type !== 'folder') break
    resolved.push(next)
    level = next.children ?? []
  }

  return resolved
}

function normalizePath(ids: string[]): string[] {
  return resolvePath(ids).map((item) => String(item.id))
}

function updatePath(newPath: string[], pushToHistory = true) {
  const normalized = normalizePath(newPath)
  pathIds.value = normalized
  if (!pushToHistory) return
  history.value = history.value.slice(0, historyIndex.value + 1)
  history.value.push(normalized)
  historyIndex.value++
}



function breadcrumbClick(index: number) {
  const newPath = index < 0 ? [] : pathIds.value.slice(0, index + 1)
  updatePath(newPath)
}

function goHome() {
  if (pathIds.value.length === 0) return
  updatePath([])
}

function goBack() {
  if (historyIndex.value === 0) return
  historyIndex.value--
  const next = history.value[historyIndex.value]
  if (next) pathIds.value = [...next]
}

function goForward() {
  if (historyIndex.value >= history.value.length - 1) return
  historyIndex.value++
  const next = history.value[historyIndex.value]
  if (next) pathIds.value = [...next]
}

function openFile(item: Item) {
  void router.push('/pdf/' + item.id)
}

function handleItemClick(item: Item) {
  if (item.type === 'folder') {
    updatePath([...pathIds.value, String(item.id)])
  } else {
    openFile(item)
  }
}

function iconFor(item: Item) {
  return item.type === 'folder' ? Folder : FileText
}

// Local payload type for optimistic addUploadedFile (used by Home.vue)
type UploadPayload = {
  name: string
  file: File
}

function addUploadedFile(payload: UploadPayload) {
  const newItem: Item = {
    id: `uploaded-${Date.now()}`,
    name: `${payload.name}.pdf`,
    type: 'file',
    size: payload.file.size as any,
  }

  const last = currentFolder.value
  if (!last) {
    tree.value.push(newItem)
  } else {
    if (!last.children) {
      ;(last as any).children = []
    }
    last.children!.push(newItem)
  }
}

function addCreatedDirectory(name: string, id?: string) {
  const newItem: Item = {
    id: id ?? `dir-${Date.now()}`,
    name,
    type: 'folder',
    children: [],
  }

  const last = currentFolder.value
  if (!last) {
    tree.value.push(newItem)
  } else {
    if (!last.children) (last as any).children = []
    last.children!.push(newItem)
  }
}

function addCreatedDocument(name: string, fileUUID?: string, size?: number) {
  const newItem: Item = {
    id: fileUUID ?? `doc-${Date.now()}`,
    name: name.endsWith('.pdf') ? name : `${name}.pdf`,
    type: 'file',
    size: size,
  }

  const last = currentFolder.value
  if (!last) {
    tree.value.push(newItem)
  } else {
    if (!last.children) (last as any).children = []
    last.children!.push(newItem)
  }
}

const deleteDialogOpen = ref(false)
const deleteTarget = ref<Item | null>(null)
const deleteLoading = ref(false)
const deleteError = ref<string | null>(null)

const contextMenuOpen = ref(false)
const contextMenuTarget = ref<Item | null>(null)
const contextTriggerRef = ref<HTMLButtonElement | null>(null)
const contextMenuPosition = ref({ x: 0, y: 0 })
const contextTriggerStyle = computed(() => ({
  position: 'fixed',
  left: `${contextMenuPosition.value.x}px`,
  top: `${contextMenuPosition.value.y}px`,
  width: '1px',
  height: '1px',
  pointerEvents: 'none',
}))

const renameDialogOpen = ref(false)
const renameTarget = ref<Item | null>(null)
const renameValue = ref('')
const renameLoading = ref(false)
const renameError = ref<string | null>(null)

const editDialogOpen = ref(false)
const editTarget = ref<Item | null>(null)
const editName = ref('')
const editDescription = ref('')
const editTags = ref('')
const editLoading = ref(false)
const editError = ref<string | null>(null)

function normalizeTagNames(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  return (raw as any[])
    .map((tag) => {
      if (!tag || typeof tag !== 'object') return ''
      const name = (tag as any).name ?? (tag as any).Name
      return typeof name === 'string' ? name.trim() : ''
    })
    .filter((name) => name.length > 0)
}

function populateEditFields(payload: { name?: string; description?: string; tags?: unknown }) {
  if (payload.name !== undefined) {
    editName.value = String(payload.name).replace(/\.pdf$/i, '')
  }
  if (payload.description !== undefined) {
    editDescription.value = String(payload.description)
  }
  if (payload.tags !== undefined) {
    editTags.value = normalizeTagNames(payload.tags).join(', ')
  }
}

function onDeleteDialogChange(val: boolean) {
  deleteDialogOpen.value = val
  if (!val) resetDeleteState()
}

function onEditDialogChange(val: boolean) {
  editDialogOpen.value = val
  if (!val) resetEditState()
}

function resetDeleteState() {
  deleteDialogOpen.value = false
  deleteTarget.value = null
  deleteLoading.value = false
  deleteError.value = null
}

function promptDelete(item: Item, event?: Event) {
  event?.stopPropagation()
  deleteTarget.value = item
  deleteDialogOpen.value = true
  deleteError.value = null
}

function openContextMenu(event: MouseEvent, item: Item) {
  event.preventDefault()
  event.stopPropagation()
  contextMenuTarget.value = item
  contextMenuPosition.value = { x: event.clientX, y: event.clientY }
  contextMenuOpen.value = false
  requestAnimationFrame(() => {
    contextMenuOpen.value = true
    contextTriggerRef.value?.focus({ preventScroll: true })
  })
}

function closeContextMenu() {
  contextMenuOpen.value = false
  contextMenuTarget.value = null
}

function onContextMenuOpenChange(open: boolean) {
  contextMenuOpen.value = open
  if (!open) contextMenuTarget.value = null
}

function startRename() {
  if (!contextMenuTarget.value) return
  renameTarget.value = contextMenuTarget.value
  renameValue.value = contextMenuTarget.value.name
  renameDialogOpen.value = true
  renameError.value = null
  closeContextMenu()
}

function onRenameDialogChange(val: boolean) {
  renameDialogOpen.value = val
  if (!val) resetRenameState()
}

function resetRenameState() {
  renameDialogOpen.value = false
  renameTarget.value = null
  renameValue.value = ''
  renameLoading.value = false
  renameError.value = null
}

function resetEditState() {
  editDialogOpen.value = false
  editTarget.value = null
  editName.value = ''
  editDescription.value = ''
  editTags.value = ''
  editLoading.value = false
  editError.value = null
}

function ensurePdfExtension(name: string) {
  return /\.pdf$/i.test(name) ? name : `${name}.pdf`
}

function applyRename(items: Item[], targetId: string, newName: string): boolean {
  for (const item of items) {
    if (item.id === targetId) {
      item.name = newName
      return true
    }
    if (item.type === 'folder' && item.children) {
      if (applyRename(item.children, targetId, newName)) return true
    }
  }
  return false
}

async function submitRename() {
  if (!renameTarget.value) return
  const nextName = renameValue.value.trim()
  if (!nextName) {
    renameError.value = 'Name is required.'
    return
  }

  renameLoading.value = true
  renameError.value = null
  const finalName = renameTarget.value.type === 'folder' ? nextName : ensurePdfExtension(nextName)

  try {
    let res: Response
    if (renameTarget.value.type === 'folder') {
      const dirId = Number(renameTarget.value.id)
      if (!Number.isFinite(dirId)) throw new Error('Invalid directory identifier.')
      res = await apiFetch(`/api/v1/directory/update/${dirId}`.toString(), {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: finalName }),
      })
    } else {
      res = await apiFetch('/api/v1/document/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ uuid: renameTarget.value.id, name: finalName }),
      })
    }

    if (!res.ok) {
      const json = await res.json().catch(() => null)
      throw new Error(json?.message || 'Failed to rename item.')
    }

    if (!applyRename(tree.value, renameTarget.value.id, finalName)) {
      await loadTree()
    }

    resetRenameState()
  } catch (err) {
    renameError.value = err instanceof Error ? err.message : 'Failed to rename item.'
  } finally {
    renameLoading.value = false
  }
}

function startEditDocument(item: Item) {
  if (item.type !== 'file') return
  editTarget.value = item
  // optimistic: use current item name, keep description/tags until we have real data
  populateEditFields({ name: item.name })
  editDialogOpen.value = true
  editError.value = null
  closeContextMenu()
  void loadDocumentDetails(item.id)
}

async function loadDocumentDetails(uuid: string) {
  try {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 5000)

    const res = await apiFetch(`/api/v1/document/get/${uuid}`, { signal: controller.signal })
    clearTimeout(timeoutId)

    if (!res.ok) return
    const json = await res.json().catch(() => null)
    if (!json || typeof json !== 'object') return

    // backend returns the document directly from c.JSON(http.StatusOK, doc)
    const doc: any = json
    populateEditFields({
      name: doc.name ?? doc.Name,
      description: doc.description ?? doc.Description,
      tags: doc.tags ?? doc.Tags,
    })
  } catch {
    // keep optimistic values on error/timeout
  }
}

async function submitEditDocument() {
  if (!editTarget.value) return
  const trimmedName = editName.value.trim()
  if (!trimmedName) {
    editError.value = 'Name is required.'
    return
  }
  const payload = {
    uuid: editTarget.value.id,
    name: ensurePdfExtension(trimmedName),
    description: editDescription.value.trim(),
    tags: editTags.value
      .split(',')
      .map((tag) => tag.trim())
      .filter((tag) => tag.length > 0),
  }

  editLoading.value = true
  editError.value = null
  try {
    const res = await apiFetch('/api/v1/document/update', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    if (!res.ok) {
      const json = await res.json().catch(() => null)
      throw new Error(json?.message || 'Failed to update document.')
    }

    if (!applyRename(tree.value, editTarget.value.id, payload.name)) {
      await loadTree()
    }

    resetEditState()
  } catch (err) {
    editError.value = err instanceof Error ? err.message : 'Failed to update document.'
  } finally {
    editLoading.value = false
  }
}

async function deleteTargetItem() {
  if (!deleteTarget.value) return
  deleteLoading.value = true
  deleteError.value = null

  const item = deleteTarget.value
  const url = item.type === 'folder'
    ? `/api/v1/directory/delete/${item.id}`
    : `/api/v1/document/delete/${item.id}`

  try {
    const res = await apiFetch(url, { method: 'DELETE' })
    if (!res.ok) {
      const json = await res.json().catch(() => null)
      throw new Error(json?.error ?? json?.message ?? `Failed to delete item. (${res.status})`)
    }

    const parentPath = pathIds.value.slice(0, -1)
    await loadTree()

    // If we were in the deleted folder, navigate up
    if (item.type === 'folder' && parentPath.length > 0 && String(item.id) === parentPath[parentPath.length - 1]) {
      updatePath(parentPath)
    }

    closeContextMenu()
    resetDeleteState()
  } catch (err) {
    deleteError.value = err instanceof Error ? err.message : 'Failed to delete item.'
  } finally {
    deleteLoading.value = false
  }
}

function deleteFromContext() {
  const target = contextMenuTarget.value
  if (!target) return
  promptDelete(target)
  closeContextMenu()
}

defineExpose({
  addUploadedFile,
  addCreatedDirectory,
  addCreatedDocument,
  reload: loadTree,
  getCurrentDirectoryId: () => {
    const last = currentFolder.value
    if (!last) return null
    const n = Number(last.id)
    return Number.isFinite(n) ? n : null
  },
  getCurrentFolderPath: () => currentPathNodes.value.map((p) => p.name).join('/'),
})
</script>

<template>
  <div class="min-h-screen bg-neutral-50 text-neutral-900 dark:bg-neutral-950 dark:text-neutral-50 transition-colors">
    <div class="min-h-screen flex flex-col">
      <header class="bg-neutral-50/90 dark:bg-neutral-950/90 backdrop-blur-sm">
        <div class="mx-auto max-w-6xl px-4 lg:px-6 py-3.5 flex items-center justify-between gap-4">
          <TooltipProvider>
            <div class="flex items-center gap-1.5 sm:gap-2">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button
                      variant="outline"
                      size="sm"
                      class="rounded-full px-3 text-xs sm:text-sm"
                      :disabled="pathIds.length === 0"
                      @click="goHome"
                  >
                    <HomeIcon class="w-4 h-4 mr-1" />
                    <span class="hidden sm:inline">Home</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Home</TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger as-child>
                  <Button
                      variant="outline"
                      size="sm"
                      class="rounded-full px-3 text-xs sm:text-sm"
                      :disabled="historyIndex === 0"
                      @click="goBack"
                  >
                    <ArrowLeft class="w-4 h-4 mr-1" />
                    <span class="hidden sm:inline">Back</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Back</TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger as-child>
                  <Button
                      variant="outline"
                      size="sm"
                      class="rounded-full px-3 text-xs sm:text-sm"
                      :disabled="historyIndex >= history.length - 1"
                      @click="goForward"
                  >
                    <ArrowRight class="w-4 h-4 mr-1" />
                    <span class="hidden sm:inline">Forward</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Forward</TooltipContent>
              </Tooltip>
            </div>
          </TooltipProvider>

          <div class="flex items-center justify-end">
            <div class="inline-flex h-9 items-center rounded-full border border-neutral-300 bg-white px-3 sm:px-4 text-[11px] sm:text-xs text-neutral-800 shadow-sm dark:border-neutral-700 dark:bg-neutral-900 dark:text-neutral-100">
              <div class="flex h-7 w-7 items-center justify-center rounded-full bg-emerald-700/10 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-300">
                <BarChart3 class="w-4 h-4" aria-hidden="true" />
              </div>

              <div class="ml-2 flex items-center gap-2 sm:gap-3">
                <span class="hidden sm:inline text-[10px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">
                  Stats
                </span>

                <span class="whitespace-nowrap">
                  <span class="font-medium">Library:</span>
                  {{ libraryStats.folders }} folders ·
                  {{ libraryStats.files }} files ·
                  {{ formatBytes(libraryStats.bytes) }}
                </span>

                <span class="hidden sm:inline text-neutral-400 dark:text-neutral-600">
                  |
                </span>

                <span class="hidden sm:inline whitespace-nowrap">
                  <span class="font-medium">Level:</span>
                  {{ currentLevelStats.folders }} folders ·
                  {{ currentLevelStats.files }} files ·
                  {{ formatBytes(currentLevelStats.bytes) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </header>

      <main class="flex-1">
        <div class="mx-auto max-w-6xl px-4 lg:px-6 py-5 lg:py-7 space-y-4">
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink as-child>
                  <button
                      type="button"
                      class="inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs sm:text-sm text-neutral-600 hover:bg-neutral-200/80 hover:text-neutral-900 transition-colors dark:text-neutral-300 dark:hover:bg-neutral-800 dark:hover:text-neutral-50"
                      @click="breadcrumbClick(-1)"
                  >
                    <span class="font-medium">Home</span>
                  </button>
                </BreadcrumbLink>
              </BreadcrumbItem>

              <template v-for="(node, idx) in currentPathNodes" :key="node.id">
                <BreadcrumbSeparator>
                  <ChevronRight class="w-3.5 h-3.5 text-neutral-400 dark:text-neutral-600" />
                </BreadcrumbSeparator>
                <BreadcrumbItem>
                  <BreadcrumbLink as-child>
                    <button
                        type="button"
                        class="inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs sm:text-sm text-neutral-600 hover:bg-neutral-200/80 hover:text-neutral-900 transition-colors dark:text-neutral-300 dark:hover:bg-neutral-800 dark:hover:text-neutral-50"
                        @click="breadcrumbClick(idx)"
                    >
                      <Folder
                          v-if="node.type === 'folder'"
                          class="w-3.5 h-3.5 text-neutral-500 dark:text-neutral-400"
                      />
                      <FileText
                          v-else
                          class="w-3.5 h-3.5 text-neutral-500 dark:text-neutral-400"
                      />
                      <span class="truncate max-w-[140px] sm:max-w-[200px]">
                        {{ node.name }}
                      </span>
                    </button>
                  </BreadcrumbLink>
                </BreadcrumbItem>
              </template>
            </BreadcrumbList>
          </Breadcrumb>

          <section class="rounded-2xl border border-neutral-200 bg-white shadow-sm shadow-neutral-200/70 overflow-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:shadow-none">
            <div class="border-b border-neutral-200 bg-gradient-to-r from-neutral-50 via-white to-emerald-50/70 px-4 sm:px-6 py-3.5 flex items-center gap-3 dark:border-neutral-800 dark:from-neutral-900 dark:via-neutral-900 dark:to-emerald-900/30">
              <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-emerald-700/10 border border-emerald-700/40 dark:bg-emerald-500/10 dark:border-emerald-500/50">
                <Folder class="w-5 h-5 text-emerald-800 dark:text-emerald-300" aria-hidden="true" />
              </div>
              <div>
                <p class="text-[11px] uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">
                  Explorer
                </p>
                <p class="text-sm font-medium">
                  {{ currentFolderLabel }}
                </p>
              </div>
            </div>

            <div class="px-4 sm:px-6 py-5 sm:py-6">
              <div v-if="loadError" class="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-200">
                {{ loadError }}
              </div>

              <div v-if="loading" class="py-10 text-center text-sm text-neutral-500 dark:text-neutral-400">
                Loading...
              </div>

              <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 sm:gap-5">
                <CardWithoutBorder
                    v-for="item in currentItems"
                    :key="item.id"
                    class="group relative flex flex-col rounded-xl border border-neutral-200 bg-neutral-50/70 transition-all hover:-translate-y-[1px] hover:border-emerald-500/80 hover:bg-white hover:shadow-lg hover:shadow-emerald-900/10 cursor-pointer dark:border-neutral-800 dark:bg-neutral-900/80 dark:hover:border-emerald-500/60 dark:hover:bg-neutral-800"
                     @click="handleItemClick(item)"
                     @contextmenu.prevent.stop="openContextMenu($event, item)"
                 >
                  <div class="flex items-start gap-3 p-4">
                    <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-neutral-900 text-neutral-50 group-hover:bg-emerald-800 transition-colors dark:bg-neutral-200 dark:text-neutral-900 dark:group-hover:bg-emerald-500">
                      <component :is="iconFor(item)" class="w-5 h-5" aria-hidden="true" />
                    </div>
                    <div class="flex-1 overflow-hidden">
                      <p class="text-sm font-medium truncate" :title="item.name">
                        {{ item.name }}
                      </p>
                      <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5">
                        {{ item.type === 'folder' ? 'Folder' : 'PDF document' }}
                      </p>
                    </div>
                  </div>
                </CardWithoutBorder>

                <div
                    v-if="currentItems.length === 0"
                    class="col-span-full flex flex-col items-center justify-center rounded-xl border border-dashed border-neutral-300 bg-neutral-50 py-10 text-center dark:border-neutral-700 dark:bg-neutral-900"
                >
                  <Folder class="w-6 h-6 text-neutral-400 mb-2 dark:text-neutral-500" aria-hidden="true" />
                  <p class="text-sm text-neutral-600 dark:text-neutral-300">
                    This folder is empty.
                  </p>
                  <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1">
                    You will be able to add or upload documents here.
                  </p>
                </div>
              </div>

              <DropdownMenu :open="contextMenuOpen" @update:open="onContextMenuOpenChange" modal="false">
                <DropdownMenuTrigger as-child>
                  <button
                      ref="contextTriggerRef"
                      type="button"
                      tabindex="-1"
                      aria-hidden="true"
                      class="h-0 w-0 opacity-0"
                      :style="contextTriggerStyle"
                  />
                </DropdownMenuTrigger>
                <DropdownMenuContent
                    v-if="contextMenuTarget"
                    class="w-52 rounded-xl border border-neutral-200 bg-white p-1 shadow-lg shadow-neutral-900/10 dark:border-neutral-800 dark:bg-neutral-900"
                    align="start"
                    side="right"
                    side-offset="6"
                 >
                  <DropdownMenuItem
                      v-if="contextMenuTarget?.type === 'folder'"
                      class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-neutral-900 data-[highlighted]:bg-emerald-600/15 data-[highlighted]:text-emerald-900 dark:text-neutral-50 dark:data-[highlighted]:bg-emerald-500/20 dark:data-[highlighted]:text-emerald-200"
                      @select.prevent="startRename"
                  >
                    <PencilLine class="h-4 w-4" />
                    Rename
                  </DropdownMenuItem>
                  <DropdownMenuItem
                      v-else
                      class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-neutral-900 data-[highlighted]:bg-emerald-600/15 data-[highlighted]:text-emerald-900 dark:text-neutral-50 dark:data-[highlighted]:bg-emerald-500/20 dark:data-[highlighted]:text-emerald-200"
                      @select.prevent="startEditDocument(contextMenuTarget!)"
                  >
                    <PencilLine class="h-4 w-4" />
                    Edit
                  </DropdownMenuItem>
                  <DropdownMenuItem
                      class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-neutral-900 data-[highlighted]:bg-emerald-600/15 data-[highlighted]:text-emerald-900 dark:text-neutral-50 dark:data-[highlighted]:bg-emerald-500/20 dark:data-[highlighted]:text-emerald-200"
                      @select.prevent="deleteFromContext"
                  >
                    <Trash2 class="h-4 w-4" />
                    Delete
                  </DropdownMenuItem>
                 </DropdownMenuContent>
               </DropdownMenu>
             </div>
           </section>
        </div>
      </main>
    </div>

    <Dialog :open="deleteDialogOpen" @update:open="onDeleteDialogChange" modal>
      <DialogContent class="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>Delete {{ deleteTarget?.type === 'folder' ? 'folder' : 'document' }}</DialogTitle>
          <DialogDescription>
            This action cannot be undone. {{ deleteTarget?.type === 'folder' ? 'All nested items will be removed.' : 'The document will be permanently removed.' }}
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-3 text-sm text-neutral-600 dark:text-neutral-300">
          <p>
            Are you sure you want to delete
            <strong>{{ deleteTarget?.name }}</strong>?
          </p>
          <p v-if="deleteError" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-200">
            {{ deleteError }}
          </p>
        </div>
        <DialogFooter class="mt-6">
          <DialogClose as-child>
            <Button variant="outline" :disabled="deleteLoading">Cancel</Button>
          </DialogClose>
          <Button variant="destructive" :disabled="deleteLoading" @click="deleteTargetItem">
            <Loader2 v-if="deleteLoading" class="mr-2 h-4 w-4 animate-spin" />
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog :open="renameDialogOpen" @update:open="onRenameDialogChange" modal>
      <DialogContent class="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>Rename {{ renameTarget?.type === 'folder' ? 'folder' : 'document' }}</DialogTitle>
          <DialogDescription>
            Choose a new name that matches our naming rules. Document names will always end with .pdf.
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-3 text-sm text-neutral-600 dark:text-neutral-300">
          <label class="space-y-1 block">
            <span class="text-xs font-medium text-neutral-500 dark:text-neutral-400">Name</span>
            <Input
                v-model="renameValue"
                placeholder="Enter a name"
                :disabled="renameLoading"
                @keydown.enter.prevent="submitRename"
            />
          </label>
          <p v-if="renameError" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-200">
            {{ renameError }}
          </p>
        </div>
        <DialogFooter class="mt-6">
          <DialogClose as-child>
            <Button variant="outline" :disabled="renameLoading" @click="resetRenameState">Cancel</Button>
          </DialogClose>
          <Button :disabled="renameLoading" @click="submitRename">
            <Loader2 v-if="renameLoading" class="mr-2 h-4 w-4 animate-spin" />
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog :open="editDialogOpen" @update:open="onEditDialogChange" modal>
      <DialogContent class="sm:max-w-[480px]">
         <DialogHeader>
          <DialogTitle>Edit document</DialogTitle>
          <DialogDescription>
            Update the document metadata. Leave a field blank to clear it.
          </DialogDescription>
        </DialogHeader>
        <form class="space-y-5" @submit.prevent="submitEditDocument">
          <div class="space-y-1">
            <label class="text-xs font-medium text-neutral-500 dark:text-neutral-400" for="edit-name">Name</label>
            <Input id="edit-name" v-model="editName" placeholder="Document name" :disabled="editLoading" />
          </div>
          <div class="space-y-1">
            <label class="text-xs font-medium text-neutral-500 dark:text-neutral-400" for="edit-description">Description</label>
            <textarea
                id="edit-description"
                v-model="editDescription"
                class="min-h-[100px] w-full rounded-md border border-neutral-300 bg-white px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 dark:border-neutral-700 dark:bg-neutral-900"
                :disabled="editLoading"
                placeholder="Optional details"
            />
          </div>
          <div class="space-y-1">
            <label class="text-xs font-medium text-neutral-500 dark:text-neutral-400" for="edit-tags">Tags</label>
            <Input
                id="edit-tags"
                v-model="editTags"
                placeholder="tag-one, tag-two"
                :disabled="editLoading"
                aria-describedby="edit-tags-help"
            />
            <p id="edit-tags-help" class="text-[11px] text-neutral-500 dark:text-neutral-400">
              Separate tags with commas. Remove text to delete a tag.
            </p>
          </div>
          <p v-if="editError" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-200">
            {{ editError }}
          </p>
          <DialogFooter class="pt-2">
            <Button type="button" variant="outline" :disabled="editLoading" @click="editDialogOpen = false">
              Cancel
            </Button>
            <Button type="submit" :disabled="editLoading">
              <Loader2 v-if="editLoading" class="mr-2 h-4 w-4 animate-spin" />
              Save changes
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </div>
</template>



