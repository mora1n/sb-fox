<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useTemplatesStore } from '../stores/templates'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import type { Template, TemplateStructure, TemplateStructureGroup } from '../api/types'
import JsonViewer from '../components/JsonViewer.vue'
import {
  PlusIcon,
  EyeIcon,
  RectangleGroupIcon,
  PencilSquareIcon,
  TrashIcon,
  ArrowDownTrayIcon,
  Bars3Icon,
  ChevronDownIcon,
  ChevronUpIcon,
} from '@heroicons/vue/24/outline'

const store = useTemplatesStore()
const ui = useUiStore()
const i18n = useI18nStore()

const viewing = ref<Template | null>(null)
const structure = ref<TemplateStructure | null>(null)
const structureFor = ref<Template | null>(null)
const dragIndex = ref<number | null>(null)

const showForm = ref(false)
const editing = ref<Template | null>(null)
const formName = ref('')
const formDesc = ref('')
const formContent = ref('')
const busy = ref(false)

const groupTags = computed(() => structure.value?.groups.map((g) => g.tag).filter(Boolean) ?? [])
const availableOutbounds = computed(() => {
  if (!structure.value) return []
  const seen = new Set<string>()
  const out: string[] = []
  const add = (tag: string) => {
    const clean = tag.trim()
    if (!clean || seen.has(clean)) return
    seen.add(clean)
    out.push(clean)
  }
  structure.value.available_outbounds?.forEach(add)
  structure.value.groups.forEach((g) => add(g.tag))
  return out
})

