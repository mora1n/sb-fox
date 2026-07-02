<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import CountryFlag from '../components/CountryFlag.vue'
import {
  COUNTRY_CODES,
  DEFAULT_COUNTRY_HEAT_ORDER,
  completeCountryHeatOrder,
  countryName,
} from '../utils/countries'
import { Bars3Icon, ChevronDownIcon, ChevronUpIcon } from '@heroicons/vue/24/outline'

const settings = useSettingsStore()
const auth = useAuthStore()
const ui = useUiStore()
const i18n = useI18nStore()

const kernelPath = ref('')
const allowPrivate = ref(false)
const appName = ref('')
const countryOrder = ref<string[]>([])
const draggedIndex = ref<number | null>(null)
const busy = ref(false)

const oldPw = ref('')
const newPw = ref('')
const confirmPw = ref('')

onMounted(async () => {
  try {
    await settings.fetchAll()
    syncSettingsFields()
    if (auth.isAdmin) await settings.fetchKernel()
  } catch (e) {
    ui.error(errMsg(e))
  }
})

function syncSettingsFields() {
  kernelPath.value = settings.settings.kernel_path || ''
  allowPrivate.value = settings.settings.subfetch_allow_private === 'true'
  appName.value = settings.appDisplayName
  countryOrder.value = completeCountryHeatOrder(settings.countryHeatOrder)
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

async function saveKernel() {
  busy.value = true
  try {
    await settings.update({ kernel_path: kernelPath.value })
    await settings.fetchKernel()
    ui.success('内核路径已保存')
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function testKernel() {
  busy.value = true
  try {
    const k = await settings.fetchKernel()
    if (k.available) ui.success(`内核可用: ${k.version || '未知版本'}`)
    else ui.error('内核不可用，请检查路径')
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

function dropCountry(targetIndex: number) {
  if (draggedIndex.value === null || draggedIndex.value === targetIndex) return
  const next = [...countryOrder.value]
  const [item] = next.splice(draggedIndex.value, 1)
  next.splice(targetIndex, 0, item)
  countryOrder.value = next
  draggedIndex.value = targetIndex
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
  if (newPw.value.length < 8) return ui.error('新密码至少 8 位')
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

    <div class="card bg-base-100 shadow-sm">
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
        <h2 class="card-title text-base">{{ i18n.t('sing-box 内核') }}</h2>
        <div class="flex items-center gap-2">
          <span class="text-sm">{{ i18n.t('状态:') }}</span>
          <span v-if="settings.kernel?.available" class="badge badge-success">{{ i18n.t('可用') }}</span>
          <span v-else class="badge badge-warning">{{ i18n.t('不可用') }}</span>
          <span v-if="settings.kernel?.version" class="text-sm opacity-70">{{ settings.kernel.version }}</span>
        </div>
        <label class="form-control">
          <span class="label-text mb-1">{{ i18n.t('内核路径') }}</span>
          <div class="join">
            <input v-model="kernelPath" class="input input-bordered input-sm join-item flex-1 mono" placeholder="/usr/local/bin/sing-box" />
            <button class="btn btn-sm join-item" @click="saveKernel" :disabled="busy">{{ i18n.t('保存') }}</button>
            <button class="btn btn-sm join-item" @click="testKernel" :disabled="busy">{{ i18n.t('测试') }}</button>
          </div>
        </label>
      </div>
    </div>

    <div v-if="auth.isAdmin" class="card bg-base-100 shadow-sm">
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
            class="flex items-center gap-2 bg-base-100 px-3 py-2"
            draggable="true"
            @dragstart="draggedIndex = index"
            @dragover.prevent
            @drop="dropCountry(index)"
            @dragend="draggedIndex = null"
          >
            <Bars3Icon class="h-4 w-4 opacity-60 cursor-move" />
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
          <input v-model="newPw" type="password" class="input input-bordered input-sm" :placeholder="i18n.t('新密码至少 8 位')" autocomplete="new-password" required />
          <input v-model="confirmPw" type="password" class="input input-bordered input-sm" :placeholder="i18n.t('确认新密码')" autocomplete="new-password" required />
          <button class="btn btn-primary btn-sm self-start" :disabled="busy">{{ i18n.t('修改密码') }}</button>
        </form>
      </div>
    </div>
  </div>
</template>
