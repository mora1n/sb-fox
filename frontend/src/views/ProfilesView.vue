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
import { readViewPref, writeViewPref } from '../utils/viewPrefs'
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
import {
  PlusIcon,
  PencilSquareIcon,
  TrashIcon,
  ArrowPathIcon,
  ListBulletIcon,
  Squares2X2Icon,
  DocumentDuplicateIcon,
} from '@heroicons/vue/24/outline'

type ViewMode = 'card' | 'list'
type SortDir = 'asc' | 'desc'
type ProfileSortKey = 'name' | 'template' | 'options' | 'link'
type EditorMode = 'group' | 'country' | 'chain'

const VIEW_MODES = ['card', 'list'] as const

const store = useProfilesStore()
const templates = useTemplatesStore()
const nodes = useNodesStore()
const nodeGroups = useNodeGroupsStore()
const settings = useSettingsStore()
const ui = useUiStore()
const i18n = useI18nStore()

const showForm = ref(false)
const editing = ref<Profile | null>(null)
const copyingFrom = ref<Profile | null>(null)
const busy = ref(false)
const formLoading = ref(false)
const suppressTemplateWatch = ref(false)
const config = ref('')
const validation = ref<KernelResult | null>(null)
const allNodes = ref<Node[]>([])
const structure = ref<TemplateStructure | null>(null)
const activeGroup = ref('')
const activeEditor = ref<EditorMode>('group')
const profileViewMode = ref<ViewMode>(readViewPref('sb-fox-view:subscriptions', 'card', VIEW_MODES))
const selectedProfiles = ref<Set<number>>(new Set())
const profileSortKey = ref<ProfileSortKey | ''>('')
const profileSortDir = ref<SortDir>('asc')
const kernelHint = computed(() => i18n.t('请选择有效 sing-box 内核或联系管理员配置内核'))
const tokenHost = computed(() => settings.subscriptionHostPrefix || '')
const allProfilesSelected = computed(
  () => store.profiles.length > 0 && store.profiles.every((p) => selectedProfiles.value.has(p.id)),
)
const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })
const sortedProfiles = computed(() => {
  if (!profileSortKey.value) return store.profiles
  return [...store.profiles].sort((a, b) => compareProfile(a, b, profileSortKey.value as ProfileSortKey, profileSortDir.value))
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
    if (suppressTemplateWatch.value || !showForm.value || !id) return
    await loadStructure(id)
  },
)

watch(profileViewMode, (value) => writeViewPref('sb-fox-view:subscriptions', value))

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

function optionText(p: Profile) {
  return optionBadges(p).join(' ')
}

function profileLinkText(p: Profile) {
  return `${tokenHost.value}/${store.subscriptionToken}/${p.name}`
}

function compareText(a: string, b: string, dir: SortDir) {
  const result = collator.compare(a || '', b || '')
  return dir === 'asc' ? result : -result
}

function compareProfile(a: Profile, b: Profile, key: ProfileSortKey, dir: SortDir) {
  if (key === 'template') return compareText(templateName(a.template_id), templateName(b.template_id), dir)
  if (key === 'options') return compareText(optionText(a), optionText(b), dir)
  if (key === 'link') return compareText(profileLinkText(a), profileLinkText(b), dir)
  return compareText(a.name, b.name, dir)
}

function toggleProfileSort(key: ProfileSortKey) {
  if (profileSortKey.value === key) profileSortDir.value = profileSortDir.value === 'asc' ? 'desc' : 'asc'
  else {
    profileSortKey.value = key
    profileSortDir.value = 'asc'
  }
}

