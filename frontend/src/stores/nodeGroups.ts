import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { NodeGroup, NodeGroupPayload } from '../api/types'

export const useNodeGroupsStore = defineStore('nodeGroups', () => {
  const groups = ref<NodeGroup[]>([])
  const loading = ref(false)
  const loaded = ref(false)
  let inFlight: Promise<void> | null = null

  async function fetchAll(force = false): Promise<void> {
    if (!force && loaded.value) return
    if (!force && inFlight) return inFlight
    loading.value = true
    inFlight = get<NodeGroup[]>('/node-groups').then((items) => {
      groups.value = items ?? []
      loaded.value = true
    }).finally(() => {
      loading.value = false
      inFlight = null
    })
    return inFlight
  }

  async function create(payload: NodeGroupPayload): Promise<NodeGroup> {
    const g = await post<NodeGroup>('/node-groups', payload)
    await fetchAll(true)
    return g
  }

  async function update(id: number, payload: NodeGroupPayload): Promise<NodeGroup> {
    const g = await put<NodeGroup>('/node-groups/' + id, payload)
    await fetchAll(true)
    return g
  }

  async function remove(id: number): Promise<void> {
    await del('/node-groups/' + id)
    groups.value = groups.value.filter((g) => g.id !== id)
    loaded.value = true
  }

  function reset(): void {
    groups.value = []
    loading.value = false
    loaded.value = false
    inFlight = null
  }

  return { groups, loading, fetchAll, create, update, remove, reset }
})
