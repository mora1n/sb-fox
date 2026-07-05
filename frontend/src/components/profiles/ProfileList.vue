<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useProfilesStore } from '../../stores/profiles'
import { useTemplatesStore } from '../../stores/templates'
import { useSettingsStore } from '../../stores/settings'
import { useUiStore } from '../../stores/ui'
import { useI18nStore } from '../../stores/i18n'
import { errMsg } from '../../utils/error'
import { readViewPref, writeViewPref } from '../../utils/viewPrefs'
import type { Profile } from '../../api/types'
import TokenLinkField from '../TokenLinkField.vue'
import {
  PlusIcon,
  PencilSquareIcon,
  TrashIcon,
  ArrowPathIcon,
  ListBulletIcon,
  Squares2X2Icon,
  DocumentDuplicateIcon,
  EyeIcon,
} from '@heroicons/vue/24/outline'

type ViewMode = 'card' | 'list'
type SortDir = 'asc' | 'desc'
type ProfileSortKey = 'name' | 'template' | 'link'

const VIEW_MODES = ['card', 'list'] as const

const emit = defineEmits<{
  create: []
  edit: [Profile]
  copy: [Profile]
  'view-config': [Profile]
}>()

const store = useProfilesStore()
const templates = useTemplatesStore()
const settings = useSettingsStore()
const ui = useUiStore()
const i18n = useI18nStore()

const busy = ref(false)
const profileViewMode = ref<ViewMode>(readViewPref('sb-fox-view:subscriptions', 'card', VIEW_MODES))
const selectedProfiles = ref<Set<number>>(new Set())
const profileSortKey = ref<ProfileSortKey | ''>('')
const profileSortDir = ref<SortDir>('asc')
const tokenHost = computed(() => settings.subscriptionHostPrefix || '')
const allProfilesSelected = computed(
  () => store.profiles.length > 0 && store.profiles.every((p) => selectedProfiles.value.has(p.id)),
)
const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })
const sortedProfiles = computed(() => {
  if (!profileSortKey.value) return store.profiles
  return [...store.profiles].sort((a, b) => compareProfile(a, b, profileSortKey.value as ProfileSortKey, profileSortDir.value))
})

watch(profileViewMode, (value) => writeViewPref('sb-fox-view:subscriptions', value))

function templateName(id: number) {
  return templates.templates.find((t) => t.id === id)?.name || `#${id}`
}

function profileLinkText(p: Profile) {
  const base = (tokenHost.value || window.location.origin).replace(/\/+$/, '')
  return `${base}/sub/${encodeURIComponent(store.subscriptionToken)}/${encodeURIComponent(p.name)}`
}

function compareText(a: string, b: string, dir: SortDir) {
  const result = collator.compare(a || '', b || '')
  return dir === 'asc' ? result : -result
}

function compareProfile(a: Profile, b: Profile, key: ProfileSortKey, dir: SortDir) {
  if (key === 'template') return compareText(templateName(a.template_id), templateName(b.template_id), dir)
  if (key === 'link') return compareText(profileLinkText(a), profileLinkText(b), dir)
  return compareText(a.name, b.name, dir)
}

function toggleProfileSort(key: ProfileSortKey) {
  if (profileSortKey.value === key) profileSortDir.value = profileSortDir.value === 'asc' ? 'desc' : 'asc'
  else {
    profileSortKey.value = key
    profileSortDir.value = 'asc'
  }
}

function sortIndicator(active: string, dir: SortDir, key: string) {
  if (active !== key) return '↕'
  return dir === 'asc' ? '↑' : '↓'
}