function sortIndicator(active: string, dir: SortDir, key: string) {
  if (active !== key) return '↕'
  return dir === 'asc' ? '↑' : '↓'
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
    options: { autoCountryGroups: false, chainProxy: false, groupSelections: {} },
  }
  config.value = ''
  validation.value = null
  structure.value = null
  activeGroup.value = ''
  activeEditor.value = 'group'
  showForm.value = true
  formLoading.value = !!form.value.template_id
  try {
    if (form.value.template_id) await loadStructure(form.value.template_id)
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
    options: parseOptions(p.options),
  }
  config.value = ''
  validation.value = null
  structure.value = null
  activeGroup.value = ''
  activeEditor.value = 'group'
  showForm.value = true
  formLoading.value = true
  try {
    const full = await store.getOne(p.id)
    const options = parseOptions(full.options)
    const legacyNodeIDs = numberArray(full.node_ids)
    const legacyNodeGroupIDs = numberArray(full.node_group_ids)
    const shouldHydrateLegacySelection =
      !selectionMapHasAny(options.groupSelections) && (legacyNodeIDs.length > 0 || legacyNodeGroupIDs.length > 0)
    editing.value = full
    form.value = {
      name: full.name,
      template_id: full.template_id,
      node_ids: legacyNodeIDs,
      node_group_ids: legacyNodeGroupIDs,
      options,
    }
    await loadStructure(full.template_id)
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
    options: parseOptions(p.options),
  }
  config.value = ''
  validation.value = null
  structure.value = null
  activeGroup.value = ''
  activeEditor.value = 'group'
  showForm.value = true
  formLoading.value = true
  try {
    const full = await store.getOne(p.id)
    const options = parseOptions(full.options)
    const legacyNodeIDs = numberArray(full.node_ids)
    const legacyNodeGroupIDs = numberArray(full.node_group_ids)
    const shouldHydrateLegacySelection =
      !selectionMapHasAny(options.groupSelections) && (legacyNodeIDs.length > 0 || legacyNodeGroupIDs.length > 0)
    copyingFrom.value = full
    form.value = {
      name: full.name,
      template_id: full.template_id,
      node_ids: legacyNodeIDs,
      node_group_ids: legacyNodeGroupIDs,
      options,
    }
    await loadStructure(full.template_id)
    if (shouldHydrateLegacySelection) hydrateLegacySelection(legacyNodeIDs, legacyNodeGroupIDs)
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    formLoading.value = false
    suppressTemplateWatch.value = false
  }
}

