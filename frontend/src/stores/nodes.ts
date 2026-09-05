import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { BulkDeleteResult, BulkNodeUsageResult, ImportPreviewResult, ImportResult, Node, NodeSummary, NodeUsage } from '../api/types'
import { useNodeGroupsStore } from './nodeGroups'
import { useProfilesStore } from './profiles'
import { useSettingsStore } from './settings'
import { nodeCountries, nodeTypes } from '../utils/nodeFilters'

export interface NodeFilters {
  search: string
  source: string
  country: string
  type: string
}

export interface NodeInput {
  raw: string
  country_code?: string
  country_source?: string
}

function defaultFilters(): NodeFilters {
  return { search: '', source: '', country: '', type: '' }
}

export const useNodesStore = defineStore('nodes', () => {
  const settings = useSettingsStore()
  const nodeGroups = useNodeGroupsStore()
  const profiles = useProfilesStore()
  const nodes = ref<NodeSummary[]>([])
  const unfilteredNodes = ref<NodeSummary[]>([])
  const summaryNodes = ref<NodeSummary[]>([])
  const loading = ref(false)
  const filters = ref<NodeFilters>(defaultFilters())
  const unfilteredLoaded = ref(false)
  const summaryLoaded = ref(false)
  const queryCache = new Map<string, NodeSummary[]>()
  const queryInFlight = new Map<string, Promise<NodeSummary[]>>()
  const fullNodes = new Map<number, Node>()
  const fullNodeInFlight = new Map<number, Promise<Node>>()
  let cacheVersion = 0
  let detailVersion = 0
  let loadingToken = 0

  function buildQuery(): string {
    const p = new URLSearchParams()
    if (filters.value.search) p.set('search', filters.value.search)
    if (filters.value.source) p.set('source', filters.value.source)
    if (filters.value.country) p.set('country', filters.value.country)
    if (filters.value.type) p.set('type', filters.value.type)
    const q = p.toString()
    return q ? '?' + q : ''
  }

  function endpointForQuery(query: string): string {
    return '/nodes?summary=1' + (query ? '&' + query.slice(1) : '')
  }

  async function fetchQuery(query: string, force = false): Promise<NodeSummary[]> {
    if (!force && queryCache.has(query)) return queryCache.get(query) ?? []
    if (!force && queryInFlight.has(query)) return queryInFlight.get(query) ?? Promise.resolve([])
    const version = cacheVersion
    const req = get<NodeSummary[]>(endpointForQuery(query)).then((items) => {
      const next = items ?? []
      if (version !== cacheVersion) return next
      queryCache.set(query, next)
      if (!query) {
        unfilteredNodes.value = next
        summaryNodes.value = next
        unfilteredLoaded.value = true
        summaryLoaded.value = true
      }
      return next
    }).finally(() => {
      queryInFlight.delete(query)
    })
    queryInFlight.set(query, req)
    return req
  }

  async function fetchAll(force = false): Promise<void> {
    const query = buildQuery()
    const version = cacheVersion
    const showLoading = force || !queryCache.has(query)
    const token = showLoading ? ++loadingToken : 0
    if (showLoading) loading.value = true
    try {
      const items = await fetchQuery(query, force)
      if (version === cacheVersion) nodes.value = items
    } finally {
      if (showLoading && token === loadingToken) loading.value = false
    }
  }

  async function fetchUnfiltered(force = false): Promise<NodeSummary[]> {
    if (!force && unfilteredLoaded.value) return unfilteredNodes.value
    const version = cacheVersion
    const items = await fetchQuery('', force)
    if (version !== cacheVersion) return items
    unfilteredNodes.value = items
    summaryNodes.value = items
    unfilteredLoaded.value = true
    summaryLoaded.value = true
    if (!buildQuery()) nodes.value = items
    return items
  }

  async function fetchSummary(force = false): Promise<NodeSummary[]> {
    if (!force && summaryLoaded.value) return summaryNodes.value
    const version = cacheVersion
    const items = await fetchQuery('', force)
    if (version !== cacheVersion) return items
    summaryNodes.value = items
    unfilteredNodes.value = items
    summaryLoaded.value = true
    unfilteredLoaded.value = true
    return items
  }

  async function getOne(id: number, force = false): Promise<Node> {
    if (!force && fullNodes.has(id)) return fullNodes.get(id) as Node
    if (!force && fullNodeInFlight.has(id)) return fullNodeInFlight.get(id) as Promise<Node>
    const version = detailVersion
    const req = get<Node>('/nodes/' + id).then((node) => {
      if (version === detailVersion) fullNodes.set(id, node)
      return node
    }).finally(() => {
      fullNodeInFlight.delete(id)
    })
    fullNodeInFlight.set(id, req)
    return req
  }

  async function prefetchOne(id: number): Promise<void> {
    await getOne(id)
  }

  function invalidateListCache(): void {
    cacheVersion++
    queryCache.clear()
    queryInFlight.clear()
    unfilteredLoaded.value = false
    summaryLoaded.value = false
  }

  function removeFromLoadedState(id: number): void {
    removeManyFromLoadedState([id])
  }

  function removeManyFromLoadedState(ids: number[]): void {
    const idSet = new Set(ids)
    nodes.value = nodes.value.filter((n) => !idSet.has(n.id))
    unfilteredNodes.value = unfilteredNodes.value.filter((n) => !idSet.has(n.id))
    summaryNodes.value = summaryNodes.value.filter((n) => !idSet.has(n.id))
    for (const id of idSet) {
      fullNodes.delete(id)
      fullNodeInFlight.delete(id)
    }
    detailVersion++
    for (const [key, items] of queryCache) {
      queryCache.set(key, items.filter((n) => !idSet.has(n.id)))
    }
    nodeGroups.removeNodeIDs(ids)
    profiles.invalidate()
  }

  async function previewBulkDelete(ids: number[]): Promise<BulkNodeUsageResult> {
    return post<BulkNodeUsageResult>('/nodes/bulk-delete/preview', { ids })
  }

  async function bulkDelete(ids: number[]): Promise<number> {
    const r = await post<BulkDeleteResult>('/nodes/bulk-delete', { ids })
    removeManyFromLoadedState(ids)
    return r.deleted
  }

  async function refreshAfterMutation(): Promise<void> {
    const refreshSummary = summaryLoaded.value
    const refreshUnfiltered = unfilteredLoaded.value
    const query = buildQuery()
    invalidateListCache()
    await fetchAll(true)
    if (refreshUnfiltered && query) await fetchUnfiltered(true)
    if (refreshSummary) await fetchSummary(true)
  }

  async function create(input: NodeInput): Promise<Node> {
    const n = await post<Node>('/nodes', input)
    detailVersion++
    fullNodes.set(n.id, n)
    await refreshAfterMutation()
    return n
  }

  async function update(id: number, input: NodeInput): Promise<Node> {
    const n = await put<Node>('/nodes/' + id, input)
    detailVersion++
    fullNodes.set(n.id, n)
    fullNodeInFlight.delete(id)
    await refreshAfterMutation()
    return n
  }

  async function remove(id: number): Promise<void> {
    await del('/nodes/' + id)
    removeFromLoadedState(id)
  }

  async function usage(id: number): Promise<NodeUsage[]> {
    return (await get<NodeUsage[]>('/nodes/' + id + '/usage')) ?? []
  }

  async function importLinks(links: string): Promise<ImportResult> {
    const r = await post<ImportResult>('/nodes/import/links', { links })
    await refreshAfterMutation()
    return r
  }

  async function previewImportLinks(links: string): Promise<ImportPreviewResult> {
    return post<ImportPreviewResult>('/nodes/import/links/preview', { links })
  }

  async function importSubscription(name: string, url: string): Promise<ImportResult> {
    const r = await post<ImportResult>('/nodes/import/subscription', { name, url })
    await refreshAfterMutation()
    return r
  }

  async function previewImportSubscription(url: string): Promise<ImportPreviewResult> {
    return post<ImportPreviewResult>('/nodes/import/subscription/preview', { url })
  }

  async function importConfig(config: string): Promise<ImportResult> {
    const r = await post<ImportResult>('/nodes/import/config', { config })
    await refreshAfterMutation()
    return r
  }

  async function previewImportConfig(config: string): Promise<ImportPreviewResult> {
    return post<ImportPreviewResult>('/nodes/import/config/preview', { config })
  }

  async function refreshCountry(nodeIds: number[]): Promise<number> {
    const r = await post<{ updated: number }>('/nodes/refresh-country', { node_ids: nodeIds })
    await refreshAfterMutation()
    return r.updated
  }

  // distinct filter option values derived from the loaded set
  const filterOptionNodes = computed(() => (unfilteredNodes.value.length ? unfilteredNodes.value : nodes.value))
  const countries = computed(() => nodeCountries(filterOptionNodes.value, settings.countryHeatOrder))
  const types = computed(() => {
    return nodeTypes(filterOptionNodes.value)
  })

  function reset(): void {
    nodes.value = []
    unfilteredNodes.value = []
    summaryNodes.value = []
    loading.value = false
    filters.value = defaultFilters()
    unfilteredLoaded.value = false
    summaryLoaded.value = false
    cacheVersion++
    detailVersion++
    loadingToken++
    queryCache.clear()
    queryInFlight.clear()
    fullNodes.clear()
    fullNodeInFlight.clear()
  }

  return {
    nodes,
    unfilteredNodes,
    summaryNodes,
    loading,
    filters,
    countries,
    types,
    fetchAll,
    fetchUnfiltered,
    fetchSummary,
    getOne,
    prefetchOne,
    create,
    update,
    remove,
    previewBulkDelete,
    bulkDelete,
    removeManyFromLoadedState,
    usage,
    importLinks,
    previewImportLinks,
    importSubscription,
    previewImportSubscription,
    importConfig,
    previewImportConfig,
    refreshCountry,
    reset,
  }
})