onMounted(load)
async function load() {
  try {
    await store.fetchAll()
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function view(t: Template) {
  try {
    viewing.value = await store.getOne(t.id)
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function editStructure(t: Template) {
  try {
    structure.value = await store.structure(t.id)
    if (!structure.value.final && structure.value.groups.length) {
      structure.value.final = structure.value.groups[0].tag
    }
    structureFor.value = t
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function exportTemplate(t: Template) {
  try {
    await store.exportTemplate(t.id, t.name)
    ui.success('模板已导出')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

function openImport() {
  editing.value = null
  formName.value = ''
  formDesc.value = ''
  formContent.value = ''
  showForm.value = true
}

async function openEdit(t: Template) {
  editing.value = t
  try {
    const full = await store.getOne(t.id)
    formName.value = full.name
    formDesc.value = full.description
    formContent.value = full.content
    showForm.value = true
  } catch (e) {
    ui.error(errMsg(e))
  }
}

function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  const reader = new FileReader()
  reader.onload = () => {
    formContent.value = String(reader.result)
    if (!formName.value) formName.value = f.name.replace(/\.json$/i, '')
  }
  reader.readAsText(f)
}

async function submitForm() {
  busy.value = true
  try {
    JSON.parse(formContent.value)
    if (editing.value) {
      const r = await store.update(editing.value.id, formContent.value, formDesc.value)
      ui.success(r.imported ? `模板已更新，已导入 ${r.imported} 个节点` : '模板已更新')
    } else {
      if (!formName.value.trim()) throw new Error('请填写模板名称')
      const r = await store.create(formName.value, formContent.value, formDesc.value)
      ui.success(r.imported ? `模板已导入，已导入 ${r.imported} 个节点` : '模板已导入')
    }
    showForm.value = false
  } catch (e) {
    ui.error(e instanceof SyntaxError ? 'content 不是合法 JSON' : errMsg(e))
  } finally {
    busy.value = false
  }
}

async function saveStructure() {
  if (!structure.value || !structureFor.value) return
  busy.value = true
  try {
    structure.value = await store.saveStructure(structureFor.value.id, structure.value)
    ui.success('分组管理已保存')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

function outboundOptions(g: TemplateStructureGroup) {
  return availableOutbounds.value.filter((tag) => tag !== g.tag)
}

function toggleOutbound(g: TemplateStructureGroup, tag: string) {
  if (g.outbounds.includes(tag)) {
    g.outbounds = g.outbounds.filter((item) => item !== tag)
  } else {
    g.outbounds = [...g.outbounds, tag]
  }
  if (g.default && !g.outbounds.includes(g.default)) g.default = ''
}

function allOutboundsSelected(g: TemplateStructureGroup) {
  const options = outboundOptions(g)
  return options.length > 0 && options.every((tag) => g.outbounds.includes(tag))
}

function selectAllOutbounds(g: TemplateStructureGroup) {
  g.outbounds = outboundOptions(g)
  if (g.default && !g.outbounds.includes(g.default)) g.default = ''
}

function clearOutbounds(g: TemplateStructureGroup) {
  g.outbounds = []
  g.default = ''
}

function moveGroup(index: number, delta: number) {
  if (!structure.value) return
  const target = index + delta
  if (target < 0 || target >= structure.value.groups.length) return
  const next = [...structure.value.groups]
  const item = next[index]
  next[index] = next[target]
  next[target] = item
  structure.value.groups = next
}

function onDragStart(index: number) {
  dragIndex.value = index
}

function onDragEnd() {
  dragIndex.value = null
}

function onDrop(index: number) {
  if (!structure.value || dragIndex.value === null || dragIndex.value === index) return
  const [item] = structure.value.groups.splice(dragIndex.value, 1)
  structure.value.groups.splice(index, 0, item)
  dragIndex.value = index
}

async function remove(t: Template) {
  if (!confirm(`删除模板 "${t.name}"？`)) return
  try {
    await store.remove(t.id)
    ui.success('模板已删除')
  } catch (e) {
    ui.error(errMsg(e))
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold">{{ i18n.t('模板') }}</h1>
      <button class="btn btn-sm btn-primary" @click="openImport"><PlusIcon class="h-4 w-4" /> {{ i18n.t('导入模板') }}</button>
    </div>

    <div v-if="store.loading" class="flex justify-center py-10"><span class="loading loading-spinner loading-lg"></span></div>
    <div v-else class="overflow-x-auto card bg-base-100 shadow-sm">
      <table class="table">
        <thead>
          <tr><th>{{ i18n.t('名称') }}</th><th>{{ i18n.t('类型') }}</th><th>{{ i18n.t('描述') }}</th><th class="text-right">{{ i18n.t('操作') }}</th></tr>
        </thead>
        <tbody>
          <tr v-for="t in store.templates" :key="t.id">
            <td class="font-semibold">{{ t.name }}</td>
            <td>
              <span class="badge badge-sm" :class="t.kind === 'builtin' ? 'badge-neutral' : 'badge-primary'">{{ t.kind }}</span>
            </td>
            <td class="text-sm opacity-70 max-w-xs truncate">{{ t.description }}</td>
            <td>
              <div class="flex gap-1 justify-end">
                <button class="btn btn-xs btn-ghost" @click="view(t)" :title="i18n.t('查看')"><EyeIcon class="h-4 w-4" /></button>
                <button class="btn btn-xs btn-ghost" @click="editStructure(t)" :title="i18n.t('分组管理')"><RectangleGroupIcon class="h-4 w-4" /></button>
                <button class="btn btn-xs btn-ghost" @click="exportTemplate(t)" :title="i18n.t('导出')"><ArrowDownTrayIcon class="h-4 w-4" /></button>
                <button v-if="t.kind === 'user'" class="btn btn-xs btn-ghost" @click="openEdit(t)" :title="i18n.t('编辑模板')"><PencilSquareIcon class="h-4 w-4" /></button>
                <button v-if="t.kind === 'user'" class="btn btn-xs btn-ghost text-error" @click="remove(t)" :title="i18n.t('删除')"><TrashIcon class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="viewing" class="modal modal-open">
      <div class="modal-box max-w-3xl">
        <h3 class="font-bold text-lg mb-3">{{ viewing.name }}</h3>
        <JsonViewer :content="viewing.content" />
        <div class="modal-action"><button class="btn" @click="viewing = null">{{ i18n.t('关闭') }}</button></div>
      </div>
      <div class="modal-backdrop" @click="viewing = null"></div>
    </div>

    <div v-if="structure" class="modal modal-open">
      <div class="modal-box max-w-5xl">
        <div class="flex items-center justify-between gap-2 mb-3">
          <h3 class="font-bold text-lg">{{ i18n.t('分组管理') }} · {{ structureFor?.name }}</h3>
        </div>
        <label class="form-control max-w-sm mb-4">
          <span class="label-text mb-1">{{ i18n.t('最终出口') }}</span>
          <select v-model="structure.final" class="select select-bordered select-sm">
            <option v-for="tag in groupTags" :key="tag" :value="tag">{{ tag }}</option>
          </select>
        </label>
        <div v-if="!structure.groups.length" class="opacity-60 text-sm">{{ i18n.t('未检测到 selector/urltest 分组。') }}</div>
        <div v-else class="flex flex-col gap-3 max-h-[62vh] overflow-y-auto pr-1">
          <div
            v-for="(g, i) in structure.groups"
            :key="g.tag + ':' + i"
            class="border border-base-300 rounded-box bg-base-100 p-3"
            :class="{ 'bg-base-200': dragIndex === i }"
            @dragover.prevent
            @drop="onDrop(i)"
          >
            <div class="grid grid-cols-1 lg:grid-cols-[32px_minmax(160px,1fr)_120px_minmax(180px,1fr)_160px_64px] gap-2 items-start">
              <div class="flex items-center gap-1">
                <button
                  class="btn btn-xs btn-ghost cursor-move"
                  type="button"
                  draggable="true"
                  :title="i18n.t('拖拽排序')"
                  @dragstart="onDragStart(i)"
                  @dragend="onDragEnd"
                >
                  <Bars3Icon class="h-4 w-4" />
                </button>
              </div>
              <label class="form-control">
                <span class="label-text mb-1">{{ i18n.t('标签') }}</span>
                <input v-model="g.tag" class="input input-bordered input-sm" disabled />
              </label>
              <label class="form-control">
                <span class="label-text mb-1">{{ i18n.t('类型') }}</span>
                <select v-model="g.type" class="select select-bordered select-sm">
                  <option value="selector">selector</option>
                  <option value="urltest">urltest</option>
                </select>
              </label>
              <div class="form-control">
                <span class="label-text mb-1 flex items-center justify-between gap-2">
                  <span>{{ i18n.t('出口') }}</span>
                  <span class="flex gap-1">
                    <button
                      class="btn btn-xs min-h-0 h-5 px-1.5 text-[10px]"
                      type="button"
                      :disabled="!outboundOptions(g).length || allOutboundsSelected(g)"
                      @click="selectAllOutbounds(g)"
                    >
                      {{ i18n.t('全选') }}
                    </button>
                    <button
                      class="btn btn-xs min-h-0 h-5 px-1.5 text-[10px]"
                      type="button"
                      :disabled="!g.outbounds.length"
                      @click="clearOutbounds(g)"
                    >
                      {{ i18n.t('全不选') }}
                    </button>
                  </span>
                </span>
                <div class="border border-base-300 rounded-box max-h-28 overflow-y-auto divide-y divide-base-200 bg-base-100">
                  <label
                    v-for="tag in outboundOptions(g)"
                    :key="tag"
                    class="flex items-center gap-2 px-2 py-1.5 cursor-pointer hover:bg-base-200"
                  >
                    <input
                      type="checkbox"
                      class="checkbox checkbox-xs"
                      :checked="g.outbounds.includes(tag)"
                      @change="toggleOutbound(g, tag)"
                    />
                    <span class="truncate text-xs" :title="tag">{{ tag }}</span>
                  </label>
                  <div v-if="!outboundOptions(g).length" class="px-2 py-3 text-xs opacity-60 text-center">
                    {{ i18n.t('无可选出口') }}
                  </div>
                </div>
              </div>
              <label class="form-control">
                <span class="label-text mb-1">{{ i18n.t('默认出口') }}</span>
                <select v-model="g.default" class="select select-bordered select-sm" :disabled="g.outbounds.length <= 1">
                  <option value="">{{ i18n.t('未指定') }}</option>
                  <option v-for="tag in g.outbounds" :key="tag" :value="tag">{{ tag }}</option>
                </select>
              </label>
              <div class="join pt-6">
                <button
                  class="btn btn-xs btn-square join-item"
                  type="button"
                  :title="i18n.t('上移')"
                  :disabled="i === 0"
                  @click="moveGroup(i, -1)"
                >
                  <ChevronUpIcon class="h-3 w-3" />
                </button>
                <button
                  class="btn btn-xs btn-square join-item"
                  type="button"
                  :title="i18n.t('下移')"
                  :disabled="i === structure.groups.length - 1"
                  @click="moveGroup(i, 1)"
                >
                  <ChevronDownIcon class="h-3 w-3" />
                </button>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-action">
          <button class="btn btn-ghost" @click="structure = null" :disabled="busy">{{ i18n.t('取消') }}</button>
          <button class="btn btn-primary" @click="saveStructure" :disabled="busy">
            <span v-if="busy" class="loading loading-spinner loading-sm"></span> {{ i18n.t('保存') }}
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="structure = null"></div>
    </div>

    <div v-if="showForm" class="modal modal-open">
      <div class="modal-box max-w-2xl">
        <h3 class="font-bold text-lg mb-3">{{ editing ? i18n.t('编辑模板') : i18n.t('导入模板') }}</h3>
        <div class="flex flex-col gap-3">
          <label v-if="!editing" class="form-control">
            <span class="label-text mb-1">{{ i18n.t('名称') }}</span>
            <input v-model="formName" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('描述') }}</span>
            <input v-model="formDesc" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('上传 JSON 文件') }}</span>
            <input type="file" accept=".json,application/json" class="file-input file-input-bordered file-input-sm" @change="onFile" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('模板内容') }}</span>
            <textarea v-model="formContent" class="textarea textarea-bordered h-56 mono text-xs" placeholder='{ "outbounds": [ ... ] }'></textarea>
          </label>
        </div>
        <div class="modal-action">
          <button class="btn btn-ghost" @click="showForm = false" :disabled="busy">{{ i18n.t('取消') }}</button>
          <button class="btn btn-primary" @click="submitForm" :disabled="busy">
            <span v-if="busy" class="loading loading-spinner loading-sm"></span> {{ i18n.t('保存') }}
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="showForm = false"></div>
    </div>
  </div>
</template>
