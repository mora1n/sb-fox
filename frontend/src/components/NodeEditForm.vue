<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ChevronDownIcon, ChevronUpIcon } from '@heroicons/vue/24/outline'
import type { Node, NodeSummary } from '../api/types'
import { useNodesStore } from '../stores/nodes'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import { COUNTRY_CODES, countryFlagEmoji, sortCountryOptions } from '../utils/countries'

type NodeFormMode = 'create' | 'edit' | 'copy'

const props = defineProps<{
  mode?: NodeFormMode
  node: Node | null
  copyFrom?: Node | null
  summary?: NodeSummary | null
  loading?: boolean
}>()
const emit = defineEmits<{ close: []; saved: [] }>()

const nodesStore = useNodesStore()
const settings = useSettingsStore()
const ui = useUiStore()
const i18n = useI18nStore()
const busy = ref(false)
const parseError = ref('')
const hydrating = ref(false)

const PROTOCOLS = [
  'shadowsocks',
  'vmess',
  'vless',
  'trojan',
  'hysteria',
  'hysteria2',
  'tuic',
  'snell',
  'anytls',
  'shadowtls',
  'naive',
  'http',
  'socks',
]
const NETWORK_STRATEGIES = ['default', 'hybrid', 'fallback']
const NETWORK_TYPES = ['wifi', 'cellular', 'ethernet', 'other']
const DOMAIN_RESOLVER_STRATEGIES = ['', 'as_is', 'prefer_ipv4', 'prefer_ipv6', 'ipv4_only', 'ipv6_only']
const SING_BOX_DURATION_PATTERN = /^[-+]?(((\d+(\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h|d))+|0)$/
const DIAL_FIELD_KEYS = [
  'detour',
  'bind_interface',
  'inet4_bind_address',
  'inet6_bind_address',
  'bind_address_no_port',
  'protect_path',
  'routing_mark',
  'reuse_addr',
  'netns',
  'connect_timeout',
  'tcp_fast_open',
  'tcp_multi_path',
  'disable_tcp_keep_alive',
  'tcp_keep_alive',
  'tcp_keep_alive_interval',
  'udp_fragment',
  'domain_resolver',
  'network_strategy',
  'network_type',
  'fallback_network_type',
  'fallback_delay',
]
const HEADERS_ERROR_PREFIX = 'headers JSON 解析失败: '
const DOMAIN_RESOLVER_ERROR_PREFIX = 'domain_resolver 配置错误: '
const NETWORK_TYPE_ERROR_PREFIX = 'network_type'
const FALLBACK_NETWORK_TYPE_ERROR_PREFIX = 'fallback_network_type'
type DomainResolverMode = 'string' | 'object'

// raw is the authoritative parsed outbound; unknown keys are preserved on save.
const raw = reactive<Record<string, any>>({})
const domainResolverMode = ref<DomainResolverMode>('string')
const showDialFields = ref(false)

// manual country override
const manualCountry = ref(false)
const countryCode = ref('')

function resetFrom(node: Node | null) {
  parseError.value = ''
  hydrating.value = true
  for (const k of Object.keys(raw)) delete raw[k]
  try {
    if (node) {
      try {
        Object.assign(raw, JSON.parse(node.raw))
      } catch (e) {
        parseError.value = 'raw JSON 解析失败: ' + errMsg(e)
      }
      showDialFields.value = false
      syncDomainResolverState()
      manualCountry.value = node.country_source === 'manual'
      countryCode.value = node.country_code || ''
    } else {
      Object.assign(raw, { type: 'shadowsocks', tag: '', server: '', server_port: 443 })
      domainResolverMode.value = 'string'
      showDialFields.value = false
      manualCountry.value = false
      countryCode.value = ''
    }
  } finally {
    hydrating.value = false
  }
}
// ensure a nested tls object exists, then return it
function tls(): Record<string, any> {
  if (!raw.tls || typeof raw.tls !== 'object') raw.tls = {}
  return raw.tls
}
function utls(): Record<string, any> {
  const t = tls()
  if (!t.utls || typeof t.utls !== 'object') t.utls = {}
  return t.utls
}
function reality(): Record<string, any> {
  const t = tls()
  if (!t.reality || typeof t.reality !== 'object') t.reality = {}
  return t.reality
}
function transport(): Record<string, any> {
  if (!raw.transport || typeof raw.transport !== 'object') raw.transport = {}
  return raw.transport
}
function objectRecord(value: unknown): Record<string, any> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, any>
}
function transportHeaders(): Record<string, any> {
  const t = transport()
  const headers = objectRecord(t.headers)
  if (headers) return headers
  t.headers = {}
  return t.headers
}
function hostText(value: unknown): string {
  if (Array.isArray(value)) return value.join(', ')
  if (value === undefined || value === null) return ''
  return String(value)
}
function setHostValue(target: Record<string, any>, key: string, value: string) {
  if (value) target[key] = value
  else delete target[key]
}
function cleanupTransportHeaders(t: Record<string, any>) {
  const headers = objectRecord(t.headers)
  if (headers && Object.keys(headers).length === 0) delete t.headers
}
function obfs(): Record<string, any> {
  if (!raw.obfs || typeof raw.obfs !== 'object') raw.obfs = {}
  return raw.obfs
}

