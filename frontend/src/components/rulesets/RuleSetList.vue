<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { RuleSet } from '../../api/types'
import { useRuleSetsStore } from '../../stores/ruleSets'
import { usePublicTokenStore } from '../../stores/publicToken'
import { useSettingsStore } from '../../stores/settings'
import { useI18nStore } from '../../stores/i18n'
import { useUiStore } from '../../stores/ui'
import { errMsg } from '../../utils/error'
import { formatDateTime } from '../../utils/time'
import { readViewPref, writeViewPref } from '../../utils/viewPrefs'
import BulkDeleteDialog from '../BulkDeleteDialog.vue'
import RuleSetLinks from './RuleSetLinks.vue'
import {
  ArrowDownTrayIcon,
  ArrowPathIcon,
  DocumentDuplicateIcon,
  ListBulletIcon,
  PencilSquareIcon,
  PlusIcon,
  Squares2X2Icon,
  TrashIcon,
} from '@heroicons/vue/24/outline'

type ViewMode = 'card' | 'list'
const VIEW_MODES = ['card', 'list'] as const

const emit = defineEmits<{ create: []; edit: [RuleSet]; copy: [RuleSet] }>()
const store = useRuleSetsStore()
const publicToken = usePublicTokenStore()
const settings = useSettingsStore()
const i18n = useI18nStore()
const ui = useUiStore()

const viewMode = ref<ViewMode>(readViewPref('sb-fox-view:rule-sets', 'card', VIEW_MODES))
const selected = ref<Set<number>>(new Set())
const refreshing = ref<Set<number>>(new Set())
const busy = ref(false)
const deleteDialog = ref({ open: false, ids: [] as number[], itemName: '', busy: false })
const sortedItems = computed(() => [...store.ruleSets].sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true })))
const allSelected = computed(() => sortedItems.value.length > 0 && sortedItems.value.every((item) => selected.value.has(item.id)))

watch(viewMode, (value) => writeViewPref('sb-fox-view:rule-sets', value))

function toggle(id: number) {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selected.value = next
}

function toggleAll() {
  selected.value = allSelected.value ? new Set() : new Set(sortedItems.value.map((item) => item.id))
}

function requestDelete(item?: RuleSet) {
  const ids = item ? [item.id] : [...selected.value]
  if (!ids.length) return ui.info(i18n.t('请先选择规则集'))
  deleteDialog.value = { open: true, ids, itemName: item?.name || '', busy: false }
}

function closeDelete() {
  if (deleteDialog.value.busy) return
  deleteDialog.value.open = false
}

async function confirmDelete() {
  if (deleteDialog.value.busy) return
  deleteDialog.value.busy = true
  busy.value = true
  try {
    const count = deleteDialog.value.ids.length === 1
      ? await store.remove(deleteDialog.value.ids[0]).then(() => 1)
      : await store.bulkDelete(deleteDialog.value.ids)
    selected.value = new Set([...selected.value].filter((id) => !deleteDialog.value.ids.includes(id)))
    ui.success(`${i18n.t('已删除')} ${count} ${i18n.t('个规则集')}`)
    deleteDialog.value.open = false
  } catch (error) {
    ui.error(errMsg(error))
  } finally {
    busy.value = false
    deleteDialog.value.busy = false
  }
}

async function refresh(item: RuleSet) {
  if (refreshing.value.has(item.id)) return
  refreshing.value = new Set(refreshing.value).add(item.id)
  try {
    await store.refresh(item.id)
    ui.success(i18n.t('规则集已刷新'))
  } catch (error) {
    ui.error(errMsg(error))
    await store.fetchAll(true).catch(() => undefined)
  } finally {
    const next = new Set(refreshing.value)
    next.delete(item.id)
    refreshing.value = next
  }
}

