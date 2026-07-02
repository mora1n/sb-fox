<script setup lang="ts">
import { computed } from 'vue'
import { useUiStore } from '../stores/ui'

const props = defineProps<{ token: string }>()
const ui = useUiStore()

// Full public subscription URL built from the current origin.
const url = computed(() => `${window.location.origin}/sub/${props.token}`)

async function copy() {
  try {
    await navigator.clipboard.writeText(url.value)
    ui.success('订阅链接已复制')
  } catch {
    ui.error('复制失败')
  }
}
</script>

<template>
  <div class="join w-full">
    <input class="input input-bordered input-sm join-item flex-1 mono text-xs" :value="url" readonly />
    <button class="btn btn-sm join-item" @click="copy">复制</button>
  </div>
</template>
