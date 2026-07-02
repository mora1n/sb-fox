<script setup lang="ts">
import type { Node } from '../api/types'
import CountryFlag from './CountryFlag.vue'
import { PencilSquareIcon, TrashIcon, ArrowsRightLeftIcon } from '@heroicons/vue/24/outline'

defineProps<{ node: Node; selected: boolean }>()
defineEmits<{ edit: []; remove: []; 'toggle-select': [] }>()

const sourceCls: Record<string, string> = {
  protocol: 'badge-primary',
  subscription: 'badge-secondary',
  config: 'badge-accent',
  manual: 'badge-neutral',
}
</script>

<template>
  <div class="card bg-base-100 shadow-sm border border-base-300" :class="{ 'ring-2 ring-primary': selected }">
    <div class="card-body p-4 gap-2">
      <div class="flex items-start justify-between gap-2">
        <div class="flex items-center gap-2 min-w-0">
          <input
            type="checkbox"
            class="checkbox checkbox-sm"
            :checked="selected"
            @change="$emit('toggle-select')"
          />
          <span class="font-semibold truncate" :title="node.tag">{{ node.tag }}</span>
        </div>
        <div class="flex gap-1 flex-none">
          <button class="btn btn-xs btn-ghost" @click="$emit('edit')"><PencilSquareIcon class="h-4 w-4" /></button>
          <button class="btn btn-xs btn-ghost text-error" @click="$emit('remove')"><TrashIcon class="h-4 w-4" /></button>
        </div>
      </div>
      <div class="text-xs opacity-70 truncate mono">{{ node.server }}:{{ node.server_port }}</div>
      <div class="flex flex-wrap items-center gap-1">
        <span class="badge badge-outline badge-sm">{{ node.type }}</span>
        <CountryFlag v-if="node.country_code" :code="node.country_code" />
        <span class="badge badge-sm" :class="sourceCls[node.source] || 'badge-ghost'">{{ node.source }}</span>
        <span v-if="node.has_detour" class="badge badge-sm badge-warning gap-1" :title="'detour: ' + node.detour">
          <ArrowsRightLeftIcon class="h-3 w-3" /> detour
        </span>
        <span v-if="node.country_source === 'manual'" class="badge badge-sm badge-info">手动</span>
      </div>
    </div>
  </div>
</template>
