<script setup lang="ts">
import { computed } from 'vue'
import type { RuleSet } from '../../api/types'
import { useI18nStore } from '../../stores/i18n'
import { useUiStore } from '../../stores/ui'

const props = defineProps<{ item: RuleSet; token: string; hostPrefix?: string }>()
const i18n = useI18nStore()
const ui = useUiStore()

const base = computed(() => (props.hostPrefix?.trim() || window.location.origin).replace(/\/+$/, ''))
const root = computed(() => `${base.value}/rules/${encodeURIComponent(props.token)}/${encodeURIComponent(props.item.name)}`)
const sourceURL = computed(() => root.value + '.json')
const binaryURL = computed(() => root.value + '.srs')

async function copy(value: string, message: string) {
  try {
    await navigator.clipboard.writeText(value)
    ui.success(message)
  } catch {
    ui.error(i18n.t('复制失败'))
  }
}
</script>

<template>
  <div class="space-y-2">
    <div v-for="entry in [
      { label: 'JSON', url: sourceURL },
      { label: 'SRS', url: binaryURL },
    ]" :key="entry.label" class="flex items-center gap-2 min-w-0">
      <span class="badge badge-ghost badge-sm w-12 shrink-0">{{ entry.label }}</span>
      <div class="join flex min-w-0 flex-1">
        <div class="input input-bordered input-xs join-item flex-1 min-w-0 mono text-xs flex items-center" :title="entry.url">
          <span class="truncate">{{ entry.url }}</span>
        </div>
        <button class="btn btn-xs join-item" type="button" @click="copy(entry.url, i18n.t('链接已复制'))">
          {{ i18n.t('复制链接') }}
        </button>
      </div>
    </div>
  </div>
</template>