// alpn is an array in sing-box; edit it as comma-separated text
const alpnText = computed({
  get: () => (Array.isArray(raw.tls?.alpn) ? raw.tls.alpn.join(', ') : ''),
  set: (v: string) => {
    const arr = v.split(',').map((s) => s.trim()).filter(Boolean)
    if (arr.length) tls().alpn = arr
    else if (raw.tls) delete raw.tls.alpn
  },
})
const headersText = computed({
  get: () => {
    if (!raw.headers || typeof raw.headers !== 'object') return ''
    try {
      return JSON.stringify(raw.headers, null, 2)
    } catch {
      return ''
    }
  },
  set: (v: string) => {
    const trimmed = v.trim()
    if (!trimmed) {
      delete raw.headers
      clearFieldError(HEADERS_ERROR_PREFIX)
      return
    }
    try {
      raw.headers = JSON.parse(trimmed)
      clearFieldError(HEADERS_ERROR_PREFIX)
    } catch (e) {
      parseError.value = HEADERS_ERROR_PREFIX + errMsg(e)
    }
  },
})
const transportHostText = computed({
  get: () => {
    const t = objectRecord(raw.transport)
    if (!t) return ''
    if (t.host !== undefined) return hostText(t.host)
    const headers = objectRecord(t.headers)
    if (!headers) return ''
    if (headers.Host !== undefined) return hostText(headers.Host)
    return hostText(headers.host)
  },
  set: (v: string) => {
    const value = v.trim()
    const t = transport()
    const headers = objectRecord(t.headers)
    if (t.host !== undefined) {
      setHostValue(t, 'host', value)
    } else if (headers?.host !== undefined && headers.Host === undefined) {
      setHostValue(headers, 'host', value)
    } else {
      setHostValue(transportHeaders(), 'Host', value)
    }
    cleanupTransportHeaders(t)
  },
})
const domainResolverText = computed({
  get: () => (typeof raw.domain_resolver === 'string' ? raw.domain_resolver : ''),
  set: (v: string) => {
    const trimmed = v.trim()
    if (!trimmed) {
      delete raw.domain_resolver
      clearFieldError(DOMAIN_RESOLVER_ERROR_PREFIX)
      return
    }
    raw.domain_resolver = trimmed
    clearFieldError(DOMAIN_RESOLVER_ERROR_PREFIX)
  },
})
function domainResolverObject(): Record<string, any> | null {
  return objectRecord(raw.domain_resolver)
}
function setDomainResolverObjectField(key: string, value: unknown) {
  const current = { ...(domainResolverObject() ?? {}) }
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (trimmed) current[key] = trimmed
    else delete current[key]
  } else if (typeof value === 'boolean') {
    if (value) current[key] = true
    else delete current[key]
  } else if (typeof value === 'number' && Number.isFinite(value)) {
    current[key] = value
  } else if (value === undefined || value === null || value === '') {
    delete current[key]
  } else {
    current[key] = value
  }
  if (Object.keys(current).length) raw.domain_resolver = current
  else delete raw.domain_resolver
  clearFieldError(DOMAIN_RESOLVER_ERROR_PREFIX)
}
function domainResolverField(key: string) {
  return computed({
    get: () => domainResolverObject()?.[key] ?? '',
    set: (value: unknown) => setDomainResolverObjectField(key, value),
  })
}
const domainResolverServer = domainResolverField('server')
const domainResolverTimeout = domainResolverField('timeout')
const domainResolverStrategy = domainResolverField('strategy')
const domainResolverRewriteTTL = domainResolverField('rewrite_ttl')
const domainResolverClientSubnet = domainResolverField('client_subnet')
const domainResolverDisableCache = domainResolverField('disable_cache')
const domainResolverDisableOptimisticCache = domainResolverField('disable_optimistic_cache')
const networkTypeText = computed({
  get: () => listFieldText(raw.network_type),
  set: (v: string) => setListField('network_type', v, NETWORK_TYPES, NETWORK_TYPE_ERROR_PREFIX),
})
const fallbackNetworkTypeText = computed({
  get: () => listFieldText(raw.fallback_network_type),
  set: (v: string) => setListField('fallback_network_type', v, NETWORK_TYPES, FALLBACK_NETWORK_TYPE_ERROR_PREFIX),
})
const snellNetworkText = computed({
  get: () => (Array.isArray(raw.network) ? raw.network.join(', ') : String(raw.network ?? '')),
  set: (v: string) => {
    const input = v.trim()
    const values = input ? input.split(',').map((item) => item.trim()) : []
    const invalid = values.filter((item) => !['tcp', 'udp'].includes(item))
    if (invalid.length) {
      parseError.value = `${i18n.t('Snell 网络')} ${i18n.t('仅支持以下值')}: tcp, udp`
      return
    }
    if (values.length === 0) delete raw.network
    else raw.network = [...new Set(values)]
    if (parseError.value.startsWith(i18n.t('Snell 网络'))) parseError.value = ''
  },
})

