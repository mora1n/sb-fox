import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { NodeGroup, NodeGroupPayload } from '../api/types'

export const useNodeGroupsStore = defineStore('nodeGroups', () => {
  const groups = ref<NodeGroup[]>([])
  const loading = ref(false)

  async function fetchAll(): Promise<void> {
    loading.value = true
    try {
      groups.value = (await get<NodeGroup[]>('/node-groups')) ?? []
    } finally {
      loading.value = false
    }
  }

  async function create(payload: NodeGroupPayload): Promise<NodeGroup> {
    const g = await post<NodeGroup>('/node-groups', payload)
    await fetchAll()
    return g
  }

  async function update(id: number, payload: NodeGroupPayload): Promise<NodeGroup> {
    const g = await put<NodeGroup>('/node-groups/' + id, payload)
    await fetchAll()
    return g
  }

  async function remove(id: number): Promise<void> {
    await del('/node-groups/' + id)
    groups.value = groups.value.filter((g) => g.id !== id)
  }

  return { groups, loading, fetchAll, create, update, remove }
})
