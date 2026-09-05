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

  async function remove(id: number, deleteNodes = false): Promise<BulkDeleteResult> {
    const suffix = deleteNodes ? '?delete_nodes=true' : ''
    const result = await del<BulkDeleteResult & { ok?: boolean }>('/node-groups/' + id + suffix)
    groups.value = groups.value.filter((g) => g.id !== id)
    loaded.value = true
    return result
  }

  async function bulkDelete(ids: number[], deleteNodes = false): Promise<BulkDeleteResult> {
    const r = await post<BulkDeleteResult>('/node-groups/bulk-delete', { ids, delete_nodes: deleteNodes })
    const idSet = new Set(ids)
    groups.value = groups.value.filter((g) => !idSet.has(g.id))
    loaded.value = true
    return r
  }

  function removeNodeIDs(ids: number[]): void {
    const idSet = new Set(ids)
    groups.value = groups.value.flatMap((group) => {
      const nodeIDs = group.node_ids.filter((id) => !idSet.has(id))
      if (group.node_ids.length > 0 && nodeIDs.length === 0) return []
      return [{ ...group, node_ids: nodeIDs }]
    })
  }

  function reset(): void {
    groups.value = []
    loading.value = false
    loaded.value = false
    inFlight = null
  }

  return { groups, loading, fetchAll, create, update, remove, bulkDelete, removeNodeIDs, reset }
})
