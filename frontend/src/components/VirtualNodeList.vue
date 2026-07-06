<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import type { NodeSummary } from '../api/types'
import CountryFlag from './CountryFlag.vue'
import { useI18nStore } from '../stores/i18n'
import { nodeSourceLabel } from '../utils/nodeSource'

const props = withDefaults(
  defineProps<{
    nodes: NodeSummary[]
    selectedIds: number[]
    disabled?: boolean
    rowHeight?: number
    maxHeight?: number
    overscan?: number
  }>(),
  {
    rowHeight: 40,
    maxHeight: 256,
    overscan: 8,
  },
)
const emit = defineEmits<{ toggle: [number] }>()
const i18n = useI18nStore()

const scrollTop = ref(0)
const scroller = ref<HTMLElement | null>(null)
let scrollFrame = 0

const selectedSet = computed(() => new Set(props.selectedIds))
const totalHeight = computed(() => props.nodes.length * props.rowHeight)
const viewportHeight = computed(() => Math.min(totalHeight.value, props.maxHeight))
const startIndex = computed(() => Math.max(0, Math.floor(scrollTop.value / props.rowHeight) - props.overscan))
const visibleCount = computed(() => Math.ceil(props.maxHeight / props.rowHeight) + props.overscan * 2)
const visibleNodes = computed(() => props.nodes.slice(startIndex.value, startIndex.value + visibleCount.value))

watch(
  () => props.nodes,
  async () => {
    scrollTop.value = 0
    await nextTick()
    if (scroller.value) scroller.value.scrollTop = 0
  },
)

onBeforeUnmount(() => {
  if (scrollFrame) cancelAnimationFrame(scrollFrame)
})

function onScroll(event: Event) {
  const target = event.currentTarget as HTMLElement
  if (scrollFrame) cancelAnimationFrame(scrollFrame)
  scrollFrame = requestAnimationFrame(() => {
    scrollTop.value = target.scrollTop
    scrollFrame = 0
  })
}
</script>

<template>
  <div
    v-if="nodes.length"
    ref="scroller"
    class="border border-base-300 rounded-box overflow-y-auto"
    :style="{ height: `${viewportHeight}px` }"
    @scroll.passive="onScroll"
  >
    <div class="relative" :style="{ height: `${totalHeight}px` }">
      <label
        v-for="(n, offset) in visibleNodes"
        :key="n.id"
        class="absolute left-0 right-0 flex items-center gap-2 border-b border-base-200 px-3 cursor-pointer hover:bg-base-200"
        :style="{
          height: `${rowHeight}px`,
          transform: `translateY(${(startIndex + offset) * rowHeight}px)`,
        }"
      >
        <input
          type="checkbox"
          class="checkbox checkbox-sm"
          :checked="selectedSet.has(n.id)"
          :disabled="disabled"
          @change="emit('toggle', n.id)"
        />
        <CountryFlag v-if="n.country_code" :code="n.country_code" :show-code="false" />
        <span class="truncate flex-1 text-sm">{{ n.tag }}</span>
        <span class="badge badge-ghost badge-sm">{{ n.type }}</span>
        <span class="badge badge-ghost badge-sm hidden sm:inline-flex">{{ i18n.t(nodeSourceLabel(n.source)) }}</span>
      </label>
    </div>
  </div>
  <div v-else class="border border-base-300 rounded-box px-3 py-4 text-sm opacity-60 text-center">
    {{ i18n.t('无匹配节点') }}
  </div>
</template>
