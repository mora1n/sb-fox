<script setup lang="ts">
import { computed, ref } from 'vue'
import type { NodeSummary } from '../api/types'
import CountryFlag from './CountryFlag.vue'
import VirtualNodeList from './VirtualNodeList.vue'
import { useI18nStore } from '../stores/i18n'
import { useSettingsStore } from '../stores/settings'
import { nodeSourceLabel } from '../utils/nodeSource'
import { emptyNodeFilters, filterNodes, nodeCountries, nodeSources, nodeTypes } from '../utils/nodeFilters'
import { Bars3Icon, XMarkIcon } from '@heroicons/vue/24/outline'

const props = defineProps<{ nodes: NodeSummary[]; modelValue: number[]; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [number[]] }>()
const i18n = useI18nStore()
const settings = useSettingsStore()

const filters = ref(emptyNodeFilters())
const dragIndex = ref<number | null>(null)
const pressedIndex = ref<number | null>(null)
const insertIndex = ref<number | null>(null)
const sourceOptions = computed(() => nodeSources(props.nodes))
const countryOptions = computed(() => nodeCountries(props.nodes, settings.countryHeatOrder))
const typeOptions = computed(() => nodeTypes(props.nodes))
const filtered = computed(() => filterNodes(props.nodes, filters.value))
const nodesByID = computed(() => new Map(props.nodes.map((node) => [node.id, node])))
const selectedItems = computed(() => {
  return props.modelValue.map((id) => ({ id, node: nodesByID.value.get(id) }))
})

function toggle(id: number) {
  if (props.disabled) return
  if (props.modelValue.includes(id)) emit('update:modelValue', props.modelValue.filter((item) => item !== id))
  else emit('update:modelValue', uniqueIDs([...props.modelValue, id]))
}
function selectAllFiltered() {
  if (props.disabled) return
  emit('update:modelValue', uniqueIDs([...props.modelValue, ...filtered.value.map((n) => n.id)]))
}
function clearAll() {
  if (props.disabled) return
  emit('update:modelValue', [])
}
function removeSelected(id: number) {
  if (props.disabled) return
  emit('update:modelValue', props.modelValue.filter((item) => item !== id))
}
function uniqueIDs(ids: number[]) {
  const seen = new Set<number>()
  return ids.filter((id) => {
    if (!id || seen.has(id)) return false
    seen.add(id)
    return true
  })
}
function selectedInsertTarget(event: DragEvent) {
  const list = event.currentTarget as HTMLElement | null
  if (!list) return selectedItems.value.length
  const rows = Array.from(list.querySelectorAll<HTMLElement>('[data-node-index]'))
  for (const row of rows) {
    const index = Number(row.dataset.nodeIndex)
    const rect = row.getBoundingClientRect()
    if (event.clientY < rect.top + rect.height / 2) return index
  }
  return selectedItems.value.length
}
function clearSelectedDrag() {
  dragIndex.value = null
  pressedIndex.value = null
  insertIndex.value = null
}
function clearSelectedPress() {
  pressedIndex.value = null
}
function leaveSelectedList(event: DragEvent) {
  const list = event.currentTarget as HTMLElement | null
  if (!list) return
  const rect = list.getBoundingClientRect()
  const outside =
    event.clientX < rect.left ||
    event.clientX > rect.right ||
    event.clientY < rect.top ||
    event.clientY > rect.bottom
  if (outside) insertIndex.value = null
}
function isControlDragTarget(event: DragEvent | PointerEvent) {
  const target = event.target as HTMLElement | null
  return !!target?.closest('button,input,select,textarea,a,[contenteditable="true"]')
}
function pressSelected(index: number, event: PointerEvent) {
  if (props.disabled || isControlDragTarget(event)) return
  pressedIndex.value = index
}
function startSelectedDrag(index: number, event: DragEvent) {
  if (props.disabled || isControlDragTarget(event)) {
    event.preventDefault()
    pressedIndex.value = null
    return
  }
  dragIndex.value = index
  pressedIndex.value = index
  insertIndex.value = null
  event.dataTransfer?.setData('text/plain', String(index))
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}
function overSelectedList(event: DragEvent) {
  if (dragIndex.value === null) {
    insertIndex.value = null
    return
  }
  const target = selectedInsertTarget(event)
  insertIndex.value = target === dragIndex.value || target === dragIndex.value + 1 ? null : target
}
function dropSelected(event: DragEvent) {
  if (dragIndex.value === null) {
    clearSelectedDrag()
    return
  }
  const source = dragIndex.value
  const target = insertIndex.value ?? selectedInsertTarget(event)
  if (target === source || target === source + 1) {
    clearSelectedDrag()
    return
  }
  const next = [...props.modelValue]
  const [item] = next.splice(source, 1)
  next.splice(target > source ? target - 1 : target, 0, item)
  emit('update:modelValue', uniqueIDs(next))
  clearSelectedDrag()
}
</script>

