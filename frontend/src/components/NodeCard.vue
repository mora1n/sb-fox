<script setup lang="ts">
import type { NodeSummary } from '../api/types'
import CountryFlag from './CountryFlag.vue'
import { useI18nStore } from '../stores/i18n'
import { nodeSourceLabel } from '../utils/nodeSource'
import { formatDateTime } from '../utils/time'
import { PencilSquareIcon, TrashIcon, ArrowsRightLeftIcon, DocumentDuplicateIcon } from '@heroicons/vue/24/outline'

defineProps<{ node: NodeSummary; selected: boolean }>()
defineEmits<{ copy: []; edit: []; remove: []; 'toggle-select': [] }>()
const i18n = useI18nStore()

const sourceCls: Record<string, string> = {
  protocol: 'badge-neutral',
  subscription: 'badge-neutral',
  config: 'badge-neutral',
  manual: 'badge-neutral',
}
</script>

<template>
  <div
    class="card bg-base-100 shadow-sm border border-base-300 cursor-pointer transition-colors hover:bg-base-200/60"
    :class="{ 'ring-2 ring-primary': selected }"
    role="button"
    tabindex="0"
    @click="$emit('toggle-select')"
    @keydown.enter.prevent="$emit('toggle-select')"
    @keydown.space.prevent="$emit('toggle-select')"
  >
    <div class="card-body p-4 gap-2">
      <div class="flex items-start justify-between gap-2">
        <div class="flex items-center gap-2 min-w-0">
          <input
            type="checkbox"
            class="checkbox checkbox-sm"
            :checked="selected"
            @click.stop
            @keydown.stop
            @change="$emit('toggle-select')"
          />
          <span class="font-semibold truncate" :title="node.tag">{{ node.tag }}</span>
        </div>
        <div class="flex gap-1 flex-none">
          <button class="btn btn-xs btn-ghost" :title="i18n.t('复制节点')" @click.stop="$emit('copy')" @keydown.stop><DocumentDuplicateIcon class="h-4 w-4" /></button>
          <button class="btn btn-xs btn-ghost" :title="i18n.t('编辑节点')" @click.stop="$emit('edit')" @keydown.stop><PencilSquareIcon class="h-4 w-4" /></button>
          <button class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click.stop="$emit('remove')" @keydown.stop><TrashIcon class="h-4 w-4" /></button>
        </div>
      </div>
      <div class="text-xs opacity-70 truncate mono">{{ node.server }}:{{ node.server_port }}</div>
      <div class="flex flex-wrap items-center gap-1">
        <span class="badge badge-outline badge-sm">{{ node.type }}</span>
        <CountryFlag v-if="node.country_code" :code="node.country_code" />
        <span class="badge badge-sm" :class="sourceCls[node.source] || 'badge-ghost'">{{ i18n.t(nodeSourceLabel(node.source)) }}</span>
        <span v-if="node.has_detour" class="badge badge-sm badge-warning gap-1" :title="'detour: ' + node.detour">
          <ArrowsRightLeftIcon class="h-3 w-3" /> detour
        </span>
        <span v-if="node.country_source === 'manual'" class="badge badge-sm badge-info">手动</span>
      </div>
      <div class="grid grid-cols-2 gap-2 text-[11px] opacity-60">
        <div class="truncate" :title="formatDateTime(node.created_at)">{{ i18n.t('导入时间') }}: {{ formatDateTime(node.created_at) }}</div>
        <div class="truncate" :title="formatDateTime(node.updated_at)">{{ i18n.t('修改时间') }}: {{ formatDateTime(node.updated_at) }}</div>
      </div>
    </div>
  </div>
</template>
