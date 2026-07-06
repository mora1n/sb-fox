import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { ImportPreviewResult, ImportResult, Node, NodeSummary, NodeUsage } from '../api/types'
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
  const nodes = ref<Node[]>([])
  const unfilteredNodes = ref<Node[]>([])
  const summaryNodes = ref<NodeSummary[]>([])
  const loading = ref(false)
  const filters = ref<NodeFilters>(defaultFilters())
  const unfilteredLoaded = ref(false)
  const summaryLoaded = ref(false)
  let unfilteredInFlight: Promise<Node[]> | null = null
  let summaryInFlight: Promise<NodeSummary[]> | null = null

  function buildQuery(): string {
    const p = new URLSearchParams()
    if (filters.value.search) p.set('search', filters.value.search)
    if (filters.value.source) p.set('source', filters.value.source)
    if (filters.value.country) p.set('country', filters.value.country)
    if (filters.value.type) p.set('type', filters.value.type)
    const q = p.toString()
    return q ? '?' + q : ''
  }

  async function fetchAll(): Promise<void> {
    loading.value = true
    try {
      const query = buildQuery()
      if (!query && unfilteredLoaded.value) {
        nodes.value = unfilteredNodes.value
        return
      }
      nodes.value = (await get<Node[]>('/nodes' + query)) ?? []
      if (!query) {
        unfilteredNodes.value = nodes.value
        unfilteredLoaded.value = true
      }
    } finally {
      loading.value = false
    }
  }

  async function fetchUnfiltered(force = false): Promise<Node[]> {
    if (!force && unfilteredLoaded.value) return unfilteredNodes.value
    if (!force && unfilteredInFlight) return unfilteredInFlight
    unfilteredInFlight = get<Node[]>('/nodes').then((items) => {
      unfilteredNodes.value = items ?? []
      unfilteredLoaded.value = true
      if (!buildQuery()) nodes.value = unfilteredNodes.value
      return unfilteredNodes.value
    }).finally(() => {
      unfilteredInFlight = null
    })
    return unfilteredInFlight
  }

  async function fetchSummary(force = false): Promise<NodeSummary[]> {
    if (!force && summaryLoaded.value) return summaryNodes.value
    if (!force && summaryInFlight) return summaryInFlight
    summaryInFlight = get<NodeSummary[]>('/nodes?summary=1').then((items) => {
      summaryNodes.value = items ?? []
      summaryLoaded.value = true
      return summaryNodes.value
    }).finally(() => {
      summaryInFlight = null
    })
    return summaryInFlight
  }

  async function refreshAfterMutation(): Promise<void> {
    const refreshSummary = summaryLoaded.value
    const refreshUnfiltered = unfilteredLoaded.value
    unfilteredLoaded.value = false
    await fetchAll()
    if (refreshUnfiltered && buildQuery()) await fetchUnfiltered(true)
    if (refreshSummary) await fetchSummary(true)
  }

  async function create(input: NodeInput): Promise<Node> {
    const n = await post<Node>('/nodes', input)
    await refreshAfterMutation()
    return n
  }

  async function update(id: number, input: NodeInput): Promise<Node> {
    const n = await put<Node>('/nodes/' + id, input)
    await refreshAfterMutation()
    return n
  }

  async function remove(id: number): Promise<void> {
    await del('/nodes/' + id)
    nodes.value = nodes.value.filter((n) => n.id !== id)
    unfilteredNodes.value = unfilteredNodes.value.filter((n) => n.id !== id)
    summaryNodes.value = summaryNodes.value.filter((n) => n.id !== id)
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
    unfilteredInFlight = null
    summaryInFlight = null
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
    create,
    update,
    remove,
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
