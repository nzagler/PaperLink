import { computed, ref } from 'vue'
import { fetchSearchIndexFromTree, type SearchIndexItem } from '@/lib/search_api'

export type Scope = 'all' | 'mine' | 'shared'
export type Sort = 'relevance' | 'recent' | 'az'

export interface SearchResult {
  id: string
  title: string
  description: string
  tags: string[]
  pages: number
  size: string
  updatedAt: string
  owner: string
  shared: boolean
  path: string
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function pathToTags(path: string) {
  const normalized = (path || '').trim().replace(/^\/+|\/+$/g, '')
  if (!normalized) return ['Root']
  return normalized.split('/').filter(Boolean)
}

export function useSearchView() {
  const searchQuery = ref('')
  const selectedScope = ref<Scope>('all')
  const selectedSort = ref<Sort>('relevance')
  const selectedTags = ref<string[]>([])
  const tagSearch = ref('')
  const isLoading = ref(false)
  const showAllTags = ref(false)
  const loadError = ref<string | null>(null)
  const results = ref<SearchResult[]>([])

  async function loadFromBackend() {
    isLoading.value = true
    loadError.value = null
    try {
      const items: SearchIndexItem[] = await fetchSearchIndexFromTree()
      results.value = items.map((it) => ({
        id: it.id,
        title: it.title,
        description: it.path ? `Folder: ${it.path}` : 'Folder: /',
        tags: pathToTags(it.path),
        pages: it.pageCount ?? 0,
        size: formatBytes(it.sizeBytes),
        updatedAt: '',
        owner: 'You',
        shared: false,
        path: it.path,
      }))
    } catch (e: any) {
      loadError.value = e?.message ?? 'Failed to load documents'
    } finally {
      isLoading.value = false
    }
  }

  const tags = computed(() => {
    const set = new Set<string>()
    for (const r of results.value) {
      for (const t of r.tags) set.add(t)
    }
    return Array.from(set).sort((a, b) => a.localeCompare(b))
  })

  const filteredTags = computed(() => {
    const q = tagSearch.value.trim().toLowerCase()
    const all = tags.value
    if (!q) return all
    return all.filter((t) => t.toLowerCase().includes(q))
  })

  const filteredResults = computed(() => {
    let list = [...results.value]

    const q = searchQuery.value.trim().toLowerCase()
    if (q) {
      list = list.filter(
        (item) =>
          item.title.toLowerCase().includes(q) ||
          item.description.toLowerCase().includes(q) ||
          item.tags.some((t) => t.toLowerCase().includes(q)),
      )
    }

    if (selectedScope.value === 'shared') {
      list = list.filter((item) => item.shared)
    }

    if (selectedTags.value.length) {
      list = list.filter((item) => selectedTags.value.every((tag) => item.tags.includes(tag)))
    }

    if (selectedSort.value === 'az') {
      list.sort((a, b) => a.title.localeCompare(b.title))
    }

    return list
  })

  function toggleTag(tag: string) {
    if (selectedTags.value.includes(tag)) {
      selectedTags.value = selectedTags.value.filter((t) => t !== tag)
    } else {
      selectedTags.value = [...selectedTags.value, tag]
    }
  }

  function resetFilters() {
    selectedScope.value = 'all'
    selectedSort.value = 'relevance'
    selectedTags.value = []
    tagSearch.value = ''
    showAllTags.value = false
  }

  function onSearch() {
  }

  return {
    searchQuery,
    selectedScope,
    selectedSort,
    selectedTags,
    tagSearch,
    isLoading,
    showAllTags,
    loadError,
    filteredTags,
    filteredResults,
    loadFromBackend,
    onSearch,
    toggleTag,
    resetFilters,
  }
}
