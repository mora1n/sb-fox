import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post } from '../api/client'
import type { ImportResult, SubscriptionSource } from '../api/types'

export const useSourcesStore = defineStore('sources', () => {
  const sources = ref<SubscriptionSource[]>([])
  const loading = ref(false)

  async function fetchAll(): Promise<void> {
    loading.value = true
    try {
      sources.value = (await get<SubscriptionSource[]>('/sources')) ?? []
    } finally {
      loading.value = false
    }
  }

  async function refresh(id: number): Promise<ImportResult> {
    const r = await post<ImportResult>('/sources/' + id + '/refresh')
    await fetchAll()
    return r
  }

  async function remove(id: number): Promise<void> {
    await del('/sources/' + id)
    sources.value = sources.value.filter((s) => s.id !== id)
  }

  function reset(): void {
    sources.value = []
    loading.value = false
  }

  return { sources, loading, fetchAll, refresh, remove, reset }
})
