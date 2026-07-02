<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { Node } from '../api/types'
import { useNodesStore } from '../stores/nodes'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { errMsg } from '../utils/error'
import { COUNTRY_CODES, sortCountryOptions } from '../utils/countries'

const props = defineProps<{ node: Node | null }>()
const emit = defineEmits<{ close: []; saved: [] }>()

const nodesStore = useNodesStore()
const settings = useSettingsStore()
const ui = useUiStore()
const busy = ref(false)
const parseError = ref('')

const PROTOCOLS = ['shadowsocks', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic']

// raw is the authoritative parsed outbound; unknown keys are preserved on save.
const raw = reactive<Record<string, any>>({})

// manual country override
const manualCountry = ref(false)
const countryCode = ref('')

function resetFrom(node: Node | null) {
  parseError.value = ''
  for (const k of Object.keys(raw)) delete raw[k]
  if (node) {
    try {
      Object.assign(raw, JSON.parse(node.raw))
    } catch (e) {
      parseError.value = 'raw JSON 解析失败: ' + errMsg(e)
    }
    manualCountry.value = node.country_source === 'manual'
    countryCode.value = node.country_code || ''
  } else {
    Object.assign(raw, { type: 'shadowsocks', tag: '', server: '', server_port: 443 })
    manualCountry.value = false
    countryCode.value = ''
  }
}
watch(() => props.node, (n) => resetFrom(n ?? null), { immediate: true })

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

// alpn is an array in sing-box; edit it as comma-separated text
const alpnText = computed({
  get: () => (Array.isArray(raw.tls?.alpn) ? raw.tls.alpn.join(', ') : ''),
  set: (v: string) => {
    const arr = v.split(',').map((s) => s.trim()).filter(Boolean)
    if (arr.length) tls().alpn = arr
    else if (raw.tls) delete raw.tls.alpn
  },
})

const countryOptions = computed(() => sortCountryOptions(COUNTRY_CODES, settings.countryHeatOrder))

async function save() {
  busy.value = true
  try {
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
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-2xl">
      <h3 class="font-bold text-lg mb-3">{{ node ? '编辑节点' : '新建节点' }}</h3>

      <div v-if="parseError" class="alert alert-error text-sm mb-3">
        <span>{{ parseError }}</span>
      </div>

      <div class="flex flex-col gap-3 max-h-[65vh] overflow-y-auto pr-1">
        <!-- header: type / tag / server / port -->
        <div class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">协议类型</span>
            <select v-model="raw.type" class="select select-bordered select-sm">
              <option v-for="p in PROTOCOLS" :key="p" :value="p">{{ p }}</option>
              <option v-if="!PROTOCOLS.includes(raw.type)" :value="raw.type">{{ raw.type }}</option>
            </select>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">标签 (tag)</span>
            <input v-model="raw.tag" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">服务器 (server)</span>
            <input v-model="raw.server" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">端口 (server_port)</span>
            <input v-model.number="raw.server_port" type="number" class="input input-bordered input-sm" />
          </label>
        </div>

        <div class="divider my-0 text-xs opacity-60">协议参数</div>

        <!-- shadowsocks -->
        <div v-if="raw.type === 'shadowsocks'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">加密方式 (method)</span>
            <input v-model="raw.method" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">密码 (password)</span>
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
            <span class="label-text mb-1">密码 (password)</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
        </div>

        <!-- hysteria2 -->
        <div v-else-if="raw.type === 'hysteria2'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">密码 (password)</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">obfs 类型</span>
            <input
              :value="raw.obfs?.type"
              class="input input-bordered input-sm"
              @input="(raw.obfs ||= {}).type = ($event.target as HTMLInputElement).value"
            />
          </label>
        </div>

        <!-- tuic -->
        <div v-else-if="raw.type === 'tuic'" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">UUID</span>
            <input v-model="raw.uuid" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">密码 (password)</span>
            <input v-model="raw.password" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">congestion_control</span>
            <input v-model="raw.congestion_control" class="input input-bordered input-sm" placeholder="bbr" />
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
          <span class="label-text">启用 TLS</span>
        </label>
        <div v-if="raw.tls?.enabled" class="grid grid-cols-2 gap-3">
          <label class="form-control">
            <span class="label-text mb-1">server_name (SNI)</span>
            <input
              :value="raw.tls?.server_name"
              class="input input-bordered input-sm"
              @input="tls().server_name = ($event.target as HTMLInputElement).value"
            />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">ALPN (逗号分隔)</span>
            <input v-model="alpnText" class="input input-bordered input-sm" placeholder="h2, http/1.1" />
          </label>
          <label class="label cursor-pointer justify-start gap-2">
            <input
              type="checkbox"
              class="toggle toggle-sm"
              :checked="!!raw.tls?.insecure"
              @change="tls().insecure = ($event.target as HTMLInputElement).checked"
            />
            <span class="label-text">允许不安全 (insecure)</span>
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
        </div>

        <!-- manual country override -->
        <div class="divider my-0 text-xs opacity-60">国家</div>
        <label class="label cursor-pointer justify-start gap-2">
          <input type="checkbox" class="toggle toggle-sm" v-model="manualCountry" />
          <span class="label-text">手动指定国家 (country_source=manual)</span>
        </label>
        <select v-if="manualCountry" v-model="countryCode" class="select select-bordered select-sm">
          <option value="">（未指定）</option>
          <option v-for="c in countryOptions" :key="c.code" :value="c.code">
            {{ c.code }} — {{ c.name }}
          </option>
        </select>
      </div>

      <div class="modal-action">
        <button class="btn btn-ghost" @click="emit('close')" :disabled="busy">取消</button>
        <button class="btn btn-primary" @click="save" :disabled="busy || !!parseError">
          <span v-if="busy" class="loading loading-spinner loading-sm"></span>
          保存
        </button>
      </div>
    </div>
    <div class="modal-backdrop" @click="emit('close')"></div>
  </div>
</template>
