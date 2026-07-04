<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useNodesStore } from '../stores/nodes'
import { useTemplatesStore } from '../stores/templates'
import { useProfilesStore } from '../stores/profiles'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
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
const i18n = useI18nStore()
const loading = ref(true)
const selectedKernelID = ref('')

onMounted(async () => {
  nodesStore.filters = { search: '', source: '', country: '', type: '' }
  try {
    await Promise.all([
      nodesStore.fetchSummary(),
      templatesStore.fetchAll(),
      profilesStore.fetchAll(),
      settingsStore.fetchAll(),
      settingsStore.fetchKernelStatus(),
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
  for (const n of nodesStore.summaryNodes) {
    const c = n.country_code || '??'
    m.set(c, (m.get(c) || 0) + 1)
  }
  return [...m.entries()]
    .sort((a, b) => b[1] - a[1] || compareCountryCodes(a[0], b[0], settingsStore.countryHeatOrder))
    .slice(0, 8)
})

const kernel = computed(() => settingsStore.kernel)
const activeKernel = computed(() => kernel.value?.active ?? null)
const validKernels = computed(() => kernel.value?.kernels ?? [])
const kernelHint = computed(() => i18n.t('请选择有效 sing-box 内核或联系管理员配置内核'))

watch(
  kernel,
  (value) => {
    selectedKernelID.value = value?.active_kernel_id || value?.active?.id || ''
  },
  { immediate: true },
)

async function switchKernel() {
  if (!selectedKernelID.value) return
  try {
    await settingsStore.setActiveKernel(selectedKernelID.value)
    ui.success('内核已切换')
  } catch (e) {
    ui.error(errMsg(e))
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-2xl font-bold">{{ i18n.t('仪表盘') }}</h1>

    <div v-if="loading" class="flex justify-center py-10">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <template v-else>
      <div class="grid grid-cols-1 xl:grid-cols-[minmax(0,1fr)_360px] gap-4">
        <div class="flex flex-col gap-4 min-w-0">
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div class="stat bg-base-100 rounded-box shadow">
              <div class="stat-figure text-primary"><ServerStackIcon class="h-8 w-8" /></div>
              <div class="stat-title">{{ i18n.t('节点') }}</div>
              <div class="stat-value text-primary">{{ nodesStore.summaryNodes.length }}</div>
            </div>
            <div class="stat bg-base-100 rounded-box shadow">
              <div class="stat-figure text-secondary"><DocumentTextIcon class="h-8 w-8" /></div>
              <div class="stat-title">{{ i18n.t('模板') }}</div>
              <div class="stat-value text-secondary">{{ templatesStore.templates.length }}</div>
            </div>
            <div class="stat bg-base-100 rounded-box shadow">
              <div class="stat-figure text-accent"><UserGroupIcon class="h-8 w-8" /></div>
              <div class="stat-title">{{ i18n.t('订阅') }}</div>
              <div class="stat-value text-accent">{{ profilesStore.profiles.length }}</div>
            </div>
          </div>

          <div class="card bg-base-100 shadow">
            <div class="card-body">
              <h2 class="card-title">{{ i18n.t('节点国家分布') }}</h2>
              <div v-if="!byCountry.length" class="opacity-60 text-sm">{{ i18n.t('暂无节点') }}</div>
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
        </div>

        <div class="card bg-base-100 shadow h-fit">
          <div class="card-body gap-3">
            <div class="flex items-center gap-2">
              <CpuChipIcon class="h-6 w-6" />
              <h2 class="card-title text-base">{{ i18n.t('内核状态') }}</h2>
            </div>
            <div class="flex items-center gap-2">
              <span v-if="kernel?.available" class="badge badge-success">{{ i18n.t('可用') }}</span>
              <span v-else class="badge badge-warning">{{ i18n.t('不可用') }}</span>
              <span v-if="activeKernel?.version" class="text-xs opacity-70 truncate">{{ activeKernel.version }}</span>
            </div>
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('当前内核') }}</span>
              <select
                v-model="selectedKernelID"
                class="select select-bordered select-sm"
                :disabled="!validKernels.length"
                :title="validKernels.length ? '' : kernelHint"
                @change="switchKernel"
              >
                <option value="" disabled>{{ i18n.t('选择内核') }}</option>
                <option v-for="item in validKernels" :key="item.id" :value="item.id">
                  {{ item.name }}{{ item.version ? ` · ${item.version}` : '' }}
                </option>
              </select>
            </label>
            <p v-if="!validKernels.length" class="text-xs opacity-60">{{ kernelHint }}</p>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
