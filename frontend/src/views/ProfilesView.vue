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
import type {
  KernelResult,
  Node,
  NodeSelection,
  PreviewPayload,
  Profile,
  ProfileOptions,
  ProfilePayload,
  TemplateStructure,
} from '../api/types'
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
const structure = ref<TemplateStructure | null>(null)
const activeGroup = ref('')
const kernelHint = computed(() => i18n.t('请先安装 sing-box 内核或在设置中配置路径'))
const tokenHost = computed(() => settings.subscriptionHostPrefix || '')

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
  options: { autoCountryGroups: true, chainProxy: false, groupSelections: {} },
})

const activeStructureGroup = computed(() => structure.value?.groups.find((g) => g.tag === activeGroup.value) ?? null)

const activeNodeIds = computed<number[]>({
  get: () => selectionFor(activeGroup.value).nodeIds,
  set: (ids) => {
    selectionFor(activeGroup.value).nodeIds = ids
  },
})

const chainNodeIds = computed<number[]>({
  get: () => chainSelection().nodeIds,
  set: (ids) => {
    chainSelection().nodeIds = ids
  },
})

onMounted(async () => {
  try {
    const [, , , loadedNodes] = await Promise.all([
      store.fetchAll(),
      store.fetchSubscriptionToken(),
      templates.fetchAll(),
      nodes.fetchUnfiltered(),
      nodeGroups.fetchAll(),
      settings.fetchKernelStatus(),
      settings.fetchAppInfo(),
    ])
    allNodes.value = loadedNodes
  } catch (e) {
    ui.error(errMsg(e))
  }
})

watch(
  () => form.value.template_id,
  async (id) => {
    if (!showForm.value || !id) return
    await loadStructure(id)
  },
)

function emptySelection(): NodeSelection {
  return { nodeIds: [], nodeGroupIds: [], outboundRefs: [], skipCountryGroups: false }
}

function parseOptions(s: string): ProfileOptions {
  try {
    const raw = JSON.parse(s || '{}')
    const groups: Record<string, NodeSelection> = {}
    for (const [tag, sel] of Object.entries(raw.groupSelections || {})) {
      const item = sel as Partial<NodeSelection>
      groups[tag] = {
        nodeIds: Array.isArray(item.nodeIds) ? item.nodeIds.map(Number).filter(Boolean) : [],
        nodeGroupIds: Array.isArray(item.nodeGroupIds) ? item.nodeGroupIds.map(Number).filter(Boolean) : [],
        outboundRefs: Array.isArray(item.outboundRefs)
          ? item.outboundRefs.map(String).map((v) => v.trim()).filter(Boolean)
          : [],
        skipCountryGroups: !!item.skipCountryGroups,
      }
    }
    const chain = raw.chainProxySelection as Partial<NodeSelection> | undefined
    const legacyChainIDs = Array.isArray(raw.chainProxyNodeIds)
      ? raw.chainProxyNodeIds.map(Number).filter(Boolean)
      : raw.chainProxyNodeId
        ? [Number(raw.chainProxyNodeId)].filter(Boolean)
        : []
    return {
      autoCountryGroups: raw.autoCountryGroups !== false,
      chainProxy: !!raw.chainProxy,
      groupSelections: groups,
      chainProxySelection: chain
        ? {
            nodeIds: Array.isArray(chain.nodeIds) ? chain.nodeIds.map(Number).filter(Boolean) : [],
            nodeGroupIds: Array.isArray(chain.nodeGroupIds) ? chain.nodeGroupIds.map(Number).filter(Boolean) : [],
            outboundRefs: [],
            skipCountryGroups: false,
          }
        : legacyChainIDs.length
          ? {
              nodeIds: legacyChainIDs,
              nodeGroupIds: [],
              outboundRefs: [],
              skipCountryGroups: false,
            }
        : undefined,
    }
  } catch {
    return { autoCountryGroups: true, chainProxy: false, groupSelections: {} }
  }
}

