import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { Profile, ProfilePayload } from '../api/types'

export const useProfilesStore = defineStore('profiles', () => {
  const profiles = ref<Profile[]>([])
  const subscriptionToken = ref('')
  const loading = ref(false)
  const loaded = ref(false)
  const tokenLoaded = ref(false)
  let inFlight: Promise<void> | null = null
  let tokenInFlight: Promise<string> | null = null

  async function fetchAll(force = false): Promise<void> {
    if (!force && loaded.value) return
    if (!force && inFlight) return inFlight
    loading.value = true
    inFlight = get<Profile[]>('/profiles').then((items) => {
      profiles.value = items ?? []
      loaded.value = true
    }).finally(() => {
      loading.value = false
      inFlight = null
    })
    return inFlight
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

  async function fetchSubscriptionToken(force = false): Promise<string> {
    if (!force && tokenLoaded.value) return subscriptionToken.value
    if (!force && tokenInFlight) return tokenInFlight
    tokenInFlight = get<{ token: string }>('/auth/subscription-token').then((r) => {
      subscriptionToken.value = r.token
      tokenLoaded.value = true
      return r.token
    }).finally(() => {
      tokenInFlight = null
    })
    return tokenInFlight
  }

  async function rotateSubscriptionToken(): Promise<string> {
    const r = await post<{ token: string }>('/auth/subscription-token/rotate')
    subscriptionToken.value = r.token
    tokenLoaded.value = true
    return r.token
  }

  function reset(): void {
    profiles.value = []
    subscriptionToken.value = ''
    loading.value = false
    loaded.value = false
    tokenLoaded.value = false
    inFlight = null
    tokenInFlight = null
  }

  return {
    profiles,
    subscriptionToken,
    loading,
    fetchAll,
    getOne,
    create,
    update,
    setSubscriptionEnabled,
    remove,
    fetchSubscriptionToken,
    rotateSubscriptionToken,
    reset,
  }
})
