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
  EyeIcon,
  ListBulletIcon,
  PencilSquareIcon,
  PlusIcon,
  Squares2X2Icon,
  TrashIcon,
} from '@heroicons/vue/24/outline'

type ViewMode = 'card' | 'list'
const VIEW_MODES = ['card', 'list'] as const

const emit = defineEmits<{ create: []; view: [RuleSet]; edit: [RuleSet]; copy: [RuleSet] }>()
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
      <h1 class="text-2xl font-bold">{{ i18n.t('规则集') }}</h1>
      <div class="flex items-center gap-2 flex-wrap justify-end">
        <div class="join bg-base-200 p-0.5 rounded-btn shadow-sm">
          <button type="button" class="btn btn-sm join-item" :class="{ 'btn-active': viewMode === 'card' }" @click="viewMode = 'card'"><Squares2X2Icon class="h-4 w-4" />{{ i18n.t('卡片') }}</button>
          <button type="button" class="btn btn-sm join-item" :class="{ 'btn-active': viewMode === 'list' }" @click="viewMode = 'list'"><ListBulletIcon class="h-4 w-4" />{{ i18n.t('列表') }}</button>
        </div>
        <button type="button" class="btn btn-sm btn-primary" @click="emit('create')"><PlusIcon class="h-4 w-4" />{{ i18n.t('新建规则集') }}</button>
      </div>
    </div>

    <div v-if="store.ruleSets.length" class="flex items-center justify-between gap-2 flex-wrap">
      <div class="flex items-center gap-2">
        <span class="badge badge-neutral">{{ store.ruleSets.length }}</span>
        <span v-if="selected.size" class="badge badge-outline">{{ i18n.t('已选') }} {{ selected.size }}</span>
      </div>
      <div class="flex items-center gap-2 flex-wrap">
        <button type="button" class="btn btn-sm" :class="{ 'btn-active': allSelected }" :disabled="!store.ruleSets.length" @click="toggleAll">
          {{ allSelected ? i18n.t('取消全选') : i18n.t('全选') }}
        </button>
        <button type="button" class="btn btn-sm text-error bg-error/10 hover:bg-error/20 border-transparent" :disabled="busy || !selected.size" @click="requestDelete()">
          <TrashIcon class="h-4 w-4" />{{ i18n.t('删除') }}
        </button>
      </div>
    </div>

    <div v-if="store.loading && !store.ruleSets.length" class="flex justify-center p-12"><span class="loading loading-spinner loading-lg"></span></div>
    <div v-else-if="!store.ruleSets.length" class="hero min-h-64 bg-base-100 rounded-box border border-base-300">
      <div class="hero-content text-center"><div><h2 class="text-lg font-semibold">{{ i18n.t('暂无规则集。') }}</h2><button class="btn btn-primary btn-sm mt-4" @click="emit('create')">{{ i18n.t('新建规则集') }}</button></div></div>
    </div>

    <div v-else-if="viewMode === 'card'" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      <article
        v-for="item in sortedItems"
        :key="item.id"
        class="card bg-base-100 border border-base-300 shadow-sm cursor-pointer transition-colors hover:bg-base-200/60"
        :class="{ 'ring-2 ring-primary': selected.has(item.id) }"
        role="button"
        tabindex="0"
        @click="toggle(item.id)"
        @keydown.enter.prevent="toggle(item.id)"
        @keydown.space.prevent="toggle(item.id)"
      >
        <div class="card-body p-4 gap-3">
          <div class="flex items-start gap-3">
            <input type="checkbox" class="checkbox checkbox-sm mt-1" :checked="selected.has(item.id)" @click.stop @keydown.stop @change="toggle(item.id)" />
            <div class="min-w-0 flex-1">
              <h2 class="font-semibold truncate" :title="item.name">{{ item.name }}</h2>
              <p v-if="item.description" class="text-sm opacity-60 line-clamp-2">{{ item.description }}</p>
            </div>
            <div class="flex gap-1 flex-none">
              <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('查看')" @click.stop="emit('view', item)"><EyeIcon class="h-4 w-4" /></button>
              <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('复制规则集')" @click.stop="emit('copy', item)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
              <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑规则集')" @click.stop="emit('edit', item)"><PencilSquareIcon class="h-4 w-4" /></button>
              <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click.stop="requestDelete(item)"><TrashIcon class="h-4 w-4" /></button>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-2 text-xs">
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

          <div v-if="publicToken.token" class="space-y-1" @click.stop @keydown.stop>
            <span class="text-xs opacity-60">{{ i18n.t('公开下载链接') }}</span>
            <RuleSetLinks :item="item" :token="publicToken.token" :host-prefix="settings.subscriptionHostPrefix" />
          </div>
          <div class="flex gap-2 justify-end" @click.stop @keydown.stop>
            <button type="button" class="btn btn-xs" :title="i18n.t('刷新')" :disabled="refreshing.has(item.id)" @click="refresh(item)"><ArrowPathIcon class="h-4 w-4" :class="{ 'animate-spin': refreshing.has(item.id) }" />{{ i18n.t('刷新') }}</button>
            <button type="button" class="btn btn-xs" @click="exportArtifact(item, 'source')"><ArrowDownTrayIcon class="h-4 w-4" />JSON</button>
            <button type="button" class="btn btn-xs" @click="exportArtifact(item, 'binary')"><ArrowDownTrayIcon class="h-4 w-4" />SRS</button>
          </div>
        </div>
      </article>
    </div>

    <div v-else class="overflow-x-auto bg-base-100 border border-base-300 rounded-box">
      <table class="table table-sm">
        <thead>
          <tr>
            <th class="w-10"></th>
            <th>{{ i18n.t('名称') }}</th>
            <th>{{ i18n.t('规则') }}</th>
            <th>{{ i18n.t('规则源') }}</th>
            <th>{{ i18n.t('产物') }}</th>
            <th>{{ i18n.t('公开下载链接') }}</th>
            <th>{{ i18n.t('发布于') }}</th>
            <th class="text-right">{{ i18n.t('操作') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="item in sortedItems"
            :key="item.id"
            class="cursor-pointer hover:bg-base-200/70"
            :class="{ 'bg-base-200': selected.has(item.id) }"
            @click="toggle(item.id)"
          >
            <td><input type="checkbox" class="checkbox checkbox-sm" :checked="selected.has(item.id)" @click.stop @change="toggle(item.id)" /></td>
            <td class="max-w-64">
              <div class="flex items-center gap-2 min-w-0">
                <span class="font-medium truncate" :title="item.name">{{ item.name }}</span>
                <span v-if="item.last_error" class="badge badge-error badge-sm shrink-0" :title="item.last_error">{{ i18n.t('失败') }}</span>
              </div>
              <div v-if="item.description" class="text-xs opacity-60 truncate" :title="item.description">{{ item.description }}</div>
            </td>
            <td>{{ item.rule_count }}</td>
            <td>{{ item.source_count }}</td>
            <td class="whitespace-nowrap" @click.stop>
              <div class="text-xs">JSON · {{ formatBytes(item.json_size) }}</div>
              <div class="text-xs">SRS · {{ formatBytes(item.srs_size) }}</div>
              <div class="flex gap-1 mt-1">
                <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('刷新')" :disabled="refreshing.has(item.id)" @click="refresh(item)"><ArrowPathIcon class="h-4 w-4" :class="{ 'animate-spin': refreshing.has(item.id) }" /></button>
                <button type="button" class="btn btn-xs btn-ghost" title="JSON" @click="exportArtifact(item, 'source')"><ArrowDownTrayIcon class="h-4 w-4" />JSON</button>
                <button type="button" class="btn btn-xs btn-ghost" title="SRS" @click="exportArtifact(item, 'binary')"><ArrowDownTrayIcon class="h-4 w-4" />SRS</button>
              </div>
            </td>
            <td class="min-w-80" @click.stop>
              <RuleSetLinks v-if="publicToken.token" :item="item" :token="publicToken.token" :host-prefix="settings.subscriptionHostPrefix" />
              <span v-else class="opacity-50">-</span>
            </td>
            <td class="whitespace-nowrap text-xs opacity-70">
              <div>{{ formatDateTime(item.published_at) }}</div>
              <div :title="item.kernel_version">{{ item.kernel_version }}</div>
            </td>
            <td class="text-right" @click.stop>
              <div class="flex gap-1 justify-end">
                <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('查看')" @click="emit('view', item)"><EyeIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('复制规则集')" @click="emit('copy', item)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑规则集')" @click="emit('edit', item)"><PencilSquareIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click="requestDelete(item)"><TrashIcon class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
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
