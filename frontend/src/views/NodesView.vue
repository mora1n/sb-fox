<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useNodesStore } from '../stores/nodes'
import { useNodeGroupsStore } from '../stores/nodeGroups'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import { downloadPost } from '../api/client'
import type { Node, NodeGroup } from '../api/types'
import { nodeSourceLabel } from '../utils/nodeSource'
import { NODE_SOURCES } from '../utils/nodeFilters'
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
  TrashIcon,
  XMarkIcon,
} from '@heroicons/vue/24/outline'

type NodeTab = 'single' | 'groups'
type ViewMode = 'card' | 'list'

const nodesStore = useNodesStore()
const nodeGroups = useNodeGroupsStore()
const ui = useUiStore()
const i18n = useI18nStore()

const showImport = ref(false)
const showEdit = ref(false)
const editing = ref<Node | null>(null)
const showGroupForm = ref(false)
const editingGroup = ref<NodeGroup | null>(null)
const selected = ref<Set<number>>(new Set())
const busy = ref(false)
const activeTab = ref<NodeTab>('single')
const nodeViewMode = ref<ViewMode>('card')
const groupViewMode = ref<ViewMode>('card')
const groupForm = ref({ name: '', description: '', node_ids: [] as number[] })

const loading = computed(() => nodesStore.loading || nodeGroups.loading)
const allNodeOptions = computed(() => (nodesStore.unfilteredNodes.length ? nodesStore.unfilteredNodes : nodesStore.nodes))
const allFilteredSelected = computed(
  () => nodesStore.nodes.length > 0 && nodesStore.nodes.every((n) => selected.value.has(n.id)),
)
const activeViewMode = computed(() => (activeTab.value === 'single' ? nodeViewMode.value : groupViewMode.value))

onMounted(load)

