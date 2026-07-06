<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useNodesStore } from '../stores/nodes'
import { useNodeGroupsStore } from '../stores/nodeGroups'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import { downloadPost } from '../api/client'
import type { Node, NodeGroup, NodeSummary } from '../api/types'
import { nodeSourceLabel } from '../utils/nodeSource'
import { NODE_SOURCES } from '../utils/nodeFilters'
import { readViewPref, writeViewPref } from '../utils/viewPrefs'
import { formatDateTime, timeSortValue } from '../utils/time'
import NodeCard from '../components/NodeCard.vue'
import NodeEditForm from '../components/NodeEditForm.vue'
import NodeMultiSelect from '../components/NodeMultiSelect.vue'
import ImportDialog from '../components/ImportDialog.vue'
import CountryFlag from '../components/CountryFlag.vue'
import {
  PlusIcon,
  ArrowDownTrayIcon,
  ArrowPathIcon,
  ListBulletIcon,
  DocumentArrowDownIcon,
  PencilSquareIcon,
  RectangleStackIcon,
  Squares2X2Icon,
  DocumentDuplicateIcon,
  TrashIcon,
  XMarkIcon,
} from '@heroicons/vue/24/outline'

type NodeTab = 'single' | 'groups'
type ViewMode = 'card' | 'list'
type SortDir = 'asc' | 'desc'
type NodeSortKey = 'tag' | 'server' | 'type' | 'country' | 'source' | 'created_at' | 'updated_at'
type GroupSortKey = 'name' | 'description' | 'nodes' | 'created_at' | 'updated_at'

const VIEW_MODES = ['card', 'list'] as const
const NODE_TABS = ['single', 'groups'] as const

const nodesStore = useNodesStore()
const nodeGroups = useNodeGroupsStore()
const ui = useUiStore()
const i18n = useI18nStore()

const showImport = ref(false)
const showEdit = ref(false)
const editing = ref<Node | null>(null)
const copyingNodeFrom = ref<Node | null>(null)
const showGroupForm = ref(false)
const editingGroup = ref<NodeGroup | null>(null)
const copyingGroupFrom = ref<NodeGroup | null>(null)
const selected = ref<Set<number>>(new Set())
const selectedGroups = ref<Set<number>>(new Set())
const busy = ref(false)
const activeTab = ref<NodeTab>(readViewPref('sb-fox-view:nodes.tab', 'single', NODE_TABS))
const nodeViewMode = ref<ViewMode>(readViewPref('sb-fox-view:nodes.single', 'card', VIEW_MODES))
const groupViewMode = ref<ViewMode>(readViewPref('sb-fox-view:nodes.groups', 'card', VIEW_MODES))
const nodeSortKey = ref<NodeSortKey | ''>('')
const nodeSortDir = ref<SortDir>('asc')
const groupSortKey = ref<GroupSortKey | ''>('')
const groupSortDir = ref<SortDir>('asc')
const groupForm = ref({ name: '', description: '', node_ids: [] as number[] })

const loading = computed(() => {
  if (activeTab.value === 'single') return nodesStore.loading && !nodesStore.nodes.length
  return nodeGroups.loading && !nodeGroups.groups.length
})
const allNodeOptions = computed(() => (nodesStore.unfilteredNodes.length ? nodesStore.unfilteredNodes : nodesStore.nodes))
const nodeLabelMap = computed(() => new Map(allNodeOptions.value.map((n) => [n.id, n.tag])))
const allFilteredSelected = computed(
  () => nodesStore.nodes.length > 0 && nodesStore.nodes.every((n) => selected.value.has(n.id)),
)
const allGroupsSelected = computed(
  () => nodeGroups.groups.length > 0 && nodeGroups.groups.every((g) => selectedGroups.value.has(g.id)),
)
const activeViewMode = computed(() => (activeTab.value === 'single' ? nodeViewMode.value : groupViewMode.value))
const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })
const sortedNodes = computed(() => {
  if (!nodeSortKey.value) return nodesStore.nodes
  return [...nodesStore.nodes].sort((a, b) => compareNode(a, b, nodeSortKey.value as NodeSortKey, nodeSortDir.value))
})
const sortedGroups = computed(() => {
  if (!groupSortKey.value) return nodeGroups.groups
  return [...nodeGroups.groups].sort((a, b) => compareGroup(a, b, groupSortKey.value as GroupSortKey, groupSortDir.value))
})
const groupNodeSortTextMap = computed(() => {
  return new Map(nodeGroups.groups.map((g) => [g.id, g.node_ids.map((id) => nodeLabel(id)).join(' ')]))
})

