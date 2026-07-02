<script setup lang="ts">
import type { ValidateStatus } from '../api/types'

defineProps<{ status: ValidateStatus | null; messages?: string }>()

const label: Record<ValidateStatus, string> = {
  ok: '有效',
  invalid: '无效',
  unavailable: '内核不可用',
}
const cls: Record<ValidateStatus, string> = {
  ok: 'badge-success',
  invalid: 'badge-error',
  unavailable: 'badge-warning',
}
</script>

<template>
  <div v-if="status" class="flex flex-col gap-2">
    <span class="badge" :class="cls[status]">{{ label[status] }}</span>
    <pre
      v-if="messages"
      class="mono text-xs whitespace-pre-wrap bg-base-200 rounded p-2 max-h-48 overflow-auto"
      >{{ messages }}</pre
    >
  </div>
</template>
