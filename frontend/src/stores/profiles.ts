import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { BulkDeleteResult, Profile, ProfilePayload } from '../api/types'

export const useProfilesStore = defineStore('profiles', () => {
  const profiles = ref<Profile[]>([])
  const loading = ref(false)
  const loaded = ref(false)
  let inFlight: Promise<void> | null = null
  let listVersion = 0

  async function fetchAll(force = false): Promise<void> {
    if (!force && loaded.value) return
    if (!force && inFlight) return inFlight
    const version = listVersion
    loading.value = true
    const request = get<Profile[]>('/profiles').then((items) => {
      if (version !== listVersion) return
      profiles.value = items ?? []
      loaded.value = true
    }).finally(() => {
      if (inFlight === request) {
        loading.value = false
        inFlight = null
      }
    })
    inFlight = request
    return request
  }

  async function getOne(id: number): Promise<Profile> {
    return get<Profile>('/profiles/' + id)
  }

  async function create(payload: ProfilePayload): Promise<Profile> {
    const p = await post<Profile>('/profiles', payload)
    await fetchAll(true)
    return p
  }

  async function update(id: number, payload: ProfilePayload): Promise<Profile> {
    const p = await put<Profile>('/profiles/' + id, payload)
    await fetchAll(true)
    return p
  }

  async function setSubscriptionEnabled(id: number, enabled: boolean): Promise<Profile> {
    const p = await put<Profile>('/profiles/' + id + '/subscription-enabled', { subscription_enabled: enabled })
    profiles.value = profiles.value.map((item) => (item.id === id ? { ...item, subscription_enabled: p.subscription_enabled } : item))
    return p
  }

  async function remove(id: number): Promise<void> {
    await del('/profiles/' + id)
    profiles.value = profiles.value.filter((p) => p.id !== id)
    loaded.value = true
  }

  async function bulkDelete(ids: number[]): Promise<number> {
    const r = await post<BulkDeleteResult>('/profiles/bulk-delete', { ids })
    const idSet = new Set(ids)
    profiles.value = profiles.value.filter((p) => !idSet.has(p.id))
    loaded.value = true
    return r.deleted
  }

  function invalidate(): void {
    listVersion++
    loaded.value = false
    loading.value = false
    inFlight = null
  }

  function reset(): void {
    profiles.value = []
    loading.value = false
    loaded.value = false
    inFlight = null
    listVersion++
  }

  return {
    profiles,
    loading,
    fetchAll,
    getOne,
    create,
    update,
    setSubscriptionEnabled,
    remove,
    bulkDelete,
    invalidate,
    reset,
  }
})
