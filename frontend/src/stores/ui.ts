import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastKind = 'success' | 'error' | 'info' | 'warning'

export interface Toast {
  id: number
  kind: ToastKind
  message: string
}

const THEME_KEY = 'sb-fox-theme'

export const useUiStore = defineStore('ui', () => {
  const toasts = ref<Toast[]>([])
  let seq = 0

  function push(kind: ToastKind, message: string) {
    const id = ++seq
    toasts.value.push({ id, kind, message })
    setTimeout(() => dismiss(id), 4000)
  }
  function dismiss(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }
  const success = (m: string) => push('success', m)
  const error = (m: string) => push('error', m)
  const info = (m: string) => push('info', m)

  const theme = ref<string>(localStorage.getItem(THEME_KEY) || 'light-neutral')
  function applyTheme() {
    document.documentElement.setAttribute('data-theme', theme.value)
  }
  function toggleTheme() {
    theme.value = theme.value === 'light-neutral' ? 'dark-neutral' : 'light-neutral'
    localStorage.setItem(THEME_KEY, theme.value)
    applyTheme()
  }
  applyTheme()

  return { toasts, push, dismiss, success, error, info, theme, toggleTheme }
})