const countryOptions = computed(() => sortCountryOptions(COUNTRY_CODES, settings.countryHeatOrder))
const currentMode = computed<NodeFormMode>(() => props.mode ?? (props.node ? 'edit' : props.copyFrom ? 'copy' : 'create'))
const formTitle = computed(() => {
  if (currentMode.value === 'edit') return i18n.t('编辑节点')
  if (currentMode.value === 'copy') return i18n.t('复制节点')
  return i18n.t('新建节点')
})
const summaryLine = computed(() => {
  const s = props.summary
  if (!s) return ''
  return [s.type, s.tag, `${s.server}:${s.server_port}`].filter(Boolean).join(' · ')
})
const configuredDialFieldCount = computed(() => {
  let count = 0
  for (const key of DIAL_FIELD_KEYS) {
    const value = raw[key]
    if (Array.isArray(value)) {
      if (value.length) count++
      continue
    }
    if (value && typeof value === 'object') {
      count++
      continue
    }
    if (typeof value === 'boolean') {
      if (value) count++
      continue
    }
    if (value !== undefined && value !== null && String(value).trim() !== '') count++
  }
  return count
})

function resetPending() {
  parseError.value = ''
  for (const k of Object.keys(raw)) delete raw[k]
  domainResolverMode.value = 'string'
  showDialFields.value = false
  manualCountry.value = false
  countryCode.value = ''
}

function defaultPortFor(type: string) {
  if (['trojan', 'hysteria', 'hysteria2', 'tuic', 'anytls', 'shadowtls', 'naive'].includes(type)) return 443
  if (type === 'socks') return 1080
  if (type === 'http') return 80
  return 443
}

function defaultTLSEnabled(type: string) {
  return ['trojan', 'hysteria', 'hysteria2', 'tuic', 'anytls', 'shadowtls', 'naive'].includes(type)
}

function clearFieldError(prefix: string) {
  if (parseError.value.startsWith(prefix)) parseError.value = ''
}

function syncDomainResolverState() {
  const value = raw.domain_resolver
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    domainResolverMode.value = 'object'
    return
  }
  domainResolverMode.value = 'string'
  if (typeof value === 'string') {
    clearFieldError(DOMAIN_RESOLVER_ERROR_PREFIX)
  }
}

function setDomainResolverMode(next: DomainResolverMode) {
  if (domainResolverMode.value === next) return
  const current = raw.domain_resolver
  if (next === 'object') {
    if (typeof current === 'string' && current.trim()) raw.domain_resolver = { server: current.trim() }
  } else {
    if (current && typeof current === 'object' && !Array.isArray(current)) {
      const server = typeof current.server === 'string' ? current.server.trim() : ''
      if (server) raw.domain_resolver = server
      else delete raw.domain_resolver
    } else if (typeof current === 'string') {
      const trimmed = current.trim()
      if (trimmed) raw.domain_resolver = trimmed
      else delete raw.domain_resolver
    } else {
      delete raw.domain_resolver
    }
  }
  clearFieldError(DOMAIN_RESOLVER_ERROR_PREFIX)
  domainResolverMode.value = next
}

function validateDomainResolver() {
  const value = raw.domain_resolver
  if (value === undefined || value === null || value === '') return
  if (typeof value === 'string') {
    if (!value.trim()) throw new Error('domain_resolver 不能为空')
    return
  }
  const resolver = objectRecord(value)
  if (!resolver) throw new Error('domain_resolver 必须是服务器标签字符串或对象')
  const allowed = new Set(['server', 'timeout', 'strategy', 'disable_cache', 'disable_optimistic_cache', 'rewrite_ttl', 'client_subnet'])
  const unknown = Object.keys(resolver).filter((key) => !allowed.has(key))
  if (unknown.length) throw new Error(`domain_resolver 包含 1.14.0 不支持的字段: ${unknown.join(', ')}`)
  if (typeof resolver.server !== 'string' || !resolver.server.trim()) throw new Error('domain_resolver.server 不能为空')
  if (resolver.timeout !== undefined && (typeof resolver.timeout !== 'string' || !SING_BOX_DURATION_PATTERN.test(resolver.timeout.trim()))) {
    throw new Error('domain_resolver.timeout 必须是持续时间，例如 5s')
  }
  if (resolver.strategy !== undefined && !DOMAIN_RESOLVER_STRATEGIES.includes(String(resolver.strategy))) {
    throw new Error(`domain_resolver.strategy 只能是: ${DOMAIN_RESOLVER_STRATEGIES.filter(Boolean).join(', ')}`)
  }
  for (const key of ['disable_cache', 'disable_optimistic_cache']) {
    if (resolver[key] !== undefined && typeof resolver[key] !== 'boolean') throw new Error(`domain_resolver.${key} 必须是布尔值`)
  }
  if (resolver.rewrite_ttl !== undefined && (!Number.isInteger(resolver.rewrite_ttl) || resolver.rewrite_ttl < 0 || resolver.rewrite_ttl > 4294967295)) {
    throw new Error('domain_resolver.rewrite_ttl 必须是 0 到 4294967295 的整数')
  }
  if (resolver.client_subnet !== undefined && typeof resolver.client_subnet !== 'string') {
    throw new Error('domain_resolver.client_subnet 必须是 CIDR 字符串')
  }
}

