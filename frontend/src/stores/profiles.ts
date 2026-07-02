import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { Profile, ProfilePayload } from '../api/types'

export const useProfilesStore = defineStore('profiles', () => {
  const profiles = ref<Profile[]>([])
  const loading = ref(false)

  async function fetchAll(): Promise<void> {
    loading.value = true
    try {
      profiles.value = (await get<Profile[]>('/profiles')) ?? []
    } finally {
      loading.value = false
    }
  }

  async function create(payload: ProfilePayload): Promise<Profile> {
    const p = await post<Profile>('/profiles', payload)
    await fetchAll()
    return p
  }

  async function update(id: number, payload: ProfilePayload): Promise<Profile> {
    const p = await put<Profile>('/profiles/' + id, payload)
    await fetchAll()
    return p
  }

  async function remove(id: number): Promise<void> {
    await del('/profiles/' + id)
    profiles.value = profiles.value.filter((p) => p.id !== id)
  }

  async function rotateToken(id: number): Promise<string> {
    const r = await post<{ token: string }>('/profiles/' + id + '/rotate-token')
    await fetchAll()
    return r.token
  }

  return { profiles, loading, fetchAll, create, update, remove, rotateToken }
})