onMounted(load)

async function load() {
  try {
    const hasFilters = Object.values(nodesStore.filters).some(Boolean)
    const jobs: Promise<unknown>[] = [nodesStore.fetchAll(), nodeGroups.fetchAll()]
    if (hasFilters) jobs.push(nodesStore.fetchUnfiltered())
    await Promise.all(jobs)
  } catch (e) {
    ui.error(errMsg(e))
  }
}

// debounce search + refetch on filter change
let t: ReturnType<typeof setTimeout>
watch(
  () => ({ ...nodesStore.filters }),
  () => {
    clearTimeout(t)
    t = setTimeout(load, 300)
  },
  { deep: true },
)

watch(activeTab, (value) => writeViewPref('sb-fox-view:nodes.tab', value))
watch(nodeViewMode, (value) => writeViewPref('sb-fox-view:nodes.single', value))
watch(groupViewMode, (value) => writeViewPref('sb-fox-view:nodes.groups', value))

function toggleSelect(id: number) {
  if (selected.value.has(id)) selected.value.delete(id)
  else selected.value.add(id)
  selected.value = new Set(selected.value)
}
function selectAll() {
  const next = new Set(selected.value)
  if (allFilteredSelected.value) {
    for (const n of nodesStore.nodes) next.delete(n.id)
  } else {
    for (const n of nodesStore.nodes) next.add(n.id)
  }
  selected.value = next
}

function toggleGroupSelect(id: number) {
  if (selectedGroups.value.has(id)) selectedGroups.value.delete(id)
  else selectedGroups.value.add(id)
  selectedGroups.value = new Set(selectedGroups.value)
}

function selectAllNodeGroups() {
  const next = new Set(selectedGroups.value)
  if (allGroupsSelected.value) {
    for (const g of nodeGroups.groups) next.delete(g.id)
  } else {
    for (const g of nodeGroups.groups) next.add(g.id)
  }
  selectedGroups.value = next
}

function openCreate() {
  editing.value = null
  copyingNodeFrom.value = null
  showEdit.value = true
}
async function loadFullNode(n: NodeSummary): Promise<Node | null> {
  try {
    return await nodesStore.getOne(n.id)
  } catch (e) {
    ui.error(errMsg(e))
    return null
  }
}

async function openEdit(n: NodeSummary) {
  if (busy.value) return
  busy.value = true
  try {
    const full = await loadFullNode(n)
    if (!full) return
    editing.value = full
    copyingNodeFrom.value = null
    showEdit.value = true
  } finally {
    busy.value = false
  }
}

async function openCopy(n: NodeSummary) {
  if (busy.value) return
  busy.value = true
  try {
    const full = await loadFullNode(n)
    if (!full) return
    editing.value = null
    copyingNodeFrom.value = full
    showEdit.value = true
  } finally {
    busy.value = false
  }
}

function closeNodeForm() {
  showEdit.value = false
  editing.value = null
  copyingNodeFrom.value = null
}

function nodeLabel(id: number) {
  return nodeLabelMap.value.get(id) || `#${id}`
}

function clearSearch() {
  nodesStore.filters.search = ''
}

function setViewMode(mode: ViewMode) {
  if (activeTab.value === 'single') nodeViewMode.value = mode
  else groupViewMode.value = mode
}

function setActiveTab(tab: NodeTab) {
  activeTab.value = tab
}

