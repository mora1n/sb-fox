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
import { useScrollPreserver } from '../utils/scrollPreserver'

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
const schemaWarnings = ref<string[]>([])
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
const TLS_PROTOCOLS = ['vmess', 'vless', 'trojan', 'hysteria', 'hysteria2', 'tuic', 'anytls', 'shadowtls', 'naive', 'http']
const NETWORK_STRATEGIES = ['default', 'hybrid', 'fallback']
const NETWORK_TYPES = ['wifi', 'cellular', 'ethernet', 'other']
const DOMAIN_RESOLVER_STRATEGIES = ['', 'as_is', 'prefer_ipv4', 'prefer_ipv6', 'ipv4_only', 'ipv6_only']
const NETWORKS = ['tcp', 'udp']
const VMESS_SECURITY = ['auto', 'none', 'zero', 'aes-128-gcm', 'chacha20-poly1305', 'aes-128-ctr']
const PACKET_ENCODINGS = ['', 'packetaddr', 'xudp']
const TLS_VERSIONS = ['', '1.0', '1.1', '1.2', '1.3']
const TLS_CURVE_PREFERENCES = ['P256', 'P384', 'P521', 'X25519', 'X25519MLKEM768']
const UTLS_FINGERPRINTS = ['', 'chrome', 'firefox', 'edge', 'safari', '360', 'qq', 'ios', 'android', 'random', 'randomized']
const SING_BOX_DURATION_PATTERN = /^[-+]?(((\d+(\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h))+|0)$/
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
const PROTOCOL_FIELD_KEYS: Record<string, string[]> = {
  shadowsocks: ['method', 'password', 'plugin', 'plugin_opts', 'network', 'udp_over_tcp', 'multiplex'],
  vmess: ['uuid', 'security', 'alter_id', 'global_padding', 'authenticated_length', 'network', 'tls', 'packet_encoding', 'multiplex', 'transport'],
  vless: ['uuid', 'flow', 'network', 'tls', 'multiplex', 'transport', 'packet_encoding'],
  trojan: ['password', 'network', 'tls', 'multiplex', 'transport'],
  hysteria: ['server_ports', 'hop_interval', 'up', 'up_mbps', 'down', 'down_mbps', 'obfs', 'auth', 'auth_str', 'recv_window_conn', 'recv_window', 'disable_mtu_discovery', 'network', 'tls'],
  hysteria2: ['server_ports', 'hop_interval', 'up_mbps', 'down_mbps', 'obfs', 'password', 'network', 'tls', 'brutal_debug'],
  tuic: ['uuid', 'password', 'congestion_control', 'udp_relay_mode', 'udp_over_stream', 'zero_rtt_handshake', 'heartbeat', 'network', 'tls'],
  snell: ['version', 'psk', 'reuse', 'userkey', 'mode', 'obfs_mode', 'obfs_host', 'network'],
  anytls: ['password', 'idle_session_check_interval', 'idle_session_timeout', 'min_idle_session', 'client_metadata', 'tls'],
  shadowtls: ['version', 'password', 'tls'],
  naive: ['username', 'password', 'insecure_concurrency', 'extra_headers', 'stream_receive_window', 'udp_over_tcp', 'quic', 'quic_congestion_control', 'quic_session_receive_window', 'tls'],
  http: ['username', 'password', 'tls', 'path', 'headers'],
  socks: ['version', 'username', 'password', 'network', 'udp_over_tcp'],
}
const TLS_FIELD_KEYS = ['enabled', 'disable_sni', 'server_name', 'insecure', 'alpn', 'min_version', 'max_version', 'cipher_suites', 'curve_preferences', 'certificate', 'certificate_path', 'certificate_public_key_sha256', 'client_certificate', 'client_certificate_path', 'client_key', 'client_key_path', 'fragment', 'fragment_fallback_delay', 'record_fragment', 'kernel_tx', 'kernel_rx', 'ech', 'utls', 'reality']
const ECH_FIELD_KEYS = ['enabled', 'config', 'config_path', 'query_server_name']
const UTLS_FIELD_KEYS = ['enabled', 'fingerprint']
const REALITY_FIELD_KEYS = ['enabled', 'public_key', 'short_id']
const HEADERS_ERROR_PREFIX = 'headers JSON 解析失败: '
const DOMAIN_RESOLVER_ERROR_PREFIX = 'domain_resolver 配置错误: '
const RAW_JSON_ERROR_PREFIX = '节点 JSON 解析失败: '
const MULTIPLEX_ERROR_PREFIX = 'multiplex JSON 解析失败: '
const UDP_OVER_TCP_ERROR_PREFIX = 'udp_over_tcp JSON 解析失败: '
const TRANSPORT_ERROR_PREFIX = 'transport JSON 解析失败: '
const ECH_ERROR_PREFIX = 'ECH JSON 解析失败: '
const REALITY_ERROR_PREFIX = 'Reality JSON 解析失败: '
const NETWORK_TYPE_ERROR_PREFIX = 'network_type'
const FALLBACK_NETWORK_TYPE_ERROR_PREFIX = 'fallback_network_type'
type DomainResolverMode = 'string' | 'object'

// raw is the authoritative parsed outbound; unknown keys are preserved on save.
const raw = reactive<Record<string, any>>({})
const domainResolverMode = ref<DomainResolverMode>('string')
const showDialFields = ref(false)
const showRawJSON = ref(false)
const rawJSONDraft = ref('')
const formScroller = ref<HTMLElement | null>(null)
const { preserveScroll, preserveScrollAfterUpdate } = useScrollPreserver(() => {
  const form = formScroller.value
  return [form, form?.closest<HTMLElement>('.modal-box')]
})

// manual country override
const manualCountry = ref(false)
const countryCode = ref('')

function resetFrom(node: Node | null) {
  parseError.value = ''
  schemaWarnings.value = []
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
      showRawJSON.value = false
      syncDomainResolverState()
      syncRawJSONDraft()
      manualCountry.value = node.country_source === 'manual'
      countryCode.value = node.country_code || ''
    } else {
      Object.assign(raw, { type: 'shadowsocks', tag: '', server: '', server_port: 443 })
      domainResolverMode.value = 'string'
      showDialFields.value = false
      showRawJSON.value = false
      syncRawJSONDraft()
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
function jsonText(value: unknown) {
  if (value === undefined || value === null) return ''
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return ''
  }
}
function setJSONField(field: string, value: string, prefix: string, allowBoolean = false) {
  const trimmed = value.trim()
  if (!trimmed) {
    delete raw[field]
    clearFieldError(prefix)
    return
  }
  try {
    const parsed = JSON.parse(trimmed)
    const valid = parsed !== null && ((typeof parsed === 'object' && !Array.isArray(parsed)) || (allowBoolean && typeof parsed === 'boolean'))
    if (!valid) {
      throw new Error(i18n.t('请输入有效 JSON 对象'))
    }
    raw[field] = parsed
    clearFieldError(prefix)
  } catch (e) {
    parseError.value = prefix + errMsg(e)
  }
}
function listText(value: unknown) {
  if (Array.isArray(value)) return value.join(', ')
  if (typeof value === 'string') return value
  return ''
}
function listComputed(field: string, allowed: string[] | null = null, prefix = field) {
  return computed({
    get: () => listText(raw[field]),
    set: (value: string) => {
      const items = value.split(',').map((item) => item.trim()).filter(Boolean)
      if (allowed) {
        const invalid = items.filter((item) => !allowed.includes(item))
        if (invalid.length) {
          parseError.value = `${prefix} ${i18n.t('仅支持以下值')}: ${allowed.join(', ')}`
          return
        }
      }
      if (!items.length) delete raw[field]
      else raw[field] = [...new Set(items)]
      clearFieldError(prefix)
    },
  })
}
function tlsObject(): Record<string, any> | null {
  return objectRecord(raw.tls)
}
function tlsJSONField(field: string) {
  return computed({
    get: () => tlsObject()?.[field] ?? '',
    set: (value: unknown) => {
      const target = tls()
      if (value === undefined || value === null || value === '') delete target[field]
      else target[field] = value
    },
  })
}
function tlsListComputed(field: string, allowed: string[] | null = null) {
  return computed({
    get: () => listText(tlsObject()?.[field]),
    set: (value: string) => {
      const items = value.split(',').map((item) => item.trim()).filter(Boolean)
      if (allowed) {
        const invalid = items.filter((item) => !allowed.some((candidate) => candidate.toLowerCase() === item.toLowerCase()))
        if (invalid.length) {
          parseError.value = `TLS.${field} ${i18n.t('仅支持以下值')}: ${allowed.join(', ')}`
          return
        }
      }
      const target = tls()
      const normalized = allowed
        ? items.map((item) => allowed.find((candidate) => candidate.toLowerCase() === item.toLowerCase()) ?? item)
        : items
      if (!normalized.length) delete target[field]
      else target[field] = [...new Set(normalized)]
      clearFieldError(`TLS.${field}`)
    },
  })
}
function syncRawJSONDraft() {
  rawJSONDraft.value = jsonText(raw)
}
function applyRawJSONDraft() {
  const trimmed = rawJSONDraft.value.trim()
  if (!trimmed) {
    parseError.value = RAW_JSON_ERROR_PREFIX + i18n.t('请输入有效 JSON 对象')
    return
  }
  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error(i18n.t('请输入有效 JSON 对象'))
    }
    for (const key of Object.keys(raw)) delete raw[key]
    Object.assign(raw, parsed)
    clearFieldError(RAW_JSON_ERROR_PREFIX)
    syncDomainResolverState()
  } catch (e) {
    parseError.value = RAW_JSON_ERROR_PREFIX + errMsg(e)
  }
}
function toggleRawJSON() {
  preserveScroll(() => {
    if (!showRawJSON.value) syncRawJSONDraft()
    showRawJSON.value = !showRawJSON.value
  })
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
  get: () => listText(raw.tls?.alpn),
  set: (v: string) => {
    const arr = v.split(',').map((s) => s.trim()).filter(Boolean)
    if (arr.length) tls().alpn = arr
    else if (raw.tls) delete raw.tls.alpn
  },
})
const tlsCipherSuitesText = tlsListComputed('cipher_suites')
const tlsCurvePreferencesText = tlsListComputed('curve_preferences', TLS_CURVE_PREFERENCES)
const tlsCertificateText = tlsListComputed('certificate')
const tlsCertificatePublicKeyText = tlsListComputed('certificate_public_key_sha256')
const tlsClientCertificateText = tlsListComputed('client_certificate')
const tlsClientKeyText = tlsListComputed('client_key')
const tlsDisableSNI = tlsJSONField('disable_sni')
const tlsMinVersion = tlsJSONField('min_version')
const tlsMaxVersion = tlsJSONField('max_version')
const tlsCertificatePath = tlsJSONField('certificate_path')
const tlsClientCertificatePath = tlsJSONField('client_certificate_path')
const tlsClientKeyPath = tlsJSONField('client_key_path')
const tlsFragment = tlsJSONField('fragment')
const tlsFragmentFallbackDelay = tlsJSONField('fragment_fallback_delay')
const tlsRecordFragment = tlsJSONField('record_fragment')
const tlsKernelTX = tlsJSONField('kernel_tx')
const tlsKernelRX = tlsJSONField('kernel_rx')
const tlsEchEnabled = computed({
  get: () => !!objectRecord(tlsObject()?.ech)?.enabled,
  set: (value: boolean) => ech().enabled = value,
})
const tlsEchConfigPath = computed({
  get: () => objectRecord(tlsObject()?.ech)?.config_path ?? '',
  set: (value: string) => {
    const target = ech()
    if (value.trim()) target.config_path = value.trim()
    else delete target.config_path
  },
})
const tlsEchQueryServerName = computed({
  get: () => objectRecord(tlsObject()?.ech)?.query_server_name ?? '',
  set: (value: string) => {
    const target = ech()
    if (value.trim()) target.query_server_name = value.trim()
    else delete target.query_server_name
  },
})
const tlsUTLSFingerprint = computed({
  get: () => objectRecord(tlsObject()?.utls)?.fingerprint ?? '',
  set: (value: string) => {
    const target = utls()
    if (value.trim()) {
      target.fingerprint = value.trim()
      target.enabled = true
    } else {
      delete target.fingerprint
    }
  },
})
const tlsUTLSEnabled = computed({
  get: () => !!objectRecord(tlsObject()?.utls)?.enabled,
  set: (value: boolean) => utls().enabled = value,
})
const tlsRealityEnabled = computed({
  get: () => !!objectRecord(tlsObject()?.reality)?.enabled,
  set: (value: boolean) => reality().enabled = value,
})
const tlsRealityPublicKey = computed({
  get: () => objectRecord(tlsObject()?.reality)?.public_key ?? '',
  set: (value: string) => {
    const target = reality()
    if (value.trim()) target.public_key = value.trim()
    else delete target.public_key
  },
})
const tlsRealityShortID = computed({
  get: () => objectRecord(tlsObject()?.reality)?.short_id ?? '',
  set: (value: string) => {
    const target = reality()
    if (value.trim()) target.short_id = value.trim()
    else delete target.short_id
  },
})
const echConfigText = computed({
  get: () => listText(objectRecord(raw.tls?.ech)?.config),
  set: (value: string) => {
    const items = value.split(',').map((item) => item.trim()).filter(Boolean)
    const target = ech()
    if (items.length) target.config = [...new Set(items)]
    else delete target.config
  },
})
const multiplexJSON = computed({
  get: () => jsonText(raw.multiplex),
  set: (value: string) => setJSONField('multiplex', value, MULTIPLEX_ERROR_PREFIX),
})
const udpOverTCPJSON = computed({
  get: () => jsonText(raw.udp_over_tcp),
  set: (value: string) => setJSONField('udp_over_tcp', value, UDP_OVER_TCP_ERROR_PREFIX, true),
})
const transportJSON = computed({
  get: () => jsonText(raw.transport),
  set: (value: string) => setJSONField('transport', value, TRANSPORT_ERROR_PREFIX),
})
const extraHeadersText = computed({
  get: () => jsonText(raw.extra_headers),
  set: (value: string) => setJSONField('extra_headers', value, HEADERS_ERROR_PREFIX),
})
function ech(): Record<string, any> {
  const t = tls()
  if (!t.ech || typeof t.ech !== 'object') t.ech = {}
  return t.ech
}
function echJSON(): string {
  return jsonText(tlsObject()?.ech)
}
function setECHJSON(value: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    if (raw.tls) delete raw.tls.ech
    clearFieldError(ECH_ERROR_PREFIX)
    return
  }
  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(i18n.t('请输入有效 JSON 对象'))
    tls().ech = parsed
    clearFieldError(ECH_ERROR_PREFIX)
  } catch (e) {
    parseError.value = ECH_ERROR_PREFIX + errMsg(e)
  }
}
const tlsEchJSON = computed({ get: echJSON, set: setECHJSON })
const tlsRealityJSON = computed({
  get: () => jsonText(tlsObject()?.reality),
  set: (value: string) => {
    const trimmed = value.trim()
    if (!trimmed) {
      if (raw.tls) delete raw.tls.reality
      clearFieldError(REALITY_ERROR_PREFIX)
      return
    }
    try {
      const parsed = JSON.parse(trimmed)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(i18n.t('请输入有效 JSON 对象'))
      tls().reality = parsed
      clearFieldError(REALITY_ERROR_PREFIX)
    } catch (e) {
      parseError.value = REALITY_ERROR_PREFIX + errMsg(e)
    }
  },
})
const networkText = listComputed('network', NETWORKS, 'network')
const serverPortsText = computed({
  get: () => listText(raw.server_ports),
  set: (value: string) => {
    const items = value.split(',').map((item) => item.trim()).filter(Boolean)
    if (items.length) raw.server_ports = [...new Set(items)]
    else delete raw.server_ports
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
const domainResolverStrategy = domainResolverField('strategy')
const domainResolverRewriteTTL = domainResolverField('rewrite_ttl')
const domainResolverClientSubnet = domainResolverField('client_subnet')
const domainResolverDisableCache = domainResolverField('disable_cache')
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
const supportsTLS = computed(() => TLS_PROTOCOLS.includes(String(raw.type)))
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
  schemaWarnings.value = []
  for (const k of Object.keys(raw)) delete raw[k]
  domainResolverMode.value = 'string'
  showDialFields.value = false
  showRawJSON.value = false
  syncRawJSONDraft()
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
  preserveScroll(() => {
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
  })
}

function toggleDialFields() {
  preserveScroll(() => {
    showDialFields.value = !showDialFields.value
  })
}

function domainResolverWarnings() {
  const warnings: string[] = []
  const value = raw.domain_resolver
  if (value === undefined || value === null || value === '') return warnings
  if (typeof value === 'string') {
    if (!value.trim()) warnings.push('domain_resolver 不能为空')
    return warnings
  }
  const resolver = objectRecord(value)
  if (!resolver) return ['domain_resolver 必须是服务器标签字符串或对象']
  const allowed = new Set(['server', 'strategy', 'disable_cache', 'rewrite_ttl', 'client_subnet'])
  const unknown = Object.keys(resolver).filter((key) => !allowed.has(key))
  if (unknown.length) warnings.push(`domain_resolver 包含 1.14.0 不支持的字段: ${unknown.join(', ')}`)
  if (typeof resolver.server !== 'string' || !resolver.server.trim()) warnings.push('domain_resolver.server 不能为空')
  if (resolver.strategy !== undefined && !DOMAIN_RESOLVER_STRATEGIES.includes(String(resolver.strategy))) {
    warnings.push(`domain_resolver.strategy 只能是: ${DOMAIN_RESOLVER_STRATEGIES.filter(Boolean).join(', ')}`)
  }
  for (const key of ['disable_cache']) {
    if (resolver[key] !== undefined && typeof resolver[key] !== 'boolean') warnings.push(`domain_resolver.${key} 必须是布尔值`)
  }
  if (resolver.rewrite_ttl !== undefined && (!Number.isInteger(resolver.rewrite_ttl) || resolver.rewrite_ttl < 0 || resolver.rewrite_ttl > 4294967295)) {
    warnings.push('domain_resolver.rewrite_ttl 必须是 0 到 4294967295 的整数')
  }
  if (resolver.client_subnet !== undefined && typeof resolver.client_subnet !== 'string') {
    warnings.push('domain_resolver.client_subnet 必须是 CIDR 字符串')
  }
  return warnings
}

function unknownObjectFields(value: Record<string, any>, allowed: string[], prefix: string) {
  const unknown = Object.keys(value).filter((key) => !allowed.includes(key))
  return unknown.length ? [`${prefix} 包含 1.14.0 未识别的字段: ${unknown.join(', ')}`] : []
}

function transportWarnings(value: unknown) {
  const transport = objectRecord(value)
  if (!transport) return []
  const type = String(transport.type ?? '')
  const allowed: Record<string, string[]> = {
    ws: ['type', 'path', 'headers', 'max_early_data', 'early_data_header_name'],
    http: ['type', 'host', 'path', 'method', 'headers', 'idle_timeout', 'ping_timeout'],
    grpc: ['type', 'service_name', 'idle_timeout', 'ping_timeout', 'permit_without_stream'],
    httpupgrade: ['type', 'host', 'path', 'headers'],
    quic: ['type'],
  }
  if (!allowed[type]) return [`transport.type ${type || i18n.t('不能为空')}，不在 sing-box 1.14.0 支持列表中`]
  return unknownObjectFields(transport, allowed[type], `transport.${type}`)
}

function nestedSchemaWarnings(value: unknown, prefix: string, allowed: string[]) {
  const object = objectRecord(value)
  return object ? unknownObjectFields(object, allowed, prefix) : []
}

function unknownSchemaWarnings() {
  const type = String(raw.type ?? '')
  const allowedProtocolFields = PROTOCOL_FIELD_KEYS[type]
  if (!allowedProtocolFields) return []
  const allowed = new Set([...DIAL_FIELD_KEYS, 'type', 'tag', 'server', 'server_port', ...allowedProtocolFields])
  const warnings = unknownObjectFields(raw, [...allowed], type)
  const tlsValue = tlsObject()
  if (tlsValue) {
    warnings.push(...unknownObjectFields(tlsValue, TLS_FIELD_KEYS, 'tls'))
    warnings.push(...nestedSchemaWarnings(tlsValue.ech, 'tls.ech', ECH_FIELD_KEYS))
    warnings.push(...nestedSchemaWarnings(tlsValue.utls, 'tls.utls', UTLS_FIELD_KEYS))
    warnings.push(...nestedSchemaWarnings(tlsValue.reality, 'tls.reality', REALITY_FIELD_KEYS))
  }
  if (raw.transport) warnings.push(...transportWarnings(raw.transport))
  return warnings
}

function collectSchemaWarnings() {
  const warnings = [...domainResolverWarnings(), ...unknownSchemaWarnings()]
  const serverPort = Number(raw.server_port)
  if (!Number.isInteger(serverPort) || serverPort < 0 || serverPort > 65535) warnings.push('server_port 必须是 0 到 65535 的整数')
  const nonNegativeIntegers = ['alter_id', 'up_mbps', 'down_mbps', 'recv_window_conn', 'recv_window', 'min_idle_session', 'insecure_concurrency']
  for (const field of nonNegativeIntegers) {
    const value = raw[field]
    if (value !== undefined && (!Number.isInteger(value) || value < 0)) warnings.push(`${field} 必须是非负整数`)
  }
  const durationFields = [
    'connect_timeout',
    'tcp_keep_alive',
    'tcp_keep_alive_interval',
    'fallback_delay',
    'hop_interval',
    'idle_session_check_interval',
    'idle_session_timeout',
    'heartbeat',
    'fragment_fallback_delay',
  ]
  for (const field of durationFields) {
    const value = raw[field]
    if (value !== undefined && (typeof value !== 'string' || !SING_BOX_DURATION_PATTERN.test(value.trim()))) {
      warnings.push(`${field} 必须是 sing-box duration，例如 5s`)
    }
  }
  const network = Array.isArray(raw.network) ? raw.network : raw.network ? [raw.network] : []
  if (network.some((value: unknown) => !NETWORKS.includes(String(value)))) warnings.push('network 只能包含 tcp 或 udp')
  if (raw.network_strategy !== undefined && raw.network_strategy !== '' && !NETWORK_STRATEGIES.includes(String(raw.network_strategy))) {
    warnings.push(`network_strategy 只能是: ${NETWORK_STRATEGIES.join(', ')}`)
  }
  if (raw.network_strategy && (raw.bind_interface || raw.inet4_bind_address || raw.inet6_bind_address)) {
    warnings.push('network_strategy 与 bind_interface/inet4_bind_address/inet6_bind_address 不能同时使用')
  }
  for (const [field, allowed] of [['network_type', NETWORK_TYPES], ['fallback_network_type', NETWORK_TYPES]] as const) {
    const values = Array.isArray(raw[field]) ? raw[field] : raw[field] ? [raw[field]] : []
    if (values.some((value: unknown) => !allowed.includes(String(value)))) warnings.push(`${field} ${i18n.t('仅支持以下值')}: ${allowed.join(', ')}`)
  }
  if (raw.type === 'vmess') {
    if (raw.security !== undefined && !VMESS_SECURITY.includes(String(raw.security))) warnings.push(`security 只能是: ${VMESS_SECURITY.join(', ')}`)
    if (raw.packet_encoding !== undefined && !PACKET_ENCODINGS.includes(String(raw.packet_encoding))) warnings.push(`packet_encoding 只能是: packetaddr, xudp`)
  }
  if (raw.type === 'vless' && raw.packet_encoding !== undefined && !PACKET_ENCODINGS.includes(String(raw.packet_encoding))) {
    warnings.push('packet_encoding 只能是: packetaddr, xudp')
  }
  if (raw.type === 'snell') {
    const version = Number(raw.version)
    if (version === 4 && raw.obfs_mode && !['none', 'http', 'tls'].includes(String(raw.obfs_mode))) warnings.push('Snell v4 obfs_mode 只能是 none、http 或 tls')
    if (version === 6 && raw.obfs_mode) warnings.push('Snell v6 不支持 obfs_mode')
    if (version === 4 && raw.mode) warnings.push('Snell v4 不支持 mode')
  }
  if (raw.type === 'hysteria2') {
    const obfsValue = objectRecord(raw.obfs)
    if (obfsValue?.type && obfsValue.type !== 'salamander') warnings.push('hysteria2.obfs.type 只能是 salamander')
  }
  if (raw.type === 'tuic') {
    if (raw.congestion_control !== undefined && !['', 'cubic', 'new_reno', 'bbr'].includes(String(raw.congestion_control))) warnings.push('congestion_control 只能是 cubic、new_reno 或 bbr')
    if (raw.udp_relay_mode !== undefined && !['', 'native', 'quic'].includes(String(raw.udp_relay_mode))) warnings.push('udp_relay_mode 只能是 native 或 quic')
    if (raw.udp_over_stream && raw.udp_relay_mode) warnings.push('udp_over_stream 与 udp_relay_mode 不能同时使用')
  }
  const tlsValue = tlsObject()
  if (tlsValue) {
    if (!supportsTLS.value) warnings.push(`${raw.type || '当前协议'} 不支持出站 TLS 设置`)
    for (const [field, allowed] of [['min_version', TLS_VERSIONS], ['max_version', TLS_VERSIONS], ['utls.fingerprint', UTLS_FINGERPRINTS]] as const) {
      const value = field === 'utls.fingerprint' ? objectRecord(tlsValue.utls)?.fingerprint : tlsValue[field]
      if (value !== undefined && value !== '' && !allowed.includes(String(value))) warnings.push(`TLS.${field} 值不在 sing-box 1.14.0 schema 中`)
    }
    for (const field of ['fragment_fallback_delay']) {
      const value = tlsValue[field]
      if (value !== undefined && (typeof value !== 'string' || !SING_BOX_DURATION_PATTERN.test(value.trim()))) warnings.push(`TLS.${field} 必须是 sing-box duration，例如 5s`)
    }
    if (objectRecord(tlsValue.reality)?.enabled && !objectRecord(tlsValue.utls)?.enabled) {
      warnings.push('TLS.reality 需要同时启用 TLS.utls')
    }
    const shortID = objectRecord(tlsValue.reality)?.short_id
    if (shortID !== undefined && (!/^[0-9a-fA-F]{0,8}$/.test(String(shortID)))) {
      warnings.push('TLS.reality.short_id 必须是最多 8 位十六进制字符串')
    }
  }
  schemaWarnings.value = [...new Set(warnings)]
  return schemaWarnings.value
}

function listFieldText(value: unknown) {
  return listText(value)
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
  if (version === 4 && raw.obfs_mode && !['none', 'http', 'tls'].includes(raw.obfs_mode)) {
    throw new Error('Snell obfs_mode 只能是 none、http 或 tls')
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
    collectSchemaWarnings()
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
    preserveScroll(() => {
      const full = props.node ?? props.copyFrom ?? null
      if (full) resetFrom(full)
      else if (currentMode.value === 'create') resetFrom(null)
      else resetPending()
    })
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
    if (!TLS_PROTOCOLS.includes(String(next))) delete raw.tls
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
watch(
  raw,
  () => {
    if (!hydrating.value) preserveScroll(() => collectSchemaWarnings())
  },
  { deep: true },
)
watch([manualCountry, parseError], () => {
  if (!hydrating.value) preserveScrollAfterUpdate()
})
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-2xl">
      <h3 class="font-bold text-lg mb-3">{{ formTitle }}</h3>

      <div v-if="parseError" class="alert alert-error text-sm mb-3">
        <span>{{ parseError }}</span>
      </div>
      <div v-if="schemaWarnings.length" class="alert alert-warning text-sm mb-3">
        <div>
          <div>{{ i18n.t('检测到 sing-box 1.14.0 设置提示，保存后请用内核校验') }}</div>
          <ul class="list-disc pl-5 mt-1">
            <li v-for="warning in schemaWarnings" :key="warning">{{ warning }}</li>
          </ul>
        </div>
      </div>

      <div v-if="loading" class="alert py-2 mb-3">
        <span class="loading loading-spinner loading-sm"></span>
        <span class="text-sm">
          {{ i18n.t('正在加载节点...') }}
          <span v-if="summaryLine" class="opacity-70">· {{ summaryLine }}</span>
        </span>
      </div>

      <fieldset ref="formScroller" class="flex flex-col gap-3 max-h-[65vh] overflow-y-auto pr-1" :disabled="loading" :class="{ 'opacity-60': loading }">
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
          <label class="form-control">
            <span class="label-text mb-1">plugin</span>
            <input v-model="raw.plugin" class="input input-bordered input-sm" placeholder="obfs-local" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">plugin_opts</span>
            <input v-model="raw.plugin_opts" class="input input-bordered input-sm" placeholder="obfs=http;obfs-host=example.com" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">network</span>
            <input v-model="networkText" class="input input-bordered input-sm" placeholder="tcp, udp" />
          </label>
          <label class="form-control col-span-2">
            <span class="label-text mb-1">udp_over_tcp / multiplex JSON</span>
            <div class="grid grid-cols-2 gap-3">
              <textarea v-model="udpOverTCPJSON" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder="true 或 { &quot;enabled&quot;: true, &quot;version&quot;: 2 }"></textarea>
              <textarea v-model="multiplexJSON" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder='{ "enabled": true, "protocol": "smux" }'></textarea>
            </div>
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
            <select v-model="raw.security" class="select select-bordered select-sm">
              <option v-for="security in VMESS_SECURITY" :key="security" :value="security">{{ security }}</option>
            </select>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">network</span>
            <input v-model="networkText" class="input input-bordered input-sm" placeholder="tcp, udp" />
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.global_padding" />
            <span class="label-text">global_padding</span>
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.authenticated_length" />
            <span class="label-text">authenticated_length</span>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">packet_encoding</span>
            <select v-model="raw.packet_encoding" class="select select-bordered select-sm">
              <option v-for="encoding in PACKET_ENCODINGS" :key="encoding" :value="encoding">{{ encoding || i18n.t('未指定') }}</option>
            </select>
          </label>
          <label class="form-control col-span-2">
            <span class="label-text mb-1">multiplex JSON</span>
            <textarea v-model="multiplexJSON" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder='{ "enabled": true, "protocol": "smux" }'></textarea>
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
          <label class="form-control">
            <span class="label-text mb-1">network</span>
            <input v-model="networkText" class="input input-bordered input-sm" placeholder="tcp, udp" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">packet_encoding</span>
            <select v-model="raw.packet_encoding" class="select select-bordered select-sm">
              <option v-for="encoding in PACKET_ENCODINGS" :key="encoding" :value="encoding">{{ encoding || i18n.t('未指定') }}</option>
            </select>
          </label>
          <label class="form-control col-span-2">
            <span class="label-text mb-1">multiplex JSON</span>
            <textarea v-model="multiplexJSON" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder='{ "enabled": true, "protocol": "smux" }'></textarea>
          </label>
        </div>

        <!-- trojan -->
        <div v-else-if="raw.type === 'trojan'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">network</span>
            <input v-model="networkText" class="input input-bordered input-sm" placeholder="tcp, udp" />
          </label>
          <label class="form-control col-span-2">
            <span class="label-text mb-1">multiplex JSON</span>
            <textarea v-model="multiplexJSON" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder='{ "enabled": true, "protocol": "smux" }'></textarea>
          </label>
        </div>

        <!-- hysteria -->
        <div v-else-if="raw.type === 'hysteria'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">server_ports</span>
            <input v-model="serverPortsText" class="input input-bordered input-sm" placeholder="2080:3000" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">hop_interval</span>
            <input v-model="raw.hop_interval" class="input input-bordered input-sm" placeholder="30s" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">auth_str</span>
            <input v-model="raw.auth_str" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">auth (base64)</span>
            <input v-model="raw.auth" class="input input-bordered input-sm" />
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
            <input v-model="networkText" class="input input-bordered input-sm" placeholder="tcp, udp" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">up / down</span>
            <div class="grid grid-cols-2 gap-2">
              <input v-model="raw.up" class="input input-bordered input-sm" placeholder="100 Mbps" />
              <input v-model="raw.down" class="input input-bordered input-sm" placeholder="100 Mbps" />
            </div>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">recv_window_conn</span>
            <input v-model.number="raw.recv_window_conn" type="number" min="0" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">recv_window</span>
            <input v-model.number="raw.recv_window" type="number" min="0" class="input input-bordered input-sm" />
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.disable_mtu_discovery" />
            <span class="label-text">disable_mtu_discovery</span>
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
            <label class="form-control">
              <span class="label-text mb-1">server_ports</span>
              <input v-model="serverPortsText" class="input input-bordered input-sm" placeholder="2080:3000" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">hop_interval</span>
              <input v-model="raw.hop_interval" class="input input-bordered input-sm" placeholder="30s" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">network</span>
              <input v-model="networkText" class="input input-bordered input-sm" placeholder="tcp, udp" />
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="raw.brutal_debug" />
              <span class="label-text">brutal_debug</span>
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
              <select v-model="raw.congestion_control" class="select select-bordered select-sm">
                <option value="">{{ i18n.t('未指定') }}</option>
                <option value="cubic">cubic</option>
                <option value="new_reno">new_reno</option>
                <option value="bbr">bbr</option>
              </select>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">udp_relay_mode</span>
              <select v-model="raw.udp_relay_mode" class="select select-bordered select-sm">
                <option value="">{{ i18n.t('未指定') }}</option>
                <option value="native">native</option>
                <option value="quic">quic</option>
              </select>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">heartbeat</span>
              <input v-model="raw.heartbeat" class="input input-bordered input-sm" placeholder="10s" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">network</span>
              <input v-model="networkText" class="input input-bordered input-sm" placeholder="tcp, udp" />
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="raw.udp_over_stream" />
              <span class="label-text">udp_over_stream</span>
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="raw.zero_rtt_handshake" />
              <span class="label-text">zero_rtt_handshake</span>
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
                <option value="tls">tls</option>
              </select>
            </label>
            <label v-if="['http', 'tls'].includes(raw.obfs_mode)" class="form-control">
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
            <label class="form-control col-span-2">
              <span class="label-text mb-1">client_metadata</span>
              <input v-model="raw.client_metadata" class="input input-bordered input-sm" />
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
            <select v-model.number="raw.version" class="select select-bordered select-sm">
              <option :value="1">1</option>
              <option :value="2">2</option>
              <option :value="3">3</option>
            </select>
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
            <label class="form-control">
              <span class="label-text mb-1">insecure_concurrency</span>
              <input v-model.number="raw.insecure_concurrency" type="number" min="0" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">stream_receive_window</span>
              <input v-model="raw.stream_receive_window" class="input input-bordered input-sm" placeholder="16MB" />
            </label>
          </div>
          <label class="form-control">
            <span class="label-text mb-1">extra_headers JSON</span>
            <textarea v-model="extraHeadersText" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder='{ "User-Agent": "..." }'></textarea>
          </label>
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
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="raw.udp_over_tcp" />
              <span class="label-text">udp_over_tcp</span>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">quic_session_receive_window</span>
              <input v-model="raw.quic_session_receive_window" class="input input-bordered input-sm" placeholder="16MB" />
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
              <input v-model="networkText" class="input input-bordered input-sm" placeholder="tcp, udp" />
            </label>
            <label class="form-control col-span-2">
              <span class="label-text mb-1">udp_over_tcp JSON</span>
              <textarea v-model="udpOverTCPJSON" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder="true 或 { &quot;enabled&quot;: true, &quot;version&quot;: 2 }"></textarea>
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
                <option value="quic">quic</option>
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
            <label class="form-control col-span-2">
              <span class="label-text mb-1">transport advanced JSON</span>
              <textarea v-model="transportJSON" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder='{ "type": "ws", "max_early_data": 2048 }'></textarea>
            </label>
          </div>
        </div>

        <!-- shared outbound TLS block -->
        <template v-if="supportsTLS">
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
              <input type="checkbox" class="toggle toggle-sm" v-model="raw.tls.insecure" />
              <span class="label-text">insecure</span>
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="tlsDisableSNI" />
              <span class="label-text">disable_sni</span>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">min_version</span>
              <select v-model="tlsMinVersion" class="select select-bordered select-sm">
                <option v-for="version in TLS_VERSIONS" :key="version" :value="version">{{ version || i18n.t('未指定') }}</option>
              </select>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">max_version</span>
              <select v-model="tlsMaxVersion" class="select select-bordered select-sm">
                <option v-for="version in TLS_VERSIONS" :key="version" :value="version">{{ version || i18n.t('未指定') }}</option>
              </select>
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">证书与密码套件</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">cipher_suites</span>
              <input v-model="tlsCipherSuitesText" class="input input-bordered input-sm" placeholder="TLS_AES_128_GCM_SHA256" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">curve_preferences</span>
              <input v-model="tlsCurvePreferencesText" class="input input-bordered input-sm" placeholder="X25519, P256" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">certificate</span>
              <input v-model="tlsCertificateText" class="input input-bordered input-sm" placeholder="PEM 内容，可逗号分隔" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">certificate_path</span>
              <input v-model="tlsCertificatePath" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">certificate_public_key_sha256</span>
              <input v-model="tlsCertificatePublicKeyText" class="input input-bordered input-sm" placeholder="base64 sha256" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">client_certificate</span>
              <input v-model="tlsClientCertificateText" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">client_certificate_path</span>
              <input v-model="tlsClientCertificatePath" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">client_key</span>
              <input v-model="tlsClientKeyText" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">client_key_path</span>
              <input v-model="tlsClientKeyPath" class="input input-bordered input-sm" />
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">TLS 高级参数</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="tlsFragment" />
              <span class="label-text">fragment</span>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">fragment_fallback_delay</span>
              <input v-model="tlsFragmentFallbackDelay" class="input input-bordered input-sm" placeholder="500ms" />
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="tlsRecordFragment" />
              <span class="label-text">record_fragment</span>
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="tlsKernelTX" />
              <span class="label-text">kernel_tx</span>
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="tlsKernelRX" />
              <span class="label-text">kernel_rx</span>
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">uTLS</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="tlsUTLSEnabled" />
              <span class="label-text">enabled</span>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">fingerprint</span>
              <select v-model="tlsUTLSFingerprint" class="select select-bordered select-sm">
                <option v-for="fingerprint in UTLS_FINGERPRINTS" :key="fingerprint" :value="fingerprint">{{ fingerprint || i18n.t('未指定') }}</option>
              </select>
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">Reality</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="tlsRealityEnabled" />
              <span class="label-text">{{ i18n.t('启用') }}</span>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">public_key</span>
              <input v-model="tlsRealityPublicKey" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">short_id</span>
              <input v-model="tlsRealityShortID" class="input input-bordered input-sm" />
            </label>
            <label class="form-control col-span-2">
              <span class="label-text mb-1">Reality advanced JSON</span>
              <textarea v-model="tlsRealityJSON" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder='{ "enabled": true, "public_key": "..." }'></textarea>
            </label>
          </div>
          <div class="divider my-0 text-xs opacity-50">ECH</div>
          <div class="grid grid-cols-2 gap-3">
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="tlsEchEnabled" />
              <span class="label-text">enabled</span>
            </label>
            <label class="form-control">
              <span class="label-text mb-1">config</span>
              <input v-model="echConfigText" class="input input-bordered input-sm" placeholder="ECH config，可逗号分隔" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">config_path</span>
              <input v-model="tlsEchConfigPath" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">query_server_name</span>
              <input v-model="tlsEchQueryServerName" class="input input-bordered input-sm" />
            </label>
            <label class="form-control col-span-2">
              <span class="label-text mb-1">ECH advanced JSON</span>
              <textarea v-model="tlsEchJSON" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-16" placeholder='{ "enabled": true, "config_path": "..." }'></textarea>
            </label>
          </div>
        </div>
        </template>

        <div class="divider my-0 text-xs opacity-60">
          <span>{{ i18n.t('拨号字段') }}</span>
          <button type="button" class="btn btn-xs btn-ghost btn-square" :title="i18n.t('拨号字段')" @click="toggleDialFields">
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
              <button type="button" class="btn btn-sm join-item" :class="domainResolverMode === 'string' ? 'btn-primary' : 'btn-ghost'" @click="setDomainResolverMode('string')">
                {{ i18n.t('简单模式') }}
              </button>
              <button type="button" class="btn btn-sm join-item" :class="domainResolverMode === 'object' ? 'btn-primary' : 'btn-ghost'" @click="setDomainResolverMode('object')">
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

        <div class="divider my-0 text-xs opacity-60">
          <span>{{ i18n.t('原始出站 JSON') }}</span>
          <button type="button" class="btn btn-xs btn-ghost btn-square" :title="i18n.t('原始出站 JSON')" @click="toggleRawJSON">
            <ChevronUpIcon v-if="showRawJSON" class="h-3 w-3" />
            <ChevronDownIcon v-else class="h-3 w-3" />
          </button>
        </div>
        <div v-if="showRawJSON" class="flex flex-col gap-2">
          <p class="text-xs opacity-70">{{ i18n.t('用于编辑结构化表单未覆盖的字段；JSON 语法错误会阻止保存，schema 提示不会阻止保存。') }}</p>
          <textarea v-model="rawJSONDraft" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-48" @input="applyRawJSONDraft"></textarea>
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
            {{ countryFlagEmoji(c.code) }}{{ c.code }} — {{ c.name }}
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
