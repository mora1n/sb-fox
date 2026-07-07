<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { Node, NodeSummary } from '../api/types'
import { useNodesStore } from '../stores/nodes'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import { COUNTRY_CODES, sortCountryOptions } from '../utils/countries'

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
  'anytls',
  'shadowtls',
  'naive',
  'http',
  'socks',
]
const NETWORK_STRATEGIES = ['default', 'hybrid', 'fallback']
const NETWORK_TYPES = ['wifi', 'cellular', 'ethernet', 'other']
const HEADERS_ERROR_PREFIX = 'headers JSON 解析失败: '
const DOMAIN_RESOLVER_ERROR_PREFIX = 'domain_resolver JSON 解析失败: '
const NETWORK_TYPE_ERROR_PREFIX = 'network_type'
const FALLBACK_NETWORK_TYPE_ERROR_PREFIX = 'fallback_network_type'
type DomainResolverMode = 'string' | 'json'

// raw is the authoritative parsed outbound; unknown keys are preserved on save.
const raw = reactive<Record<string, any>>({})
const domainResolverMode = ref<DomainResolverMode>('string')
const domainResolverObjectDraft = ref('{\n  "server": ""\n}')
let lastDomainResolverObject: Record<string, any> | null = null

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
      syncDomainResolverState()
      manualCountry.value = node.country_source === 'manual'
      countryCode.value = node.country_code || ''
    } else {
      Object.assign(raw, { type: 'shadowsocks', tag: '', server: '', server_port: 443 })
      lastDomainResolverObject = null
      domainResolverMode.value = 'string'
      domainResolverObjectDraft.value = '{\n  "server": ""\n}'
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
const domainResolverJSON = computed({
  get: () => {
    const value = raw.domain_resolver
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      rememberDomainResolverObject(value)
      try {
        domainResolverObjectDraft.value = JSON.stringify(value, null, 2)
      } catch {
        domainResolverObjectDraft.value = '{\n  "server": ""\n}'
      }
    }
    return domainResolverObjectDraft.value
  },
  set: (v: string) => {
    domainResolverObjectDraft.value = v
    const trimmed = v.trim()
    if (!trimmed) {
      delete raw.domain_resolver
      clearFieldError(DOMAIN_RESOLVER_ERROR_PREFIX)
      return
    }
    try {
      const parsed = JSON.parse(trimmed)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        throw new Error(i18n.t('请输入有效 JSON 对象'))
      }
      raw.domain_resolver = parsed
      rememberDomainResolverObject(parsed)
      clearFieldError(DOMAIN_RESOLVER_ERROR_PREFIX)
    } catch (e) {
      parseError.value = DOMAIN_RESOLVER_ERROR_PREFIX + errMsg(e)
    }
  },
})
const networkTypeText = computed({
  get: () => listFieldText(raw.network_type),
  set: (v: string) => setListField('network_type', v, NETWORK_TYPES, NETWORK_TYPE_ERROR_PREFIX),
})
const fallbackNetworkTypeText = computed({
  get: () => listFieldText(raw.fallback_network_type),
  set: (v: string) => setListField('fallback_network_type', v, NETWORK_TYPES, FALLBACK_NETWORK_TYPE_ERROR_PREFIX),
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

function resetPending() {
  parseError.value = ''
  for (const k of Object.keys(raw)) delete raw[k]
  lastDomainResolverObject = null
  domainResolverMode.value = 'string'
  domainResolverObjectDraft.value = '{\n  "server": ""\n}'
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

function rememberDomainResolverObject(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return
  try {
    lastDomainResolverObject = JSON.parse(JSON.stringify(value))
  } catch {
    lastDomainResolverObject = null
  }
}

function syncDomainResolverState() {
  const value = raw.domain_resolver
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    domainResolverMode.value = 'json'
    rememberDomainResolverObject(value)
    try {
      domainResolverObjectDraft.value = JSON.stringify(value, null, 2)
    } catch {
      domainResolverObjectDraft.value = '{\n  "server": ""\n}'
    }
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
  if (next === 'json') {
    if (current && typeof current === 'object' && !Array.isArray(current)) {
      rememberDomainResolverObject(current)
      domainResolverObjectDraft.value = JSON.stringify(current, null, 2)
    } else if (lastDomainResolverObject) {
      const nextObj = JSON.parse(JSON.stringify(lastDomainResolverObject))
      if (typeof current === 'string' && current.trim()) nextObj.server = current.trim()
      domainResolverObjectDraft.value = JSON.stringify(nextObj, null, 2)
    } else if (typeof current === 'string' && current.trim()) {
      domainResolverObjectDraft.value = JSON.stringify({ server: current.trim() }, null, 2)
    } else {
      domainResolverObjectDraft.value = '{\n  "server": ""\n}'
    }
  } else {
    if (current && typeof current === 'object' && !Array.isArray(current)) {
      rememberDomainResolverObject(current)
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
    if (defaultTLSEnabled(next)) tls().enabled = true
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
        <div v-else-if="raw.type === 'hysteria2'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('obfs 类型') }}</span>
            <input
              :value="raw.obfs?.type"
              class="input input-bordered input-sm"
              @input="(raw.obfs ||= {}).type = ($event.target as HTMLInputElement).value"
            />
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

        <!-- tuic -->
        <div v-else-if="raw.type === 'tuic'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">UUID</span>
            <input v-model="raw.uuid" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
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

        <!-- anytls -->
        <div v-else-if="raw.type === 'anytls'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
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
        <div v-else-if="raw.type === 'naive'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('用户名') }}</span>
            <input v-model="raw.username" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input type="checkbox" class="toggle toggle-sm" v-model="raw.quic" />
            <span class="label-text">QUIC</span>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">quic_congestion_control</span>
            <input v-model="raw.quic_congestion_control" class="input input-bordered input-sm" placeholder="bbr" />
          </label>
        </div>

        <!-- http -->
        <div v-else-if="raw.type === 'http'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('用户名') }}</span>
            <input v-model="raw.username" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">path</span>
            <input v-model="raw.path" class="input input-bordered input-sm" />
          </label>
          <label class="form-control col-span-2">
            <span class="label-text mb-1">headers JSON</span>
            <textarea v-model="headersText" class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-20" placeholder='{ "Host": "example.com" }'></textarea>
          </label>
        </div>

        <!-- socks -->
        <div v-else-if="raw.type === 'socks'" class="grid grid-cols-2 gap-3">
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
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('用户名') }}</span>
            <input v-model="raw.username" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
        </div>

        <div v-if="['vmess', 'vless', 'trojan'].includes(raw.type)" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">transport.type</span>
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
            <span class="label-text mb-1">transport.path</span>
            <input
              :value="raw.transport?.path"
              class="input input-bordered input-sm"
              @input="transport().path = ($event.target as HTMLInputElement).value"
            />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">grpc.service_name</span>
            <input
              :value="raw.transport?.service_name"
              class="input input-bordered input-sm"
              @input="transport().service_name = ($event.target as HTMLInputElement).value"
            />
          </label>
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
        <div v-if="raw.tls?.enabled" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('服务名') }}</span>
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
            <span class="label-text">{{ i18n.t('允许不安全') }}</span>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">uTLS 指纹</span>
            <input
              :value="raw.tls?.utls?.fingerprint"
              class="input input-bordered input-sm"
              placeholder="chrome"
              @input="((utls().fingerprint = ($event.target as HTMLInputElement).value), (utls().enabled = true))"
            />
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input
              type="checkbox"
              class="toggle toggle-sm"
              :checked="!!raw.tls?.reality?.enabled"
              @change="reality().enabled = ($event.target as HTMLInputElement).checked"
            />
            <span class="label-text">Reality</span>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">Reality public_key</span>
            <input
              :value="raw.tls?.reality?.public_key"
              class="input input-bordered input-sm"
              @input="((reality().public_key = ($event.target as HTMLInputElement).value), (reality().enabled = true))"
            />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">Reality short_id</span>
            <input
              :value="raw.tls?.reality?.short_id"
              class="input input-bordered input-sm"
              @input="((reality().short_id = ($event.target as HTMLInputElement).value), (reality().enabled = true))"
            />
          </label>
        </div>

        <div class="divider my-0 text-xs opacity-60">{{ i18n.t('拨号字段') }}</div>
        <div class="grid grid-cols-2 gap-3">
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
          <label class="form-control col-span-2">
            <span class="label-text mb-1">{{ i18n.t('域名解析器') }}</span>
            <div class="join mb-2">
              <button type="button" class="btn btn-sm join-item" :class="{ 'btn-active': domainResolverMode === 'string' }" @click="setDomainResolverMode('string')">
                {{ i18n.t('简单模式') }}
              </button>
              <button type="button" class="btn btn-sm join-item" :class="{ 'btn-active': domainResolverMode === 'json' }" @click="setDomainResolverMode('json')">
                {{ i18n.t('高级 JSON') }}
              </button>
            </div>
            <input
              v-if="domainResolverMode === 'string'"
              v-model="domainResolverText"
              class="input input-bordered input-sm"
              :placeholder="i18n.t('域名解析器标签')"
            />
            <textarea
              v-else
              v-model="domainResolverJSON"
              class="textarea textarea-bordered textarea-sm font-mono text-xs min-h-24"
              :placeholder="i18n.t('域名解析器 JSON')"
            ></textarea>
          </label>
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
            {{ c.code }} — {{ c.name }}
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
