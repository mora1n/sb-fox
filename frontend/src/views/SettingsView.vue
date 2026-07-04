<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import type { KernelProbe, KernelProfile } from '../api/types'
import CountryFlag from '../components/CountryFlag.vue'
import {
  COUNTRY_CODES,
  DEFAULT_COUNTRY_HEAT_ORDER,
  completeCountryHeatOrder,
  countryName,
} from '../utils/countries'
import { Bars3Icon, ChevronDownIcon, ChevronUpIcon, MinusIcon, PlusIcon } from '@heroicons/vue/24/outline'

const settings = useSettingsStore()
const auth = useAuthStore()
const ui = useUiStore()
const i18n = useI18nStore()

const kernelCards = ref<KernelProbe[]>([])
const allowPrivate = ref(false)
const appName = ref('')
const subscriptionHostPrefix = ref('')
const countryOrder = ref<string[]>([])
const draggedIndex = ref<number | null>(null)
const countryInsertIndex = ref<number | null>(null)
const busy = ref(false)

const oldPw = ref('')
const newPw = ref('')
const confirmPw = ref('')

onMounted(async () => {
  try {
    await settings.fetchAll()
    syncSettingsFields()
    if (auth.isAdmin) {
      await settings.fetchKernels()
      syncKernelCards()
    }
  } catch (e) {
    ui.error(errMsg(e))
  }
})

function syncSettingsFields() {
  allowPrivate.value = settings.settings.subfetch_allow_private === 'true'
  appName.value = settings.appDisplayName
  subscriptionHostPrefix.value = settings.settings.subscription_host_prefix || ''
  countryOrder.value = completeCountryHeatOrder(settings.countryHeatOrder)
}

function syncKernelCards() {
  kernelCards.value = (settings.kernel?.kernels ?? []).map((k) => ({ ...k, path: k.path || '' }))
  if (!kernelCards.value.length) {
    kernelCards.value = [{ id: '', name: 'sing-box', path: settings.settings.kernel_path || 'sing-box', available: false, valid: false }]
  }
}

