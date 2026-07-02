<script setup lang="ts">
import { useUiStore } from '../stores/ui'

const ui = useUiStore()

const cls: Record<string, string> = {
  success: 'alert-success',
  error: 'alert-error',
  info: 'alert-info',
  warning: 'alert-warning',
}
</script>

<template>
  <div class="toast toast-end z-50">
    <div v-for="t in ui.toasts" :key="t.id" class="alert" :class="cls[t.kind]">
      <span>{{ t.message }}</span>
      <button class="btn btn-xs btn-ghost" @click="ui.dismiss(t.id)">✕</button>
    </div>
  </div>
</template>
