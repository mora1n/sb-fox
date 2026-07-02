import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { ImportResult, Node } from '../api/types'
import { useSettingsStore } from './settings'
import { sortCountryCodes } from '../utils/countries'

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

export const useNodesStore = defineStore('nodes', () => {
  const settings = useSettingsStore()
  const nodes = ref<Node[]>([])
  const loading = ref(false)
  const filters = ref<NodeFilters>({ search: '', source: '', country: '', type: '' })

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
      nodes.value = (await get<Node[]>('/nodes' + buildQuery())) ?? []
    } finally {
      loading.value = false
    }
  }

  async function create(input: NodeInput): Promise<Node> {
    const n = await post<Node>('/nodes', input)
    await fetchAll()
    return n
  }

  async function update(id: number, input: NodeInput): Promise<Node> {
    const n = await put<Node>('/nodes/' + id, input)
    await fetchAll()
    return n
  }

  async function remove(id: number): Promise<void> {
    await del('/nodes/' + id)
    nodes.value = nodes.value.filter((n) => n.id !== id)
  }

  async function importLinks(links: string): Promise<ImportResult> {
    const r = await post<ImportResult>('/nodes/import/links', { links })
    await fetchAll()
    return r
  }

  async function importSubscription(name: string, url: string): Promise<ImportResult> {
    const r = await post<ImportResult>('/nodes/import/subscription', { name, url })
    await fetchAll()
    return r
  }

  async function importConfig(config: string): Promise<ImportResult> {
    const r = await post<ImportResult>('/nodes/import/config', { config })
    await fetchAll()
    return r
  }

  async function refreshCountry(nodeIds: number[]): Promise<number> {
    const r = await post<{ updated: number }>('/nodes/refresh-country', { node_ids: nodeIds })
    await fetchAll()
    return r.updated
  }

  // distinct filter option values derived from the loaded set
  const countries = computed(() =>
    sortCountryCodes([...new Set(nodes.value.map((n) => n.country_code).filter(Boolean))], settings.countryHeatOrder),
  )
  const types = computed(() => [...new Set(nodes.value.map((n) => n.type).filter(Boolean))].sort())

  return {
    nodes,
    loading,
    filters,
    countries,
    types,
    fetchAll,
    create,
    update,
    remove,
    importLinks,
    importSubscription,
    importConfig,
    refreshCountry,
  }
})