function templateName(id: number) {
  return templates.templates.find((t) => t.id === id)?.name || `#${id}`
}

function optionBadges(p: Profile) {
  const opts = parseOptions(p.options)
  const badges: string[] = []
  if (Object.keys(opts.groupSelections ?? {}).length) badges.push(i18n.t('出口选择'))
  if (opts.chainProxy) badges.push(i18n.t('链式代理'))
  return badges
}

async function loadStructure(templateID: number) {
  structure.value = await templates.structure(templateID)
  if (!structure.value.groups.length) {
    activeGroup.value = ''
    return
  }
  if (!activeGroup.value || !structure.value.groups.some((g) => g.tag === activeGroup.value)) {
    activeGroup.value = structure.value.groups[0].tag
  }
  ensureGroupSelections()
}

function ensureGroupSelections() {
  if (!structure.value) return
  if (!form.value.options.groupSelections) form.value.options.groupSelections = {}
  for (const g of structure.value.groups) selectionFor(g.tag)
}

function selectionFor(tag: string): NodeSelection {
  if (!form.value.options.groupSelections) form.value.options.groupSelections = {}
  if (!form.value.options.groupSelections[tag]) form.value.options.groupSelections[tag] = emptySelection()
  return form.value.options.groupSelections[tag]
}

function chainSelection(): NodeSelection {
  if (!form.value.options.chainProxySelection) form.value.options.chainProxySelection = emptySelection()
  return form.value.options.chainProxySelection
}

function openCreate() {
  editing.value = null
  form.value = {
    name: '',
    template_id: templates.templates[0]?.id || 0,
    node_ids: [],
    node_group_ids: [],
    options: { autoCountryGroups: true, chainProxy: false, groupSelections: {} },
  }
  config.value = ''
  validation.value = null
  structure.value = null
  activeGroup.value = ''
  showForm.value = true
  if (form.value.template_id) loadStructure(form.value.template_id).catch((e) => ui.error(errMsg(e)))
}

function openEdit(p: Profile) {
  const options = parseOptions(p.options)
  const legacyNodeIDs = [...p.node_ids]
  const legacyNodeGroupIDs = [...(p.node_group_ids ?? [])]
  const shouldHydrateLegacySelection =
    !selectionMapHasAny(options.groupSelections) && (legacyNodeIDs.length > 0 || legacyNodeGroupIDs.length > 0)
  editing.value = p
  form.value = {
    name: p.name,
    template_id: p.template_id,
    node_ids: legacyNodeIDs,
    node_group_ids: legacyNodeGroupIDs,
    options,
  }
  config.value = ''
  validation.value = null
  structure.value = null
  activeGroup.value = ''
  showForm.value = true
  loadStructure(p.template_id)
    .then(() => {
      if (shouldHydrateLegacySelection) hydrateLegacySelection(legacyNodeIDs, legacyNodeGroupIDs)
    })
    .catch((e) => ui.error(errMsg(e)))
}

function hydrateLegacySelection(nodeIDs: number[], nodeGroupIDs: number[]) {
  const finalTag = structure.value?.final || structure.value?.groups[0]?.tag || ''
  if (!finalTag) return
  const sel = selectionFor(finalTag)
  if (hasSelection(sel)) return
  sel.nodeIds = [...nodeIDs]
  sel.nodeGroupIds = [...nodeGroupIDs]
}

