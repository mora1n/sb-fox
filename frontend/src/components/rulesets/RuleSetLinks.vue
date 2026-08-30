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
const sourceSnippet = computed(() => snippet('source', sourceURL.value))
const binarySnippet = computed(() => snippet('binary', binaryURL.value))

function snippet(format: 'source' | 'binary', url: string) {
  return JSON.stringify({
    type: 'remote',
    tag: props.item.name,
    format,
    url,
    update_interval: '24h',
  }, null, 2)
}

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
  <details class="collapse collapse-arrow bg-base-200/60 border border-base-300">
    <summary class="collapse-title min-h-0 py-2 text-sm font-medium">{{ i18n.t('公开下载链接') }}</summary>
    <div class="collapse-content space-y-3">
      <div v-for="entry in [
        { label: 'Source JSON', url: sourceURL, snippet: sourceSnippet },
        { label: 'Binary SRS', url: binaryURL, snippet: binarySnippet },
      ]" :key="entry.label" class="space-y-1">
        <div class="text-xs font-medium">{{ entry.label }}</div>
        <div class="join flex min-w-0">
          <div class="input input-bordered input-sm join-item flex-1 min-w-0 mono text-xs flex items-center" :title="entry.url">
            <span class="truncate">{{ entry.url }}</span>
          </div>
          <button class="btn btn-sm join-item" type="button" @click="copy(entry.url, i18n.t('链接已复制'))">
            {{ i18n.t('复制链接') }}
          </button>
          <button class="btn btn-sm join-item" type="button" @click="copy(entry.snippet, i18n.t('配置片段已复制'))">
            {{ i18n.t('复制配置') }}
          </button>
        </div>
      </div>
    </div>
  </details>
</template>