async function exportArtifact(item: RuleSet, format: 'source' | 'binary') {
  try {
    await store.exportArtifact(item.id, item.name, format)
  } catch (error) {
    ui.error(errMsg(error))
  }
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`
  return `${(value / 1024 / 1024).toFixed(2)} MiB`
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <div>
        <h1 class="text-2xl font-bold">{{ i18n.t('规则集') }}</h1>
      </div>
      <div class="flex items-center gap-2">
        <div class="join bg-base-200 p-0.5 rounded-btn">
          <button class="btn btn-sm join-item" :class="{ 'btn-active': viewMode === 'card' }" @click="viewMode = 'card'"><Squares2X2Icon class="h-4 w-4" />{{ i18n.t('卡片') }}</button>
          <button class="btn btn-sm join-item" :class="{ 'btn-active': viewMode === 'list' }" @click="viewMode = 'list'"><ListBulletIcon class="h-4 w-4" />{{ i18n.t('列表') }}</button>
        </div>
        <button class="btn btn-sm btn-primary" @click="emit('create')"><PlusIcon class="h-4 w-4" />{{ i18n.t('新建规则集') }}</button>
      </div>
    </div>

    <div v-if="store.ruleSets.length" class="flex items-center gap-2">
      <label class="label cursor-pointer gap-2"><input type="checkbox" class="checkbox checkbox-sm" :checked="allSelected" @change="toggleAll" />{{ i18n.t('全选') }}</label>
      <button class="btn btn-sm btn-error btn-outline" :disabled="busy || !selected.size" @click="requestDelete()"><TrashIcon class="h-4 w-4" />{{ i18n.t('删除所选') }}</button>
      <span class="text-xs opacity-60">{{ i18n.t('已选') }} {{ selected.size }}</span>
    </div>

    <div v-if="store.loading && !store.ruleSets.length" class="flex justify-center p-12"><span class="loading loading-spinner loading-lg"></span></div>
    <div v-else-if="!store.ruleSets.length" class="hero min-h-64 bg-base-100 rounded-box border border-base-300">
      <div class="hero-content text-center"><div><h2 class="text-lg font-semibold">{{ i18n.t('暂无规则集。') }}</h2><button class="btn btn-primary btn-sm mt-4" @click="emit('create')">{{ i18n.t('新建规则集') }}</button></div></div>
    </div>

    <div v-else :class="viewMode === 'card' ? 'grid xl:grid-cols-2 gap-4' : 'flex flex-col gap-3'">
      <article v-for="item in sortedItems" :key="item.id" class="card bg-base-100 border border-base-300 shadow-sm">
        <div class="card-body p-4 gap-3">
          <div class="flex items-start gap-3">
            <input type="checkbox" class="checkbox checkbox-sm mt-1" :checked="selected.has(item.id)" @change="toggle(item.id)" />
            <div class="min-w-0 flex-1">
              <h2 class="font-semibold truncate" :title="item.name">{{ item.name }}</h2>
              <p v-if="item.description" class="text-sm opacity-60 line-clamp-2">{{ item.description }}</p>
            </div>
            <div class="flex gap-1 flex-wrap justify-end">
              <button class="btn btn-xs btn-ghost" :title="i18n.t('刷新')" :disabled="refreshing.has(item.id)" @click="refresh(item)"><ArrowPathIcon class="h-4 w-4" :class="{ 'animate-spin': refreshing.has(item.id) }" /></button>
              <button class="btn btn-xs btn-ghost" :title="i18n.t('编辑')" @click="emit('edit', item)"><PencilSquareIcon class="h-4 w-4" /></button>
              <button class="btn btn-xs btn-ghost" :title="i18n.t('复制')" @click="emit('copy', item)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
              <button class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click="requestDelete(item)"><TrashIcon class="h-4 w-4" /></button>
            </div>
          </div>

          <div class="grid grid-cols-2 md:grid-cols-4 gap-2 text-xs">
            <div class="stat bg-base-200 rounded-box p-2"><div class="stat-title text-xs">{{ i18n.t('规则') }}</div><div class="font-semibold">{{ item.rule_count }}</div></div>
            <div class="stat bg-base-200 rounded-box p-2"><div class="stat-title text-xs">{{ i18n.t('规则源') }}</div><div class="font-semibold">{{ item.source_count }}</div></div>
            <div class="stat bg-base-200 rounded-box p-2"><div class="stat-title text-xs">JSON</div><div class="font-semibold">{{ formatBytes(item.json_size) }}</div></div>
            <div class="stat bg-base-200 rounded-box p-2"><div class="stat-title text-xs">SRS</div><div class="font-semibold">{{ formatBytes(item.srs_size) }}</div></div>
          </div>

          <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs opacity-60">
            <span>{{ i18n.t('发布于') }} {{ formatDateTime(item.published_at) }}</span>
            <span>{{ item.kernel_version }}</span>
          </div>
          <div v-if="item.last_error" class="alert alert-error py-2 text-xs"><span class="break-all">{{ item.last_error }}</span></div>

          <RuleSetLinks v-if="publicToken.token" :item="item" :token="publicToken.token" :host-prefix="settings.subscriptionHostPrefix" />
          <div class="flex gap-2 justify-end">
            <button class="btn btn-xs" @click="exportArtifact(item, 'source')"><ArrowDownTrayIcon class="h-4 w-4" />JSON</button>
            <button class="btn btn-xs" @click="exportArtifact(item, 'binary')"><ArrowDownTrayIcon class="h-4 w-4" />SRS</button>
          </div>
        </div>
      </article>
    </div>

    <BulkDeleteDialog
      :open="deleteDialog.open"
      :title="i18n.t('删除规则集')"
      :count="deleteDialog.ids.length"
      :item-name="deleteDialog.itemName"
      :busy="deleteDialog.busy"
      :item-label="i18n.t('规则集')"
      @close="closeDelete"
      @confirm="confirmDelete"
    />
  </div>
</template>
