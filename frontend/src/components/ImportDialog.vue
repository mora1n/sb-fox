<script setup lang="ts">
import { ref, watch } from 'vue'
import { useNodesStore } from '../stores/nodes'
import { useNodeGroupsStore } from '../stores/nodeGroups'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import type { ImportPreviewResult, ImportResult } from '../api/types'
import { nodeSourceLabel } from '../utils/nodeSource'
import CountryFlag from './CountryFlag.vue'

const emit = defineEmits<{ close: []; imported: [] }>()
const nodesStore = useNodesStore()
const nodeGroupsStore = useNodeGroupsStore()
const ui = useUiStore()
const i18n = useI18nStore()

type Tab = 'links' | 'subscription' | 'config'
const tab = ref<Tab>('links')
const busy = ref(false)

const links = ref('')
const subName = ref('')
const subUrl = ref('')
const configText = ref('')
const preview = ref<ImportPreviewResult | null>(null)
const createGroup = ref(false)
const groupName = ref('')
const groupNameTouched = ref(false)

watch(tab, () => {
  resetPreviewState()
})

function setTab(next: Tab) {
  if (busy.value || preview.value) return
  tab.value = next
}

function clearPreview() {
  resetPreviewState()
}

function resetPreviewState() {
  preview.value = null
  createGroup.value = false
  groupName.value = ''
  groupNameTouched.value = false
}

function defaultGroupName() {
  if (tab.value === 'subscription' && subName.value.trim()) return subName.value.trim()
  return `${i18n.t('导入节点')} ${timestampLabel()}`
}

