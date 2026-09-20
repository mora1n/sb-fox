<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useSourcesStore } from '../stores/sources'
import { useNodesStore } from '../stores/nodes'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import { formatDateTime } from '../utils/time'

const emit = defineEmits<{ close: []; changed: [] }>()
const sourcesStore = useSourcesStore()
const nodesStore = useNodesStore()
const ui = useUiStore()
const i18n = useI18nStore()
const busy = ref<number | null>(null)
const saving = ref<number | null>(null)
const selected = ref<Set<number>>(new Set())
const deleting = ref(false)
const drafts = ref<Record<number, { auto: boolean; value: number; unit: 'days' | 'hours' | 'minutes' }>>({})
const allSelected = computed(() => sourcesStore.sources.length > 0 && sourcesStore.sources.every((source) => selected.value.has(source.id)))

let timer: ReturnType<typeof setInterval> | null = null
async function refreshList() {
  try {
    await sourcesStore.fetchAll(true)
    selected.value = new Set([...selected.value].filter((id) => sourcesStore.sources.some((source) => source.id === id)))
  } catch (e) { ui.error(errMsg(e)) }
}
function onVisibilityChange() {
  if (document.visibilityState === 'visible') void refreshList()
}

onMounted(async () => {
  try { await sourcesStore.fetchAll(true); syncDrafts() } catch (e) { ui.error(errMsg(e)) }
  timer = setInterval(() => void refreshList(), 30000)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisibilityChange)
})

function syncDrafts() {
  const next: typeof drafts.value = {}
  for (const source of sourcesStore.sources) {
    const minutes = source.refresh_interval_minutes || 1440
    const unit = minutes % 1440 === 0 ? 'days' : minutes % 60 === 0 ? 'hours' : 'minutes'
    const divisor = unit === 'days' ? 1440 : unit === 'hours' ? 60 : 1
    next[source.id] = { auto: source.auto_refresh, value: minutes / divisor, unit }
  }
  drafts.value = next
}

function toggleAll() {
  selected.value = allSelected.value ? new Set() : new Set(sourcesStore.sources.map((source) => source.id))
}

async function removeSelected() {
  const ids = [...selected.value]
  if (!ids.length || deleting.value) return
  if (!confirm(`确认删除选中的 ${ids.length} 个订阅源？`)) return
  deleting.value = true
  try {
    for (const id of ids) await sourcesStore.remove(id)
    selected.value = new Set()
    await nodesStore.fetchAll(true)
    ui.success('订阅源已删除')
    emit('changed')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    await refreshList()
    deleting.value = false
  }
}

function intervalMinutes(id: number): number {
  const draft = drafts.value[id]
  const multiplier = draft.unit === 'days' ? 1440 : draft.unit === 'hours' ? 60 : 1
  return Math.max(1, Math.round((Number(draft.value) || 1) * multiplier))
}

async function save(sourceId: number) {
  const draft = drafts.value[sourceId]
  if (!draft) return
  saving.value = sourceId
  try {
    await sourcesStore.updateSchedule(sourceId, draft.auto, intervalMinutes(sourceId))
    ui.success(i18n.t('刷新设置已保存'))
  } catch (e) { ui.error(errMsg(e)) } finally { saving.value = null }
}

async function refresh(sourceId: number) {
  busy.value = sourceId
  try {
    await sourcesStore.refresh(sourceId)
    await nodesStore.fetchAll(true)
    ui.success(i18n.t('订阅已刷新'))
    emit('changed')
  } catch (e) { ui.error(errMsg(e, i18n.t('订阅刷新失败'))) } finally { busy.value = null }
}

function sourceHost(url: string): string {
  try { return new URL(url.split('\n')[0]).host } catch { return url.split('\n')[0].slice(0, 48) }
}
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-4xl">
      <div class="flex items-center justify-between gap-3 mb-4">
        <h3 class="font-bold text-lg">{{ i18n.t('订阅源') }}</h3>
        <div class="flex items-center gap-2">
          <button type="button" class="btn btn-sm btn-ghost" :disabled="!sourcesStore.sources.length || deleting" @click="toggleAll">
            {{ allSelected ? '取消全选' : '全选' }}
          </button>
          <button type="button" class="btn btn-sm btn-error" :disabled="!selected.size || deleting" @click="removeSelected">
            <span v-if="deleting" class="loading loading-spinner loading-xs"></span>{{ i18n.t('删除') }}<span v-if="selected.size">（{{ selected.size }}）</span>
          </button>
          <button type="button" class="btn btn-sm btn-ghost" @click="emit('close')">{{ i18n.t('关闭') }}</button>
        </div>
      </div>
      <div v-if="sourcesStore.loading && !sourcesStore.sources.length" class="flex justify-center py-8">
        <span class="loading loading-spinner loading-lg"></span>
      </div>
      <div v-else-if="!sourcesStore.sources.length" class="py-8 text-center opacity-60">{{ i18n.t('暂无订阅源') }}</div>
      <div v-else class="flex flex-col gap-3">
        <div v-for="source in sourcesStore.sources" :key="source.id" class="rounded-box border border-base-300 p-3 flex flex-col gap-3">
          <div class="flex items-start justify-between gap-3 flex-wrap">
            <div class="min-w-0 flex items-start gap-2">
              <input v-model="selected" type="checkbox" class="checkbox checkbox-sm mt-1" :value="source.id" :disabled="deleting" />
              <div class="min-w-0">
                <div class="font-semibold truncate">{{ source.name || i18n.t('未命名订阅') }}</div>
                <div class="text-xs opacity-60 truncate">{{ sourceHost(source.url) }} · {{ source.node_count }} {{ i18n.t('个节点') }}</div>
                <div class="text-xs opacity-60">{{ i18n.t('上次刷新') }}: {{ source.last_fetch_at ? formatDateTime(source.last_fetch_at) : '-' }} · {{ source.last_status || i18n.t('未刷新') }}</div>
                <div class="text-xs opacity-60">{{ i18n.t('下次刷新') }}: {{ source.next_refresh_at ? formatDateTime(source.next_refresh_at) : '-' }}</div>
              </div>
            </div>
            <button type="button" class="btn btn-sm btn-outline" :disabled="busy === source.id" @click="refresh(source.id)">
              <span v-if="busy === source.id" class="loading loading-spinner loading-xs"></span>{{ i18n.t('立即刷新') }}
            </button>
          </div>
          <div v-if="drafts[source.id]" class="flex items-end gap-2 flex-wrap">
            <label class="label cursor-pointer justify-start gap-2 mr-auto">
              <input v-model="drafts[source.id].auto" type="checkbox" class="toggle toggle-sm" />
              <span class="label-text">{{ i18n.t('自动刷新') }}</span>
            </label>
            <input v-model.number="drafts[source.id].value" type="number" min="1" step="1" class="input input-bordered input-sm w-28" :disabled="!drafts[source.id].auto" />
            <select v-model="drafts[source.id].unit" class="select select-bordered select-sm" :disabled="!drafts[source.id].auto">
              <option value="days">{{ i18n.t('天') }}</option>
              <option value="hours">{{ i18n.t('小时') }}</option>
              <option value="minutes">{{ i18n.t('分钟') }}</option>
            </select>
            <button type="button" class="btn btn-sm btn-primary" :disabled="saving === source.id" @click="save(source.id)">
              <span v-if="saving === source.id" class="loading loading-spinner loading-xs"></span>{{ i18n.t('保存') }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div class="modal-backdrop" @click="emit('close')"></div>
  </div>
</template>
