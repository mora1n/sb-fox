<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { post } from '../../api/client'
import { useProfilesStore } from '../../stores/profiles'
import { useTemplatesStore } from '../../stores/templates'
import { useNodesStore } from '../../stores/nodes'
import { useNodeGroupsStore } from '../../stores/nodeGroups'
import { useSettingsStore } from '../../stores/settings'
import { useUiStore } from '../../stores/ui'
import { useI18nStore } from '../../stores/i18n'
import { errMsg } from '../../utils/error'
import type {
  KernelResult,
  NodeSummary,
  NodeSelection,
  PreviewPayload,
  Profile,
  ProfileOptions,
  ProfilePayload,
  TemplateStructure,
} from '../../api/types'
import NodeMultiSelect from '../NodeMultiSelect.vue'
import NodeGroupMultiSelect from '../NodeGroupMultiSelect.vue'
import JsonViewer from '../JsonViewer.vue'
import ValidationBadge from '../ValidationBadge.vue'

type EditorMode = 'group' | 'country' | 'chain'

const props = defineProps<{ mode: 'create' | 'edit' | 'copy'; profile?: Profile | null }>()
const emit = defineEmits<{ close: []; saved: [] }>()

const store = useProfilesStore()
const templates = useTemplatesStore()
const nodes = useNodesStore()
const nodeGroups = useNodeGroupsStore()
const settings = useSettingsStore()
const ui = useUiStore()
const i18n = useI18nStore()

const editing = ref<Profile | null>(null)
const copyingFrom = ref<Profile | null>(null)
const busy = ref(false)
const formLoading = ref(false)
const suppressTemplateWatch = ref(false)
const config = ref('')
const validation = ref<KernelResult | null>(null)
const allNodes = ref<NodeSummary[]>([])
const structure = ref<TemplateStructure | null>(null)
const activeGroup = ref('')
const activeEditor = ref<EditorMode>('group')
const kernelHint = computed(() => i18n.t('请选择有效 sing-box 内核或联系管理员配置内核'))

const form = ref<{
  name: string
  template_id: number
  node_ids: number[]
  node_group_ids: number[]
  subscription_enabled: boolean
  options: ProfileOptions
}>({
  name: '',
  template_id: 0,
  node_ids: [],
  node_group_ids: [],
  subscription_enabled: true,
  options: { autoCountryGroups: false, chainProxy: false, groupSelections: {} },
})

const activeStructureGroup = computed(() => structure.value?.groups.find((g) => g.tag === activeGroup.value) ?? null)
const formLocked = computed(() => busy.value || formLoading.value)
const formTitle = computed(() => {
  if (editing.value) return i18n.t('编辑订阅')
  if (copyingFrom.value) return i18n.t('复制订阅')
  return i18n.t('新建订阅')
})

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

const autoCountryNodeIds = computed<number[]>({
  get: () => autoCountrySelection().nodeIds,
  set: (ids) => {
    autoCountrySelection().nodeIds = ids
  },
})

onMounted(() => {
  void openInitial()
})

watch(
  () => form.value.template_id,
  async (id) => {
    if (suppressTemplateWatch.value || !id) return
    formLoading.value = !templates.structures[id]
    try {
      await loadStructure(id)
    } catch (e) {
      ui.error(errMsg(e))
    } finally {
      formLoading.value = false
    }
  },
)

async function openInitial() {
  if (props.mode === 'edit' && props.profile) {
    await openEdit(props.profile)
    return
  }
  if (props.mode === 'copy' && props.profile) {
    await openCopy(props.profile)
    return
  }
  await openCreate()
}

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
    const autoCountry = raw.autoCountrySelection as Partial<NodeSelection> | undefined
    const legacyChainIDs = Array.isArray(raw.chainProxyNodeIds)
      ? raw.chainProxyNodeIds.map(Number).filter(Boolean)
      : raw.chainProxyNodeId
        ? [Number(raw.chainProxyNodeId)].filter(Boolean)
        : []
    return {
      autoCountryGroups: !!raw.autoCountryGroups,
      chainProxy: !!raw.chainProxy,
      groupSelections: groups,
      autoCountrySelection: autoCountry
        ? {
            nodeIds: Array.isArray(autoCountry.nodeIds) ? autoCountry.nodeIds.map(Number).filter(Boolean) : [],
            nodeGroupIds: Array.isArray(autoCountry.nodeGroupIds) ? autoCountry.nodeGroupIds.map(Number).filter(Boolean) : [],
            outboundRefs: [],
            skipCountryGroups: false,
          }
        : undefined,
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
    return { autoCountryGroups: false, chainProxy: false, groupSelections: {} }
  }
}