function listFieldText(value: unknown) {
  return Array.isArray(value) ? value.join(', ') : ''
}

function setListField(field: string, rawValue: string, allowed: string[], prefix: string) {
  const trimmed = rawValue.trim()
  if (!trimmed) {
    delete raw[field]
    clearFieldError(prefix)
    return
  }
  const items = trimmed.split(',').map((item) => item.trim()).filter(Boolean)
  const invalid = items.filter((item) => !allowed.includes(item))
  if (invalid.length) {
    parseError.value = `${prefix} ${i18n.t('仅支持以下值')}: ${allowed.join(', ')}`
    return
  }
  raw[field] = [...new Set(items)]
  clearFieldError(prefix)
}

function normalizeSnellFields(defaultVersion = false) {
  if (raw.type !== 'snell') return
  const version = Number(raw.version)
  if (version !== 4 && version !== 6) {
    if (!defaultVersion) return
    raw.version = 4
  }
  delete raw.tls
  if (typeof raw.network === 'string' && !raw.network.trim()) delete raw.network
  if (Array.isArray(raw.network) && raw.network.length === 0) delete raw.network
  if (raw.mode === '') delete raw.mode
  if (raw.obfs_mode === '') delete raw.obfs_mode
  if (Number(raw.version) === 6) {
    delete raw.obfs_mode
    delete raw.obfs_host
  } else {
    delete raw.mode
  }
}

function validateSnellFields() {
  const version = Number(raw.version)
  if (version !== 4 && version !== 6) throw new Error('Snell version 只能是 4 或 6')
  const psk = String(raw.psk ?? '').trim()
  if (!psk) throw new Error('Snell PSK 不能为空')
  const pskBytes = new TextEncoder().encode(psk).length
  if (version === 6 && (pskBytes < 12 || pskBytes > 255)) {
    throw new Error('Snell v6 PSK 长度必须为 12-255 字节')
  }
  const networkRaw = String(raw.network ?? '').trim()
  const network = Array.isArray(raw.network) ? raw.network : networkRaw ? networkRaw.split(',').map((item) => item.trim()) : []
  if (network.some((item: unknown) => !['tcp', 'udp'].includes(String(item).trim()))) {
    throw new Error('Snell network 只能是 tcp 或 udp')
  }
  if (version === 6 && raw.obfs_mode) throw new Error('Snell obfs_mode 仅适用于 version 4')
  if (version === 4 && raw.mode) throw new Error('Snell mode 仅适用于 version 6')
  if (version === 4 && raw.obfs_mode && !['none', 'http'].includes(raw.obfs_mode)) {
    throw new Error('Snell obfs_mode 只能是 none 或 http')
  }
  if (version === 6 && raw.mode && !['default', 'unshaped', 'unsafe-raw'].includes(raw.mode)) {
    throw new Error('Snell mode 只能是 default、unshaped 或 unsafe-raw')
  }
}