function compareText(a: string, b: string, dir: SortDir) {
  const result = collator.compare(a || '', b || '')
  return dir === 'asc' ? result : -result
}

function compareNumber(a: number, b: number, dir: SortDir) {
  const result = a === b ? 0 : a > b ? 1 : -1
  return dir === 'asc' ? result : -result
}

function compareTime(a: string, b: string, dir: SortDir) {
  return compareNumber(timeSortValue(a), timeSortValue(b), dir)
}

function compareNode(a: NodeSummary, b: NodeSummary, key: NodeSortKey, dir: SortDir) {
  if (key === 'server') return compareText(`${a.server}:${a.server_port}`, `${b.server}:${b.server_port}`, dir)
  if (key === 'country') return compareText(a.country_code, b.country_code, dir)
  if (key === 'created_at' || key === 'updated_at') return compareTime(a[key], b[key], dir)
  return compareText(String(a[key] ?? ''), String(b[key] ?? ''), dir)
}

function compareGroup(a: NodeGroup, b: NodeGroup, key: GroupSortKey, dir: SortDir) {
  if (key === 'created_at' || key === 'updated_at') return compareTime(a[key], b[key], dir)
  if (key === 'nodes') {
    const byNames = compareText(groupNodeSortText(a), groupNodeSortText(b), dir)
    if (byNames !== 0) return byNames
    return compareNumber(a.node_ids.length, b.node_ids.length, dir)
  }
  return compareText(String(a[key] ?? ''), String(b[key] ?? ''), dir)
}

function groupNodeSortText(group: NodeGroup) {
  return groupNodeSortTextMap.value.get(group.id) || ''
}

function toggleNodeSort(key: NodeSortKey) {
  if (nodeSortKey.value === key) nodeSortDir.value = nodeSortDir.value === 'asc' ? 'desc' : 'asc'
  else {
    nodeSortKey.value = key
    nodeSortDir.value = 'asc'
  }
}

function toggleGroupSort(key: GroupSortKey) {
  if (groupSortKey.value === key) groupSortDir.value = groupSortDir.value === 'asc' ? 'desc' : 'asc'
  else {
    groupSortKey.value = key
    groupSortDir.value = 'asc'
  }
}

function sortIndicator(active: string, dir: SortDir, key: string) {
  if (active !== key) return '↕'
  return dir === 'asc' ? '↑' : '↓'
}

function openCreateGroup() {
  editingGroup.value = null
  copyingGroupFrom.value = null
  groupForm.value = { name: '', description: '', node_ids: [...selected.value] }
  showGroupForm.value = true
}

function openEditGroup(g: NodeGroup) {
  editingGroup.value = g
  copyingGroupFrom.value = null
  groupForm.value = { name: g.name, description: g.description, node_ids: [...g.node_ids] }
  showGroupForm.value = true
}

function openCopyGroup(g: NodeGroup) {
  editingGroup.value = null
  copyingGroupFrom.value = g
  groupForm.value = { name: g.name, description: g.description, node_ids: [...g.node_ids] }
  showGroupForm.value = true
}

function closeGroupForm() {
  showGroupForm.value = false
  editingGroup.value = null
  copyingGroupFrom.value = null
}