function timestampLabel() {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function groupDescription(count: number) {
  return `${i18n.t('导入节点')} · ${count} ${i18n.t('个节点')}`
}

function syncGroupName() {
  if (preview.value && !groupNameTouched.value) groupName.value = defaultGroupName()
}

function onGroupNameInput(value: string) {
  groupNameTouched.value = true
  groupName.value = value
}

watch(preview, syncGroupName)
watch([subName, tab], () => {
  if (!groupNameTouched.value) groupName.value = defaultGroupName()
})

async function runImport(): Promise<ImportResult> {
  if (tab.value === 'links') {
    if (!links.value.trim()) throw new Error('请粘贴分享链接')
    return nodesStore.importLinks(links.value)
  }
  if (tab.value === 'subscription') {
    if (!subUrl.value.trim()) throw new Error('请填写订阅 URL')
    return nodesStore.importSubscription(subName.value, subUrl.value)
  }
  if (!configText.value.trim()) throw new Error('请粘贴 config JSON')
  return nodesStore.importConfig(configText.value)
}

async function previewImport() {
  busy.value = true
  try {
    if (tab.value === 'links') {
      if (!links.value.trim()) throw new Error('请粘贴分享链接')
      preview.value = await nodesStore.previewImportLinks(links.value)
    } else if (tab.value === 'subscription') {
      if (!subUrl.value.trim()) throw new Error('请填写订阅 URL')
      preview.value = await nodesStore.previewImportSubscription(subUrl.value)
    } else {
      if (!configText.value.trim()) throw new Error('请粘贴 config JSON')
      preview.value = await nodesStore.previewImportConfig(configText.value)
    }
  } catch (e) {
    ui.error(errMsg(e, i18n.t('解析预览失败')))
  } finally {
    busy.value = false
  }
}

async function confirmImport() {
  busy.value = true
  try {
    if (createGroup.value && !groupName.value.trim()) throw new Error('请填写组合名称')
    const result = await runImport()
    const count = result.imported
    const deduped = result.deduped ?? 0
    const warnings = result.warnings ?? []
    let createdGroupName = ''
    let groupError = ''
    if (createGroup.value && result.nodes.length) {
      try {
        const group = await nodeGroupsStore.create({
          name: groupName.value.trim(),
          description: groupDescription(result.nodes.length),
          node_ids: result.nodes.map((node) => node.id),
        })
        createdGroupName = group.name
      } catch (e) {
        groupError = errMsg(e)
      }
    }
    if (createdGroupName) ui.success(`成功导入 ${count} 个节点，并创建组合节点「${createdGroupName}」`)
    else ui.success(`成功导入 ${count} 个节点`)
    if (groupError) ui.error(`${i18n.t('节点已导入，但组合节点创建失败')}：${groupError}`)
    if (deduped) ui.info(`已跳过 ${deduped} 个重复节点`)
    warnings.slice(0, 3).forEach((warning) => ui.info(warning))
    emit('imported')
    emit('close')
  } catch (e) {
    ui.error(errMsg(e, '导入失败'))
  } finally {
    busy.value = false
  }
}

function previewCountLabel() {
  if (!preview.value) return ''
  return `${i18n.t('预计导入')} ${preview.value.importable} / ${preview.value.parsed}`
}

function successfulFetchCount() {
  return preview.value?.fetches?.filter((item) => item.ok).length ?? 0
}

function failedFetchCount() {
  return preview.value?.fetches?.filter((item) => !item.ok).length ?? 0
}
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-2xl">
      <h3 class="font-bold text-lg mb-3">{{ i18n.t('导入节点') }}</h3>
      <div role="tablist" class="tabs tabs-boxed mb-4">
        <a role="tab" class="tab" :class="{ 'tab-active': tab === 'links', 'tab-disabled': preview }" @click="setTab('links')">{{ i18n.t('分享链接') }}</a>
        <a role="tab" class="tab" :class="{ 'tab-active': tab === 'subscription', 'tab-disabled': preview }" @click="setTab('subscription')">{{ i18n.t('订阅 URL') }}</a>
        <a role="tab" class="tab" :class="{ 'tab-active': tab === 'config', 'tab-disabled': preview }" @click="setTab('config')">Config JSON</a>
      </div>

      <div v-show="tab === 'links'">
        <p class="text-sm opacity-70 mb-2">{{ i18n.t('每行一个 ss/vmess/vless/trojan/hysteria2/tuic/naive 链接，或粘贴 base64、SIP008、Clash/Mihomo 订阅内容。') }}</p>
        <textarea v-model="links" class="textarea textarea-bordered w-full h-48 mono text-xs" placeholder="vmess://...&#10;ss://..." :disabled="!!preview || busy"></textarea>
      </div>

      <div v-show="tab === 'subscription'" class="flex flex-col gap-3">
        <label class="form-control">
          <span class="label-text mb-1">{{ i18n.t('名称') }}</span>
          <input v-model="subName" class="input input-bordered" :placeholder="i18n.t('我的订阅')" :disabled="!!preview || busy" />
        </label>
        <label class="form-control">
          <span class="label-text mb-1">{{ i18n.t('订阅 URL') }}</span>
          <textarea
            v-model="subUrl"
            class="textarea textarea-bordered w-full h-28 mono text-xs"
            placeholder="https://example.com/sub&#10;https://example.com/sub2#noCache"
            :disabled="!!preview || busy"
          ></textarea>
        </label>
        <p class="text-xs opacity-60">{{ i18n.t('每行一个订阅 URL，可使用 #noCache、#insecure、#ua=...、#headers=...、#cacheTtl=秒 参数。') }}</p>
        <p class="text-xs opacity-60">{{ i18n.t('服务端抓取，默认拒绝私网地址。') }}</p>
      </div>

      <div v-show="tab === 'config'">
        <p class="text-sm opacity-70 mb-2">{{ i18n.t('粘贴完整 config.json，将从 outbounds 中提取代理节点，跳过 selector/direct 等分组。') }}</p>
        <textarea v-model="configText" class="textarea textarea-bordered w-full h-48 mono text-xs" placeholder='{ "outbounds": [ ... ] }' :disabled="!!preview || busy"></textarea>
      </div>

      <div v-if="preview" class="mt-4 rounded-box border border-base-300 bg-base-200/40 p-3 flex flex-col gap-3">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div class="font-semibold">{{ i18n.t('导入预览') }}</div>
          <div class="flex flex-wrap gap-2">
            <span class="badge badge-neutral">{{ previewCountLabel() }}</span>
            <span class="badge badge-outline">{{ i18n.t('重复节点') }} {{ preview.deduped }}</span>
          </div>
        </div>
        <div v-if="preview.warnings?.length" class="text-xs text-warning flex flex-col gap-1">
          <div v-for="warning in preview.warnings.slice(0, 3)" :key="warning" class="truncate" :title="warning">{{ warning }}</div>
        </div>
        <div v-if="preview.importable > 0" class="rounded-box border border-base-300 bg-base-100 p-3 flex flex-col gap-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="font-semibold text-sm">{{ i18n.t('新建组合') }}</div>
              <div class="text-xs opacity-60">{{ i18n.t('使用本次新增节点创建组合节点') }}</div>
            </div>
            <input v-model="createGroup" type="checkbox" class="toggle toggle-sm" :disabled="busy" />
          </div>
          <label v-if="createGroup" class="form-control">
            <span class="label-text mb-1">{{ i18n.t('组合名称') }}</span>
            <input
              :value="groupName"
              class="input input-bordered input-sm"
              :placeholder="defaultGroupName()"
              :disabled="busy"
              @input="onGroupNameInput(($event.target as HTMLInputElement).value)"
            />
          </label>
        </div>
        <div v-if="preview.fetches?.length" class="rounded-box border border-base-300 bg-base-100">
          <div class="flex items-center justify-between gap-2 px-3 py-2 border-b border-base-200 text-xs">
            <span class="font-semibold">{{ i18n.t('拉取结果') }}</span>
            <span class="flex items-center gap-2">
              <span class="badge badge-sm badge-success badge-outline">{{ i18n.t('成功') }} {{ successfulFetchCount() }}</span>
              <span class="badge badge-sm badge-error badge-outline">{{ i18n.t('失败') }} {{ failedFetchCount() }}</span>
            </span>
          </div>
          <div class="max-h-24 overflow-y-auto">
            <div
              v-for="item in preview.fetches"
              :key="item.url"
              class="flex items-center gap-2 px-3 py-1.5 border-b border-base-200 last:border-b-0 text-xs"
            >
              <span class="badge badge-xs" :class="item.ok ? 'badge-success' : 'badge-error'">{{ item.ok ? i18n.t('成功') : i18n.t('失败') }}</span>
              <span class="min-w-0 flex-1 truncate" :title="item.error ? `${item.url}: ${item.error}` : item.url">{{ item.url }}</span>
              <span v-if="item.nodes" class="badge badge-outline badge-xs">{{ item.nodes }} {{ i18n.t('个节点') }}</span>
              <span v-if="item.from_cache" class="badge badge-outline badge-xs">{{ i18n.t('缓存') }}</span>
            </div>
          </div>
        </div>
        <div v-if="preview.nodes.length" class="max-h-48 overflow-y-auto rounded-box border border-base-300 bg-base-100">
          <div
            v-for="node in preview.nodes.slice(0, 20)"
            :key="node.tag + node.server + node.server_port"
            class="flex items-center gap-2 px-3 py-2 border-b border-base-200 last:border-b-0 text-sm"
          >
            <span class="font-medium min-w-0 flex-1 truncate" :title="node.tag">{{ node.tag }}</span>
            <span class="badge badge-outline badge-sm">{{ node.type }}</span>
            <CountryFlag v-if="node.country_code" :code="node.country_code" />
            <span class="badge badge-sm badge-neutral">{{ i18n.t(nodeSourceLabel(node.source)) }}</span>
          </div>
          <div v-if="preview.nodes.length > 20" class="px-3 py-2 text-xs opacity-60">
            +{{ preview.nodes.length - 20 }} {{ i18n.t('个节点') }}
          </div>
        </div>
        <div v-else class="text-sm opacity-70">
          {{ i18n.t('没有可新增节点。') }}
        </div>
      </div>

      <div class="modal-action">
        <button class="btn" @click="emit('close')" :disabled="busy">{{ i18n.t('取消') }}</button>
        <button v-if="preview" class="btn" @click="clearPreview" :disabled="busy">{{ i18n.t('返回修改') }}</button>
        <button v-if="!preview" class="btn btn-primary" @click="previewImport" :disabled="busy">
          <span v-if="busy" class="loading loading-spinner loading-sm"></span>
          {{ i18n.t('解析预览') }}
        </button>
        <button v-else class="btn btn-primary" @click="confirmImport" :disabled="busy || preview.importable === 0">
          <span v-if="busy" class="loading loading-spinner loading-sm"></span>
          {{ i18n.t('确认导入') }}
        </button>
      </div>
    </div>
    <div class="modal-backdrop" @click="emit('close')"></div>
  </div>
</template>