async function loadStructure(templateID: number) {
  structure.value = await templates.structure(templateID)
  config.value = ''
  validation.value = null
  sanitizeGroupSelectionsForStructure()
  if (!structure.value.groups.length) {
    activeGroup.value = ''
    return
  }
  if (!activeGroup.value || !structure.value.groups.some((g) => g.tag === activeGroup.value)) {
    activeGroup.value = structure.value.groups[0].tag
  }
  ensureGroupSelections()
}

async function loadEditorDependencies() {
  const [loadedNodes] = await Promise.all([
    nodes.fetchSummary(),
    nodeGroups.fetchAll(),
  ])
  allNodes.value = loadedNodes
}

function sanitizeGroupSelectionsForStructure() {
  if (!structure.value) return
  const current = form.value.options.groupSelections ?? {}
  const next: Record<string, NodeSelection> = {}
  for (const g of structure.value.groups) {
    const sel = current[g.tag] ?? emptySelection()
    const allowedRefs = new Set(g.outbounds.map((tag) => tag.trim()).filter((tag) => tag && tag !== g.tag))
    next[g.tag] = {
      nodeIds: [...sel.nodeIds],
      nodeGroupIds: [...sel.nodeGroupIds],
      outboundRefs: sel.outboundRefs.map((tag) => tag.trim()).filter((tag) => allowedRefs.has(tag)),
      skipCountryGroups: !!sel.skipCountryGroups,
    }
  }
  form.value.options.groupSelections = next
}

