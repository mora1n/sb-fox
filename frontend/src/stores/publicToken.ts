import { defineStore } from 'pinia'
import { ref } from 'vue'
import { get, post } from '../api/client'

export const usePublicTokenStore = defineStore('publicToken', () => {
  const token = ref('')
  const loaded = ref(false)
  let inFlight: Promise<string> | null = null

  async function fetchToken(force = false): Promise<string> {
    if (!force && loaded.value) return token.value
    if (!force && inFlight) return inFlight
    inFlight = get<{ token: string }>('/auth/subscription-token').then((result) => {
      token.value = result.token
      loaded.value = true
      return result.token
    }).finally(() => {
      inFlight = null
    })
    return inFlight
  }

  async function rotate(): Promise<string> {
    const result = await post<{ token: string }>('/auth/subscription-token/rotate')
    token.value = result.token
    loaded.value = true
    return result.token
  }

  function reset(): void {
    token.value = ''
    loaded.value = false
    inFlight = null
  }

  return { token, fetchToken, rotate, reset }
})