async function load() {
  try {
    await Promise.all([nodesStore.fetchAll(), nodesStore.fetchUnfiltered(), nodeGroups.fetchAll()])
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

function openCreate() {
  editing.value = null
  showEdit.value = true
}
function openEdit(n: Node) {
  editing.value = n
  showEdit.value = true
}

function nodeLabel(id: number) {
  return allNodeOptions.value.find((n) => n.id === id)?.tag || `#${id}`
}

function clearSearch() {
  nodesStore.filters.search = ''
}

function setViewMode(mode: ViewMode) {
  if (activeTab.value === 'single') nodeViewMode.value = mode
  else groupViewMode.value = mode
}

function openCreateGroup() {
  editingGroup.value = null
  groupForm.value = { name: '', description: '', node_ids: [...selected.value] }
  showGroupForm.value = true
}

function openEditGroup(g: NodeGroup) {
  editingGroup.value = g
  groupForm.value = { name: g.name, description: g.description, node_ids: [...g.node_ids] }
  showGroupForm.value = true
}

async function submitGroup() {
  busy.value = true
  try {
    if (!groupForm.value.name.trim()) throw new Error('请填写组合名称')
    const payload = { ...groupForm.value, name: groupForm.value.name.trim() }
    if (editingGroup.value) {
      await nodeGroups.update(editingGroup.value.id, payload)
      ui.success('组合节点已更新')
    } else {
      await nodeGroups.create(payload)
      ui.success('组合节点已创建')
    }
    showGroupForm.value = false
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function remove(n: Node) {
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

async function removeGroup(g: NodeGroup) {
  if (!confirm(`删除组合节点 "${g.name}"？`)) return
  try {
    await nodeGroups.remove(g.id)
    ui.success('组合节点已删除')
  } catch (e) {
    ui.error(errMsg(e))
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
      <div role="tablist" class="tabs tabs-boxed">
        <button role="tab" class="tab" :class="{ 'tab-active': activeTab === 'single' }" @click="activeTab = 'single'">
          {{ i18n.t('单节点') }}
        </button>
        <button role="tab" class="tab" :class="{ 'tab-active': activeTab === 'groups' }" @click="activeTab = 'groups'">
          {{ i18n.t('组合节点') }}
        </button>
      </div>
      <div class="join">
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
            <button class="btn btn-sm btn-ghost" @click="selectAll" :disabled="!nodesStore.nodes.length">
              {{ allFilteredSelected ? i18n.t('取消全选') : i18n.t('全选') }}
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
            v-for="n in nodesStore.nodes"
            :key="n.id"
            :node="n"
            :selected="selected.has(n.id)"
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
                <th>{{ i18n.t('标签') }}</th>
                <th>{{ i18n.t('服务器') }}</th>
                <th>{{ i18n.t('协议类型') }}</th>
                <th>{{ i18n.t('国家') }}</th>
                <th>{{ i18n.t('来源') }}</th>
                <th class="text-right">{{ i18n.t('操作') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="n in nodesStore.nodes"
                :key="n.id"
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
                <td class="text-right">
                  <div class="flex justify-end gap-1">
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
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <h2 class="font-semibold">{{ i18n.t('组合节点') }}</h2>
            <span class="badge badge-neutral">{{ nodeGroups.groups.length }}</span>
          </div>
          <button class="btn btn-sm btn-primary" @click="openCreateGroup">
            <RectangleStackIcon class="h-4 w-4" /> {{ i18n.t('新建组合') }}
          </button>
        </div>
        <div v-if="!nodeGroups.groups.length" class="text-center py-10 opacity-60 bg-base-100 border border-base-300 rounded-box">
          {{ i18n.t('暂无组合节点。') }}
        </div>
        <div v-else-if="groupViewMode === 'card'" class="flex flex-col gap-3">
          <div v-for="g in nodeGroups.groups" :key="g.id" class="card bg-base-100 border border-base-300 shadow-sm">
            <div class="card-body p-4 gap-3">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <h3 class="font-semibold truncate" :title="g.name">{{ g.name }}</h3>
                  <p v-if="g.description" class="text-xs opacity-70 truncate">{{ g.description }}</p>
                </div>
                <div class="flex gap-1">
                  <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑组合节点')" @click="openEditGroup(g)"><PencilSquareIcon class="h-4 w-4" /></button>
                  <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click="removeGroup(g)"><TrashIcon class="h-4 w-4" /></button>
                </div>
              </div>
              <div class="flex flex-wrap gap-1">
                <span v-for="id in g.node_ids.slice(0, 8)" :key="id" class="badge badge-sm badge-ghost max-w-full truncate">{{ nodeLabel(id) }}</span>
                <span v-if="g.node_ids.length > 8" class="badge badge-sm">+{{ g.node_ids.length - 8 }}</span>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="overflow-x-auto bg-base-100 border border-base-300 rounded-box">
          <table class="table table-sm">
            <thead>
              <tr>
                <th>{{ i18n.t('名称') }}</th>
                <th>{{ i18n.t('描述') }}</th>
                <th>{{ i18n.t('节点') }}</th>
                <th class="text-right">{{ i18n.t('操作') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="g in nodeGroups.groups" :key="g.id" class="hover:bg-base-200/70">
                <td class="font-medium max-w-64 truncate" :title="g.name">{{ g.name }}</td>
                <td class="max-w-80 truncate opacity-70" :title="g.description">{{ g.description || '-' }}</td>
                <td>
                  <div class="flex flex-wrap gap-1">
                    <span v-for="id in g.node_ids.slice(0, 6)" :key="id" class="badge badge-sm badge-ghost max-w-full truncate">{{ nodeLabel(id) }}</span>
                    <span v-if="g.node_ids.length > 6" class="badge badge-sm">+{{ g.node_ids.length - 6 }}</span>
                  </div>
                </td>
                <td class="text-right">
                  <div class="flex justify-end gap-1">
                    <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑组合节点')" @click="openEditGroup(g)"><PencilSquareIcon class="h-4 w-4" /></button>
                    <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click="removeGroup(g)"><TrashIcon class="h-4 w-4" /></button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <ImportDialog v-if="showImport" @close="showImport = false" @imported="load" />
    <NodeEditForm v-if="showEdit" :node="editing" @close="showEdit = false" @saved="load" />

    <div v-if="showGroupForm" class="modal modal-open">
      <div class="modal-box max-w-2xl">
        <h3 class="font-bold text-lg mb-3">{{ editingGroup ? i18n.t('编辑组合节点') : i18n.t('新建组合节点') }}</h3>
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
          <button class="btn btn-ghost" @click="showGroupForm = false" :disabled="busy">{{ i18n.t('取消') }}</button>
          <button class="btn btn-primary" @click="submitGroup" :disabled="busy">
            <span v-if="busy" class="loading loading-spinner loading-sm"></span> {{ i18n.t('保存') }}
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="showGroupForm = false"></div>
    </div>
  </div>
</template>
