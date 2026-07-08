<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useTemplatesStore } from '../stores/templates'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import { readViewPref, writeViewPref } from '../utils/viewPrefs'
import { formatDateTime, timeSortValue } from '../utils/time'
import type { Template, TemplateStructure, TemplateStructureGroup, TemplateSummary } from '../api/types'
import JsonViewer from '../components/JsonViewer.vue'
import BulkDeleteDialog from '../components/BulkDeleteDialog.vue'
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
  ListBulletIcon,
  Squares2X2Icon,
  DocumentDuplicateIcon,
} from '@heroicons/vue/24/outline'

type ViewMode = 'card' | 'list'
type SortDir = 'asc' | 'desc'
type TemplateSortKey = 'name' | 'description' | 'created_at' | 'updated_at'

const VIEW_MODES = ['card', 'list'] as const

const store = useTemplatesStore()
const ui = useUiStore()
const i18n = useI18nStore()

const viewing = ref<Template | null>(null)
const structure = ref<TemplateStructure | null>(null)
const structureFor = ref<TemplateSummary | null>(null)
const dragIndex = ref<number | null>(null)
const groupPressedIndex = ref<number | null>(null)
const groupInsertIndex = ref<number | null>(null)

const showForm = ref(false)
const editing = ref<TemplateSummary | null>(null)
const copyingFrom = ref<TemplateSummary | null>(null)
const formName = ref('')
const formDesc = ref('')
const formContent = ref('')
const formLoading = ref(false)
const structureLoading = ref(false)
const busy = ref(false)
const templateViewMode = ref<ViewMode>(readViewPref('sb-fox-view:templates', 'card', VIEW_MODES))
const selectedTemplates = ref<Set<number>>(new Set())
const bulkDeleteDialog = ref({
  open: false,
  ids: [] as number[],
  itemName: '',
  busy: false,
})
const templateSortKey = ref<TemplateSortKey | ''>('')
const templateSortDir = ref<SortDir>('asc')
let formSeq = 0
let structureSeq = 0

const selectableTemplates = computed(() => store.templates.filter((t) => t.kind === 'user'))
const allTemplatesSelected = computed(
  () => selectableTemplates.value.length > 0 && selectableTemplates.value.every((t) => selectedTemplates.value.has(t.id)),
)
const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })
const sortedTemplates = computed(() => {
  if (!templateSortKey.value) return store.templates
  return [...store.templates].sort((a, b) => compareTemplate(a, b, templateSortKey.value as TemplateSortKey, templateSortDir.value))
})
const formTitle = computed(() => {
  if (editing.value) return i18n.t('编辑模板')
  if (copyingFrom.value) return i18n.t('复制模板')
  return i18n.t('导入模板')
})
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
const finalOutboundOptions = computed(() => {
  const out = [...availableOutbounds.value]
  const final = structure.value?.final?.trim() || ''
  if (final && !out.includes(final)) out.unshift(final)
  return out
})

onMounted(load)
watch(templateViewMode, (value) => writeViewPref('sb-fox-view:templates', value))
async function load() {
  try {
    await store.fetchAll()
  } catch (e) {
    ui.error(errMsg(e))
  }
}

function compareText(a: string, b: string, dir: SortDir) {
  const result = collator.compare(a || '', b || '')
  return dir === 'asc' ? result : -result
}

function compareNumber(a: number, b: number, dir: SortDir) {
  const result = a === b ? 0 : a > b ? 1 : -1
  return dir === 'asc' ? result : -result
}

function compareTemplate(a: TemplateSummary, b: TemplateSummary, key: TemplateSortKey, dir: SortDir) {
  if (key === 'created_at' || key === 'updated_at') {
    return compareNumber(timeSortValue(a[key]), timeSortValue(b[key]), dir)
  }
  return compareText(String(a[key] ?? ''), String(b[key] ?? ''), dir)
}

function toggleTemplateSort(key: TemplateSortKey) {
  if (templateSortKey.value === key) templateSortDir.value = templateSortDir.value === 'asc' ? 'desc' : 'asc'
  else {
    templateSortKey.value = key
    templateSortDir.value = 'asc'
  }
}

function sortIndicator(active: string, dir: SortDir, key: string) {
  if (active !== key) return '↕'
  return dir === 'asc' ? '↑' : '↓'
}

