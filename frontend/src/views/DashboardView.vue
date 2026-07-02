<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useNodesStore } from '../stores/nodes'
import { useTemplatesStore } from '../stores/templates'
import { useProfilesStore } from '../stores/profiles'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { errMsg } from '../utils/error'
import CountryFlag from '../components/CountryFlag.vue'
import { compareCountryCodes } from '../utils/countries'
import {
  ServerStackIcon,
  DocumentTextIcon,
  UserGroupIcon,
  CpuChipIcon,
} from '@heroicons/vue/24/outline'

const nodesStore = useNodesStore()
const templatesStore = useTemplatesStore()
const profilesStore = useProfilesStore()
const settingsStore = useSettingsStore()
const ui = useUiStore()
const loading = ref(true)

onMounted(async () => {
  nodesStore.filters = { search: '', source: '', country: '', type: '' }
  try {
    await Promise.all([
      nodesStore.fetchAll(),
      templatesStore.fetchAll(),
      profilesStore.fetchAll(),
      settingsStore.fetchAll(),
      settingsStore.fetchKernel(),
    ])
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    loading.value = false
  }
})

// top countries by node count
const byCountry = computed(() => {
  const m = new Map<string, number>()
  for (const n of nodesStore.nodes) {
    const c = n.country_code || '??'
    m.set(c, (m.get(c) || 0) + 1)
  }
  return [...m.entries()]
    .sort((a, b) => b[1] - a[1] || compareCountryCodes(a[0], b[0], settingsStore.countryHeatOrder))
    .slice(0, 8)
})

const kernel = computed(() => settingsStore.kernel)
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-2xl font-bold">仪表盘</h1>

    <div v-if="loading" class="flex justify-center py-10">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <template v-else>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div class="stat bg-base-100 rounded-box shadow">
          <div class="stat-figure text-primary"><ServerStackIcon class="h-8 w-8" /></div>
          <div class="stat-title">节点</div>
          <div class="stat-value text-primary">{{ nodesStore.nodes.length }}</div>
        </div>
        <div class="stat bg-base-100 rounded-box shadow">
          <div class="stat-figure text-secondary"><DocumentTextIcon class="h-8 w-8" /></div>
          <div class="stat-title">模板</div>
          <div class="stat-value text-secondary">{{ templatesStore.templates.length }}</div>
        </div>
        <div class="stat bg-base-100 rounded-box shadow">
          <div class="stat-figure text-accent"><UserGroupIcon class="h-8 w-8" /></div>
          <div class="stat-title">订阅分组</div>
          <div class="stat-value text-accent">{{ profilesStore.profiles.length }}</div>
        </div>
        <div class="stat bg-base-100 rounded-box shadow">
          <div class="stat-figure"><CpuChipIcon class="h-8 w-8" /></div>
          <div class="stat-title">内核状态</div>
          <div class="stat-value text-lg mt-1">
            <span v-if="kernel?.available" class="badge badge-success">可用</span>
            <span v-else class="badge badge-warning">不可用</span>
          </div>
          <div class="stat-desc mt-1">{{ kernel?.version || kernel?.path || '未配置' }}</div>
        </div>
      </div>

      <div class="card bg-base-100 shadow">
        <div class="card-body">
          <h2 class="card-title">节点国家分布</h2>
          <div v-if="!byCountry.length" class="opacity-60 text-sm">暂无节点</div>
          <div class="flex flex-wrap gap-3">
            <div
              v-for="[code, count] in byCountry"
              :key="code"
              class="flex items-center gap-2 bg-base-200 rounded-full px-3 py-1"
            >
              <CountryFlag :code="code === '??' ? '' : code" />
              <span class="font-semibold">{{ count }}</span>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
