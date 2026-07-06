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
export type UiThemeMode = 'system' | UiTheme

const SYSTEM_THEME = 'system'
const DEFAULT_THEME: UiTheme = 'light-neutral'
const DARK_THEME: UiTheme = 'dark-neutral'
const THEME_COLORS: Record<UiTheme, { bg: string; fg: string; scheme: 'light' | 'dark'; themeColor: string }> = {
  'light-neutral': { bg: '#f5f5f7', fg: '#27272a', scheme: 'light', themeColor: '#f5f5f7' },
  'dark-neutral': { bg: '#1a1a1a', fg: '#e5e5e5', scheme: 'dark', themeColor: '#1a1a1a' },
}

function isUiThemeMode(value: string | null): value is UiThemeMode {
  return value === SYSTEM_THEME || value === DEFAULT_THEME || value === DARK_THEME
}

function saveThemeMode(next: UiThemeMode): void {
  try {
    localStorage.setItem(THEME_KEY, next)
  } catch (e) {
    console.warn('sb-fox: unable to save theme preference', e)
  }
}

function readThemeMode(): UiThemeMode {
  try {
    const stored = localStorage.getItem(THEME_KEY)
    if (isUiThemeMode(stored)) return stored
    if (stored) console.warn(`sb-fox: unsupported theme "${stored}", reset to ${SYSTEM_THEME}`)
  } catch (e) {
    console.warn('sb-fox: unable to read theme preference', e)
    return SYSTEM_THEME
  }
  saveThemeMode(SYSTEM_THEME)
  return SYSTEM_THEME
}

function systemTheme(): UiTheme {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? DARK_THEME : DEFAULT_THEME
}

function resolveTheme(mode: UiThemeMode): UiTheme {
  return mode === SYSTEM_THEME ? systemTheme() : mode
}

function applyTheme(next: UiTheme): void {
  const colors = THEME_COLORS[next]
  const root = document.documentElement
  const app = document.getElementById('app')
  root.setAttribute('data-theme', next)
  root.style.colorScheme = colors.scheme
  root.style.setProperty('--sb-theme-bg', colors.bg)
  root.style.setProperty('--sb-theme-fg', colors.fg)
  root.style.backgroundColor = colors.bg
  root.style.color = colors.fg
  document.body.style.backgroundColor = colors.bg
  document.body.style.color = colors.fg
  if (app) {
    app.style.backgroundColor = colors.bg
    app.style.color = colors.fg
  }
  document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')?.setAttribute('content', colors.themeColor)
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

  const themeMode = ref<UiThemeMode>(readThemeMode())
  const effectiveTheme = ref<UiTheme>(resolveTheme(themeMode.value))

  function applyEffectiveTheme(): void {
    const next = resolveTheme(themeMode.value)
    effectiveTheme.value = next
    applyTheme(next)
  }

  function setThemeMode(next: UiThemeMode): void {
    themeMode.value = next
    saveThemeMode(next)
    applyEffectiveTheme()
  }

  function toggleTheme(): void {
    setThemeMode(effectiveTheme.value === DEFAULT_THEME ? DARK_THEME : DEFAULT_THEME)
  }

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (themeMode.value === SYSTEM_THEME) applyEffectiveTheme()
  })
  applyTheme(effectiveTheme.value)

  return {
    toasts,
    push,
    dismiss,
    success,
    error,
    info,
    themeMode,
    effectiveTheme,
    theme: effectiveTheme,
    setThemeMode,
    toggleTheme,
  }
})