function ensureGroupSelections() {
  if (!structure.value) return
  if (!form.value.options.groupSelections) form.value.options.groupSelections = {}
  for (const g of structure.value.groups) {
    const sel = selectionFor(g.tag)
    if (!hasSelection(sel)) sel.outboundRefs = outboundRefOptions(g.tag)
  }
  if (form.value.options.autoCountryGroups && !form.value.options.autoCountrySelection) {
    const merged = mergeGroupSelections(form.value.options.groupSelections)
    if (hasSelection(merged)) form.value.options.autoCountrySelection = merged
  }
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

function autoCountrySelection(): NodeSelection {
  if (!form.value.options.autoCountrySelection) form.value.options.autoCountrySelection = emptySelection()
  return form.value.options.autoCountrySelection
}

function mergeGroupSelections(selections: Record<string, NodeSelection> | undefined): NodeSelection {
  const merged = emptySelection()
  const nodeSet = new Set<number>()
  const groupSet = new Set<number>()
  for (const sel of Object.values(selections ?? {})) {
    for (const id of sel.nodeIds) nodeSet.add(id)
    for (const id of sel.nodeGroupIds) groupSet.add(id)
  }
  merged.nodeIds = [...nodeSet]
  merged.nodeGroupIds = [...groupSet]
  return merged
}

function numberArray(value: unknown): number[] {
  return Array.isArray(value) ? value.map(Number).filter(Boolean) : []
}

async function openCreate() {
  suppressTemplateWatch.value = true
  editing.value = null
  copyingFrom.value = null
  form.value = {
    name: '',
    template_id: templates.templates[0]?.id || 0,
    node_ids: [],
    node_group_ids: [],
    subscription_enabled: true,
    options: { autoCountryGroups: false, chainProxy: false, groupSelections: {} },
  }
  config.value = ''
  validation.value = null
  structure.value = null
  activeGroup.value = ''
  activeEditor.value = 'group'
  formLoading.value = true
  try {
    await Promise.all([
      form.value.template_id ? loadStructure(form.value.template_id) : Promise.resolve(),
      loadEditorDependencies(),
    ])
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    formLoading.value = false
    suppressTemplateWatch.value = false
  }
}

async function openEdit(p: Profile) {
  suppressTemplateWatch.value = true
  editing.value = p
  copyingFrom.value = null
  form.value = {
    name: p.name,
    template_id: p.template_id,
    node_ids: numberArray(p.node_ids),
    node_group_ids: numberArray(p.node_group_ids),
    subscription_enabled: p.subscription_enabled,
    options: parseOptions(p.options),
  }
  config.value = ''
  validation.value = null
  structure.value = null
  activeGroup.value = ''
  activeEditor.value = 'group'
  formLoading.value = true
  try {
    const options = parseOptions(p.options)
    const legacyNodeIDs = numberArray(p.node_ids)
    const legacyNodeGroupIDs = numberArray(p.node_group_ids)
    const shouldHydrateLegacySelection =
      !selectionMapHasAny(options.groupSelections) && (legacyNodeIDs.length > 0 || legacyNodeGroupIDs.length > 0)
    form.value = {
      name: p.name,
      template_id: p.template_id,
      node_ids: legacyNodeIDs,
      node_group_ids: legacyNodeGroupIDs,
      subscription_enabled: p.subscription_enabled,
      options,
    }
    await Promise.all([loadStructure(p.template_id), loadEditorDependencies()])
    if (shouldHydrateLegacySelection) hydrateLegacySelection(legacyNodeIDs, legacyNodeGroupIDs)
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    formLoading.value = false
    suppressTemplateWatch.value = false
  }
}

async function openCopy(p: Profile) {
  suppressTemplateWatch.value = true
  editing.value = null
  copyingFrom.value = p
  form.value = {
    name: p.name,
    template_id: p.template_id,
    node_ids: numberArray(p.node_ids),
    node_group_ids: numberArray(p.node_group_ids),
    subscription_enabled: true,
    options: parseOptions(p.options),
  }
  config.value = ''
  validation.value = null
  structure.value = null
  activeGroup.value = ''
  activeEditor.value = 'group'
  formLoading.value = true
  try {
    const options = parseOptions(p.options)
    const legacyNodeIDs = numberArray(p.node_ids)
    const legacyNodeGroupIDs = numberArray(p.node_group_ids)
    const shouldHydrateLegacySelection =
      !selectionMapHasAny(options.groupSelections) && (legacyNodeIDs.length > 0 || legacyNodeGroupIDs.length > 0)
    form.value = {
      name: p.name,
      template_id: p.template_id,
      node_ids: legacyNodeIDs,
      node_group_ids: legacyNodeGroupIDs,
      subscription_enabled: true,
      options,
    }
    await Promise.all([loadStructure(p.template_id), loadEditorDependencies()])
    if (shouldHydrateLegacySelection) hydrateLegacySelection(legacyNodeIDs, legacyNodeGroupIDs)
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    formLoading.value = false
    suppressTemplateWatch.value = false
  }
}

function closeForm() {
  emit('close')
}

function hydrateLegacySelection(nodeIDs: number[], nodeGroupIDs: number[]) {
  const targetTag = managedFinalGroupTag() || structure.value?.groups[0]?.tag || ''
  if (!targetTag) return
  const sel = selectionFor(targetTag)
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
  if (next.autoCountryGroups) {
    const auto = autoCountrySelection()
    next.autoCountrySelection = {
      nodeIds: [...auto.nodeIds],
      nodeGroupIds: [...auto.nodeGroupIds],
      outboundRefs: [],
      skipCountryGroups: false,
    }
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
    subscription_enabled: form.value.subscription_enabled,
    options: cleanSelections(form.value.options),
  }
}

function hasSelection(sel: NodeSelection) {
  return sel.nodeIds.length > 0 || sel.nodeGroupIds.length > 0 || sel.outboundRefs.length > 0
}

function selectionMapHasAny(selections: Record<string, NodeSelection> | undefined) {
  return Object.values(selections ?? {}).some(hasSelection)
}

function autoCountryFillsSelection(sel: NodeSelection) {
  return form.value.options.autoCountryGroups && hasSelection(autoCountrySelection()) && !sel.skipCountryGroups
}

function validateForm() {
  if (!form.value.name.trim()) throw new Error('请填写名称')
  if (!form.value.template_id) throw new Error('请选择模板')
  if (!structure.value) throw new Error('模板没有可用出口分组')
  if (form.value.options.autoCountryGroups && !hasSelection(autoCountrySelection())) {
    throw new Error('请选择自动国家分组来源节点')
  }
  const finalTag = managedFinalGroupTag()
  for (const g of structure.value.groups) {
    const sel = selectionFor(g.tag)
    const isFinalGroup = g.tag === finalTag
    const chainFillsFinal = form.value.options.chainProxy && isFinalGroup && hasSelection(chainSelection())
    const autoCountryFillsGroup = autoCountryFillsSelection(sel)
    if (!hasSelection(sel) && !chainFillsFinal && !autoCountryFillsGroup) {
      if (isFinalGroup) continue
      throw new Error(`出口分组 "${g.tag}" 不能为空`)
    }
  }
  if (finalTag) {
    const finalSelection = selectionFor(finalTag)
    const chainFillsFinal = form.value.options.chainProxy && hasSelection(chainSelection())
    if (!hasSelection(finalSelection) && !chainFillsFinal && !autoCountryFillsSelection(finalSelection)) {
      throw new Error(`最终出口 "${finalTag}" 至少需要选择一个节点、组合节点或引用出口`)
    }
  }
  if (form.value.options.chainProxy && !hasSelection(chainSelection())) {
    throw new Error('请选择链式代理节点')
  }
}

function managedFinalGroupTag() {
  const final = structure.value?.final?.trim() || ''
  if (!final) return ''
  return structure.value?.groups.some((g) => g.tag === final) ? final : ''
}

function groupSelectionCount(tag: string) {
  const sel = selectionFor(tag)
  return sel.nodeIds.length + sel.nodeGroupIds.length + sel.outboundRefs.length
}

function selectionCount(sel: NodeSelection) {
  return sel.nodeIds.length + sel.nodeGroupIds.length + sel.outboundRefs.length
}

function setActiveGroup(tag: string) {
  activeGroup.value = tag
  activeEditor.value = 'group'
}

function setActivePanel(mode: EditorMode) {
  activeEditor.value = mode
}

function outboundRefOptions(tag: string) {
  const group = structure.value?.groups.find((g) => g.tag === tag)
  if (!group) return []
  const seen = new Set<string>()
  return group.outbounds.filter((item) => {
    const clean = item.trim()
    if (!clean || clean === tag || seen.has(clean)) return false
    seen.add(clean)
    if (selectedOutboundRefs(tag).includes(clean)) return true
    if (groupRefCreatesCycle(tag, clean)) return false
    return true
  })
}

function selectedOutboundRefs(tag: string) {
  return form.value.options.groupSelections?.[tag]?.outboundRefs ?? []
}

function groupRefCreatesCycle(source: string, target: string) {
  if (!structure.value) return false
  const groupSet = new Set(structure.value.groups.map((g) => g.tag).filter(Boolean))
  if (!groupSet.has(target)) return false
  const graph = new Map<string, string[]>()
  for (const g of structure.value.groups) {
    const refs = [...selectedOutboundRefs(g.tag)]
    if (g.tag === source && !refs.includes(target)) refs.push(target)
    graph.set(g.tag, refs.filter((ref) => groupSet.has(ref)))
  }
  return groupCanReach(target, source, graph, new Set())
}

function groupCanReach(current: string, target: string, graph: Map<string, string[]>, seen: Set<string>): boolean {
  if (current === target) return true
  if (seen.has(current)) return false
  seen.add(current)
  return (graph.get(current) ?? []).some((next) => groupCanReach(next, target, graph, seen))
}

function toggleOutboundRef(groupTag: string, sel: NodeSelection, tag: string) {
  const set = new Set(sel.outboundRefs)
  if (set.has(tag)) set.delete(tag)
  else {
    if (groupRefCreatesCycle(groupTag, tag)) {
      ui.error(i18n.t('不能选择会造成循环引用的分组'))
      return
    }
    set.add(tag)
  }
  sel.outboundRefs = [...set]
}

async function submit() {
  if (formLoading.value) return ui.info('正在加载订阅...')
  busy.value = true
  try {
    if (copyingFrom.value && form.value.name.trim() === copyingFrom.value.name.trim()) {
      throw new Error(i18n.t('复制订阅需要修改名称后保存'))
    }
    validateForm()
    const payload = buildPayload()
    if (editing.value) {
      await store.update(editing.value.id, payload)
      ui.success('订阅已更新')
    } else {
      await store.create(payload)
      ui.success('订阅已创建')
    }
    emit('saved')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function generate() {
  if (formLoading.value) return ui.info('正在加载订阅...')
  busy.value = true
  validation.value = null
  config.value = ''
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
  if (formLoading.value) return ui.info('正在加载订阅...')
  if (!config.value) return ui.info('请先生成配置')
  if (!settings.kernel) {
    try {
      await settings.fetchKernelStatus()
    } catch (e) {
      ui.error(errMsg(e))
      return
    }
  }
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
  if (formLoading.value) return ui.info('正在加载订阅...')
  if (!config.value) return ui.info('请先生成配置')
  if (!settings.kernel) {
    try {
      await settings.fetchKernelStatus()
    } catch (e) {
      ui.error(errMsg(e))
      return
    }
  }
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
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box w-[96vw] max-w-[100rem] max-h-[90vh] overflow-y-auto">
      <div class="mb-3 flex items-center justify-between gap-3">
        <h3 class="font-bold text-lg truncate">{{ formTitle }}</h3>
        <div class="flex shrink-0 items-center gap-2">
          <button class="btn btn-ghost btn-sm" @click="closeForm" :disabled="busy">{{ i18n.t('取消') }}</button>
          <button class="btn btn-primary btn-sm" @click="submit" :disabled="formLocked">
            <span v-if="formLocked" class="loading loading-spinner loading-sm"></span> {{ i18n.t('保存') }}
          </button>
        </div>
      </div>
      <div v-if="formLoading" class="alert py-2 mb-3">
        <span class="loading loading-spinner loading-sm"></span>
        <span class="text-sm">{{ i18n.t('正在加载订阅...') }}</span>
      </div>
      <div class="grid grid-cols-1 xl:grid-cols-[280px_minmax(320px,0.9fr)_minmax(480px,1.4fr)] gap-4">
        <div class="flex flex-col gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('名称') }}</span>
            <input v-model="form.name" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('模板') }}</span>
            <select v-model.number="form.template_id" class="select select-bordered select-sm">
              <option :value="0" disabled>{{ i18n.t('选择模板') }}</option>
              <option v-for="t in templates.templates" :key="t.id" :value="t.id">{{ t.name }}</option>
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
                :class="{ 'bg-base-200': activeEditor === 'group' && activeGroup === g.tag }"
                @click="setActiveGroup(g.tag)"
              >
                <span class="truncate text-sm">{{ g.tag }}</span>
                <span class="badge badge-sm">{{ groupSelectionCount(g.tag) }}</span>
              </button>
              <div v-if="!structure?.groups.length" class="px-3 py-4 text-sm opacity-60 text-center">
                {{ i18n.t('未检测到 selector/urltest 分组。') }}
              </div>
            </div>
          </div>
          <div class="form-control">
            <span class="label-text mb-1">{{ i18n.t('生成选项') }}</span>
            <div class="border border-base-300 rounded-box divide-y divide-base-200 overflow-hidden">
              <button
                type="button"
                class="w-full flex items-center justify-between gap-2 px-3 py-2 text-left hover:bg-base-200"
                :class="{ 'bg-base-200': activeEditor === 'country' }"
                @click="setActivePanel('country')"
              >
                <span class="truncate text-sm">{{ i18n.t('自动国家分组') }}</span>
                <span class="badge badge-sm" :class="form.options.autoCountryGroups ? 'badge-neutral' : 'badge-ghost'">
                  {{ form.options.autoCountryGroups ? selectionCount(autoCountrySelection()) : i18n.t('关闭') }}
                </span>
              </button>
              <button
                type="button"
                class="w-full flex items-center justify-between gap-2 px-3 py-2 text-left hover:bg-base-200"
                :class="{ 'bg-base-200': activeEditor === 'chain' }"
                @click="setActivePanel('chain')"
              >
                <span class="truncate text-sm">{{ i18n.t('链式代理') }}</span>
                <span class="badge badge-sm" :class="form.options.chainProxy ? 'badge-neutral' : 'badge-ghost'">
                  {{ form.options.chainProxy ? selectionCount(chainSelection()) : i18n.t('关闭') }}
                </span>
              </button>
            </div>
          </div>
        </div>

        <div class="flex flex-col gap-3 min-w-0">
          <div v-if="activeEditor === 'group'" class="rounded-box border border-base-300 p-3 bg-base-100">
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
                    @change="toggleOutboundRef(activeGroup, selectionFor(activeGroup), tag)"
                  />
                  <span class="truncate flex-1 text-sm" :title="tag">{{ tag }}</span>
                </label>
                <div v-if="!outboundRefOptions(activeGroup).length" class="px-3 py-4 text-sm opacity-60 text-center">
                  {{ i18n.t('无可选出口') }}
                </div>
              </div>
            </div>
            <NodeMultiSelect :nodes="allNodes" v-model="activeNodeIds" :disabled="formLoading" />
            <div class="mt-3">
              <NodeGroupMultiSelect
                :groups="nodeGroups.groups"
                v-model="selectionFor(activeGroup).nodeGroupIds"
                :disabled="formLoading"
              />
            </div>
          </div>

          <div v-else-if="activeEditor === 'country'" class="rounded-box border border-base-300 p-3 bg-base-100">
            <div class="flex items-center justify-between gap-2 flex-wrap mb-3">
              <h4 class="font-semibold text-sm">{{ i18n.t('自动国家分组来源') }}</h4>
              <label class="label cursor-pointer justify-start gap-2 p-0">
                <input type="checkbox" class="toggle toggle-sm" v-model="form.options.autoCountryGroups" />
                <span class="label-text text-xs">{{ form.options.autoCountryGroups ? i18n.t('开启') : i18n.t('关闭') }}</span>
              </label>
            </div>
            <NodeMultiSelect :nodes="allNodes" v-model="autoCountryNodeIds" :disabled="formLoading || !form.options.autoCountryGroups" />
            <div class="mt-3">
              <NodeGroupMultiSelect
                :groups="nodeGroups.groups"
                v-model="autoCountrySelection().nodeGroupIds"
                :disabled="formLoading || !form.options.autoCountryGroups"
              />
            </div>
          </div>

          <div v-else class="rounded-box border border-base-300 p-3 bg-base-100">
            <div class="flex items-center justify-between gap-2 flex-wrap mb-3">
              <h4 class="font-semibold text-sm">{{ i18n.t('链式代理节点') }}</h4>
              <label class="label cursor-pointer justify-start gap-2 p-0">
                <input type="checkbox" class="toggle toggle-sm" v-model="form.options.chainProxy" />
                <span class="label-text text-xs">{{ form.options.chainProxy ? i18n.t('开启') : i18n.t('关闭') }}</span>
              </label>
            </div>
            <NodeMultiSelect :nodes="allNodes" v-model="chainNodeIds" :disabled="formLoading || !form.options.chainProxy" />
            <div class="mt-3">
              <NodeGroupMultiSelect
                :groups="nodeGroups.groups"
                v-model="chainSelection().nodeGroupIds"
                :disabled="formLoading || !form.options.chainProxy"
              />
            </div>
          </div>
        </div>

        <div class="flex flex-col gap-3 min-w-0">
          <div class="flex gap-2 flex-wrap">
            <button class="btn btn-primary btn-sm w-24 justify-center" @click="generate" :disabled="formLocked">
              <span v-if="formLocked" class="loading loading-spinner loading-sm"></span>
              <span>{{ i18n.t('生成') }}</span>
            </button>
            <button class="btn btn-sm" @click="validateGenerated" :disabled="formLocked" :class="{ 'opacity-50 cursor-not-allowed': !config || !settings.kernel?.available }" :title="settings.kernel?.available ? '' : kernelHint">{{ i18n.t('校验') }}</button>
            <button class="btn btn-sm" @click="formatGenerated" :disabled="formLocked" :class="{ 'opacity-50 cursor-not-allowed': !config || !settings.kernel?.available }" :title="settings.kernel?.available ? '' : kernelHint">{{ i18n.t('格式化') }}</button>
          </div>
          <ValidationBadge :status="validation?.status ?? null" :messages="validation?.messages" />
          <div class="border border-base-300 rounded-box bg-base-100 min-h-[28rem] min-w-80 overflow-auto resize-y">
            <JsonViewer v-if="config" :content="config" max-height="none" />
            <div v-else class="opacity-60 text-sm py-24 text-center">{{ i18n.t('点击「生成」查看配置。') }}</div>
          </div>
        </div>
      </div>
    </div>
    <div class="modal-backdrop" @click="closeForm"></div>
  </div>
</template>