function cleanSelections(options: ProfileOptions): ProfileOptions {
  const groupSelections: Record<string, NodeSelection> = {}
  if (structure.value) {
    for (const g of structure.value.groups) {
      const sel = selectionFor(g.tag)
      groupSelections[g.tag] = {
        nodeIds: [...sel.nodeIds],
        nodeGroupIds: [...sel.nodeGroupIds],
        outboundRefs: [...sel.outboundRefs],
        skipCountryGroups: !!sel.skipCountryGroups,
      }
    }
  }
  const next: ProfileOptions = {
    autoCountryGroups: !!options.autoCountryGroups,
    chainProxy: !!options.chainProxy,
    groupSelections,
  }
  if (next.chainProxy) {
    const chain = chainSelection()
    next.chainProxySelection = {
      nodeIds: [...chain.nodeIds],
      nodeGroupIds: [...chain.nodeGroupIds],
      outboundRefs: [],
      skipCountryGroups: false,
    }
  }
  return next
}

function buildPayload(): ProfilePayload {
  return {
    name: form.value.name.trim(),
    template_id: form.value.template_id,
    node_ids: [],
    node_group_ids: [],
    options: cleanSelections(form.value.options),
  }
}

function hasSelection(sel: NodeSelection) {
  return sel.nodeIds.length > 0 || sel.nodeGroupIds.length > 0 || sel.outboundRefs.length > 0
}

function selectionMapHasAny(selections: Record<string, NodeSelection> | undefined) {
  return Object.values(selections ?? {}).some(hasSelection)
}

function validateForm() {
  if (!form.value.name.trim()) throw new Error('请填写名称')
  if (!form.value.template_id) throw new Error('请选择模板')
  if (!structure.value || !structure.value.groups.length) throw new Error('模板没有可用出口分组')
  const finalTag = structure.value.final || structure.value.groups[0].tag
  const finalSelection = selectionFor(finalTag)
  if (!hasSelection(finalSelection) && !form.value.options.chainProxy) {
    throw new Error(`最终出口 "${finalTag}" 至少需要选择一个节点、组合节点或引用出口`)
  }
  if (form.value.options.chainProxy && !hasSelection(chainSelection())) {
    throw new Error('请选择链式代理节点')
  }
}

function groupSelectionCount(tag: string) {
  const sel = selectionFor(tag)
  return sel.nodeIds.length + sel.nodeGroupIds.length + sel.outboundRefs.length
}

function outboundRefOptions(tag: string) {
  const group = structure.value?.groups.find((g) => g.tag === tag)
  if (!group) return []
  const seen = new Set<string>()
  return group.outbounds.filter((item) => {
    const clean = item.trim()
    if (!clean || clean === tag || seen.has(clean)) return false
    seen.add(clean)
    return true
  })
}

function toggleOutboundRef(sel: NodeSelection, tag: string) {
  const set = new Set(sel.outboundRefs)
  if (set.has(tag)) set.delete(tag)
  else set.add(tag)
  sel.outboundRefs = [...set]
}

function toggleGroupForSelection(sel: NodeSelection, id: number) {
  const set = new Set(sel.nodeGroupIds)
  if (set.has(id)) set.delete(id)
  else set.add(id)
  sel.nodeGroupIds = [...set]
}

function selectAllGroups(sel: NodeSelection) {
  sel.nodeGroupIds = nodeGroups.groups.map((g) => g.id)
}

