<script setup lang="ts">
import { computed } from 'vue'
import { useUiStore } from '../stores/ui'

const props = defineProps<{ content: string; maxHeight?: string }>()
const ui = useUiStore()

// Pretty-print JSON when possible; otherwise show raw text.
const pretty = computed(() => {
  try {
    return JSON.stringify(JSON.parse(props.content), null, 2)
  } catch {
    return props.content
  }
})

async function copy() {
  try {
    await navigator.clipboard.writeText(pretty.value)
    ui.success('已复制到剪贴板')
  } catch {
    ui.error('复制失败')
  }
}
</script>

<template>
  <div class="relative">
    <button class="btn btn-xs btn-ghost absolute right-2 top-2 z-10" @click="copy">复制</button>
    <pre
      class="mono text-xs whitespace-pre bg-base-200 rounded p-3 overflow-auto"
      :style="{ maxHeight: maxHeight || '60vh' }"
      >{{ pretty }}</pre
    >
  </div>
</template>
