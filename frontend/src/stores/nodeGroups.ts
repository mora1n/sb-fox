import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { BulkDeleteResult, NodeGroup, NodeGroupPayload } from '../api/types'

export const useNodeGroupsStore = defineStore('nodeGroups', () => {
  const groups = ref<NodeGroup[]>([])
  const loading = ref(false)
  const loaded = ref(false)
  let inFlight: Promise<void> | null = null

  function normalizeGroup(group: NodeGroup): NodeGroup {
    return {
      ...group,
      node_ids: Array.isArray(group.node_ids) ? group.node_ids : [],
    }
  }

  function normalizeGroups(items: NodeGroup[] | null | undefined): NodeGroup[] {
    return (items ?? []).map(normalizeGroup)
  }

  async function fetchAll(force = false): Promise<void> {
    if (!force && loaded.value) return
    if (!force && inFlight) return inFlight
    loading.value = true
    inFlight = get<NodeGroup[]>('/node-groups').then((items) => {
      groups.value = normalizeGroups(items)
      loaded.value = true
    }).finally(() => {
      loading.value = false
      inFlight = null
    })
    return inFlight
  }

  async function create(payload: NodeGroupPayload): Promise<NodeGroup> {
    const g = normalizeGroup(await post<NodeGroup>('/node-groups', payload))
    await fetchAll(true)
    return g
  }

  async function update(id: number, payload: NodeGroupPayload): Promise<NodeGroup> {
    const g = normalizeGroup(await put<NodeGroup>('/node-groups/' + id, payload))
    await fetchAll(true)
    return g
  }

  async function remove(id: number): Promise<void> {
    await del('/node-groups/' + id)
    groups.value = groups.value.filter((g) => g.id !== id)
    loaded.value = true
  }

  async function bulkDelete(ids: number[]): Promise<number> {
    const r = await post<BulkDeleteResult>('/node-groups/bulk-delete', { ids })
    const idSet = new Set(ids)
    groups.value = groups.value.filter((g) => !idSet.has(g.id))
    loaded.value = true
    return r.deleted
  }

  function reset(): void {
    groups.value = []
    loading.value = false
    loaded.value = false
    inFlight = null
  }

  return { groups, loading, fetchAll, create, update, remove, bulkDelete, reset }
})
