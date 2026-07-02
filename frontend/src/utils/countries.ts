// Country codes supported by the merge country detector.
export const COUNTRY_CODES: { code: string; name: string }[] = [
  { code: 'HK', name: '香港' },
  { code: 'CN', name: '中国大陆' },
  { code: 'US', name: '美国' },
  { code: 'JP', name: '日本' },
  { code: 'SG', name: '新加坡' },
  { code: 'TW', name: '台湾' },
  { code: 'KR', name: '韩国' },
  { code: 'GB', name: '英国' },
  { code: 'DE', name: '德国' },
  { code: 'CA', name: '加拿大' },
  { code: 'AU', name: '澳大利亚' },
  { code: 'FR', name: '法国' },
  { code: 'NL', name: '荷兰' },
  { code: 'IN', name: '印度' },
  { code: 'RU', name: '俄罗斯' },
  { code: 'BR', name: '巴西' },
  { code: 'AR', name: '阿根廷' },
  { code: 'TR', name: '土耳其' },
  { code: 'TH', name: '泰国' },
  { code: 'MY', name: '马来西亚' },
  { code: 'PH', name: '菲律宾' },
  { code: 'VN', name: '越南' },
  { code: 'ID', name: '印度尼西亚' },
  { code: 'IT', name: '意大利' },
  { code: 'ES', name: '西班牙' },
  { code: 'CH', name: '瑞士' },
  { code: 'SE', name: '瑞典' },
  { code: 'NO', name: '挪威' },
  { code: 'FI', name: '芬兰' },
  { code: 'PL', name: '波兰' },
  { code: 'AT', name: '奥地利' },
  { code: 'BE', name: '比利时' },
  { code: 'DK', name: '丹麦' },
  { code: 'PT', name: '葡萄牙' },
  { code: 'GR', name: '希腊' },
  { code: 'IE', name: '爱尔兰' },
  { code: 'CZ', name: '捷克' },
  { code: 'RO', name: '罗马尼亚' },
  { code: 'UA', name: '乌克兰' },
  { code: 'LT', name: '立陶宛' },
  { code: 'LV', name: '拉脱维亚' },
  { code: 'EE', name: '爱沙尼亚' },
  { code: 'BG', name: '保加利亚' },
  { code: 'HR', name: '克罗地亚' },
  { code: 'SK', name: '斯洛伐克' },
  { code: 'SI', name: '斯洛文尼亚' },
  { code: 'HU', name: '匈牙利' },
  { code: 'IL', name: '以色列' },
  { code: 'AE', name: '阿联酋' },
  { code: 'SA', name: '沙特阿拉伯' },
  { code: 'KW', name: '科威特' },
  { code: 'PK', name: '巴基斯坦' },
  { code: 'BD', name: '孟加拉国' },
  { code: 'KZ', name: '哈萨克斯坦' },
  { code: 'UZ', name: '乌兹别克斯坦' },
  { code: 'MX', name: '墨西哥' },
  { code: 'CL', name: '智利' },
  { code: 'CO', name: '哥伦比亚' },
  { code: 'PE', name: '秘鲁' },
  { code: 'VE', name: '委内瑞拉' },
  { code: 'ZA', name: '南非' },
  { code: 'EG', name: '埃及' },
  { code: 'NG', name: '尼日利亚' },
  { code: 'NZ', name: '新西兰' },
  { code: 'FJ', name: '斐济' },
]

export const DEFAULT_COUNTRY_HEAT_ORDER = ['JP', 'CN', 'HK', 'US', 'TW', 'SG']

const EUROPE_PRIORITY = ['GB', 'NL', 'FR', 'DE', 'CH']
const REGION_FALLBACK: Record<string, number> = {
  europe: 3,
  asia: 4,
  americas: 5,
  africa: 6,
  oceania: 7,
}
const REGION_SETS: Record<string, string[]> = {
  asia: ['HK', 'CN', 'JP', 'KR', 'SG', 'TW', 'IN', 'TH', 'MY', 'PH', 'VN', 'ID', 'IL', 'AE', 'SA', 'KW', 'PK', 'BD', 'KZ', 'UZ', 'TR', 'RU'],
  americas: ['US', 'CA', 'BR', 'MX', 'AR', 'CL', 'CO', 'PE', 'VE'],
  europe: ['GB', 'FR', 'DE', 'NL', 'CH', 'IT', 'ES', 'SE', 'NO', 'FI', 'PL', 'AT', 'BE', 'DK', 'PT', 'GR', 'IE', 'CZ', 'RO', 'UA', 'LT', 'LV', 'EE', 'BG', 'HR', 'SK', 'SI', 'HU'],
  africa: ['ZA', 'EG', 'NG'],
  oceania: ['AU', 'NZ', 'FJ'],
}

const COUNTRY_SET = new Set(COUNTRY_CODES.map((c) => c.code))
const COUNTRY_NAME = new Map(COUNTRY_CODES.map((c) => [c.code, c.name]))
const REGION_BY_CODE = new Map<string, string>()
for (const [region, codes] of Object.entries(REGION_SETS)) {
  for (const code of codes) REGION_BY_CODE.set(code, region)
}

interface SortInfo {
  bucket: number
  priority: number
}

export function countryName(code: string): string {
  return COUNTRY_NAME.get(code) || code
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
  assign(normalizeCountryHeatOrder(heatOrder).length ? normalizeCountryHeatOrder(heatOrder) : DEFAULT_COUNTRY_HEAT_ORDER, 1)
  assign(EUROPE_PRIORITY, 2)
  for (const c of COUNTRY_CODES) {
    if (cache.has(c.code)) continue
    const region = REGION_BY_CODE.get(c.code) || 'unknown'
    cache.set(c.code, { bucket: REGION_FALLBACK[region] || 999, priority: 999 })
  }
  return cache
}

export function compareCountryCodes(a: string, b: string, heatOrder: string[]): number {
  if (a === b) return 0
  if (a === '??') return 1
  if (b === '??') return -1
  const cache = buildSortCache(heatOrder)
  const ai = cache.get(a) || { bucket: 999, priority: 999 }
  const bi = cache.get(b) || { bucket: 999, priority: 999 }
  if (ai.bucket !== bi.bucket) return ai.bucket - bi.bucket
  if (ai.priority !== bi.priority) return ai.priority - bi.priority
  return a.localeCompare(b)
}

export function sortCountryCodes(codes: string[], heatOrder: string[]): string[] {
  return [...codes].sort((a, b) => compareCountryCodes(a, b, heatOrder))
}

export function sortCountryOptions(options: typeof COUNTRY_CODES, heatOrder: string[]) {
  return [...options].sort((a, b) => compareCountryCodes(a.code, b.code, heatOrder))
}
