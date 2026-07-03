<script setup lang="ts">
import { computed } from 'vue'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'

const props = defineProps<{ token: string; profileName: string; hostPrefix?: string }>()
const ui = useUiStore()
const i18n = useI18nStore()

const url = computed(() => {
  const base = (props.hostPrefix?.trim() || window.location.origin).replace(/\/+$/, '')
  return `${base}/sub/${encodeURIComponent(props.token)}/${encodeURIComponent(props.profileName)}`
})

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
    <button class="btn btn-sm join-item" @click="copy">{{ i18n.t('复制') }}</button>
  </div>
</template>