function closeForm() {
  showForm.value = false
  editing.value = null
  copyingFrom.value = null
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
  if (form.value.options.autoCountryGroups && !hasSelection(autoCountrySelection())) {
    throw new Error('请选择自动国家分组来源节点')
  }
  for (const g of structure.value.groups) {
    const sel = selectionFor(g.tag)
    const chainFillsFinal = form.value.options.chainProxy && g.tag === (structure.value.final || '') && hasSelection(chainSelection())
    if (!hasSelection(sel) && !chainFillsFinal) {
      throw new Error(`出口分组 "${g.tag}" 不能为空`)
    }
  }
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
    closeForm()
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

function toggleProfileSelect(id: number) {
  if (selectedProfiles.value.has(id)) selectedProfiles.value.delete(id)
  else selectedProfiles.value.add(id)
  selectedProfiles.value = new Set(selectedProfiles.value)
}

function selectAllProfiles() {
  const next = new Set(selectedProfiles.value)
  if (allProfilesSelected.value) {
    for (const p of store.profiles) next.delete(p.id)
  } else {
    for (const p of store.profiles) next.add(p.id)
  }
  selectedProfiles.value = next
}

async function removeSelectedProfiles() {
  const ids = store.profiles.filter((p) => selectedProfiles.value.has(p.id)).map((p) => p.id)
  if (!ids.length) return ui.info('请先选择订阅')
  if (!confirm(`删除选中的 ${ids.length} 个订阅？`)) return
  busy.value = true
  try {
    for (const id of ids) await store.remove(id)
    selectedProfiles.value = new Set()
    ui.success(`已删除 ${ids.length} 个订阅`)
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function remove(p: Profile) {
  if (!confirm(`删除订阅 "${p.name}"？`)) return
  try {
    await store.remove(p.id)
    selectedProfiles.value.delete(p.id)
    selectedProfiles.value = new Set(selectedProfiles.value)
    ui.success('订阅已删除')
  } catch (e) {
    ui.error(errMsg(e))
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <h1 class="text-2xl font-bold">{{ i18n.t('订阅') }}</h1>
      <div class="flex items-center gap-2 flex-wrap justify-end">
        <div class="join">
          <button
            type="button"
            class="btn btn-sm join-item"
            :class="{ 'btn-active': profileViewMode === 'card' }"
            @click="profileViewMode = 'card'"
          >
            <Squares2X2Icon class="h-4 w-4" /> {{ i18n.t('卡片') }}
          </button>
          <button
            type="button"
            class="btn btn-sm join-item"
            :class="{ 'btn-active': profileViewMode === 'list' }"
            @click="profileViewMode = 'list'"
          >
            <ListBulletIcon class="h-4 w-4" /> {{ i18n.t('列表') }}
          </button>
        </div>
        <button class="btn btn-sm btn-primary" @click="openCreate">
          <PlusIcon class="h-4 w-4" /> {{ i18n.t('新建订阅') }}
        </button>
      </div>
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

    <div v-if="store.profiles.length" class="flex items-center justify-between gap-2 flex-wrap">
      <div class="flex items-center gap-2">
        <span class="badge badge-neutral">{{ store.profiles.length }}</span>
        <span v-if="selectedProfiles.size" class="badge badge-outline">{{ i18n.t('已选') }} {{ selectedProfiles.size }}</span>
      </div>
      <div class="flex items-center gap-2 flex-wrap">
        <button class="btn btn-sm" @click="selectAllProfiles" :disabled="!store.profiles.length">
          {{ allProfilesSelected ? i18n.t('取消全选') : i18n.t('全选') }}
        </button>
        <button class="btn btn-sm btn-error btn-outline" @click="removeSelectedProfiles" :disabled="busy || !selectedProfiles.size">
          <TrashIcon class="h-4 w-4" /> {{ i18n.t('删除') }}
        </button>
      </div>
    </div>

    <div v-if="store.loading" class="flex justify-center py-10"><span class="loading loading-spinner loading-lg"></span></div>
    <div v-else-if="!store.profiles.length" class="text-center py-10 opacity-60">{{ i18n.t('暂无订阅。') }}</div>
    <div v-else-if="profileViewMode === 'card'" class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div
        v-for="p in store.profiles"
        :key="p.id"
        class="card bg-base-100 shadow-sm border border-base-300 cursor-pointer transition-colors hover:bg-base-200/60"
        :class="{ 'ring-2 ring-primary': selectedProfiles.has(p.id) }"
        role="button"
        tabindex="0"
        @click="toggleProfileSelect(p.id)"
        @keydown.enter.prevent="toggleProfileSelect(p.id)"
        @keydown.space.prevent="toggleProfileSelect(p.id)"
      >
        <div class="card-body p-4 gap-3">
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-start gap-2 min-w-0">
              <input
                type="checkbox"
                class="checkbox checkbox-sm mt-1"
                :checked="selectedProfiles.has(p.id)"
                @click.stop
                @keydown.stop
                @change="toggleProfileSelect(p.id)"
              />
              <div class="min-w-0">
                <h2 class="card-title text-base truncate" :title="p.name">{{ p.name }}</h2>
                <div class="text-xs opacity-70 mt-1 flex flex-wrap gap-2">
                  <span>{{ i18n.t('模板:') }} {{ templateName(p.template_id) }}</span>
                </div>
              </div>
            </div>
            <div class="flex gap-1 flex-none">
              <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('复制订阅')" @click.stop="openCopy(p)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
              <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑订阅')" @click.stop="openEdit(p)"><PencilSquareIcon class="h-4 w-4" /></button>
              <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click.stop="remove(p)"><TrashIcon class="h-4 w-4" /></button>
            </div>
          </div>
          <div class="flex flex-wrap gap-1">
            <span v-for="badge in optionBadges(p)" :key="badge" class="badge badge-sm badge-neutral">{{ badge }}</span>
          </div>
          <div v-if="store.subscriptionToken" @click.stop @keydown.stop>
            <TokenLinkField
              :token="store.subscriptionToken"
              :profile-name="p.name"
              :host-prefix="tokenHost"
            />
          </div>
          <span class="text-xs opacity-60">{{ i18n.t('公开订阅链接') }}</span>
        </div>
      </div>
    </div>
    <div v-else class="overflow-x-auto bg-base-100 border border-base-300 rounded-box">
      <table class="table table-sm">
        <thead>
          <tr>
            <th class="w-10"></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleProfileSort('name')">{{ i18n.t('名称') }} {{ sortIndicator(profileSortKey, profileSortDir, 'name') }}</button></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleProfileSort('template')">{{ i18n.t('模板') }} {{ sortIndicator(profileSortKey, profileSortDir, 'template') }}</button></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleProfileSort('options')">{{ i18n.t('选项') }} {{ sortIndicator(profileSortKey, profileSortDir, 'options') }}</button></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleProfileSort('link')">{{ i18n.t('公开订阅链接') }} {{ sortIndicator(profileSortKey, profileSortDir, 'link') }}</button></th>
            <th class="text-right">{{ i18n.t('操作') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="p in sortedProfiles"
            :key="p.id"
            class="cursor-pointer hover:bg-base-200/70"
            :class="{ 'bg-base-200': selectedProfiles.has(p.id) }"
            @click="toggleProfileSelect(p.id)"
          >
            <td>
              <input
                type="checkbox"
                class="checkbox checkbox-sm"
                :checked="selectedProfiles.has(p.id)"
                @click.stop
                @change="toggleProfileSelect(p.id)"
              />
            </td>
            <td class="font-medium max-w-64 truncate" :title="p.name">{{ p.name }}</td>
            <td class="max-w-64 truncate" :title="templateName(p.template_id)">{{ templateName(p.template_id) }}</td>
            <td>
              <div class="flex flex-wrap gap-1">
                <span v-for="badge in optionBadges(p)" :key="badge" class="badge badge-sm badge-neutral">{{ badge }}</span>
                <span v-if="!optionBadges(p).length" class="opacity-50">-</span>
              </div>
            </td>
            <td class="min-w-80" @click.stop>
              <TokenLinkField
                v-if="store.subscriptionToken"
                :token="store.subscriptionToken"
                :profile-name="p.name"
                :host-prefix="tokenHost"
              />
              <span v-else class="opacity-50">-</span>
            </td>
            <td class="text-right">
              <div class="flex gap-1 justify-end">
                <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('复制订阅')" @click.stop="openCopy(p)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑订阅')" @click.stop="openEdit(p)"><PencilSquareIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click.stop="remove(p)"><TrashIcon class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showForm" class="modal modal-open">
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
        <div class="grid grid-cols-1 xl:grid-cols-[280px_minmax(320px,0.9fr)_minmax(480px,1.4fr)] gap-4" :class="{ 'pointer-events-none opacity-60': formLoading }">
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

            <div v-else-if="activeEditor === 'country'" class="rounded-box border border-base-300 p-3 bg-base-100">
              <div class="flex items-center justify-between gap-2 flex-wrap mb-3">
                <h4 class="font-semibold text-sm">{{ i18n.t('自动国家分组来源') }}</h4>
                <label class="label cursor-pointer justify-start gap-2 p-0">
                  <input type="checkbox" class="toggle toggle-sm" v-model="form.options.autoCountryGroups" />
                  <span class="label-text text-xs">{{ form.options.autoCountryGroups ? i18n.t('开启') : i18n.t('关闭') }}</span>
                </label>
              </div>
              <NodeMultiSelect :nodes="allNodes" v-model="autoCountryNodeIds" :disabled="!form.options.autoCountryGroups" />
              <div class="mt-3">
                <div class="flex items-center justify-between gap-2 flex-wrap mb-2">
                  <span class="label-text">{{ i18n.t('组合节点') }}</span>
                  <span class="flex gap-1 flex-wrap">
                    <button class="btn btn-xs min-h-7 h-7" type="button" @click="selectAllGroups(autoCountrySelection())" :disabled="!form.options.autoCountryGroups">{{ i18n.t('全选') }}</button>
                    <button class="btn btn-xs min-h-7 h-7" type="button" @click="clearGroups(autoCountrySelection())" :disabled="!form.options.autoCountryGroups">{{ i18n.t('全不选') }}</button>
                  </span>
                </div>
                <div class="border border-base-300 rounded-box max-h-32 overflow-y-auto divide-y divide-base-200">
                  <label v-for="g in nodeGroups.groups" :key="g.id" class="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-base-200" :class="{ 'opacity-60': !form.options.autoCountryGroups }">
                    <input
                      type="checkbox"
                      class="checkbox checkbox-sm"
                      :checked="autoCountrySelection().nodeGroupIds.includes(g.id)"
                      :disabled="!form.options.autoCountryGroups"
                      @change="toggleGroupForSelection(autoCountrySelection(), g.id)"
                    />
                    <span class="truncate flex-1 text-sm">{{ g.name }}</span>
                    <span class="badge badge-ghost badge-sm">{{ g.node_ids.length }}</span>
                  </label>
                  <div v-if="!nodeGroups.groups.length" class="px-3 py-4 text-sm opacity-60 text-center">{{ i18n.t('暂无组合节点。') }}</div>
                </div>
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
              <NodeMultiSelect :nodes="allNodes" v-model="chainNodeIds" :disabled="!form.options.chainProxy" />
              <div class="mt-3">
                <div class="flex items-center justify-between gap-2 flex-wrap mb-2">
                  <span class="label-text">{{ i18n.t('组合节点') }}</span>
                  <span class="flex gap-1 flex-wrap">
                    <button class="btn btn-xs min-h-7 h-7" type="button" @click="selectAllGroups(chainSelection())" :disabled="!form.options.chainProxy">{{ i18n.t('全选') }}</button>
                    <button class="btn btn-xs min-h-7 h-7" type="button" @click="clearGroups(chainSelection())" :disabled="!form.options.chainProxy">{{ i18n.t('全不选') }}</button>
                  </span>
                </div>
                <div class="border border-base-300 rounded-box max-h-32 overflow-y-auto divide-y divide-base-200">
                  <label v-for="g in nodeGroups.groups" :key="g.id" class="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-base-200" :class="{ 'opacity-60': !form.options.chainProxy }">
                    <input
                      type="checkbox"
                      class="checkbox checkbox-sm"
                      :checked="chainSelection().nodeGroupIds.includes(g.id)"
                      :disabled="!form.options.chainProxy"
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
  </div>
</template>