<template>
  <div class="flex flex-col gap-2" :class="{ 'opacity-60': disabled }">
    <div class="flex items-center gap-2 flex-wrap">
      <input v-model="filters.search" class="input input-bordered input-sm min-w-0 flex-1" :placeholder="i18n.t('搜索节点...')" :disabled="disabled" />
      <button type="button" class="btn btn-xs btn-soft-action min-h-7 h-7 shrink-0" @click="selectAllFiltered" :disabled="disabled">{{ i18n.t('全选') }}</button>
      <button type="button" class="btn btn-xs btn-soft-action min-h-7 h-7 shrink-0" @click="clearAll" :disabled="disabled">{{ i18n.t('清空') }}</button>
      <span class="badge badge-primary shrink-0">{{ modelValue.length }}</span>
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
      <select v-model="filters.source" class="select select-bordered select-sm min-w-0" :disabled="disabled">
        <option value="">{{ i18n.t('全部来源') }}</option>
        <option v-for="source in sourceOptions" :key="source" :value="source">
          {{ i18n.t(nodeSourceLabel(source)) }}
        </option>
      </select>
      <select v-model="filters.country" class="select select-bordered select-sm min-w-0" :disabled="disabled">
        <option value="">{{ i18n.t('全部国家') }}</option>
        <option v-for="country in countryOptions" :key="country" :value="country">{{ country }}</option>
      </select>
      <select v-model="filters.type" class="select select-bordered select-sm min-w-0" :disabled="disabled">
        <option value="">{{ i18n.t('全部协议') }}</option>
        <option v-for="type in typeOptions" :key="type" :value="type">{{ type }}</option>
      </select>
    </div>
    <div v-if="selectedItems.length" class="flex flex-col gap-1">
      <div class="label-text">{{ i18n.t('已选节点') }}</div>
      <div
        class="sort-list border border-base-300 rounded-box max-h-44 overflow-y-auto divide-y divide-base-200"
        @dragover.prevent="overSelectedList"
        @drop.prevent="dropSelected"
        @dragleave="leaveSelectedList"
      >
        <div
          v-for="(item, index) in selectedItems"
          :key="item.id"
          class="sort-item flex items-center gap-2 bg-base-100 px-3 py-1.5 border-y-2 border-transparent select-none hover:bg-base-200/60"
          :class="{
            'is-pressed': pressedIndex === index && dragIndex === null,
            'is-dragging ring-1 ring-base-content/30': dragIndex === index,
            'is-insert-before': insertIndex === index,
            'is-insert-after': insertIndex === index + 1,
          }"
          :data-node-index="index"
          :draggable="!disabled"
          @pointerdown="pressSelected(index, $event)"
          @pointerup="clearSelectedPress"
          @pointercancel="clearSelectedPress"
          @pointerleave="clearSelectedPress"
          @dragstart="startSelectedDrag(index, $event)"
          @dragend="clearSelectedDrag"
        >
          <span class="grid h-7 w-7 place-items-center text-base-content/60" :title="i18n.t('拖拽排序')">
            <Bars3Icon class="h-4 w-4" />
          </span>
          <span class="badge badge-sm w-8">{{ index + 1 }}</span>
          <CountryFlag v-if="item.node?.country_code" :code="item.node.country_code" :show-code="false" />
          <span class="truncate flex-1 text-sm" :title="item.node?.tag || '#' + item.id">{{ item.node?.tag || '#' + item.id }}</span>
          <span v-if="item.node" class="badge badge-ghost badge-sm">{{ item.node.type }}</span>
          <button
            type="button"
            class="btn btn-xs btn-ghost btn-square"
            :title="i18n.t('删除')"
            :disabled="disabled"
            @click="removeSelected(item.id)"
          >
            <XMarkIcon class="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
    <div class="label-text">{{ i18n.t('节点列表') }}</div>
    <VirtualNodeList :nodes="filtered" :selected-ids="modelValue" :disabled="disabled" @toggle="toggle" />
  </div>
</template>