function clearGroups(sel: NodeSelection) {
  sel.nodeGroupIds = []
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
      node_ids: [],
      node_group_ids: [],
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

async function rotateSharedToken() {
  if (!confirm('轮换共享 token 后所有订阅链接都会变化，确认？')) return
  try {
    await store.rotateSubscriptionToken()
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
      <button class="btn btn-sm btn-primary" @click="openCreate">
        <PlusIcon class="h-4 w-4" /> {{ i18n.t('新建订阅') }}
      </button>
    </div>

    <div class="card bg-base-100 shadow-sm border border-base-300">
      <div class="card-body p-4 gap-3">
        <div class="flex items-center justify-between gap-3 flex-wrap">
          <div>
            <h2 class="card-title text-base">{{ i18n.t('共享 token') }}</h2>
            <p class="text-xs opacity-60">{{ i18n.t('所有订阅链接共享同一 token，并按订阅名称区分。') }}</p>
          </div>
          <button class="btn btn-sm" @click="rotateSharedToken">
            <ArrowPathIcon class="h-4 w-4" /> {{ i18n.t('轮换共享 token') }}
          </button>
        </div>
        <div class="mono text-xs bg-base-200 rounded-box px-3 py-2 break-all">{{ store.subscriptionToken || '...' }}</div>
      </div>
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
              </div>
            </div>
            <div class="flex gap-1">
              <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑订阅')" @click="openEdit(p)"><PencilSquareIcon class="h-4 w-4" /></button>
              <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click="remove(p)"><TrashIcon class="h-4 w-4" /></button>
            </div>
          </div>
          <div class="flex flex-wrap gap-1">
            <span v-for="badge in optionBadges(p)" :key="badge" class="badge badge-sm badge-neutral">{{ badge }}</span>
          </div>
          <TokenLinkField
            v-if="store.subscriptionToken"
            :token="store.subscriptionToken"
            :profile-name="p.name"
            :host-prefix="tokenHost"
          />
          <span class="text-xs opacity-60">{{ i18n.t('公开订阅链接') }}</span>
        </div>
      </div>
    </div>

    <div v-if="showForm" class="modal modal-open">
      <div class="modal-box max-w-7xl max-h-[88vh] overflow-y-auto">
        <h3 class="font-bold text-lg mb-3">{{ editing ? i18n.t('编辑订阅') : i18n.t('新建订阅') }}</h3>
        <div class="grid grid-cols-1 xl:grid-cols-[280px_minmax(0,1fr)_minmax(0,1fr)] gap-4">
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
            <div class="form-control">
              <span class="label-text mb-1">{{ i18n.t('分组管理') }}</span>
              <div class="border border-base-300 rounded-box divide-y divide-base-200 overflow-hidden">
                <button
                  v-for="g in structure?.groups ?? []"
                  :key="g.tag"
                  type="button"
                  class="w-full flex items-center justify-between gap-2 px-3 py-2 text-left hover:bg-base-200"
                  :class="{ 'bg-base-200': activeGroup === g.tag }"
                  @click="activeGroup = g.tag"
                >
                  <span class="truncate text-sm">{{ g.tag }}</span>
                  <span class="badge badge-sm">{{ groupSelectionCount(g.tag) }}</span>
                </button>
                <div v-if="!structure?.groups.length" class="px-3 py-4 text-sm opacity-60 text-center">
                  {{ i18n.t('未检测到 selector/urltest 分组。') }}
                </div>
              </div>
            </div>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="form.options.autoCountryGroups" />
              <span class="label-text">{{ i18n.t('自动国家分组') }}</span>
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="form.options.chainProxy" />
              <span class="label-text">{{ i18n.t('链式代理') }}</span>
            </label>
          </div>

          <div class="flex flex-col gap-3 min-w-0">
            <div class="rounded-box border border-base-300 p-3 bg-base-100">
              <div class="flex items-start justify-between gap-2 flex-wrap mb-3">
                <h4 class="font-semibold text-sm">{{ i18n.t('当前出口分组') }} · {{ activeGroup || '-' }}</h4>
                <label v-if="activeGroup" class="label cursor-pointer justify-start gap-2 p-0">
                  <input
                    type="checkbox"
                    class="toggle toggle-xs"
                    v-model="selectionFor(activeGroup).skipCountryGroups"
                    :disabled="!form.options.autoCountryGroups"
                  />
                  <span class="label-text text-xs">{{ i18n.t('跳过国家分组') }}</span>
                </label>
              </div>
              <div v-if="activeStructureGroup" class="mb-3">
                <div class="label-text mb-1">{{ i18n.t('引用出口') }}</div>
                <div class="border border-base-300 rounded-box max-h-32 overflow-y-auto divide-y divide-base-200">
                  <label
                    v-for="tag in outboundRefOptions(activeGroup)"
                    :key="tag"
                    class="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-base-200"
                  >
                    <input
                      type="checkbox"
                      class="checkbox checkbox-sm"
                      :checked="selectionFor(activeGroup).outboundRefs.includes(tag)"
                      @change="toggleOutboundRef(selectionFor(activeGroup), tag)"
                    />
                    <span class="truncate flex-1 text-sm" :title="tag">{{ tag }}</span>
                  </label>
                  <div v-if="!outboundRefOptions(activeGroup).length" class="px-3 py-4 text-sm opacity-60 text-center">
                    {{ i18n.t('无可选出口') }}
                  </div>
                </div>
              </div>
              <NodeMultiSelect :nodes="allNodes" v-model="activeNodeIds" />
              <div class="mt-3">
                <div class="flex items-center justify-between gap-2 flex-wrap mb-2">
                  <span class="label-text">{{ i18n.t('组合节点') }}</span>
                  <span class="flex gap-1 flex-wrap">
                    <button class="btn btn-xs min-h-7 h-7" type="button" @click="selectAllGroups(selectionFor(activeGroup))">{{ i18n.t('全选') }}</button>
                    <button class="btn btn-xs min-h-7 h-7" type="button" @click="clearGroups(selectionFor(activeGroup))">{{ i18n.t('全不选') }}</button>
                  </span>
                </div>
                <div class="border border-base-300 rounded-box max-h-36 overflow-y-auto divide-y divide-base-200">
                  <label v-for="g in nodeGroups.groups" :key="g.id" class="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-base-200">
                    <input
                      type="checkbox"
                      class="checkbox checkbox-sm"
                      :checked="selectionFor(activeGroup).nodeGroupIds.includes(g.id)"
                      @change="toggleGroupForSelection(selectionFor(activeGroup), g.id)"
                    />
                    <span class="truncate flex-1 text-sm">{{ g.name }}</span>
                    <span class="badge badge-ghost badge-sm">{{ g.node_ids.length }}</span>
                  </label>
                  <div v-if="!nodeGroups.groups.length" class="px-3 py-4 text-sm opacity-60 text-center">{{ i18n.t('暂无组合节点。') }}</div>
                </div>
              </div>
            </div>

            <div v-if="form.options.chainProxy" class="rounded-box border border-base-300 p-3 bg-base-100">
              <h4 class="font-semibold text-sm mb-2">{{ i18n.t('链式代理节点') }}</h4>
              <NodeMultiSelect :nodes="allNodes" v-model="chainNodeIds" />
              <div class="mt-3">
                <div class="flex items-center justify-between gap-2 flex-wrap mb-2">
                  <span class="label-text">{{ i18n.t('组合节点') }}</span>
                  <span class="flex gap-1 flex-wrap">
                    <button class="btn btn-xs min-h-7 h-7" type="button" @click="selectAllGroups(chainSelection())">{{ i18n.t('全选') }}</button>
                    <button class="btn btn-xs min-h-7 h-7" type="button" @click="clearGroups(chainSelection())">{{ i18n.t('全不选') }}</button>
                  </span>
                </div>
                <div class="border border-base-300 rounded-box max-h-32 overflow-y-auto divide-y divide-base-200">
                  <label v-for="g in nodeGroups.groups" :key="g.id" class="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-base-200">
                    <input
                      type="checkbox"
                      class="checkbox checkbox-sm"
                      :checked="chainSelection().nodeGroupIds.includes(g.id)"
                      @change="toggleGroupForSelection(chainSelection(), g.id)"
                    />
                    <span class="truncate flex-1 text-sm">{{ g.name }}</span>
                    <span class="badge badge-ghost badge-sm">{{ g.node_ids.length }}</span>
                  </label>
                  <div v-if="!nodeGroups.groups.length" class="px-3 py-4 text-sm opacity-60 text-center">{{ i18n.t('暂无组合节点。') }}</div>
                </div>
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
