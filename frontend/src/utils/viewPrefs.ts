export function readViewPref<T extends string>(key: string, fallback: T, allowed: readonly T[]): T {
  try {
    const value = localStorage.getItem(key) as T | null
    return value && allowed.includes(value) ? value : fallback
  } catch (e) {
    console.warn('sb-fox: unable to read view preference', e)
    return fallback
  }
}

export function writeViewPref(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch (e) {
    console.warn('sb-fox: unable to save view preference', e)
  }
}