async function view(t: TemplateSummary) {
  try {
    viewing.value = await store.getOne(t.id)
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function editStructure(t: TemplateSummary) {
  const seq = ++structureSeq
  structure.value = null
  structureFor.value = t
  structureLoading.value = true
  try {
    const next = await store.structure(t.id)
    if (seq !== structureSeq || structureFor.value?.id !== t.id) return
    structure.value = next
  } catch (e) {
    if (seq !== structureSeq) return
    ui.error(errMsg(e))
    closeStructure()
  } finally {
    if (seq === structureSeq) structureLoading.value = false
  }
}

function prefetchTemplate(t: TemplateSummary) {
  void store.prefetchOne(t.id).catch(() => undefined)
}

function prefetchStructure(t: TemplateSummary) {
  void store.prefetchStructure(t.id).catch(() => undefined)
}

async function exportTemplate(t: TemplateSummary) {
  try {
    await store.exportTemplate(t.id, t.name)
    ui.success('模板已导出')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

function openImport() {
  formSeq++
  editing.value = null
  copyingFrom.value = null
  formName.value = ''
  formDesc.value = ''
  formContent.value = ''
  formLoading.value = false
  showForm.value = true
}

async function openEdit(t: TemplateSummary) {
  const seq = ++formSeq
  editing.value = t
  copyingFrom.value = null
  formName.value = t.name
  formDesc.value = t.description
  formContent.value = ''
  formLoading.value = true
  showForm.value = true
  try {
    const full = await store.getOne(t.id)
    if (seq !== formSeq || !showForm.value) return
    formName.value = full.name
    formDesc.value = full.description
    formContent.value = full.content
  } catch (e) {
    if (seq !== formSeq) return
    ui.error(errMsg(e))
    closeForm()
  } finally {
    if (seq === formSeq) formLoading.value = false
  }
}

async function openCopy(t: TemplateSummary) {
  const seq = ++formSeq
  editing.value = null
  copyingFrom.value = t
  formName.value = t.name
  formDesc.value = t.description
  formContent.value = ''
  formLoading.value = true
  showForm.value = true
  try {
    const full = await store.getOne(t.id)
    if (seq !== formSeq || !showForm.value) return
    copyingFrom.value = full
    formName.value = full.name
    formDesc.value = full.description
    formContent.value = full.content
  } catch (e) {
    if (seq !== formSeq) return
    ui.error(errMsg(e))
    closeForm()
  } finally {
    if (seq === formSeq) formLoading.value = false
  }
}

function closeForm() {
  formSeq++
  showForm.value = false
  editing.value = null
  copyingFrom.value = null
  formLoading.value = false
}

function closeStructure() {
  structureSeq++
  structure.value = null
  structureFor.value = null
  structureLoading.value = false
  dragIndex.value = null
  groupPressedIndex.value = null
  groupInsertIndex.value = null
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

function templateImportMessage(action: string, imported: number, deduped = 0) {
  const parts = [action]
  if (imported) parts.push(`已导入 ${imported} 个节点`)
  if (deduped) parts.push(`已跳过 ${deduped} 个重复节点`)
  return parts.join('，')
}

async function submitForm() {
  if (formLoading.value) return ui.info(i18n.t('正在加载模板...'))
  busy.value = true
  try {
    JSON.parse(formContent.value)
    if (editing.value) {
      const r = await store.update(editing.value.id, formContent.value, formDesc.value)
      ui.success(templateImportMessage('模板已更新', r.imported, r.deduped))
    } else {
      const name = formName.value.trim()
      if (!name) throw new Error('请填写模板名称')
      if (copyingFrom.value && name === copyingFrom.value.name.trim()) {
        throw new Error(i18n.t('复制模板需要修改名称后保存'))
      }
      const existing = await store.findByName(name)
      if (existing) {
        if (copyingFrom.value) throw new Error(i18n.t('模板名称已存在'))
        if (existing.kind !== 'user') throw new Error(`模板 "${name}" 不能覆盖更新`)
        if (!confirm(`模板 "${name}" 已存在，是否覆盖更新？`)) return
        const r = await store.update(existing.id, formContent.value, formDesc.value)
        ui.success(templateImportMessage('模板已更新', r.imported, r.deduped))
      } else {
        const r = await store.create(name, formContent.value, formDesc.value)
        ui.success(templateImportMessage('模板已导入', r.imported, r.deduped))
      }
    }
    closeForm()
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
    await store.saveStructure(structureFor.value.id, structure.value)
    closeStructure()
    ui.success('分组管理已保存')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

function groupRefCreatesCycle(source: string, target: string) {
  if (!structure.value) return false
  const groupSet = new Set(structure.value.groups.map((g) => g.tag).filter(Boolean))
  if (!groupSet.has(target)) return false
  const graph = new Map<string, string[]>()
  for (const g of structure.value.groups) {
    const refs = [...g.outbounds]
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

function outboundOptions(g: TemplateStructureGroup) {
  return availableOutbounds.value.filter((tag) => {
    if (tag === g.tag) return false
    if (g.outbounds.includes(tag)) return true
    return !groupRefCreatesCycle(g.tag, tag)
  })
}

function toggleOutbound(g: TemplateStructureGroup, tag: string) {
  if (g.outbounds.includes(tag)) {
    g.outbounds = g.outbounds.filter((item) => item !== tag)
  } else {
    if (groupRefCreatesCycle(g.tag, tag)) {
      ui.error(i18n.t('不能选择会造成循环引用的分组'))
      return
    }
    g.outbounds = [...g.outbounds, tag]
  }
  if (!groupSupportsDefault(g) || (g.default && !g.outbounds.includes(g.default))) g.default = ''
}

function allOutboundsSelected(g: TemplateStructureGroup) {
  const options = outboundOptions(g)
  return options.length > 0 && options.every((tag) => g.outbounds.includes(tag))
}

function selectAllOutbounds(g: TemplateStructureGroup) {
  g.outbounds = outboundOptions(g)
  if (!groupSupportsDefault(g) || (g.default && !g.outbounds.includes(g.default))) g.default = ''
}

function clearOutbounds(g: TemplateStructureGroup) {
  g.outbounds = []
  g.default = ''
}

function groupSupportsDefault(g: TemplateStructureGroup) {
  return g.type === 'selector'
}

function onGroupTypeChange(g: TemplateStructureGroup) {
  if (!groupSupportsDefault(g)) g.default = ''
  else if (g.default && !g.outbounds.includes(g.default)) g.default = ''
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

function isControlDragTarget(event: DragEvent | PointerEvent) {
  const target = event.target as HTMLElement | null
  return !!target?.closest('button,input,select,textarea,a,[contenteditable="true"]')
}

function pressGroup(index: number, event: PointerEvent) {
  if (isControlDragTarget(event)) return
  groupPressedIndex.value = index
}

function onDragStart(index: number, event: DragEvent) {
  if (isControlDragTarget(event)) {
    event.preventDefault()
    groupPressedIndex.value = null
    return
  }
  dragIndex.value = index
  groupPressedIndex.value = index
  groupInsertIndex.value = null
  event.dataTransfer?.setData('text/plain', String(index))
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}

function clearGroupDrag() {
  dragIndex.value = null
  groupPressedIndex.value = null
  groupInsertIndex.value = null
}

function clearGroupPress() {
  groupPressedIndex.value = null
}

function groupInsertTarget(event: DragEvent) {
  const list = event.currentTarget as HTMLElement | null
  if (!list || !structure.value) return structure.value?.groups.length ?? 0
  const rows = Array.from(list.querySelectorAll<HTMLElement>('[data-group-index]'))
  for (const row of rows) {
    const index = Number(row.dataset.groupIndex)
    const rect = row.getBoundingClientRect()
    if (event.clientY < rect.top + rect.height / 2) return index
  }
  return structure.value.groups.length
}

function onDragOver(event: DragEvent) {
  if (dragIndex.value === null) {
    groupInsertIndex.value = null
    return
  }
  const target = groupInsertTarget(event)
  groupInsertIndex.value = target === dragIndex.value || target === dragIndex.value + 1 ? null : target
}

function leaveGroupList(event: DragEvent) {
  const list = event.currentTarget as HTMLElement | null
  if (!list) return
  const rect = list.getBoundingClientRect()
  const outside =
    event.clientX < rect.left ||
    event.clientX > rect.right ||
    event.clientY < rect.top ||
    event.clientY > rect.bottom
  if (outside) groupInsertIndex.value = null
}

function onDrop(event: DragEvent) {
  if (!structure.value || dragIndex.value === null) {
    clearGroupDrag()
    return
  }
  const source = dragIndex.value
  const target = groupInsertIndex.value ?? groupInsertTarget(event)
  if (target === source || target === source + 1) {
    clearGroupDrag()
    return
  }
  const [item] = structure.value.groups.splice(source, 1)
  structure.value.groups.splice(target > source ? target - 1 : target, 0, item)
  clearGroupDrag()
}

function toggleTemplateSelect(t: TemplateSummary) {
  if (t.kind !== 'user') return
  if (selectedTemplates.value.has(t.id)) selectedTemplates.value.delete(t.id)
  else selectedTemplates.value.add(t.id)
  selectedTemplates.value = new Set(selectedTemplates.value)
}

function selectAllTemplates() {
  const next = new Set(selectedTemplates.value)
  if (allTemplatesSelected.value) {
    for (const t of selectableTemplates.value) next.delete(t.id)
  } else {
    for (const t of selectableTemplates.value) next.add(t.id)
  }
  selectedTemplates.value = next
}

async function removeSelectedTemplates() {
  const ids = selectableTemplates.value.filter((t) => selectedTemplates.value.has(t.id)).map((t) => t.id)
  if (!ids.length) return ui.info('请先选择模板')
  bulkDeleteDialog.value = { open: true, ids, itemName: '', busy: false }
}

function closeBulkDeleteDialog() {
  if (bulkDeleteDialog.value.busy) return
  bulkDeleteDialog.value = { ...bulkDeleteDialog.value, open: false, itemName: '' }
}

async function confirmBulkDelete() {
  const ids = bulkDeleteDialog.value.ids
  if (!bulkDeleteDialog.value.open || bulkDeleteDialog.value.busy || !ids.length) return
  bulkDeleteDialog.value = { ...bulkDeleteDialog.value, busy: true }
  busy.value = true
  try {
    const deleted = await store.bulkDelete(ids)
    selectedTemplates.value = removeSelectedIDs(selectedTemplates.value, ids)
    ui.success(`已删除 ${deleted} 个模板`)
    bulkDeleteDialog.value = { ...bulkDeleteDialog.value, open: false, itemName: '', busy: false }
  } catch (e) {
    ui.error(errMsg(e))
    bulkDeleteDialog.value = { ...bulkDeleteDialog.value, busy: false }
  } finally {
    busy.value = false
  }
}

function remove(t: TemplateSummary) {
  bulkDeleteDialog.value = { open: true, ids: [t.id], itemName: t.name, busy: false }
}

function removeSelectedIDs(current: Set<number>, ids: number[]) {
  const next = new Set(current)
  for (const id of ids) next.delete(id)
  return next
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <h1 class="text-2xl font-bold">{{ i18n.t('模板') }}</h1>
      <div class="flex items-center gap-2 flex-wrap">
        <div class="join bg-base-200 p-0.5 rounded-btn shadow-sm">
          <button
            type="button"
            class="btn btn-sm join-item"
            :class="{ 'btn-active': templateViewMode === 'card' }"
            @click="templateViewMode = 'card'"
          >
            <Squares2X2Icon class="h-4 w-4" /> {{ i18n.t('卡片') }}
          </button>
          <button
            type="button"
            class="btn btn-sm join-item"
            :class="{ 'btn-active': templateViewMode === 'list' }"
            @click="templateViewMode = 'list'"
          >
            <ListBulletIcon class="h-4 w-4" /> {{ i18n.t('列表') }}
          </button>
        </div>
        <button class="btn btn-sm btn-primary" @click="openImport"><PlusIcon class="h-4 w-4" /> {{ i18n.t('导入模板') }}</button>
      </div>
    </div>

    <div v-if="store.templates.length" class="flex items-center justify-between gap-2 flex-wrap">
      <div class="flex items-center gap-2">
        <span class="badge badge-neutral">{{ store.templates.length }}</span>
        <span v-if="selectedTemplates.size" class="badge badge-outline">{{ i18n.t('已选') }} {{ selectedTemplates.size }}</span>
      </div>
      <div class="flex items-center gap-2 flex-wrap">
        <button
          class="btn btn-sm"
          :class="{ 'btn-active': allTemplatesSelected }"
          @click="selectAllTemplates"
          :disabled="!selectableTemplates.length"
        >
          {{ allTemplatesSelected ? i18n.t('取消全选') : i18n.t('全选') }}
        </button>
        <button class="btn btn-sm text-error bg-error/10 hover:bg-error/20 border-transparent" @click="removeSelectedTemplates" :disabled="busy || !selectedTemplates.size">
          <TrashIcon class="h-4 w-4" /> {{ i18n.t('删除') }}
        </button>
      </div>
    </div>

    <div v-if="store.loading && !store.templates.length" class="flex justify-center py-10"><span class="loading loading-spinner loading-lg"></span></div>
    <div v-else-if="!store.templates.length" class="text-center py-10 opacity-60 bg-base-100 border border-base-300 rounded-box">
      {{ i18n.t('暂无模板。') }}
    </div>
    <div v-else-if="templateViewMode === 'card'" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      <div
        v-for="t in sortedTemplates"
        :key="t.id"
        v-memo="[t.id, t.name, t.description, t.kind, t.updated_at, selectedTemplates.has(t.id), i18n.locale]"
        class="card bg-base-100 border border-base-300 shadow-sm transition-colors"
        :class="[
          t.kind === 'user' ? 'cursor-pointer hover:bg-base-200/60' : '',
          selectedTemplates.has(t.id) ? 'ring-2 ring-primary' : '',
        ]"
        :role="t.kind === 'user' ? 'button' : undefined"
        :tabindex="t.kind === 'user' ? 0 : -1"
        @click="toggleTemplateSelect(t)"
        @keydown.enter.prevent="toggleTemplateSelect(t)"
        @keydown.space.prevent="toggleTemplateSelect(t)"
      >
        <div class="card-body p-4 gap-3">
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-start gap-2 min-w-0">
              <input
                v-if="t.kind === 'user'"
                type="checkbox"
                class="checkbox checkbox-sm mt-0.5"
                :checked="selectedTemplates.has(t.id)"
                @click.stop
                @keydown.stop
                @change="toggleTemplateSelect(t)"
              />
              <div class="min-w-0">
                <h2 class="font-semibold truncate" :title="t.name">{{ t.name }}</h2>
                <p v-if="t.description" class="text-xs opacity-70 truncate" :title="t.description">{{ t.description }}</p>
              </div>
            </div>
          </div>
          <div class="grid grid-cols-2 gap-2 text-[11px] opacity-60">
            <div class="truncate" :title="formatDateTime(t.created_at)">{{ i18n.t('创建时间') }}: {{ formatDateTime(t.created_at) }}</div>
            <div class="truncate" :title="formatDateTime(t.updated_at)">{{ i18n.t('修改时间') }}: {{ formatDateTime(t.updated_at) }}</div>
          </div>
          <div class="flex gap-1 justify-end">
            <button type="button" class="btn btn-xs btn-ghost" @pointerenter="prefetchTemplate(t)" @focus="prefetchTemplate(t)" @click.stop="view(t)" :title="i18n.t('查看')"><EyeIcon class="h-4 w-4" /></button>
            <button type="button" class="btn btn-xs btn-ghost" @pointerenter="prefetchStructure(t)" @focus="prefetchStructure(t)" @click.stop="editStructure(t)" :title="i18n.t('分组管理')"><RectangleGroupIcon class="h-4 w-4" /></button>
            <button type="button" class="btn btn-xs btn-ghost" @click.stop="exportTemplate(t)" :title="i18n.t('导出')"><ArrowDownTrayIcon class="h-4 w-4" /></button>
            <button type="button" class="btn btn-xs btn-ghost" @pointerenter="prefetchTemplate(t)" @focus="prefetchTemplate(t)" @click.stop="openCopy(t)" :title="i18n.t('复制模板')"><DocumentDuplicateIcon class="h-4 w-4" /></button>
            <button v-if="t.kind === 'user'" type="button" class="btn btn-xs btn-ghost" @pointerenter="prefetchTemplate(t)" @focus="prefetchTemplate(t)" @click.stop="openEdit(t)" :title="i18n.t('编辑模板')"><PencilSquareIcon class="h-4 w-4" /></button>
            <button v-if="t.kind === 'user'" type="button" class="btn btn-xs btn-ghost text-error" @click.stop="remove(t)" :title="i18n.t('删除')"><TrashIcon class="h-4 w-4" /></button>
          </div>
        </div>
      </div>
    </div>
    <div v-else class="overflow-x-auto card bg-base-100 shadow-sm">
      <table class="table">
        <thead>
          <tr>
            <th class="w-10"></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleTemplateSort('name')">{{ i18n.t('名称') }} {{ sortIndicator(templateSortKey, templateSortDir, 'name') }}</button></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleTemplateSort('description')">{{ i18n.t('描述') }} {{ sortIndicator(templateSortKey, templateSortDir, 'description') }}</button></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleTemplateSort('created_at')">{{ i18n.t('创建时间') }} {{ sortIndicator(templateSortKey, templateSortDir, 'created_at') }}</button></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleTemplateSort('updated_at')">{{ i18n.t('修改时间') }} {{ sortIndicator(templateSortKey, templateSortDir, 'updated_at') }}</button></th>
            <th class="text-right">{{ i18n.t('操作') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="t in sortedTemplates"
            :key="t.id"
            v-memo="[t.id, t.name, t.description, t.kind, t.updated_at, selectedTemplates.has(t.id), i18n.locale]"
            :class="[
              t.kind === 'user' ? 'cursor-pointer hover:bg-base-200/70' : '',
              selectedTemplates.has(t.id) ? 'bg-base-200' : '',
            ]"
            @click="toggleTemplateSelect(t)"
          >
            <td>
              <input
                v-if="t.kind === 'user'"
                type="checkbox"
                class="checkbox checkbox-sm"
                :checked="selectedTemplates.has(t.id)"
                @click.stop
                @change="toggleTemplateSelect(t)"
              />
            </td>
            <td class="font-semibold">{{ t.name }}</td>
            <td class="text-sm opacity-70 max-w-xs truncate">{{ t.description }}</td>
            <td class="whitespace-nowrap text-xs opacity-70" :title="t.created_at">{{ formatDateTime(t.created_at) }}</td>
            <td class="whitespace-nowrap text-xs opacity-70" :title="t.updated_at">{{ formatDateTime(t.updated_at) }}</td>
            <td>
              <div class="flex gap-1 justify-end">
                <button type="button" class="btn btn-xs btn-ghost" @pointerenter="prefetchTemplate(t)" @focus="prefetchTemplate(t)" @click.stop="view(t)" :title="i18n.t('查看')"><EyeIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost" @pointerenter="prefetchStructure(t)" @focus="prefetchStructure(t)" @click.stop="editStructure(t)" :title="i18n.t('分组管理')"><RectangleGroupIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost" @click.stop="exportTemplate(t)" :title="i18n.t('导出')"><ArrowDownTrayIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost" @pointerenter="prefetchTemplate(t)" @focus="prefetchTemplate(t)" @click.stop="openCopy(t)" :title="i18n.t('复制模板')"><DocumentDuplicateIcon class="h-4 w-4" /></button>
                <button v-if="t.kind === 'user'" type="button" class="btn btn-xs btn-ghost" @pointerenter="prefetchTemplate(t)" @focus="prefetchTemplate(t)" @click.stop="openEdit(t)" :title="i18n.t('编辑模板')"><PencilSquareIcon class="h-4 w-4" /></button>
                <button v-if="t.kind === 'user'" type="button" class="btn btn-xs btn-ghost text-error" @click.stop="remove(t)" :title="i18n.t('删除')"><TrashIcon class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <BulkDeleteDialog
      :open="bulkDeleteDialog.open"
      :title="i18n.t('删除模板')"
      :item-label="i18n.t('个模板')"
      :count="bulkDeleteDialog.ids.length"
      :item-name="bulkDeleteDialog.itemName"
      :busy="bulkDeleteDialog.busy"
      @close="closeBulkDeleteDialog"
      @confirm="confirmBulkDelete"
    />

    <div v-if="viewing" class="modal modal-open">
      <div class="modal-box max-w-3xl">
        <h3 class="font-bold text-lg mb-3">{{ viewing.name }}</h3>
        <JsonViewer :content="viewing.content" />
        <div class="modal-action"><button class="btn" @click="viewing = null">{{ i18n.t('关闭') }}</button></div>
      </div>
      <div class="modal-backdrop" @click="viewing = null"></div>
    </div>

    <div v-if="structureFor" class="modal modal-open">
      <div class="modal-box max-w-5xl">
        <div class="flex items-center justify-between gap-2 mb-3">
          <h3 class="font-bold text-lg">{{ i18n.t('分组管理') }} · {{ structureFor?.name }}</h3>
        </div>
        <div v-if="structureLoading && !structure" class="alert py-2 mb-3">
          <span class="loading loading-spinner loading-sm"></span>
          <span class="text-sm">{{ i18n.t('正在加载模板分组...') }}</span>
        </div>
        <template v-if="structure">
          <label class="form-control max-w-sm mb-4">
            <span class="label-text mb-1">{{ i18n.t('最终出口') }}</span>
            <select v-model="structure.final" class="select select-bordered select-sm">
              <option value="">{{ i18n.t('使用 sing-box 默认') }}</option>
              <option v-for="tag in finalOutboundOptions" :key="tag" :value="tag">{{ tag }}</option>
            </select>
          </label>
          <div v-if="!structure.groups.length" class="opacity-60 text-sm">{{ i18n.t('未检测到 selector/urltest 分组。') }}</div>
          <div
            v-else
            class="sort-list flex flex-col gap-3 max-h-[62vh] overflow-y-auto pr-1"
            @dragover.prevent="onDragOver"
            @drop.prevent="onDrop"
            @dragleave="leaveGroupList"
          >
            <div
              v-for="(g, i) in structure.groups"
              :key="g.tag + ':' + i"
              class="sort-item border border-base-300 border-y-2 rounded-box bg-base-100 p-3 hover:bg-base-200/50"
              :class="{
                'is-pressed': groupPressedIndex === i && dragIndex === null,
                'is-dragging ring-1 ring-base-content/30': dragIndex === i,
                'is-insert-before': groupInsertIndex === i,
                'is-insert-after': groupInsertIndex === i + 1,
              }"
              :data-group-index="i"
              draggable="true"
              @pointerdown="pressGroup(i, $event)"
              @pointerup="clearGroupPress"
              @pointercancel="clearGroupPress"
              @pointerleave="clearGroupPress"
              @dragstart="onDragStart(i, $event)"
              @dragend="clearGroupDrag"
            >
              <div class="grid grid-cols-1 lg:grid-cols-[32px_minmax(160px,1fr)_120px_minmax(180px,1fr)_160px_64px] gap-2 items-start">
                <div class="flex items-center gap-1">
                  <span
                    class="grid h-7 w-7 place-items-center text-base-content/60"
                    :title="i18n.t('拖拽排序')"
                  >
                    <Bars3Icon class="h-4 w-4" />
                  </span>
                </div>
                <label class="form-control">
                  <span class="label-text mb-1">{{ i18n.t('标签') }}</span>
                  <input v-model="g.tag" class="input input-bordered input-sm" disabled />
                </label>
                <label class="form-control">
                  <span class="label-text mb-1">{{ i18n.t('类型') }}</span>
                  <select v-model="g.type" class="select select-bordered select-sm" @change="onGroupTypeChange(g)">
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
                <label v-if="groupSupportsDefault(g)" class="form-control">
                  <span class="label-text mb-1">{{ i18n.t('默认出口') }}</span>
                  <select v-model="g.default" class="select select-bordered select-sm" :disabled="g.outbounds.length === 0">
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
        </template>
        <div class="modal-action">
          <button class="btn" @click="closeStructure" :disabled="busy">{{ i18n.t('取消') }}</button>
          <button class="btn btn-primary" @click="saveStructure" :disabled="busy || structureLoading || !structure">
            <span v-if="busy || structureLoading" class="loading loading-spinner loading-sm"></span> {{ i18n.t('保存') }}
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="closeStructure"></div>
    </div>

    <div v-if="showForm" class="modal modal-open">
      <div class="modal-box max-w-2xl">
        <h3 class="font-bold text-lg mb-3">{{ formTitle }}</h3>
        <div v-if="formLoading" class="alert py-2 mb-3">
          <span class="loading loading-spinner loading-sm"></span>
          <span class="text-sm">{{ i18n.t('正在加载模板...') }}</span>
        </div>
        <fieldset class="flex flex-col gap-3" :disabled="formLoading" :class="{ 'opacity-60': formLoading }">
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
        </fieldset>
        <div class="modal-action">
          <button class="btn" @click="closeForm" :disabled="busy">{{ i18n.t('取消') }}</button>
          <button class="btn btn-primary" @click="submitForm" :disabled="busy || formLoading">
            <span v-if="busy || formLoading" class="loading loading-spinner loading-sm"></span> {{ i18n.t('保存') }}
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="closeForm"></div>
    </div>
  </div>
</template>