async function submitGroup() {
  busy.value = true
  try {
    if (!groupForm.value.name.trim()) throw new Error('请填写组合名称')
    if (copyingGroupFrom.value && groupForm.value.name.trim() === copyingGroupFrom.value.name.trim()) {
      throw new Error(i18n.t('复制组合节点需要修改名称后保存'))
    }
    const payload = { ...groupForm.value, name: groupForm.value.name.trim() }
    if (editingGroup.value) {
      await nodeGroups.update(editingGroup.value.id, payload)
      ui.success('组合节点已更新')
    } else {
      await nodeGroups.create(payload)
      ui.success('组合节点已创建')
    }
    closeGroupForm()
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function remove(n: NodeSummary) {
  let message = `删除节点 "${n.tag}"？`
  try {
    const usage = await nodesStore.usage(n.id)
    if (usage.length) {
      const names = [...new Set(usage.map((u) => u.profile_name))]
      message = `删除节点 "${n.tag}"？\n\n该节点正在被以下订阅使用，删除后可能导致订阅失效：\n${names.join('\n')}`
    }
  } catch (e) {
    ui.error(errMsg(e))
    return
  }
  if (!confirm(message)) return
  try {
    await nodesStore.remove(n.id)
    selected.value.delete(n.id)
    ui.success('节点已删除')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function removeSelectedNodes() {
  const ids = [...selected.value]
  if (!ids.length) return ui.info('请先选择节点')
  const affectedProfiles = new Set<string>()
  try {
    for (const id of ids) {
      const usage = await nodesStore.usage(id)
      for (const item of usage) affectedProfiles.add(item.profile_name)
    }
  } catch (e) {
    ui.error(errMsg(e))
    return
  }

  let message = `删除选中的 ${ids.length} 个节点？`
  if (affectedProfiles.size) {
    message += `\n\n这些节点正在被以下订阅使用，删除后可能导致订阅失效：\n${[...affectedProfiles].join('\n')}`
  }
  if (!confirm(message)) return

  busy.value = true
  try {
    for (const id of ids) await nodesStore.remove(id)
    selected.value = new Set()
    ui.success(`已删除 ${ids.length} 个节点`)
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function removeGroup(g: NodeGroup) {
  if (!confirm(`删除组合节点 "${g.name}"？`)) return
  try {
    await nodeGroups.remove(g.id)
    selectedGroups.value.delete(g.id)
    selectedGroups.value = new Set(selectedGroups.value)
    ui.success('组合节点已删除')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function removeSelectedGroups() {
  const ids = nodeGroups.groups.filter((g) => selectedGroups.value.has(g.id)).map((g) => g.id)
  if (!ids.length) return ui.info('请先选择组合节点')
  if (!confirm(`删除选中的 ${ids.length} 个组合节点？`)) return
  busy.value = true
  try {
    for (const id of ids) await nodeGroups.remove(id)
    selectedGroups.value = new Set()
    ui.success(`已删除 ${ids.length} 个组合节点`)
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function refreshCountry() {
  if (!selected.value.size) return ui.info('请先选择节点')
  busy.value = true
  try {
    const n = await nodesStore.refreshCountry([...selected.value])
    ui.success(`已更新 ${n} 个节点的国家`)
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function exportTemplate() {
  if (!selected.value.size) return ui.info('请先选择节点')
  busy.value = true
  try {
    await downloadPost(
      '/nodes/export/template',
      { node_ids: [...selected.value], tag_country: true },
      'nodes-template.json',
    )
    ui.success('已导出 nodes-template.json')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function exportLinks() {
  if (!selected.value.size) return ui.info('请先选择节点')
  busy.value = true
  try {
    await downloadPost('/nodes/export/links', { node_ids: [...selected.value] }, 'nodes-links.txt')
    ui.success('已导出 nodes-links.txt')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between flex-wrap gap-2">
      <h1 class="text-2xl font-bold">{{ i18n.t('节点') }} <span class="text-base font-normal opacity-60">· {{ nodesStore.nodes.length }}</span></h1>
    </div>

    <!-- toolbar -->
    <div class="card bg-base-100 shadow-sm">
      <div class="card-body p-3 flex-row flex-wrap gap-2 items-center">
        <div class="join flex-1 min-w-48">
          <button class="btn btn-sm join-item" :disabled="!nodesStore.filters.search" @click="clearSearch" :title="i18n.t('清空')">
            <XMarkIcon class="h-4 w-4" />
          </button>
          <input
            v-model="nodesStore.filters.search"
            class="input input-bordered input-sm join-item flex-1"
            :placeholder="i18n.t('搜索 tag / server...')"
          />
        </div>
        <select v-model="nodesStore.filters.source" class="select select-bordered select-sm">
          <option value="">{{ i18n.t('全部来源') }}</option>
          <option v-for="s in NODE_SOURCES" :key="s" :value="s">{{ i18n.t(nodeSourceLabel(s)) }}</option>
        </select>
        <select v-model="nodesStore.filters.country" class="select select-bordered select-sm">
          <option value="">{{ i18n.t('全部国家') }}</option>
          <option v-for="c in nodesStore.countries" :key="c" :value="c">{{ c }}</option>
        </select>
        <select v-model="nodesStore.filters.type" class="select select-bordered select-sm">
          <option value="">{{ i18n.t('全部协议') }}</option>
          <option v-for="ty in nodesStore.types" :key="ty" :value="ty">{{ ty }}</option>
        </select>
      </div>
    </div>

    <div class="flex items-center justify-between gap-2 flex-wrap">
      <div role="tablist" class="join bg-base-200 p-0.5 rounded-btn shadow-sm">
        <button role="tab" class="btn btn-sm join-item" :class="{ 'btn-active': activeTab === 'single' }" @click="setActiveTab('single')">
          {{ i18n.t('单节点') }}
        </button>
        <button role="tab" class="btn btn-sm join-item" :class="{ 'btn-active': activeTab === 'groups' }" @click="setActiveTab('groups')">
          {{ i18n.t('组合节点') }}
        </button>
      </div>
      <div class="join bg-base-200 p-0.5 rounded-btn shadow-sm">
        <button type="button" class="btn btn-sm join-item" :class="{ 'btn-active': activeViewMode === 'card' }" @click="setViewMode('card')">
          <Squares2X2Icon class="h-4 w-4" /> {{ i18n.t('卡片') }}
        </button>
        <button type="button" class="btn btn-sm join-item" :class="{ 'btn-active': activeViewMode === 'list' }" @click="setViewMode('list')">
          <ListBulletIcon class="h-4 w-4" /> {{ i18n.t('列表') }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="flex justify-center py-10">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    <template v-else>
      <section v-if="activeTab === 'single'" class="flex flex-col gap-3">
        <div class="flex items-center justify-between flex-wrap gap-2">
          <div class="flex items-center gap-2">
            <h2 class="font-semibold">{{ i18n.t('节点列表') }}</h2>
            <span class="badge badge-neutral">{{ nodesStore.nodes.length }}</span>
            <span v-if="selected.size" class="badge badge-outline">{{ i18n.t('已选') }} {{ selected.size }}</span>
          </div>
          <div class="flex gap-2 flex-wrap">
            <button
              class="btn btn-sm"
              :class="{ 'btn-active': allFilteredSelected }"
              @click="selectAll"
              :disabled="!nodesStore.nodes.length"
            >
              {{ allFilteredSelected ? i18n.t('取消全选') : i18n.t('全选') }}
            </button>
            <button class="btn btn-sm text-error bg-error/10 hover:bg-error/20 border-transparent" @click="removeSelectedNodes" :disabled="busy || !selected.size">
              <TrashIcon class="h-4 w-4" /> {{ i18n.t('删除') }}
            </button>
            <button class="btn btn-sm" @click="refreshCountry" :disabled="busy || !selected.size">
              <ArrowPathIcon class="h-4 w-4" /> {{ i18n.t('刷新国家') }}
            </button>
            <div class="dropdown dropdown-end">
              <button tabindex="0" type="button" class="btn btn-sm" :disabled="busy || !selected.size">
                <DocumentArrowDownIcon class="h-4 w-4" /> {{ i18n.t('导出') }}
              </button>
              <ul tabindex="0" class="dropdown-content menu bg-base-100 rounded-box z-20 w-40 p-2 shadow border border-base-300">
                <li><button type="button" @click="exportTemplate">{{ i18n.t('模板 JSON') }}</button></li>
                <li><button type="button" @click="exportLinks">{{ i18n.t('协议链接') }}</button></li>
              </ul>
            </div>
            <button class="btn btn-sm btn-primary" @click="showImport = true">
              <ArrowDownTrayIcon class="h-4 w-4" /> {{ i18n.t('导入') }}
            </button>
            <button class="btn btn-sm btn-primary" @click="openCreate">
              <PlusIcon class="h-4 w-4" /> {{ i18n.t('新建') }}
            </button>
          </div>
        </div>
        <div v-if="!nodesStore.nodes.length" class="text-center py-10 opacity-60 bg-base-100 border border-base-300 rounded-box">
          {{ i18n.t('暂无节点，点击「导入」或「新建」添加。') }}
        </div>
        <div v-else-if="nodeViewMode === 'card'" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          <NodeCard
            v-for="n in sortedNodes"
            :key="n.id"
            v-memo="[n.id, n.tag, n.server, n.server_port, n.type, n.country_code, n.source, n.has_detour, n.updated_at, selected.has(n.id), i18n.locale]"
            :node="n"
            :selected="selected.has(n.id)"
            @copy="openCopy(n)"
            @edit="openEdit(n)"
            @remove="remove(n)"
            @toggle-select="toggleSelect(n.id)"
          />
        </div>
        <div v-else class="overflow-x-auto bg-base-100 border border-base-300 rounded-box">
          <table class="table table-sm">
            <thead>
              <tr>
                <th class="w-10"></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleNodeSort('tag')">{{ i18n.t('标签') }} {{ sortIndicator(nodeSortKey, nodeSortDir, 'tag') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleNodeSort('server')">{{ i18n.t('服务器') }} {{ sortIndicator(nodeSortKey, nodeSortDir, 'server') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleNodeSort('type')">{{ i18n.t('协议类型') }} {{ sortIndicator(nodeSortKey, nodeSortDir, 'type') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleNodeSort('country')">{{ i18n.t('国家') }} {{ sortIndicator(nodeSortKey, nodeSortDir, 'country') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleNodeSort('source')">{{ i18n.t('来源') }} {{ sortIndicator(nodeSortKey, nodeSortDir, 'source') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleNodeSort('created_at')">{{ i18n.t('导入时间') }} {{ sortIndicator(nodeSortKey, nodeSortDir, 'created_at') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleNodeSort('updated_at')">{{ i18n.t('修改时间') }} {{ sortIndicator(nodeSortKey, nodeSortDir, 'updated_at') }}</button></th>
                <th class="text-right">{{ i18n.t('操作') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="n in sortedNodes"
                :key="n.id"
                v-memo="[n.id, n.tag, n.server, n.server_port, n.type, n.country_code, n.source, n.has_detour, n.updated_at, selected.has(n.id), i18n.locale]"
                class="cursor-pointer hover:bg-base-200/70"
                :class="{ 'bg-base-200': selected.has(n.id) }"
                @click="toggleSelect(n.id)"
              >
                <td>
                  <input type="checkbox" class="checkbox checkbox-sm" :checked="selected.has(n.id)" @click.stop @change="toggleSelect(n.id)" />
                </td>
                <td class="font-medium max-w-64 truncate" :title="n.tag">{{ n.tag }}</td>
                <td class="mono text-xs opacity-80">{{ n.server }}:{{ n.server_port }}</td>
                <td><span class="badge badge-outline badge-sm">{{ n.type }}</span></td>
                <td>
                  <CountryFlag v-if="n.country_code" :code="n.country_code" />
                  <span v-else class="opacity-50">-</span>
                </td>
                <td><span class="badge badge-sm badge-neutral">{{ i18n.t(nodeSourceLabel(n.source)) }}</span></td>
                <td class="whitespace-nowrap text-xs opacity-70" :title="n.created_at">{{ formatDateTime(n.created_at) }}</td>
                <td class="whitespace-nowrap text-xs opacity-70" :title="n.updated_at">{{ formatDateTime(n.updated_at) }}</td>
                <td class="text-right">
                  <div class="flex justify-end gap-1">
                    <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('复制节点')" @click.stop="openCopy(n)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
                    <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑节点')" @click.stop="openEdit(n)"><PencilSquareIcon class="h-4 w-4" /></button>
                    <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click.stop="remove(n)"><TrashIcon class="h-4 w-4" /></button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-else class="flex flex-col gap-3">
        <div class="flex items-center justify-between flex-wrap gap-2">
          <div class="flex items-center gap-2">
            <h2 class="font-semibold">{{ i18n.t('组合节点') }}</h2>
            <span class="badge badge-neutral">{{ nodeGroups.groups.length }}</span>
            <span v-if="selectedGroups.size" class="badge badge-outline">{{ i18n.t('已选') }} {{ selectedGroups.size }}</span>
          </div>
          <div class="flex gap-2 flex-wrap">
            <button
              class="btn btn-sm"
              :class="{ 'btn-active': allGroupsSelected }"
              @click="selectAllNodeGroups"
              :disabled="!nodeGroups.groups.length"
            >
              {{ allGroupsSelected ? i18n.t('取消全选') : i18n.t('全选') }}
            </button>
            <button class="btn btn-sm text-error bg-error/10 hover:bg-error/20 border-transparent" @click="removeSelectedGroups" :disabled="busy || !selectedGroups.size">
              <TrashIcon class="h-4 w-4" /> {{ i18n.t('删除') }}
            </button>
            <button class="btn btn-sm btn-primary" @click="openCreateGroup">
              <RectangleStackIcon class="h-4 w-4" /> {{ i18n.t('新建组合') }}
            </button>
          </div>
        </div>
        <div v-if="!nodeGroups.groups.length" class="text-center py-10 opacity-60 bg-base-100 border border-base-300 rounded-box">
          {{ i18n.t('暂无组合节点。') }}
        </div>
        <div v-else-if="groupViewMode === 'card'" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          <div
            v-for="g in sortedGroups"
            :key="g.id"
            class="card bg-base-100 border border-base-300 shadow-sm cursor-pointer transition-colors hover:bg-base-200/60"
            :class="{ 'ring-2 ring-primary': selectedGroups.has(g.id) }"
            role="button"
            tabindex="0"
            @click="toggleGroupSelect(g.id)"
            @keydown.enter.prevent="toggleGroupSelect(g.id)"
            @keydown.space.prevent="toggleGroupSelect(g.id)"
          >
            <div class="card-body p-4 gap-3">
              <div class="flex items-start justify-between gap-2">
                <div class="flex items-start gap-2 min-w-0">
                  <input
                    type="checkbox"
                    class="checkbox checkbox-sm mt-0.5"
                    :checked="selectedGroups.has(g.id)"
                    @click.stop
                    @keydown.stop
                    @change="toggleGroupSelect(g.id)"
                  />
                  <div class="min-w-0">
                    <h3 class="font-semibold truncate" :title="g.name">{{ g.name }}</h3>
                    <p v-if="g.description" class="text-xs opacity-70 truncate">{{ g.description }}</p>
                  </div>
                </div>
                <div class="flex gap-1">
                  <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('复制组合节点')" @click.stop="openCopyGroup(g)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
                  <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑组合节点')" @click.stop="openEditGroup(g)"><PencilSquareIcon class="h-4 w-4" /></button>
                  <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click.stop="removeGroup(g)"><TrashIcon class="h-4 w-4" /></button>
                </div>
              </div>
              <div class="flex flex-wrap gap-1">
                <span v-for="id in g.node_ids.slice(0, 8)" :key="id" class="badge badge-sm badge-ghost max-w-full truncate">{{ nodeLabel(id) }}</span>
                <span v-if="g.node_ids.length > 8" class="badge badge-sm">+{{ g.node_ids.length - 8 }}</span>
              </div>
              <div class="grid grid-cols-2 gap-2 text-[11px] opacity-60">
                <div class="truncate" :title="formatDateTime(g.created_at)">{{ i18n.t('创建时间') }}: {{ formatDateTime(g.created_at) }}</div>
                <div class="truncate" :title="formatDateTime(g.updated_at)">{{ i18n.t('修改时间') }}: {{ formatDateTime(g.updated_at) }}</div>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="overflow-x-auto bg-base-100 border border-base-300 rounded-box">
          <table class="table table-sm">
            <thead>
              <tr>
                <th class="w-10"></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleGroupSort('name')">{{ i18n.t('名称') }} {{ sortIndicator(groupSortKey, groupSortDir, 'name') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleGroupSort('description')">{{ i18n.t('描述') }} {{ sortIndicator(groupSortKey, groupSortDir, 'description') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleGroupSort('nodes')">{{ i18n.t('节点') }} {{ sortIndicator(groupSortKey, groupSortDir, 'nodes') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleGroupSort('created_at')">{{ i18n.t('创建时间') }} {{ sortIndicator(groupSortKey, groupSortDir, 'created_at') }}</button></th>
                <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleGroupSort('updated_at')">{{ i18n.t('修改时间') }} {{ sortIndicator(groupSortKey, groupSortDir, 'updated_at') }}</button></th>
                <th class="text-right">{{ i18n.t('操作') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="g in sortedGroups"
                :key="g.id"
                class="cursor-pointer hover:bg-base-200/70"
                :class="{ 'bg-base-200': selectedGroups.has(g.id) }"
                @click="toggleGroupSelect(g.id)"
              >
                <td>
                  <input type="checkbox" class="checkbox checkbox-sm" :checked="selectedGroups.has(g.id)" @click.stop @change="toggleGroupSelect(g.id)" />
                </td>
                <td class="font-medium max-w-64 truncate" :title="g.name">{{ g.name }}</td>
                <td class="max-w-80 truncate opacity-70" :title="g.description">{{ g.description || '-' }}</td>
                <td>
                  <div class="flex flex-wrap gap-1">
                    <span v-for="id in g.node_ids.slice(0, 6)" :key="id" class="badge badge-sm badge-ghost max-w-full truncate">{{ nodeLabel(id) }}</span>
                    <span v-if="g.node_ids.length > 6" class="badge badge-sm">+{{ g.node_ids.length - 6 }}</span>
                  </div>
                </td>
                <td class="whitespace-nowrap text-xs opacity-70" :title="g.created_at">{{ formatDateTime(g.created_at) }}</td>
                <td class="whitespace-nowrap text-xs opacity-70" :title="g.updated_at">{{ formatDateTime(g.updated_at) }}</td>
                <td class="text-right">
                  <div class="flex justify-end gap-1">
                    <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('复制组合节点')" @click.stop="openCopyGroup(g)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
                    <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑组合节点')" @click.stop="openEditGroup(g)"><PencilSquareIcon class="h-4 w-4" /></button>
                    <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click.stop="removeGroup(g)"><TrashIcon class="h-4 w-4" /></button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <ImportDialog v-if="showImport" @close="showImport = false" />
    <NodeEditForm v-if="showEdit" :node="editing" :copy-from="copyingNodeFrom" @close="closeNodeForm" />

    <div v-if="showGroupForm" class="modal modal-open">
      <div class="modal-box max-w-2xl">
        <h3 class="font-bold text-lg mb-3">{{ editingGroup ? i18n.t('编辑组合节点') : copyingGroupFrom ? i18n.t('复制组合节点') : i18n.t('新建组合节点') }}</h3>
        <div class="flex flex-col gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('名称') }}</span>
            <input v-model="groupForm.name" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('描述') }}</span>
            <input v-model="groupForm.description" class="input input-bordered input-sm" />
          </label>
          <div class="form-control">
            <span class="label-text mb-1">{{ i18n.t('节点') }}</span>
            <NodeMultiSelect :nodes="allNodeOptions" v-model="groupForm.node_ids" />
          </div>
        </div>
        <div class="modal-action">
          <button class="btn" @click="closeGroupForm" :disabled="busy">{{ i18n.t('取消') }}</button>
          <button class="btn btn-primary" @click="submitGroup" :disabled="busy">
            <span v-if="busy" class="loading loading-spinner loading-sm"></span> {{ i18n.t('保存') }}
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="closeGroupForm"></div>
    </div>
  </div>
</template>