async function rotateSharedToken() {
  if (!confirm('轮换共享 token 后所有订阅链接都会变化，确认？')) return
  try {
    await store.rotateSubscriptionToken()
    ui.success('token 已轮换')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

function toggleProfileSelect(id: number) {
  if (selectedProfiles.value.has(id)) selectedProfiles.value.delete(id)
  else selectedProfiles.value.add(id)
  selectedProfiles.value = new Set(selectedProfiles.value)
}

function selectAllProfiles() {
  const next = new Set(selectedProfiles.value)
  if (allProfilesSelected.value) {
    for (const p of store.profiles) next.delete(p.id)
  } else {
    for (const p of store.profiles) next.add(p.id)
  }
  selectedProfiles.value = next
}

async function removeSelectedProfiles() {
  const ids = store.profiles.filter((p) => selectedProfiles.value.has(p.id)).map((p) => p.id)
  if (!ids.length) return ui.info('请先选择订阅')
  if (!confirm(`删除选中的 ${ids.length} 个订阅？`)) return
  busy.value = true
  try {
    for (const id of ids) await store.remove(id)
    selectedProfiles.value = new Set()
    ui.success(`已删除 ${ids.length} 个订阅`)
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function remove(p: Profile) {
  if (!confirm(`删除订阅 "${p.name}"？`)) return
  try {
    await store.remove(p.id)
    selectedProfiles.value.delete(p.id)
    selectedProfiles.value = new Set(selectedProfiles.value)
    ui.success('订阅已删除')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function setProfileSubscriptionEnabled(profile: Profile, enabled: boolean) {
  if (profile.subscription_enabled === enabled) return
  busy.value = true
  try {
    await store.setSubscriptionEnabled(profile.id, enabled)
    ui.success(enabled ? i18n.t('分享已开启') : i18n.t('分享已关闭'))
  } catch (e) {
    ui.error(errMsg(e))
    try {
      await store.fetchAll()
    } catch (refreshErr) {
      ui.error(errMsg(refreshErr))
    }
  } finally {
    busy.value = false
  }
}

function onProfileSubscriptionEnabledChange(profile: Profile, event: Event) {
  setProfileSubscriptionEnabled(profile, (event.target as HTMLInputElement).checked)
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <h1 class="text-2xl font-bold">{{ i18n.t('订阅') }}</h1>
      <div class="flex items-center gap-2 flex-wrap justify-end">
        <div class="join">
          <button
            type="button"
            class="btn btn-sm join-item"
            :class="{ 'btn-active': profileViewMode === 'card' }"
            @click="profileViewMode = 'card'"
          >
            <Squares2X2Icon class="h-4 w-4" /> {{ i18n.t('卡片') }}
          </button>
          <button
            type="button"
            class="btn btn-sm join-item"
            :class="{ 'btn-active': profileViewMode === 'list' }"
            @click="profileViewMode = 'list'"
          >
            <ListBulletIcon class="h-4 w-4" /> {{ i18n.t('列表') }}
          </button>
        </div>
        <button class="btn btn-sm btn-primary" @click="emit('create')">
          <PlusIcon class="h-4 w-4" /> {{ i18n.t('新建订阅') }}
        </button>
      </div>
    </div>

    <div class="card bg-base-100 shadow-sm border border-base-300">
      <div class="card-body p-4 gap-3">
        <div class="flex items-center justify-between gap-3 flex-wrap">
          <div>
            <h2 class="card-title text-base">{{ i18n.t('共享 token') }}</h2>
            <p class="text-xs opacity-60">{{ i18n.t('所有订阅链接共享同一 token，并按订阅名称区分。') }}</p>
          </div>
          <button class="btn btn-sm" @click="rotateSharedToken">
            <ArrowPathIcon class="h-4 w-4" /> {{ i18n.t('轮换共享 token') }}
          </button>
        </div>
        <div class="mono text-xs bg-base-200 rounded-box px-3 py-2 break-all">{{ store.subscriptionToken || '...' }}</div>
      </div>
    </div>

    <div v-if="store.profiles.length" class="flex items-center justify-between gap-2 flex-wrap">
      <div class="flex items-center gap-2">
        <span class="badge badge-neutral">{{ store.profiles.length }}</span>
        <span v-if="selectedProfiles.size" class="badge badge-outline">{{ i18n.t('已选') }} {{ selectedProfiles.size }}</span>
      </div>
      <div class="flex items-center gap-2 flex-wrap">
        <button class="btn btn-sm" @click="selectAllProfiles" :disabled="!store.profiles.length">
          {{ allProfilesSelected ? i18n.t('取消全选') : i18n.t('全选') }}
        </button>
        <button class="btn btn-sm btn-error btn-outline" @click="removeSelectedProfiles" :disabled="busy || !selectedProfiles.size">
          <TrashIcon class="h-4 w-4" /> {{ i18n.t('删除') }}
        </button>
      </div>
    </div>

    <div v-if="store.loading" class="flex justify-center py-10"><span class="loading loading-spinner loading-lg"></span></div>
    <div v-else-if="!store.profiles.length" class="text-center py-10 opacity-60">{{ i18n.t('暂无订阅。') }}</div>
    <div v-else-if="profileViewMode === 'card'" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      <div
        v-for="p in store.profiles"
        :key="p.id"
        class="card bg-base-100 shadow-sm border border-base-300 cursor-pointer transition-colors hover:bg-base-200/60"
        :class="{ 'ring-2 ring-primary': selectedProfiles.has(p.id) }"
        role="button"
        tabindex="0"
        @click="toggleProfileSelect(p.id)"
        @keydown.enter.prevent="toggleProfileSelect(p.id)"
        @keydown.space.prevent="toggleProfileSelect(p.id)"
      >
        <div class="card-body p-4 gap-2">
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-start gap-2 min-w-0">
              <input
                type="checkbox"
                class="checkbox checkbox-sm mt-1"
                :checked="selectedProfiles.has(p.id)"
                @click.stop
                @keydown.stop
                @change="toggleProfileSelect(p.id)"
              />
              <div class="min-w-0">
                <h2 class="card-title text-base truncate" :title="p.name">{{ p.name }}</h2>
                <div class="text-xs opacity-70 mt-1 flex flex-wrap gap-2">
                  <span>{{ i18n.t('模板:') }} {{ templateName(p.template_id) }}</span>
                </div>
              </div>
            </div>
            <div class="flex gap-1 flex-none">
              <button
                type="button"
                class="btn btn-xs btn-ghost"
                :title="i18n.t('查看配置')"
                @click.stop="emit('view-config', p)"
              >
                <EyeIcon class="h-4 w-4" />
              </button>
              <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('复制订阅')" @click.stop="emit('copy', p)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
              <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑订阅')" @click.stop="emit('edit', p)"><PencilSquareIcon class="h-4 w-4" /></button>
              <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click.stop="remove(p)"><TrashIcon class="h-4 w-4" /></button>
            </div>
          </div>
          <div v-if="store.subscriptionToken" class="space-y-1" @click.stop @keydown.stop>
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs opacity-60">{{ i18n.t('订阅链接') }}</span>
              <label class="label cursor-pointer gap-2 p-0">
                <span class="label-text text-xs whitespace-nowrap">{{ i18n.t('分享订阅') }}</span>
                <input
                  type="checkbox"
                  class="toggle toggle-sm"
                  :checked="p.subscription_enabled"
                  :disabled="busy"
                  @change="onProfileSubscriptionEnabledChange(p, $event)"
                />
              </label>
            </div>
            <TokenLinkField
              :token="store.subscriptionToken"
              :profile-name="p.name"
              :host-prefix="tokenHost"
            />
          </div>
        </div>
      </div>
    </div>
    <div v-else class="overflow-x-auto bg-base-100 border border-base-300 rounded-box">
      <table class="table table-sm">
        <thead>
          <tr>
            <th class="w-10"></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleProfileSort('name')">{{ i18n.t('名称') }} {{ sortIndicator(profileSortKey, profileSortDir, 'name') }}</button></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleProfileSort('template')">{{ i18n.t('模板') }} {{ sortIndicator(profileSortKey, profileSortDir, 'template') }}</button></th>
            <th><button type="button" class="btn btn-xs btn-ghost px-1" @click="toggleProfileSort('link')">{{ i18n.t('订阅链接') }} {{ sortIndicator(profileSortKey, profileSortDir, 'link') }}</button></th>
            <th class="text-right">{{ i18n.t('操作') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="p in sortedProfiles"
            :key="p.id"
            class="cursor-pointer hover:bg-base-200/70"
            :class="{ 'bg-base-200': selectedProfiles.has(p.id) }"
            @click="toggleProfileSelect(p.id)"
          >
            <td>
              <input
                type="checkbox"
                class="checkbox checkbox-sm"
                :checked="selectedProfiles.has(p.id)"
                @click.stop
                @change="toggleProfileSelect(p.id)"
              />
            </td>
            <td class="font-medium max-w-64 truncate" :title="p.name">{{ p.name }}</td>
            <td class="max-w-64 truncate" :title="templateName(p.template_id)">{{ templateName(p.template_id) }}</td>
            <td class="min-w-80" @click.stop>
              <div v-if="store.subscriptionToken" class="flex items-center gap-2 min-w-0">
                <TokenLinkField
                  class="flex-1"
                  :token="store.subscriptionToken"
                  :profile-name="p.name"
                  :host-prefix="tokenHost"
                />
                <label class="label cursor-pointer gap-2 p-0 shrink-0">
                  <span class="label-text text-xs whitespace-nowrap">{{ i18n.t('分享订阅') }}</span>
                  <input
                    type="checkbox"
                    class="toggle toggle-sm"
                    :checked="p.subscription_enabled"
                    :disabled="busy"
                    @change="onProfileSubscriptionEnabledChange(p, $event)"
                  />
                </label>
              </div>
              <span v-else class="opacity-50">-</span>
            </td>
            <td class="text-right">
              <div class="flex gap-1 justify-end">
                <button
                  type="button"
                  class="btn btn-xs btn-ghost"
                  :title="i18n.t('查看配置')"
                  @click.stop="emit('view-config', p)"
                >
                  <EyeIcon class="h-4 w-4" />
                </button>
                <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('复制订阅')" @click.stop="emit('copy', p)"><DocumentDuplicateIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost" :title="i18n.t('编辑订阅')" @click.stop="emit('edit', p)"><PencilSquareIcon class="h-4 w-4" /></button>
                <button type="button" class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click.stop="remove(p)"><TrashIcon class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
