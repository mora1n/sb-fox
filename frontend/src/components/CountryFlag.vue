<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ code?: string; showCode?: boolean }>()

// Convert a 2-letter ISO country code to its regional-indicator flag emoji.
const flag = computed(() => {
  const c = (props.code || '').trim().toUpperCase()
  if (!/^[A-Z]{2}$/.test(c)) return '🏳️'
  const base = 0x1f1e6
  return String.fromCodePoint(base + (c.charCodeAt(0) - 65), base + (c.charCodeAt(1) - 65))
})
</script>

<template>
  <span class="inline-flex items-center gap-1">
    <span class="text-base leading-none">{{ flag }}</span>
    <span v-if="showCode !== false && code" class="text-xs opacity-70">{{ code.toUpperCase() }}</span>
  </span>
</template>
