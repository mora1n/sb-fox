<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { post } from '../api/client'
import { useProfilesStore } from '../stores/profiles'
import { useTemplatesStore } from '../stores/templates'
import { useNodesStore } from '../stores/nodes'
import { useNodeGroupsStore } from '../stores/nodeGroups'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import type { KernelResult, Node, PreviewPayload, Profile, ProfileOptions, ProfilePayload } from '../api/types'
import TokenLinkField from '../components/TokenLinkField.vue'
import NodeMultiSelect from '../components/NodeMultiSelect.vue'
import JsonViewer from '../components/JsonViewer.vue'
import ValidationBadge from '../components/ValidationBadge.vue'
import { PlusIcon, PencilSquareIcon, TrashIcon, ArrowPathIcon } from '@heroicons/vue/24/outline'

const store = useProfilesStore()
const templates = useTemplatesStore()
const nodes = useNodesStore()
const nodeGroups = useNodeGroupsStore()
const settings = useSettingsStore()
const ui = useUiStore()
const i18n = useI18nStore()

const showForm = ref(false)
const editing = ref<Profile | null>(null)
const busy = ref(false)
const config = ref('')
const validation = ref<KernelResult | null>(null)
const allNodes = ref<Node[]>([])
const kernelHint = computed(() => i18n.t('请先安装 sing-box 内核或在设置中配置路径'))
const chainProxyNodeIds = computed<number[]>({
  get: () => form.value.options.chainProxyNodeIds ?? [],
  set: (ids) => {
    form.value.options.chainProxyNodeIds = ids
  },
})

const form = ref<{
  name: string
  template_id: number
  node_ids: number[]
  node_group_ids: number[]
  options: ProfileOptions
}>({
  name: '',
  template_id: 0,
  node_ids: [],
  node_group_ids: [],
  options: { autoCountryGroups: true, chainProxy: false, chainProxyNodeIds: [] },
})

onMounted(async () => {
  try {
    const [, , loadedNodes] = await Promise.all([
      store.fetchAll(),
      templates.fetchAll(),
      nodes.fetchUnfiltered(),
      nodeGroups.fetchAll(),
      settings.fetchKernelStatus(),
    ])
    allNodes.value = loadedNodes
  } catch (e) {
    ui.error(errMsg(e))
  }
})

watch(
  () => form.value.node_ids,
  () => {
    const selected = new Set(form.value.node_ids)
    form.value.options.chainProxyNodeIds = (form.value.options.chainProxyNodeIds ?? []).filter((id) => selected.has(id))
  },
)

function parseOptions(s: string): ProfileOptions {
  try {
    const o = JSON.parse(s || '{}')
    const legacyID = Number(o.chainProxyNodeId || 0)
    return {
      autoCountryGroups: !!o.autoCountryGroups,
      chainProxy: !!o.chainProxy,
      chainProxyNodeId: legacyID,
      chainProxyNodeIds: Array.isArray(o.chainProxyNodeIds)
        ? o.chainProxyNodeIds.map((id: unknown) => Number(id)).filter(Boolean)
        : legacyID
          ? [legacyID]
          : [],
    }
  } catch {
    return { autoCountryGroups: false, chainProxy: false, chainProxyNodeIds: [] }
  }
}

function templateName(id: number) {
  return templates.templates.find((t) => t.id === id)?.name || `#${id}`
}

function groupName(id: number) {
  return nodeGroups.groups.find((g) => g.id === id)?.name || `#${id}`
}

function optionBadges(p: Profile) {
  const opts = parseOptions(p.options)
  const badges: string[] = []
  if (opts.autoCountryGroups) badges.push(i18n.t('自动国家分组'))
  if (opts.chainProxy) badges.push(i18n.t('链式代理'))
  return badges
}

function openCreate() {
  editing.value = null
  form.value = {
    name: '',
    template_id: templates.templates[0]?.id || 0,
    node_ids: [],
    node_group_ids: [],
    options: { autoCountryGroups: true, chainProxy: false, chainProxyNodeIds: [] },
  }
  config.value = ''
  validation.value = null
  showForm.value = true
}

function openEdit(p: Profile) {
  editing.value = p
  form.value = {
    name: p.name,
    template_id: p.template_id,
    node_ids: [...p.node_ids],
    node_group_ids: [...(p.node_group_ids ?? [])],
    options: parseOptions(p.options),
  }
  config.value = ''
  validation.value = null
  showForm.value = true
}

function toggleGroup(id: number) {
  const set = new Set(form.value.node_group_ids)
  if (set.has(id)) set.delete(id)
  else set.add(id)
  form.value.node_group_ids = [...set]
}

function buildPayload(): ProfilePayload {
  const options = { ...form.value.options }
  options.chainProxyNodeIds = options.chainProxy ? [...(options.chainProxyNodeIds ?? [])] : []
  options.chainProxyNodeId = 0
  return {
    name: form.value.name.trim(),
    template_id: form.value.template_id,
    node_ids: form.value.node_ids,
    node_group_ids: form.value.node_group_ids,
    options,
  }
}

