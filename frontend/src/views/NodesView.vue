<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useNodesStore } from '../stores/nodes'
import { useUiStore } from '../stores/ui'
import { errMsg } from '../utils/error'
import { downloadPost } from '../api/client'
import type { Node } from '../api/types'
import NodeCard from '../components/NodeCard.vue'
import NodeEditForm from '../components/NodeEditForm.vue'
import ImportDialog from '../components/ImportDialog.vue'
import {
  PlusIcon,
  ArrowDownTrayIcon,
  ArrowPathIcon,
  DocumentArrowDownIcon,
} from '@heroicons/vue/24/outline'

const nodesStore = useNodesStore()
const ui = useUiStore()

const showImport = ref(false)
const showEdit = ref(false)
const editing = ref<Node | null>(null)
const selected = ref<Set<number>>(new Set())
const busy = ref(false)

const SOURCES = ['protocol', 'subscription', 'config', 'manual']

onMounted(load)

async function load() {
  try {
    await nodesStore.fetchAll()
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
  if (selected.value.size === nodesStore.nodes.length) selected.value = new Set()
  else selected.value = new Set(nodesStore.nodes.map((n) => n.id))
}

function openCreate() {
  editing.value = null
  showEdit.value = true
}
function openEdit(n: Node) {
  editing.value = n
  showEdit.value = true
}

async function remove(n: Node) {
  if (!confirm(`删除节点 "${n.tag}"？`)) return
  try {
    await nodesStore.remove(n.id)
    selected.value.delete(n.id)
    ui.success('节点已删除')
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
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between flex-wrap gap-2">
      <h1 class="text-2xl font-bold">节点 <span class="text-base font-normal opacity-60">({{ nodesStore.nodes.length }})</span></h1>
      <div class="flex gap-2 flex-wrap">
        <button class="btn btn-sm" @click="refreshCountry" :disabled="busy || !selected.size">
          <ArrowPathIcon class="h-4 w-4" /> 刷新国家
        </button>
        <button class="btn btn-sm" @click="exportTemplate" :disabled="busy || !selected.size">
          <DocumentArrowDownIcon class="h-4 w-4" /> 导出模板
        </button>
        <button class="btn btn-sm btn-primary" @click="showImport = true">
          <ArrowDownTrayIcon class="h-4 w-4" /> 导入
        </button>
        <button class="btn btn-sm btn-primary" @click="openCreate">
          <PlusIcon class="h-4 w-4" /> 新建
        </button>
      </div>
    </div>

    <!-- toolbar -->
    <div class="card bg-base-100 shadow-sm">
      <div class="card-body p-3 flex-row flex-wrap gap-2 items-center">
        <input
          v-model="nodesStore.filters.search"
          class="input input-bordered input-sm flex-1 min-w-40"
          placeholder="搜索 tag / server..."
        />
        <select v-model="nodesStore.filters.source" class="select select-bordered select-sm">
          <option value="">全部来源</option>
          <option v-for="s in SOURCES" :key="s" :value="s">{{ s }}</option>
        </select>
        <select v-model="nodesStore.filters.country" class="select select-bordered select-sm">
          <option value="">全部国家</option>
          <option v-for="c in nodesStore.countries" :key="c" :value="c">{{ c }}</option>
        </select>
        <select v-model="nodesStore.filters.type" class="select select-bordered select-sm">
          <option value="">全部类型</option>
          <option v-for="ty in nodesStore.types" :key="ty" :value="ty">{{ ty }}</option>
        </select>
        <button class="btn btn-sm btn-ghost" @click="selectAll">
          {{ selected.size === nodesStore.nodes.length && nodesStore.nodes.length ? '取消全选' : '全选' }}
        </button>
        <span v-if="selected.size" class="badge badge-neutral">已选 {{ selected.size }}</span>
      </div>
    </div>

    <div v-if="nodesStore.loading" class="flex justify-center py-10">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    <div v-else-if="!nodesStore.nodes.length" class="text-center py-10 opacity-60">
      暂无节点，点击「导入」或「新建」添加。
    </div>
    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
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

    <ImportDialog v-if="showImport" @close="showImport = false" @imported="load" />
    <NodeEditForm v-if="showEdit" :node="editing" @close="showEdit = false" @saved="load" />
  </div>
</template>
