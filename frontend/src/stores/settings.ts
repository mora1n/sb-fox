import { defineStore } from 'pinia'
import { ref } from 'vue'
import { get, post, put } from '../api/client'
import type { AppInfo, KernelProfile, KernelProbe, KernelStatus, Settings } from '../api/types'
import { completeCountryHeatOrder, DEFAULT_COUNTRY_HEAT_ORDER } from '../utils/countries'

const DEFAULT_APP_DISPLAY_NAME = 'App'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings>({})
  const kernel = ref<KernelStatus | null>(null)
  const appDisplayName = ref('')
  const appInfoLoaded = ref(false)
  const publicCountryHeatOrder = ref<string[]>(completeCountryHeatOrder(DEFAULT_COUNTRY_HEAT_ORDER))
  const countryHeatOrder = ref<string[]>(completeCountryHeatOrder(DEFAULT_COUNTRY_HEAT_ORDER))
  const registrationEnabled = ref(false)
  const subscriptionHostPrefix = ref('')
  const loading = ref(false)
  const settingsLoaded = ref(false)
  const kernelLoaded = ref(false)
  let settingsInFlight: Promise<void> | null = null
  let appInfoInFlight: Promise<void> | null = null
  let kernelStatusInFlight: Promise<KernelStatus> | null = null

  async function fetchAll(force = false): Promise<void> {
    if (!force && settingsLoaded.value) return
    if (!force && settingsInFlight) return settingsInFlight
    loading.value = true
    settingsInFlight = get<Settings>('/settings').then((items) => {
      settings.value = items ?? {}
      applySettings(settings.value)
      settingsLoaded.value = true
    }).finally(() => {
      loading.value = false
      settingsInFlight = null
    })
    return settingsInFlight
  }

  async function fetchAppInfo(force = false): Promise<void> {
    if (!force && appInfoLoaded.value) return
    if (!force && appInfoInFlight) return appInfoInFlight
    appInfoInFlight = get<AppInfo>('/app', true).then((app) => {
      appDisplayName.value = app?.display_name?.trim() || DEFAULT_APP_DISPLAY_NAME
      appInfoLoaded.value = true
      publicCountryHeatOrder.value = completeCountryHeatOrder(app?.country_heat_order || DEFAULT_COUNTRY_HEAT_ORDER)
      countryHeatOrder.value = [...publicCountryHeatOrder.value]
      registrationEnabled.value = !!app?.registration_enabled
      subscriptionHostPrefix.value = app?.subscription_host_prefix?.trim() || ''
    }).finally(() => {
      appInfoInFlight = null
    })
    return appInfoInFlight
  }

  async function update(patch: Settings): Promise<void> {
    await put('/settings', patch)
    settingsLoaded.value = false
    await fetchAll(true)
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
    kernelLoaded.value = false
    return fetchKernels()
  }

  async function testKernel(profile: KernelProfile): Promise<KernelProbe> {
    return post<KernelProbe>('/settings/kernels/test', profile)
  }

  async function fetchKernelStatus(force = false): Promise<KernelStatus> {
    if (!force && kernelLoaded.value && kernel.value) return kernel.value
    if (!force && kernelStatusInFlight) return kernelStatusInFlight
    kernelStatusInFlight = get<KernelStatus>('/kernel/status').then((status) => {
      kernel.value = status
      kernelLoaded.value = true
      return status
    }).finally(() => {
      kernelStatusInFlight = null
    })
    return kernelStatusInFlight
  }

  async function setActiveKernel(id: string): Promise<KernelStatus> {
    kernel.value = await put<KernelStatus>('/kernel/active', { id })
    kernelLoaded.value = true
    return kernel.value
  }

  function applySettings(next: Settings): void {
    if ('app_display_name' in next) {
      appDisplayName.value = next.app_display_name?.trim() || DEFAULT_APP_DISPLAY_NAME
      appInfoLoaded.value = true
    }
    if ('subscription_host_prefix' in next) {
      subscriptionHostPrefix.value = next.subscription_host_prefix?.trim() || ''
    }
    if ('country_heat_order' in next) {
      const rawOrder = next.country_heat_order ? JSON.parse(next.country_heat_order) : DEFAULT_COUNTRY_HEAT_ORDER
      countryHeatOrder.value = completeCountryHeatOrder(rawOrder)
    }
  }

  function resetSessionState(): void {
    settings.value = {}
    kernel.value = null
    countryHeatOrder.value = [...publicCountryHeatOrder.value]
    loading.value = false
    settingsLoaded.value = false
    kernelLoaded.value = false
    settingsInFlight = null
    kernelStatusInFlight = null
  }

  return {
    settings,
    kernel,
    appDisplayName,
    appInfoLoaded,
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
    resetSessionState,
  }
})
