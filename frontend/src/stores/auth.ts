import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { get, post } from '../api/client'

interface MeResponse {
  username: string
}

export const useAuthStore = defineStore('auth', () => {
  const username = ref<string | null>(null)
  const checked = ref(false) // whether we've called /auth/me at least once

  const isAuthenticated = computed(() => username.value !== null)

  async function me(): Promise<boolean> {
    try {
      const data = await get<MeResponse>('/auth/me', true)
      username.value = data.username
    } catch {
      username.value = null
    } finally {
      checked.value = true
    }
    return isAuthenticated.value
  }

  async function login(u: string, p: string): Promise<void> {
    const data = await post<MeResponse>('/auth/login', { username: u, password: p })
    username.value = data.username
    checked.value = true
  }

  async function logout(): Promise<void> {
    try {
      await post('/auth/logout')
    } finally {
      username.value = null
    }
  }

  async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
    await post('/auth/password', { old_password: oldPassword, new_password: newPassword })
  }

  return { username, checked, isAuthenticated, me, login, logout, changePassword }
})
