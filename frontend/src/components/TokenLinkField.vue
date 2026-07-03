<script setup lang="ts">
import { computed } from 'vue'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'

const props = defineProps<{
  token: string
  profileName: string
  hostPrefix?: string
  enabled?: boolean
  showEnabled?: boolean
  disabled?: boolean
}>()
const emit = defineEmits<{ 'update:enabled': [enabled: boolean] }>()
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

function onEnabledChange(event: Event) {
  emit('update:enabled', (event.target as HTMLInputElement).checked)
}
</script>

<template>
  <div class="flex items-center gap-2 min-w-0 w-full">
    <div class="join min-w-0 flex-1">
      <div class="input input-bordered input-sm join-item flex-1 min-w-0 mono text-xs flex items-center" :title="url">
        <span class="truncate">{{ url }}</span>
      </div>
      <button class="btn btn-sm join-item" type="button" @click="copy">{{ i18n.t('复制') }}</button>
    </div>
    <input
      v-if="showEnabled"
      type="checkbox"
      class="toggle toggle-sm"
      :checked="!!enabled"
      :disabled="disabled"
      :title="i18n.t('公开')"
      @change="onEnabledChange"
    />
  </div>
</template>
