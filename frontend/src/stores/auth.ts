import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { get, post } from '../api/client'
import type { User } from '../api/types'
import { resetSessionStores } from './session'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const checked = ref(false) // whether we've called /auth/me at least once

  const username = computed(() => user.value?.username ?? null)
  const isAuthenticated = computed(() => user.value !== null)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function me(): Promise<boolean> {
    try {
      const next = await get<User>('/auth/me', true)
      if (user.value?.id !== next.id) resetSessionStores()
      user.value = next
    } catch {
      clearSessionState()
    } finally {
      checked.value = true
    }
    return isAuthenticated.value
  }

  async function login(u: string, p: string): Promise<void> {
    const next = await post<User>('/auth/login', { username: u, password: p })
    resetSessionStores()
    user.value = next
    checked.value = true
  }

  async function register(u: string, p: string): Promise<void> {
    const next = await post<User>('/auth/register', { username: u, password: p })
    resetSessionStores()
    user.value = next
    checked.value = true
  }

  async function logout(): Promise<void> {
    try {
      await post('/auth/logout')
    } finally {
      clearSessionState()
      checked.value = true
    }
  }

  function clearSessionState(): void {
    user.value = null
    resetSessionStores()
  }

  async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
    await post('/auth/password', { old_password: oldPassword, new_password: newPassword })
  }

  return { user, username, checked, isAuthenticated, isAdmin, me, login, register, logout, clearSessionState, changePassword }
})
