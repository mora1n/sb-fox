<script setup lang="ts">
import { computed } from 'vue'
import { CUSTOM_COUNTRY, countryFlagEmoji } from '../utils/countries'
import { useI18nStore } from '../stores/i18n'

const props = defineProps<{ code?: string; showCode?: boolean }>()
const i18n = useI18nStore()

const flag = computed(() => countryFlagEmoji(props.code || ''))
</script>

<template>
  <span class="inline-flex items-center">
    <span class="text-base leading-none">{{ flag }}</span>
    <span v-if="showCode !== false && code" class="text-xs opacity-70">{{ code.toUpperCase() === CUSTOM_COUNTRY.code ? i18n.t(CUSTOM_COUNTRY.name) : code.toUpperCase() }}</span>
  </span>
</template>
