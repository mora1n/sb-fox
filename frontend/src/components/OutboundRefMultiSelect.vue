<script setup lang="ts">
import { computed, ref } from 'vue'
import { Bars3Icon, XMarkIcon } from '@heroicons/vue/24/outline'
import { useI18nStore } from '../stores/i18n'

const props = defineProps<{ options: string[]; modelValue: string[]; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [string[]] }>()
const i18n = useI18nStore()

const dragIndex = ref<number | null>(null)
const pressedIndex = ref<number | null>(null)
const insertIndex = ref<number | null>(null)
const availableOptions = computed(() => uniqueStrings(props.options))
const selectedSet = computed(() => new Set(props.modelValue))

function uniqueStrings(values: string[]) {
  const seen = new Set<string>()
  return values.filter((value) => {
    if (!value || seen.has(value)) return false
    seen.add(value)
    return true
  })
}

function toggle(tag: string) {
  if (props.disabled) return
  if (selectedSet.value.has(tag)) emit('update:modelValue', props.modelValue.filter((item) => item !== tag))
  else emit('update:modelValue', uniqueStrings([...props.modelValue, tag]))
}

function selectAll() {
  if (props.disabled) return
  emit('update:modelValue', uniqueStrings([...props.modelValue, ...availableOptions.value]))
}

function clearAll() {
  if (props.disabled) return
  emit('update:modelValue', [])
}

function removeSelected(tag: string) {
  if (props.disabled) return
  emit('update:modelValue', props.modelValue.filter((item) => item !== tag))
}

function selectedInsertTarget(event: DragEvent) {
  const list = event.currentTarget as HTMLElement | null
  if (!list) return props.modelValue.length
  const rows = Array.from(list.querySelectorAll<HTMLElement>('[data-outbound-ref-index]'))
  for (const row of rows) {
    const index = Number(row.dataset.outboundRefIndex)
    const rect = row.getBoundingClientRect()
    if (event.clientY < rect.top + rect.height / 2) return index
  }
  return props.modelValue.length
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
  emit('update:modelValue', uniqueStrings(next))
  clearSelectedDrag()
}
</script>

<template>
  <div class="flex flex-col gap-2" :class="{ 'opacity-60': disabled }">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <span class="label-text">{{ i18n.t('引用出口') }}</span>
      <span class="flex items-center gap-1 flex-wrap">
        <button class="btn btn-xs min-h-7 h-7" type="button" @click="selectAll" :disabled="disabled || !availableOptions.length">
          {{ i18n.t('全选') }}
        </button>
        <button class="btn btn-xs min-h-7 h-7" type="button" @click="clearAll" :disabled="disabled || !modelValue.length">
          {{ i18n.t('全不选') }}
        </button>
        <span class="badge badge-primary shrink-0">{{ modelValue.length }}</span>
      </span>
    </div>

    <div v-if="modelValue.length" class="flex flex-col gap-1">
      <div class="label-text">{{ i18n.t('已选出口') }}</div>
      <div
        class="sort-list border border-base-300 rounded-box max-h-32 overflow-y-auto divide-y divide-base-200"
        @dragover.prevent="overSelectedList"
        @drop.prevent="dropSelected"
        @dragleave="leaveSelectedList"
      >
        <div
          v-for="(tag, index) in modelValue"
          :key="tag"
          class="sort-item flex items-center gap-2 bg-base-100 px-3 py-1.5 border-y-2 border-transparent select-none hover:bg-base-200/60"
          :class="{
            'is-pressed': pressedIndex === index && dragIndex === null,
            'is-dragging ring-1 ring-base-content/30': dragIndex === index,
            'is-insert-before': insertIndex === index,
            'is-insert-after': insertIndex === index + 1,
          }"
          :data-outbound-ref-index="index"
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
          <span class="truncate flex-1 text-sm" :title="tag">{{ tag }}</span>
          <button
            type="button"
            class="btn btn-xs btn-ghost btn-square"
            :title="i18n.t('删除')"
            :disabled="disabled"
            @click="removeSelected(tag)"
          >
            <XMarkIcon class="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>

    <div class="label-text">{{ i18n.t('出口列表') }}</div>
    <div class="border border-base-300 rounded-box max-h-32 overflow-y-auto divide-y divide-base-200">
      <label
        v-for="tag in availableOptions"
        :key="tag"
        class="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-base-200"
        :class="{ 'opacity-60': disabled }"
      >
        <input
          type="checkbox"
          class="checkbox checkbox-sm"
          :checked="selectedSet.has(tag)"
          :disabled="disabled"
          @change="toggle(tag)"
        />
        <span class="truncate flex-1 text-sm" :title="tag">{{ tag }}</span>
      </label>
      <div v-if="!availableOptions.length" class="px-3 py-4 text-sm opacity-60 text-center">
        {{ i18n.t('无可选出口') }}
      </div>
    </div>
  </div>
</template>
