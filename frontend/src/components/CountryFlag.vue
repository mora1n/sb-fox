<script setup lang="ts">
import { computed } from 'vue'
import { countryFlagEmoji } from '../utils/countries'

const props = defineProps<{ code?: string; showCode?: boolean }>()

const flag = computed(() => countryFlagEmoji(props.code || ''))
</script>

<template>
  <span class="inline-flex items-center gap-1">
    <span class="text-base leading-none">{{ flag }}</span>
    <span v-if="showCode !== false && code" class="text-xs opacity-70">{{ code.toUpperCase() }}</span>
  </span>
</template>