async function saveAppName() {
  busy.value = true
  try {
    await settings.update({ app_display_name: appName.value })
    syncSettingsFields()
    ui.success('名称已保存')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

function kernelPayload(): KernelProfile[] {
  return kernelCards.value.map((k) => ({
    id: k.id,
    name: k.name.trim(),
    path: (k.path || '').trim(),
  }))
}

function addKernel() {
  kernelCards.value = [
    ...kernelCards.value,
    { id: '', name: `sing-box ${kernelCards.value.length + 1}`, path: '', available: false, valid: false },
  ]
}

function removeKernel(index: number) {
  kernelCards.value = kernelCards.value.filter((_, i) => i !== index)
}

async function saveKernels() {
  busy.value = true
  try {
    await settings.saveKernels(kernelPayload())
    syncKernelCards()
    ui.success('内核已保存')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function saveSubscriptionHostPrefix() {
  busy.value = true
  try {
    await settings.update({ subscription_host_prefix: subscriptionHostPrefix.value })
    syncSettingsFields()
    ui.success('订阅 Host 已保存')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function testKernel(index: number) {
  busy.value = true
  try {
    const result = await settings.testKernel(kernelPayload()[index])
    kernelCards.value[index] = { ...kernelCards.value[index], ...result }
    if (result.valid) ui.success(`内核有效: ${result.version || '未知版本'}`)
    else ui.error(result.error || '内核无效，请检查路径')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function saveAllowPrivate() {
  try {
    await settings.update({ subfetch_allow_private: allowPrivate.value ? 'true' : 'false' })
    ui.success('已保存')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

function moveCountry(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= countryOrder.value.length) return
  const next = [...countryOrder.value]
  const item = next[index]
  next[index] = next[target]
  next[target] = item
  countryOrder.value = next
}

function countryInsertTarget(index: number, event: DragEvent) {
  const row = event.currentTarget as HTMLElement | null
  if (!row) return index
  const rect = row.getBoundingClientRect()
  return event.clientY < rect.top + rect.height / 2 ? index : index + 1
}

function clearCountryDrag() {
  draggedIndex.value = null
  countryInsertIndex.value = null
}

function dropCountry(index: number, event: DragEvent) {
  if (draggedIndex.value === null) {
    clearCountryDrag()
    return
  }
  const source = draggedIndex.value
  const target = countryInsertIndex.value ?? countryInsertTarget(index, event)
  if (target === source || target === source + 1) {
    clearCountryDrag()
    return
  }
  const next = [...countryOrder.value]
  const [item] = next.splice(source, 1)
  next.splice(target > source ? target - 1 : target, 0, item)
  countryOrder.value = next
  clearCountryDrag()
}

function startCountryDrag(index: number) {
  draggedIndex.value = index
  countryInsertIndex.value = null
}

function overCountry(index: number, event: DragEvent) {
  if (draggedIndex.value === null) {
    countryInsertIndex.value = null
    return
  }
  const target = countryInsertTarget(index, event)
  countryInsertIndex.value = target === draggedIndex.value || target === draggedIndex.value + 1 ? null : target
}

function resetCountryOrder() {
  countryOrder.value = completeCountryHeatOrder(DEFAULT_COUNTRY_HEAT_ORDER)
}

async function saveCountryOrder() {
  busy.value = true
  try {
    await settings.update({ country_heat_order: JSON.stringify(countryOrder.value) })
    syncSettingsFields()
    ui.success('国家排序已保存')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function changePassword() {
  if (newPw.value.length < 4) return ui.error('新密码至少 4 位')
  if (newPw.value !== confirmPw.value) return ui.error('两次输入的新密码不一致')
  busy.value = true
  try {
    await auth.changePassword(oldPw.value, newPw.value)
    ui.success('密码已修改')
    oldPw.value = newPw.value = confirmPw.value = ''
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-4 max-w-3xl">
    <h1 class="text-2xl font-bold">{{ i18n.t('设置') }}</h1>

    <div v-if="auth.isAdmin" class="card bg-base-100 shadow-sm">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">{{ i18n.t('显示名称') }}</h2>
        <label class="form-control">
          <span class="label-text mb-1">{{ i18n.t('名称') }}</span>
          <div class="join">
            <input v-model="appName" class="input input-bordered input-sm join-item flex-1" />
            <button class="btn btn-sm join-item" @click="saveAppName" :disabled="busy">{{ i18n.t('保存') }}</button>
          </div>
        </label>
      </div>
    </div>

    <div v-if="auth.isAdmin" class="card bg-base-100 shadow-sm">
      <div class="card-body gap-3">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <h2 class="card-title text-base">{{ i18n.t('sing-box 内核') }}</h2>
          <div class="flex gap-2">
            <button class="btn btn-sm" type="button" @click="addKernel" :disabled="busy">
              <PlusIcon class="h-4 w-4" /> {{ i18n.t('添加内核') }}
            </button>
            <button class="btn btn-sm btn-primary" type="button" @click="saveKernels" :disabled="busy">
              {{ i18n.t('保存') }}
            </button>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-3">
          <div
            v-for="(kernel, index) in kernelCards"
            :key="kernel.id || index"
            class="rounded-box border border-base-300 bg-base-100 p-3 flex flex-col gap-3"
          >
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2 min-w-0">
                <span v-if="kernel.valid" class="badge badge-success">{{ i18n.t('有效') }}</span>
                <span v-else-if="kernel.available" class="badge badge-warning">{{ i18n.t('无效') }}</span>
                <span v-else class="badge badge-ghost">{{ i18n.t('未测试') }}</span>
                <span v-if="kernel.version" class="text-xs opacity-70 truncate">{{ kernel.version }}</span>
              </div>
              <button
                class="btn btn-xs btn-ghost text-error"
                type="button"
                :title="i18n.t('删除内核')"
                @click="removeKernel(index)"
                :disabled="busy"
              >
                <MinusIcon class="h-4 w-4" />
              </button>
            </div>
            <div class="grid grid-cols-1 md:grid-cols-[minmax(0,180px)_minmax(0,1fr)_auto] gap-2">
              <label class="form-control">
                <span class="label-text mb-1">{{ i18n.t('内核名称') }}</span>
                <input v-model="kernel.name" class="input input-bordered input-sm" placeholder="sing-box" />
              </label>
              <label class="form-control">
                <span class="label-text mb-1">{{ i18n.t('内核路径') }}</span>
                <input v-model="kernel.path" class="input input-bordered input-sm mono" placeholder="/usr/local/bin/sing-box" />
              </label>
              <div class="flex items-end">
                <button class="btn btn-sm w-full md:w-auto" type="button" @click="testKernel(index)" :disabled="busy">
                  {{ i18n.t('测试') }}
                </button>
              </div>
            </div>
            <p v-if="kernel.error" class="text-xs text-error break-all">{{ kernel.error }}</p>
          </div>
        </div>
      </div>
    </div>

    <div v-if="auth.isAdmin" class="card bg-base-100 shadow-sm">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">{{ i18n.t('订阅 Host') }}</h2>
        <label class="form-control">
          <span class="label-text mb-1">{{ i18n.t('订阅链接前缀') }}</span>
          <div class="join">
            <input v-model="subscriptionHostPrefix" class="input input-bordered input-sm join-item flex-1 mono" placeholder="https://example.com" />
            <button class="btn btn-sm join-item" @click="saveSubscriptionHostPrefix" :disabled="busy">{{ i18n.t('保存') }}</button>
          </div>
        </label>
        <p class="text-xs opacity-60">{{ i18n.t('留空时使用当前浏览器 Host。') }}</p>
      </div>
    </div>

    <div class="card bg-base-100 shadow-sm">
      <div class="card-body gap-3">
        <div class="flex items-center justify-between gap-3">
          <h2 class="card-title text-base">{{ i18n.t('国家热度排序') }}</h2>
          <div class="flex gap-2">
            <button class="btn btn-sm" @click="resetCountryOrder" :disabled="busy">{{ i18n.t('重置') }}</button>
            <button class="btn btn-sm btn-primary" @click="saveCountryOrder" :disabled="busy">{{ i18n.t('保存') }}</button>
          </div>
        </div>
        <div class="max-h-96 overflow-y-auto divide-y divide-base-200 border border-base-300 rounded-box">
          <div
            v-for="(code, index) in countryOrder"
            :key="code"
            class="flex items-center gap-2 bg-base-100 px-3 py-2 transition-colors hover:bg-base-200/60 border-y-2 border-transparent"
            :class="{
              'opacity-60 ring-1 ring-base-content/30': draggedIndex === index,
              'border-t-base-content': countryInsertIndex === index,
              'border-b-base-content': countryInsertIndex === index + 1,
            }"
            @dragenter.prevent="overCountry(index, $event)"
            @dragover.prevent="overCountry(index, $event)"
            @drop="dropCountry(index, $event)"
          >
            <button
              class="btn btn-xs btn-ghost cursor-move"
              type="button"
              draggable="true"
              :title="i18n.t('拖拽排序')"
              @dragstart="startCountryDrag(index)"
              @dragend="clearCountryDrag"
            >
              <Bars3Icon class="h-4 w-4" />
            </button>
            <span class="badge badge-sm w-8">{{ index + 1 }}</span>
            <CountryFlag :code="code" />
            <span class="flex-1 min-w-0 truncate">{{ code }} — {{ countryName(code) }}</span>
            <div class="join">
              <button
                class="btn btn-xs btn-square join-item"
                :title="i18n.t('上移')"
                :disabled="index === 0"
                @click="moveCountry(index, -1)"
              >
                <ChevronUpIcon class="h-3 w-3" />
              </button>
              <button
                class="btn btn-xs btn-square join-item"
                :title="i18n.t('下移')"
                :disabled="index === countryOrder.length - 1"
                @click="moveCountry(index, 1)"
              >
                <ChevronDownIcon class="h-3 w-3" />
              </button>
            </div>
          </div>
        </div>
        <div class="text-xs opacity-60">{{ COUNTRY_CODES.length }} {{ i18n.t('个国家/地区') }}</div>
      </div>
    </div>

    <div v-if="auth.isAdmin" class="card bg-base-100 shadow-sm">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">{{ i18n.t('订阅抓取') }}</h2>
        <label class="label cursor-pointer justify-start gap-2">
          <input type="checkbox" class="toggle" v-model="allowPrivate" @change="saveAllowPrivate" />
          <span class="label-text">{{ i18n.t('允许抓取私网地址') }}</span>
        </label>
        <p class="text-xs opacity-60">{{ i18n.t('关闭时会拒绝私网、环回、链路本地、CGNAT、组播和云元数据地址；仅在可信内网订阅源中开启。') }}</p>
      </div>
    </div>

    <div class="card bg-base-100 shadow-sm">
      <div class="card-body gap-3">
        <h2 class="card-title text-base">{{ i18n.t('修改密码') }}</h2>
        <form @submit.prevent="changePassword" class="flex flex-col gap-3">
          <input v-model="oldPw" type="password" class="input input-bordered input-sm" :placeholder="i18n.t('当前密码')" autocomplete="current-password" required />
          <input v-model="newPw" type="password" class="input input-bordered input-sm" :placeholder="i18n.t('新密码至少 4 位')" autocomplete="new-password" required />
          <input v-model="confirmPw" type="password" class="input input-bordered input-sm" :placeholder="i18n.t('确认新密码')" autocomplete="new-password" required />
          <button class="btn btn-primary btn-sm self-start" :disabled="busy">{{ i18n.t('修改密码') }}</button>
        </form>
      </div>
    </div>
  </div>
</template>
