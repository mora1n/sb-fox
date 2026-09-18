import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { ImportResult, SubscriptionSource } from '../api/types'

export const useSourcesStore = defineStore('sources', () => {
  const sources = ref<SubscriptionSource[]>([])
  const loading = ref(false)
  const loaded = ref(false)
  let inFlight: Promise<void> | null = null

  async function fetchAll(force = false): Promise<void> {
    if (!force && loaded.value) return
    if (!force && inFlight) return inFlight
    loading.value = true
    inFlight = get<SubscriptionSource[]>('/sources').then((items) => {
      sources.value = items ?? []
      loaded.value = true
    }).finally(() => {
      loading.value = false
      inFlight = null
    })
    return inFlight
  }

  async function refresh(id: number): Promise<ImportResult> {
    try {
      return await post<ImportResult>('/sources/' + id + '/refresh')
    } finally {
      await fetchAll(true).catch(() => undefined)
    }
  }

  async function updateSchedule(id: number, autoRefresh: boolean, refreshIntervalMinutes: number): Promise<void> {
    const updated = await put<SubscriptionSource>('/sources/' + id + '/schedule', {
      auto_refresh: autoRefresh,
      refresh_interval_minutes: refreshIntervalMinutes,
    })
    const index = sources.value.findIndex((source) => source.id === id)
    if (index >= 0) sources.value[index] = updated
  }

  async function remove(id: number): Promise<void> {
    await del('/sources/' + id)
    sources.value = sources.value.filter((s) => s.id !== id)
    loaded.value = true
  }

  function reset(): void {
    sources.value = []
    loading.value = false
    loaded.value = false
    inFlight = null
  }

  return { sources, loading, fetchAll, refresh, updateSchedule, remove, reset }
})