function validateForm() {
  if (!form.value.name.trim()) throw new Error('请填写名称')
  if (!form.value.template_id) throw new Error('请选择模板')
  if (form.value.options.chainProxy && !(form.value.options.chainProxyNodeIds?.length)) {
    throw new Error('请选择链式代理节点')
  }
  if (
    form.value.options.chainProxy &&
    (form.value.options.chainProxyNodeIds?.length ?? 0) >= form.value.node_ids.length &&
    !form.value.node_group_ids.length
  ) {
    throw new Error('链式代理需要至少一个上游节点')
  }
}

function chainProxyCandidates() {
  const selected = new Set(form.value.node_ids)
  return allNodes.value.filter((n) => selected.has(n.id))
}

async function submit() {
  busy.value = true
  try {
    validateForm()
    const payload = buildPayload()
    if (editing.value) {
      await store.update(editing.value.id, payload)
      ui.success('订阅已更新')
    } else {
      await store.create(payload)
      ui.success('订阅已创建')
    }
    showForm.value = false
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function generate() {
  busy.value = true
  validation.value = null
  try {
    validateForm()
    const payload: PreviewPayload = {
      template_id: form.value.template_id,
      node_ids: form.value.node_ids,
      node_group_ids: form.value.node_group_ids,
      options: buildPayload().options,
    }
    const r = await post<{ config: string }>('/generate/preview', payload)
    config.value = r.config
    ui.success('已生成配置')
  } catch (e) {
    ui.error(errMsg(e, '生成失败'))
  } finally {
    busy.value = false
  }
}

async function validateGenerated() {
  if (!config.value) return ui.info('请先生成配置')
  if (!settings.kernel?.available) return ui.info(kernelHint.value)
  busy.value = true
  try {
    validation.value = await post<KernelResult>('/generate/validate', { config: config.value })
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function formatGenerated() {
  if (!config.value) return ui.info('请先生成配置')
  if (!settings.kernel?.available) return ui.info(kernelHint.value)
  busy.value = true
  try {
    const r = await post<KernelResult>('/generate/format', { config: config.value })
    if (r.status === 'ok' && r.formatted) {
      config.value = r.formatted
      ui.success('已格式化')
    } else if (r.status === 'unavailable') {
      ui.info('内核不可用，无法格式化')
    } else {
      ui.error('格式化失败: ' + (r.messages || '配置无效'))
    }
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function rotate(p: Profile) {
  if (!confirm('轮换 token 后旧订阅链接立即失效，确认？')) return
  try {
    await store.rotateToken(p.id)
    ui.success('token 已轮换')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function remove(p: Profile) {
  if (!confirm(`删除订阅 "${p.name}"？`)) return
  try {
    await store.remove(p.id)
    ui.success('订阅已删除')
  } catch (e) {
    ui.error(errMsg(e))
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold">{{ i18n.t('订阅') }}</h1>
      <button class="btn btn-sm btn-primary" @click="openCreate"><PlusIcon class="h-4 w-4" /> {{ i18n.t('新建订阅') }}</button>
    </div>

    <div v-if="store.loading" class="flex justify-center py-10"><span class="loading loading-spinner loading-lg"></span></div>
    <div v-else-if="!store.profiles.length" class="text-center py-10 opacity-60">{{ i18n.t('暂无订阅。') }}</div>
    <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div v-for="p in store.profiles" :key="p.id" class="card bg-base-100 shadow-sm border border-base-300">
        <div class="card-body p-4 gap-3">
          <div class="flex items-start justify-between">
            <div>
              <h2 class="card-title text-base">{{ p.name }}</h2>
              <div class="text-xs opacity-70 mt-1 flex flex-wrap gap-2">
                <span>{{ i18n.t('模板:') }} {{ templateName(p.template_id) }}</span>
                <span>{{ p.node_ids.length }} {{ i18n.t('个节点') }}</span>
                <span v-if="p.node_group_ids?.length">{{ p.node_group_ids.length }} {{ i18n.t('个组合') }}</span>
              </div>
            </div>
            <div class="flex gap-1">
              <button class="btn btn-xs btn-ghost" @click="openEdit(p)"><PencilSquareIcon class="h-4 w-4" /></button>
              <button class="btn btn-xs btn-ghost text-error" @click="remove(p)"><TrashIcon class="h-4 w-4" /></button>
            </div>
          </div>
          <div class="flex flex-wrap gap-1">
            <span v-for="badge in optionBadges(p)" :key="badge" class="badge badge-sm badge-neutral">{{ badge }}</span>
            <span v-for="gid in p.node_group_ids ?? []" :key="gid" class="badge badge-sm badge-ghost">{{ groupName(gid) }}</span>
          </div>
          <TokenLinkField :token="p.token" />
          <div class="flex items-center justify-between">
            <span class="text-xs opacity-60">{{ i18n.t('公开订阅链接') }}</span>
            <button class="btn btn-xs btn-ghost" @click="rotate(p)"><ArrowPathIcon class="h-3 w-3" /> {{ i18n.t('轮换 token') }}</button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showForm" class="modal modal-open">
      <div class="modal-box max-w-6xl">
        <h3 class="font-bold text-lg mb-3">{{ editing ? i18n.t('编辑订阅') : i18n.t('新建订阅') }}</h3>
        <div class="grid grid-cols-1 xl:grid-cols-[minmax(0,420px)_minmax(0,1fr)] gap-4">
          <div class="flex flex-col gap-3">
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('名称') }}</span>
              <input v-model="form.name" class="input input-bordered input-sm" />
            </label>
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('模板') }}</span>
              <select v-model.number="form.template_id" class="select select-bordered select-sm">
                <option :value="0" disabled>{{ i18n.t('选择模板') }}</option>
                <option v-for="t in templates.templates" :key="t.id" :value="t.id">{{ t.name }} · {{ t.kind }}</option>
              </select>
            </label>
            <div class="flex flex-wrap gap-4">
              <label class="label cursor-pointer justify-start gap-2">
                <input type="checkbox" class="toggle toggle-sm" v-model="form.options.autoCountryGroups" />
                <span class="label-text">{{ i18n.t('自动国家分组') }}</span>
              </label>
              <label class="label cursor-pointer justify-start gap-2">
                <input type="checkbox" class="toggle toggle-sm" v-model="form.options.chainProxy" />
                <span class="label-text">{{ i18n.t('链式代理') }}</span>
              </label>
            </div>
            <label v-if="form.options.chainProxy" class="form-control">
              <span class="label-text mb-1">{{ i18n.t('链式代理节点') }}</span>
              <NodeMultiSelect :nodes="chainProxyCandidates()" v-model="chainProxyNodeIds" />
            </label>
            <div class="form-control">
              <span class="label-text mb-1">{{ i18n.t('单节点') }}</span>
              <NodeMultiSelect :nodes="allNodes" v-model="form.node_ids" />
            </div>
            <div class="form-control">
              <span class="label-text mb-1">{{ i18n.t('组合节点') }}</span>
              <div class="border border-base-300 rounded-box max-h-44 overflow-y-auto divide-y divide-base-200">
                <label
                  v-for="g in nodeGroups.groups"
                  :key="g.id"
                  class="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-base-200"
                >
                  <input
                    type="checkbox"
                    class="checkbox checkbox-sm"
                    :checked="form.node_group_ids.includes(g.id)"
                    @change="toggleGroup(g.id)"
                  />
                  <span class="truncate flex-1 text-sm">{{ g.name }}</span>
                  <span class="badge badge-ghost badge-sm">{{ g.node_ids.length }}</span>
                </label>
                <div v-if="!nodeGroups.groups.length" class="px-3 py-4 text-sm opacity-60 text-center">{{ i18n.t('暂无组合节点。') }}</div>
              </div>
            </div>
          </div>

          <div class="flex flex-col gap-3 min-w-0">
            <div class="flex gap-2 flex-wrap">
              <button class="btn btn-primary btn-sm w-24 justify-center" @click="generate" :disabled="busy">
                <span v-if="busy" class="loading loading-spinner loading-sm"></span>
                <span>{{ i18n.t('生成') }}</span>
              </button>
              <button class="btn btn-sm" @click="validateGenerated" :disabled="busy" :class="{ 'opacity-50 cursor-not-allowed': !config || !settings.kernel?.available }" :title="settings.kernel?.available ? '' : kernelHint">{{ i18n.t('校验') }}</button>
              <button class="btn btn-sm" @click="formatGenerated" :disabled="busy" :class="{ 'opacity-50 cursor-not-allowed': !config || !settings.kernel?.available }" :title="settings.kernel?.available ? '' : kernelHint">{{ i18n.t('格式化') }}</button>
            </div>
            <ValidationBadge :status="validation?.status ?? null" :messages="validation?.messages" />
            <div class="border border-base-300 rounded-box bg-base-100 min-h-80">
              <JsonViewer v-if="config" :content="config" max-height="60vh" />
              <div v-else class="opacity-60 text-sm py-16 text-center">{{ i18n.t('点击「生成」查看配置。') }}</div>
            </div>
          </div>
        </div>
        <div class="modal-action">
          <button class="btn btn-ghost" @click="showForm = false" :disabled="busy">{{ i18n.t('取消') }}</button>
          <button class="btn btn-primary" @click="submit" :disabled="busy">
            <span v-if="busy" class="loading loading-spinner loading-sm"></span> {{ i18n.t('保存') }}
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="showForm = false"></div>
    </div>
  </div>
</template>
