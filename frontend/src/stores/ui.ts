import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastKind = 'success' | 'error' | 'info' | 'warning'

export interface Toast {
  id: number
  kind: ToastKind
  message: string
}

const THEME_KEY = 'sb-fox-theme'
export type UiTheme = 'light-neutral' | 'dark-neutral'

const DEFAULT_THEME: UiTheme = 'light-neutral'
const DARK_THEME: UiTheme = 'dark-neutral'

function isUiTheme(value: string | null): value is UiTheme {
  return value === DEFAULT_THEME || value === DARK_THEME
}

function saveTheme(next: UiTheme): void {
  try {
    localStorage.setItem(THEME_KEY, next)
  } catch (e) {
    console.warn('sb-fox: unable to save theme preference', e)
  }
}

function readTheme(): UiTheme {
  try {
    const stored = localStorage.getItem(THEME_KEY)
    if (isUiTheme(stored)) return stored
    if (stored) console.warn(`sb-fox: unsupported theme "${stored}", reset to ${DEFAULT_THEME}`)
  } catch (e) {
    console.warn('sb-fox: unable to read theme preference', e)
    return DEFAULT_THEME
  }
  saveTheme(DEFAULT_THEME)
  return DEFAULT_THEME
}

function applyTheme(next: UiTheme): void {
  document.documentElement.setAttribute('data-theme', next)
  document.documentElement.style.colorScheme = next === DARK_THEME ? 'dark' : 'light'
}

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

  const theme = ref<UiTheme>(readTheme())
  function toggleTheme() {
    theme.value = theme.value === DEFAULT_THEME ? DARK_THEME : DEFAULT_THEME
    saveTheme(theme.value)
    applyTheme(theme.value)
  }
  applyTheme(theme.value)

  return { toasts, push, dismiss, success, error, info, theme, toggleTheme }
})
