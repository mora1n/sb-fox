import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { User, UserRole } from '../api/types'

export interface UserPayload {
  username: string
  password?: string
  role: UserRole
  node_limit: number
  profile_limit: number
  template_limit: number
}

export const useUsersStore = defineStore('users', () => {
  const users = ref<User[]>([])
  const loading = ref(false)
  const loaded = ref(false)
  let inFlight: Promise<void> | null = null

  async function fetchAll(force = false): Promise<void> {
    if (!force && loaded.value) return
    if (!force && inFlight) return inFlight
    loading.value = true
    inFlight = get<User[]>('/users').then((items) => {
      users.value = items ?? []
      loaded.value = true
    }).finally(() => {
      loading.value = false
      inFlight = null
    })
    return inFlight
  }

  async function create(payload: UserPayload): Promise<User> {
    const u = await post<User>('/users', payload)
    await fetchAll(true)
    return u
  }

  async function update(id: number, payload: UserPayload): Promise<User> {
    const u = await put<User>('/users/' + id, payload)
    await fetchAll(true)
    return u
  }

  async function remove(id: number): Promise<void> {
    await del('/users/' + id)
    users.value = users.value.filter((u) => u.id !== id)
  }

  async function resetPassword(id: number): Promise<string> {
    const r = await post<{ password: string }>('/users/' + id + '/reset-password')
    return r.password
  }

  function reset(): void {
    users.value = []
    loading.value = false
    loaded.value = false
    inFlight = null
  }

  return { users, loading, fetchAll, create, update, remove, resetPassword, reset }
})