async function save() {
  if (
    props.loading ||
    (currentMode.value === 'edit' && !props.node) ||
    (currentMode.value === 'copy' && !props.copyFrom)
  ) {
    ui.info(i18n.t('正在加载节点...'))
    return
  }
  busy.value = true
  try {
    if (props.copyFrom && String(raw.tag ?? '').trim() === props.copyFrom.tag.trim()) {
      throw new Error(i18n.t('复制节点需要修改标签后保存'))
    }
    if (raw.type === 'snell') {
      normalizeSnellFields()
      validateSnellFields()
    }
    validateDomainResolver()
    const payload = {
      raw: JSON.stringify(raw),
      country_code: manualCountry.value ? countryCode.value : undefined,
      country_source: manualCountry.value ? 'manual' : undefined,
    }
    if (props.node) await nodesStore.update(props.node.id, payload)
    else await nodesStore.create(payload)
    ui.success(props.node ? '节点已更新' : '节点已创建')
    emit('saved')
    emit('close')
  } catch (e) {
    ui.error(errMsg(e, '保存失败'))
  } finally {
    busy.value = false
  }
}
watch(
  () => [props.node, props.copyFrom, props.loading, currentMode.value, props.summary?.id],
  () => {
    const full = props.node ?? props.copyFrom ?? null
    if (full) resetFrom(full)
    else if (currentMode.value === 'create') resetFrom(null)
    else resetPending()
  },
  { immediate: true },
)
watch(
  () => raw.type,
  (next, prev) => {
    if (!next || next === prev || hydrating.value) return
    if (!raw.server_port || raw.server_port === defaultPortFor(prev || '')) {
      raw.server_port = defaultPortFor(next)
    }
    if (next === 'snell') normalizeSnellFields(true)
    if (defaultTLSEnabled(next)) tls().enabled = true
  },
)
watch(
  () => raw.version,
  () => {
    if (!hydrating.value && raw.type === 'snell') normalizeSnellFields()
  },
)
watch(
  () => raw.mode,
  (next) => {
    if (!hydrating.value && raw.type === 'snell' && next === '') delete raw.mode
  },
)
watch(
  () => raw.obfs_mode,
  (next) => {
    if (!hydrating.value && raw.type === 'snell' && next === '') delete raw.obfs_mode
  },
)
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-2xl">
      <h3 class="font-bold text-lg mb-3">{{ formTitle }}</h3>

      <div v-if="parseError" class="alert alert-error text-sm mb-3">
        <span>{{ parseError }}</span>
      </div>

      <div v-if="loading" class="alert py-2 mb-3">
        <span class="loading loading-spinner loading-sm"></span>
        <span class="text-sm">
          {{ i18n.t('正在加载节点...') }}
          <span v-if="summaryLine" class="opacity-70">· {{ summaryLine }}</span>
        </span>
      </div>

      <fieldset class="flex flex-col gap-3 max-h-[65vh] overflow-y-auto pr-1" :disabled="loading" :class="{ 'opacity-60': loading }">
        <!-- header: type / tag / server / port -->
        <div class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('协议类型') }}</span>
            <select v-model="raw.type" class="select select-bordered select-sm">
              <option v-for="p in PROTOCOLS" :key="p" :value="p">{{ p }}</option>
              <option v-if="!PROTOCOLS.includes(raw.type)" :value="raw.type">{{ raw.type }}</option>
            </select>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('标签') }}</span>
            <input v-model="raw.tag" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('服务器') }}</span>
            <input v-model="raw.server" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('端口') }}</span>
            <input v-model.number="raw.server_port" type="number" class="input input-bordered input-sm" />
          </label>
        </div>

        <div class="divider my-0 text-xs opacity-60">{{ i18n.t('协议参数') }}</div>

        <!-- shadowsocks -->
        <div v-if="raw.type === 'shadowsocks'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('加密方式') }}</span>
            <input v-model="raw.method" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
        </div>

        <!-- vmess -->
        <div v-else-if="raw.type === 'vmess'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">UUID</span>
            <input v-model="raw.uuid" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">alter_id</span>
            <input v-model.number="raw.alter_id" type="number" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">security</span>
            <input v-model="raw.security" class="input input-bordered input-sm" placeholder="auto" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">network</span>
            <input v-model="raw.network" class="input input-bordered input-sm" placeholder="tcp" />
          </label>
        </div>

        <!-- vless -->
        <div v-else-if="raw.type === 'vless'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">UUID</span>
            <input v-model="raw.uuid" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">flow</span>
            <input v-model="raw.flow" class="input input-bordered input-sm" placeholder="xtls-rprx-vision" />
          </label>
        </div>

        <!-- trojan -->
        <div v-else-if="raw.type === 'trojan'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
        </div>

        <!-- hysteria -->
        <div v-else-if="raw.type === 'hysteria'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">auth_str</span>
            <input v-model="raw.auth_str" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('obfs 类型') }}</span>
            <input v-model="raw.obfs" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">up_mbps</span>
            <input v-model.number="raw.up_mbps" type="number" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">down_mbps</span>
            <input v-model.number="raw.down_mbps" type="number" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">network</span>
            <input v-model="raw.network" class="input input-bordered input-sm" placeholder="tcp,udp" />
          </label>
        </div>

        <!-- hysteria2 -->
        <div v-else-if="raw.type === 'hysteria2'" class="flex flex-col gap-3">
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
              <input v-model="raw.password" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">up_mbps</span>
              <input v-model.number="raw.up_mbps" type="number" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">down_mbps</span>
              <input v-model.number="raw.down_mbps" type="number" class="input input-bordered input-sm" />
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">obfs</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">type</span>
              <input
                :value="raw.obfs?.type"
                class="input input-bordered input-sm"
                @input="obfs().type = ($event.target as HTMLInputElement).value"
              />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">password</span>
              <input
                :value="raw.obfs?.password"
                class="input input-bordered input-sm"
                @input="obfs().password = ($event.target as HTMLInputElement).value"
              />
            </label>
          </div>
        </div>

        <!-- tuic -->
        <div v-else-if="raw.type === 'tuic'" class="flex flex-col gap-3">
          <div class="divider my-0 text-xs opacity-50">{{ i18n.t('认证') }}</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">UUID</span>
              <input v-model="raw.uuid" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
              <input v-model="raw.password" class="input input-bordered input-sm" />
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">QUIC</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">congestion_control</span>
              <input v-model="raw.congestion_control" class="input input-bordered input-sm" placeholder="bbr" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">udp_relay_mode</span>
              <input v-model="raw.udp_relay_mode" class="input input-bordered input-sm" placeholder="native" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">heartbeat</span>
              <input v-model="raw.heartbeat" class="input input-bordered input-sm" placeholder="10s" />
            </label>
          </div>
        </div>

        <!-- snell -->
        <div v-else-if="raw.type === 'snell'" class="flex flex-col gap-3">
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">PSK</span>
              <input v-model="raw.psk" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">userkey</span>
              <input v-model="raw.userkey" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">version</span>
              <select v-model.number="raw.version" class="select select-bordered select-sm">
                <option :value="4">4</option>
                <option :value="6">6</option>
              </select>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">network</span>
              <input v-model="snellNetworkText" class="input input-bordered input-sm" placeholder="tcp, udp" />
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="raw.reuse" />
              <span class="label-text">reuse</span>
            </label>
          </div>
          <div v-if="Number(raw.version) === 6" class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">mode</span>
              <select v-model="raw.mode" class="select select-bordered select-sm">
                <option value="">{{ i18n.t('未指定') }}</option>
                <option value="default">default</option>
                <option value="unshaped">unshaped</option>
                <option value="unsafe-raw">unsafe-raw</option>
              </select>
            </label>
          </div>
          <div v-else class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">obfs_mode</span>
              <select v-model="raw.obfs_mode" class="select select-bordered select-sm">
                <option value="">{{ i18n.t('未指定') }}</option>
                <option value="none">none</option>
                <option value="http">http</option>
              </select>
            </label>
            <label v-if="raw.obfs_mode === 'http'" class="form-control">
              <span class="label-text mb-1">obfs_host</span>
              <input v-model="raw.obfs_host" class="input input-bordered input-sm" />
            </label>
          </div>
        </div>

        <!-- anytls -->
        <div v-else-if="raw.type === 'anytls'" class="flex flex-col gap-3">
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
              <input v-model="raw.password" class="input input-bordered input-sm" />
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">{{ i18n.t('会话') }}</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">idle_session_check_interval</span>
              <input v-model="raw.idle_session_check_interval" class="input input-bordered input-sm" placeholder="30s" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">idle_session_timeout</span>
              <input v-model="raw.idle_session_timeout" class="input input-bordered input-sm" placeholder="30s" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">min_idle_session</span>
              <input v-model.number="raw.min_idle_session" type="number" class="input input-bordered input-sm" />
            </label>
          </div>
        </div>

        <!-- shadowtls -->
        <div v-else-if="raw.type === 'shadowtls'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">version</span>
            <input v-model.number="raw.version" type="number" class="input input-bordered input-sm" placeholder="3" />
          </label>
        </div>

        <!-- naive -->
        <div v-else-if="raw.type === 'naive'" class="flex flex-col gap-3">
          <div class="divider my-0 text-xs opacity-50">{{ i18n.t('认证') }}</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('用户名') }}</span>
              <input v-model="raw.username" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
              <input v-model="raw.password" class="input input-bordered input-sm" />
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">QUIC</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="raw.quic" />
              <span class="label-text">quic</span>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">quic_congestion_control</span>
              <input v-model="raw.quic_congestion_control" class="input input-bordered input-sm" placeholder="bbr" />
            </label>
          </div>
        </div>

        <!-- http -->
        <div v-else-if="raw.type === 'http'" class="flex flex-col gap-3">
          <div class="divider my-0 text-xs opacity-50">{{ i18n.t('认证') }}</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('用户名') }}</span>
              <input v-model="raw.username" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
              <input v-model="raw.password" class="input input-bordered input-sm" />
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">{{ i18n.t('请求') }}</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">path</span>
              <input v-model="raw.path" class="input input-bordered input-sm" />
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">headers</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control col-span-2">
              <span class="label-text mb-1">JSON</span>
              <textarea v-model="headersText" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-20" placeholder='{ "Host": "example.com" }'></textarea>
            </label>
          </div>
        </div>

        <!-- socks -->
        <div v-else-if="raw.type === 'socks'" class="flex flex-col gap-3">
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">version</span>
              <select v-model="raw.version" class="select select-bordered select-sm">
                <option value="5">5</option>
                <option value="4">4</option>
                <option value="4a">4a</option>
              </select>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">network</span>
              <input v-model="raw.network" class="input input-bordered input-sm" placeholder="tcp,udp" />
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">{{ i18n.t('认证') }}</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('用户名') }}</span>
              <input v-model="raw.username" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
              <input v-model="raw.password" class="input input-bordered input-sm" />
            </label>
          </div>
        </div>

        <div v-if="['vmess', 'vless', 'trojan'].includes(raw.type)" class="flex flex-col gap-3">
          <div class="divider my-0 text-xs opacity-50">transport</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">type</span>
              <select
                :value="raw.transport?.type || ''"
                class="select select-bordered select-sm"
                @change="transport().type = ($event.target as HTMLSelectElement).value"
              >
                <option value="">{{ i18n.t('未指定') }}</option>
                <option value="ws">ws</option>
                <option value="grpc">grpc</option>
                <option value="http">http</option>
                <option value="httpupgrade">httpupgrade</option>
              </select>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">path</span>
              <input
                :value="raw.transport?.path"
                class="input input-bordered input-sm"
                @input="transport().path = ($event.target as HTMLInputElement).value"
              />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">service_name</span>
              <input
                :value="raw.transport?.service_name"
                class="input input-bordered input-sm"
                @input="transport().service_name = ($event.target as HTMLInputElement).value"
              />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">host</span>
              <input v-model="transportHostText" class="input input-bordered input-sm" />
            </label>
          </div>
        </div>

        <!-- shared TLS block (all except plain shadowsocks may still use it) -->
        <div class="divider my-0 text-xs opacity-60">TLS</div>
        <label class="label cursor-pointer justify-start gap-2">
          <input
            type="checkbox"
            class="toggle toggle-sm"
            :checked="!!raw.tls?.enabled"
            @change="tls().enabled = ($event.target as HTMLInputElement).checked"
          />
          <span class="label-text">{{ i18n.t('启用 TLS') }}</span>
        </label>
        <div v-if="raw.tls?.enabled" class="flex flex-col gap-3">
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">server_name</span>
              <input
                :value="raw.tls?.server_name"
                class="input input-bordered input-sm"
                @input="tls().server_name = ($event.target as HTMLInputElement).value"
              />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">ALPN</span>
              <input v-model="alpnText" class="input input-bordered input-sm" placeholder="h2, http/1.1" />
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input
                type="checkbox"
                class="toggle toggle-sm"
                :checked="!!raw.tls?.insecure"
                @change="tls().insecure = ($event.target as HTMLInputElement).checked"
              />
              <span class="label-text">insecure</span>
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">uTLS</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">fingerprint</span>
              <input
                :value="raw.tls?.utls?.fingerprint"
                class="input input-bordered input-sm"
                placeholder="chrome"
                @input="((utls().fingerprint = ($event.target as HTMLInputElement).value), (utls().enabled = true))"
              />
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">Reality</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="label cursor-pointer justify-start gap-2">
              <input
                type="checkbox"
                class="toggle toggle-sm"
                :checked="!!raw.tls?.reality?.enabled"
                @change="reality().enabled = ($event.target as HTMLInputElement).checked"
              />
              <span class="label-text">{{ i18n.t('启用') }}</span>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">public_key</span>
              <input
                :value="raw.tls?.reality?.public_key"
                class="input input-bordered input-sm"
                @input="((reality().public_key = ($event.target as HTMLInputElement).value), (reality().enabled = true))"
              />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">short_id</span>
              <input
                :value="raw.tls?.reality?.short_id"
                class="input input-bordered input-sm"
                @input="((reality().short_id = ($event.target as HTMLInputElement).value), (reality().enabled = true))"
              />
            </label>
          </div>
        </div>

        <div class="divider my-0 text-xs opacity-60">
          <span>{{ i18n.t('拨号字段') }}</span>
          <button type="button" class="btn btn-xs btn-ghost btn-square" :title="i18n.t('拨号字段')" @click="showDialFields = !showDialFields">
            <ChevronUpIcon v-if="showDialFields" class="h-3 w-3" />
            <ChevronDownIcon v-else class="h-3 w-3" />
          </button>
        </div>
        <div v-if="configuredDialFieldCount" class="flex items-center justify-between gap-3 -mt-2">
          <span class="text-xs opacity-70">
            {{ i18n.t('已配置拨号字段') }}: {{ configuredDialFieldCount }}
          </span>
        </div>
        <div v-if="showDialFields" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('上游出口') }}</span>
            <input v-model="raw.detour" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('绑定接口') }}</span>
            <input v-model="raw.bind_interface" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('IPv4 绑定地址') }}</span>
            <input v-model="raw.inet4_bind_address" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('IPv6 绑定地址') }}</span>
            <input v-model="raw.inet6_bind_address" class="input input-bordered input-sm" />
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.bind_address_no_port" />
            <span class="label-text">{{ i18n.t('绑定地址不保留端口') }}</span>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('Protect Path') }}</span>
            <input v-model="raw.protect_path" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('路由标记') }}</span>
            <input v-model="raw.routing_mark" class="input input-bordered input-sm" placeholder="0x1234" />
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.reuse_addr" />
            <span class="label-text">{{ i18n.t('重用地址') }}</span>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('网络命名空间') }}</span>
            <input v-model="raw.netns" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('连接超时') }}</span>
            <input v-model="raw.connect_timeout" class="input input-bordered input-sm" placeholder="5s" />
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.tcp_fast_open" />
            <span class="label-text">{{ i18n.t('TCP Fast Open') }}</span>
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.tcp_multi_path" />
            <span class="label-text">{{ i18n.t('TCP Multi Path') }}</span>
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.disable_tcp_keep_alive" />
            <span class="label-text">{{ i18n.t('禁用 TCP Keep Alive') }}</span>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('TCP Keep Alive') }}</span>
            <input v-model="raw.tcp_keep_alive" class="input input-bordered input-sm" placeholder="5m" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('TCP Keep Alive 间隔') }}</span>
            <input v-model="raw.tcp_keep_alive_interval" class="input input-bordered input-sm" placeholder="75s" />
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.udp_fragment" />
            <span class="label-text">{{ i18n.t('UDP 分段') }}</span>
          </label>
          <div class="form-control col-span-2">
            <span class="label-text mb-1">{{ i18n.t('域名解析器') }}</span>
            <div class="join mb-2">
              <button type="button" class="btn btn-sm join-item" :class="{ 'btn-active': domainResolverMode === 'string' }" @click="setDomainResolverMode('string')">
                {{ i18n.t('简单模式') }}
              </button>
              <button type="button" class="btn btn-sm join-item" :class="{ 'btn-active': domainResolverMode === 'object' }" @click="setDomainResolverMode('object')">
                {{ i18n.t('结构化模式') }}
              </button>
            </div>
            <input
              v-if="domainResolverMode === 'string'"
              v-model="domainResolverText"
              class="input input-bordered input-sm"
              :placeholder="i18n.t('域名解析器标签')"
            />
            <div v-else class="grid grid-cols-2 gap-3">
              <label class="form-control col-span-2">
                <span class="label-text mb-1">server <span class="text-error">*</span></span>
                <input v-model="domainResolverServer" class="input input-bordered input-sm" placeholder="dns.local" />
              </label>
              <label class="form-control">
                <span class="label-text mb-1">timeout</span>
                <input v-model="domainResolverTimeout" class="input input-bordered input-sm" placeholder="5s" />
              </label>
              <label class="form-control">
                <span class="label-text mb-1">strategy</span>
                <select v-model="domainResolverStrategy" class="select select-bordered select-sm">
                  <option v-for="strategy in DOMAIN_RESOLVER_STRATEGIES" :key="strategy" :value="strategy">
                    {{ strategy || i18n.t('未指定') }}
                  </option>
                </select>
              </label>
              <label class="form-control">
                <span class="label-text mb-1">rewrite_ttl</span>
                <input v-model.number="domainResolverRewriteTTL" type="number" min="0" max="4294967295" step="1" class="input input-bordered input-sm" placeholder="60" />
              </label>
              <label class="form-control">
                <span class="label-text mb-1">client_subnet</span>
                <input v-model="domainResolverClientSubnet" class="input input-bordered input-sm" placeholder="192.0.2.0/24" />
              </label>
              <label class="label cursor-pointer justify-start gap-2">
                <input type="checkbox" class="toggle toggle-sm" v-model="domainResolverDisableCache" />
                <span class="label-text">disable_cache</span>
              </label>
              <label class="label cursor-pointer justify-start gap-2">
                <input type="checkbox" class="toggle toggle-sm" v-model="domainResolverDisableOptimisticCache" />
                <span class="label-text">disable_optimistic_cache</span>
              </label>
            </div>
          </div>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('网络策略') }}</span>
            <select v-model="raw.network_strategy" class="select select-bordered select-sm">
              <option value="">{{ i18n.t('未指定') }}</option>
              <option v-for="strategy in NETWORK_STRATEGIES" :key="strategy" :value="strategy">{{ strategy }}</option>
            </select>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('网络类型') }}</span>
            <input v-model="networkTypeText" class="input input-bordered input-sm" placeholder="wifi, cellular" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('回退网络类型') }}</span>
            <input v-model="fallbackNetworkTypeText" class="input input-bordered input-sm" placeholder="cellular" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('回退延迟') }}</span>
            <input v-model="raw.fallback_delay" class="input input-bordered input-sm" placeholder="1s" />
          </label>
        </div>

        <!-- manual country override -->
        <div class="divider my-0 text-xs opacity-60">{{ i18n.t('国家') }}</div>
        <label class="label cursor-pointer justify-start gap-2">
          <input type="checkbox" class="toggle toggle-sm" v-model="manualCountry" />
          <span class="label-text">{{ i18n.t('手动指定国家') }}</span>
        </label>
        <select v-if="manualCountry" v-model="countryCode" class="select select-bordered select-sm">
          <option value="">{{ i18n.t('未指定') }}</option>
          <option v-for="c in countryOptions" :key="c.code" :value="c.code">
            {{ countryFlagEmoji(c.code) }} {{ c.code }} — {{ c.name }}
          </option>
        </select>
      </fieldset>

      <div class="modal-action">
        <button class="btn" @click="emit('close')" :disabled="busy">{{ i18n.t('取消') }}</button>
        <button class="btn btn-primary" @click="save" :disabled="busy || loading || !!parseError">
          <span v-if="busy || loading" class="loading loading-spinner loading-sm"></span>
          {{ i18n.t('保存') }}
        </button>
      </div>
    </div>
    <div class="modal-backdrop" @click="emit('close')"></div>
  </div>
</template>
