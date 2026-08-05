import { COUNTRY_CODES } from './countryCatalog'
import type { CountryOption } from './countryCatalog'

export { COUNTRY_CODES }
export type { CountryOption } from './countryCatalog'

export const DEFAULT_COUNTRY_HEAT_ORDER = ['JP', 'CN', 'HK', 'US', 'TW', 'SG']

const EUROPE_PRIORITY = ['GB', 'NL', 'FR', 'DE', 'CH']
const REGION_FALLBACK: Record<string, number> = {
  europe: 3,
  asia: 4,
  americas: 5,
  africa: 6,
  oceania: 7,
}
const COUNTRY_SET = new Set(COUNTRY_CODES.map((c) => c.code))
const COUNTRY_NAME = new Map(COUNTRY_CODES.map((c) => [c.code, c.name]))
const REGION_BY_CODE = new Map(COUNTRY_CODES.map((c) => [c.code, c.region]))

interface SortInfo {
  bucket: number
  priority: number
}

export function countryName(code: string): string {
  return COUNTRY_NAME.get(code) || code
}

export function countryFlagEmoji(code: string): string {
  const normalized = code.trim().toUpperCase()
  if (!/^[A-Z]{2}$/.test(normalized)) return '🏳️'
  const regionalIndicatorA = 0x1f1e6
  return String.fromCodePoint(
    regionalIndicatorA + normalized.charCodeAt(0) - 65,
    regionalIndicatorA + normalized.charCodeAt(1) - 65,
  )
}

export function normalizeCountryHeatOrder(codes: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of codes) {
    const code = String(raw).trim().toUpperCase()
    if (!COUNTRY_SET.has(code) || seen.has(code)) continue
    seen.add(code)
    out.push(code)
  }
  return out
}

export function completeCountryHeatOrder(codes: string[]): string[] {
  const out = normalizeCountryHeatOrder(codes)
  const seen = new Set(out)
  for (const c of COUNTRY_CODES) {
    if (!seen.has(c.code)) out.push(c.code)
  }
  return out
}

function buildSortCache(heatOrder: string[]): Map<string, SortInfo> {
  const cache = new Map<string, SortInfo>()
  const assign = (codes: string[], bucket: number) => {
    codes.forEach((code, index) => {
      if (!cache.has(code)) cache.set(code, { bucket, priority: index + 1 })
    })
  }
  const normalizedHeatOrder = normalizeCountryHeatOrder(heatOrder)
  assign(normalizedHeatOrder.length ? normalizedHeatOrder : DEFAULT_COUNTRY_HEAT_ORDER, 1)
  assign(EUROPE_PRIORITY, 2)
  for (const c of COUNTRY_CODES) {
    if (cache.has(c.code)) continue
    const region = REGION_BY_CODE.get(c.code) || 'unknown'
    cache.set(c.code, { bucket: REGION_FALLBACK[region] || 999, priority: 999 })
  }
  return cache
}

function compareCountryCodesWithCache(a: string, b: string, cache: Map<string, SortInfo>): number {
  if (a === b) return 0
  if (a === '??') return 1
  if (b === '??') return -1
  const ai = cache.get(a) || { bucket: 999, priority: 999 }
  const bi = cache.get(b) || { bucket: 999, priority: 999 }
  if (ai.bucket !== bi.bucket) return ai.bucket - bi.bucket
  if (ai.priority !== bi.priority) return ai.priority - bi.priority
  return a.localeCompare(b)
}

export function compareCountryCodes(a: string, b: string, heatOrder: string[]): number {
  return compareCountryCodesWithCache(a, b, buildSortCache(heatOrder))
}

export function sortCountryCodes(codes: string[], heatOrder: string[]): string[] {
  const cache = buildSortCache(heatOrder)
  return [...codes].sort((a, b) => compareCountryCodesWithCache(a, b, cache))
}

export function sortCountryOptions(options: CountryOption[], heatOrder: string[]) {
  const cache = buildSortCache(heatOrder)
  return [...options].sort((a, b) => compareCountryCodesWithCache(a.code, b.code, cache))
}
