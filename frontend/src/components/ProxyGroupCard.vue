<script setup lang="ts">
import type { ProxyGroup } from '../api/types'
import { useI18nStore } from '../stores/i18n'

defineProps<{ group: ProxyGroup }>()
const i18n = useI18nStore()
</script>

<template>
  <div class="card bg-base-200 shadow-sm">
    <div class="card-body p-4 gap-2">
      <div class="flex items-center justify-between">
        <span class="font-semibold">{{ group.tag }}</span>
        <span class="badge" :class="group.type === 'urltest' ? 'badge-info' : 'badge-neutral'">
          {{ group.type }}
        </span>
      </div>
      <div class="flex flex-wrap gap-1">
        <span
          v-for="(o, i) in group.outbounds"
          :key="i"
          class="badge badge-outline badge-sm"
          >{{ o }}</span
        >
        <span v-if="!group.outbounds.length" class="text-xs opacity-60">{{ i18n.t('无出口') }}</span>
      </div>
    </div>
  </div>
</template>
