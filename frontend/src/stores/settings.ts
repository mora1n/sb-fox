import { defineStore } from 'pinia'
import { ref } from 'vue'
import { get, post, put } from '../api/client'
import type { AppInfo, KernelProfile, KernelProbe, KernelStatus, Settings } from '../api/types'
import { completeCountryHeatOrder, DEFAULT_COUNTRY_HEAT_ORDER } from '../utils/countries'

const DEFAULT_APP_DISPLAY_NAME = 'sb-fox'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings>({})
  const kernel = ref<KernelStatus | null>(null)
  const appDisplayName = ref(DEFAULT_APP_DISPLAY_NAME)
  const countryHeatOrder = ref<string[]>(completeCountryHeatOrder(DEFAULT_COUNTRY_HEAT_ORDER))
  const registrationEnabled = ref(false)
  const subscriptionHostPrefix = ref('')
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
    subscriptionHostPrefix.value = app?.subscription_host_prefix?.trim() || ''
  }

  async function update(patch: Settings): Promise<void> {
    await put('/settings', patch)
    await fetchAll()
  }

  async function fetchKernel(): Promise<KernelStatus> {
    kernel.value = await get<KernelStatus>('/settings/kernel')
    return kernel.value
  }

  async function fetchKernels(): Promise<KernelStatus> {
    kernel.value = await get<KernelStatus>('/settings/kernels')
    return kernel.value
  }

  async function saveKernels(kernels: KernelProfile[]): Promise<KernelStatus> {
    await put('/settings/kernels', { kernels })
    return fetchKernels()
  }

  async function testKernel(profile: KernelProfile): Promise<KernelProbe> {
    return post<KernelProbe>('/settings/kernels/test', profile)
  }

  async function fetchKernelStatus(): Promise<KernelStatus> {
    kernel.value = await get<KernelStatus>('/kernel/status')
    return kernel.value
  }

  async function setActiveKernel(id: string): Promise<KernelStatus> {
    kernel.value = await put<KernelStatus>('/kernel/active', { id })
    return kernel.value
  }

  function applySettings(next: Settings): void {
    appDisplayName.value = next.app_display_name?.trim() || DEFAULT_APP_DISPLAY_NAME
    subscriptionHostPrefix.value = next.subscription_host_prefix?.trim() || ''
    const rawOrder = next.country_heat_order ? JSON.parse(next.country_heat_order) : DEFAULT_COUNTRY_HEAT_ORDER
    countryHeatOrder.value = completeCountryHeatOrder(rawOrder)
  }

  return {
    settings,
    kernel,
    appDisplayName,
    countryHeatOrder,
    registrationEnabled,
    subscriptionHostPrefix,
    loading,
    fetchAll,
    fetchAppInfo,
    update,
    fetchKernel,
    fetchKernels,
    saveKernels,
    testKernel,
    fetchKernelStatus,
    setActiveKernel,
  }
})
