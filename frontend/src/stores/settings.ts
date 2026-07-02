import { defineStore } from 'pinia'
import { ref } from 'vue'
import { get, put } from '../api/client'
import type { AppInfo, KernelStatus, Settings } from '../api/types'
import { completeCountryHeatOrder, DEFAULT_COUNTRY_HEAT_ORDER } from '../utils/countries'

const DEFAULT_APP_DISPLAY_NAME = 'sb-fox'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings>({})
  const kernel = ref<KernelStatus | null>(null)
  const appDisplayName = ref(DEFAULT_APP_DISPLAY_NAME)
  const countryHeatOrder = ref<string[]>(completeCountryHeatOrder(DEFAULT_COUNTRY_HEAT_ORDER))
  const registrationEnabled = ref(false)
  const loading = ref(false)

  async function fetchAll(): Promise<void> {
    loading.value = true
    try {
      settings.value = (await get<Settings>('/settings')) ?? {}
      applySettings(settings.value)
    } finally {
      loading.value = false
    }
  }

  async function fetchAppInfo(): Promise<void> {
    const app = await get<AppInfo>('/app', true)
    appDisplayName.value = app?.display_name?.trim() || DEFAULT_APP_DISPLAY_NAME
    countryHeatOrder.value = completeCountryHeatOrder(app?.country_heat_order || DEFAULT_COUNTRY_HEAT_ORDER)
    registrationEnabled.value = !!app?.registration_enabled
  }

  async function update(patch: Settings): Promise<void> {
    await put('/settings', patch)
    await fetchAll()
  }

  async function fetchKernel(): Promise<KernelStatus> {
    kernel.value = await get<KernelStatus>('/settings/kernel')
    return kernel.value
  }

  async function fetchKernelStatus(): Promise<KernelStatus> {
    kernel.value = await get<KernelStatus>('/kernel/status')
    return kernel.value
  }

  function applySettings(next: Settings): void {
    appDisplayName.value = next.app_display_name?.trim() || DEFAULT_APP_DISPLAY_NAME
    const rawOrder = next.country_heat_order ? JSON.parse(next.country_heat_order) : DEFAULT_COUNTRY_HEAT_ORDER
    countryHeatOrder.value = completeCountryHeatOrder(rawOrder)
  }

  return {
    settings,
    kernel,
    appDisplayName,
    countryHeatOrder,
    registrationEnabled,
    loading,
    fetchAll,
    fetchAppInfo,
    update,
    fetchKernel,
    fetchKernelStatus,
  }
})
