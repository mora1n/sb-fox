<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { EyeIcon, EyeSlashIcon, MoonIcon, SunIcon } from '@heroicons/vue/24/outline'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import { useI18nStore } from '../stores/i18n'
import { useUiStore } from '../stores/ui'
import { ApiRequestError } from '../api/client'

const auth = useAuthStore()
const settings = useSettingsStore()
const i18n = useI18nStore()
const ui = useUiStore()
const router = useRouter()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const mode = ref<'login' | 'register'>('login')
const showPassword = ref(false)

onMounted(async () => {
  // if already logged in, skip to dashboard
  if (!auth.checked) await auth.me()
  if (auth.isAuthenticated) router.push({ name: 'dashboard' })
})

async function submit() {
  error.value = ''
  loading.value = true
  try {
    if (mode.value === 'register') {
      if (password.value.length < 4) throw new Error('密码至少 4 位')
      await auth.register(username.value, password.value)
    } else {
      await auth.login(username.value, password.value)
    }
    router.push({ name: 'dashboard' })
  } catch (e) {
    if (mode.value === 'login' && e instanceof ApiRequestError && e.status === 401 && e.code === 'unauthorized') {
      error.value = i18n.t('用户名或密码错误')
    } else {
      error.value = e instanceof ApiRequestError || e instanceof Error ? e.message : '登录失败'
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-base-200 p-4">
    <div class="fixed right-4 top-4 flex items-center gap-2">
      <button
        class="btn btn-ghost btn-sm min-w-16"
        :title="i18n.t('语言')"
        @click="i18n.toggleLocale()"
      >
        <span aria-hidden="true">🌐</span>
        <span>{{ i18n.isEnglish ? '中' : 'EN' }}</span>
      </button>
      <button
        class="btn btn-ghost btn-sm"
        :title="ui.theme === 'light-neutral' ? i18n.t('切换深色模式') : i18n.t('切换浅色模式')"
        :aria-label="ui.theme === 'light-neutral' ? i18n.t('切换深色模式') : i18n.t('切换浅色模式')"
        @click="ui.toggleTheme()"
      >
        <MoonIcon v-if="ui.theme === 'light-neutral'" class="h-5 w-5" />
        <SunIcon v-else class="h-5 w-5" />
      </button>
    </div>
    <div class="card w-full max-w-sm bg-base-100 shadow-xl">
      <div class="card-body">
        <h1 class="text-2xl font-bold text-center min-h-8">
          <span v-if="settings.appInfoLoaded">{{ settings.appDisplayName }}</span>
          <span v-else class="inline-block h-7 w-32 rounded bg-base-300 align-middle"></span>
        </h1>
        <p class="text-center text-sm opacity-60 mb-2">{{ i18n.t('sing-box 配置订阅管理') }}</p>
        <div v-if="settings.registrationEnabled" class="tabs tabs-boxed grid grid-cols-2 mb-2">
          <button class="tab" :class="{ 'tab-active': mode === 'login' }" @click="mode = 'login'">{{ i18n.t('登录') }}</button>
          <button class="tab" :class="{ 'tab-active': mode === 'register' }" @click="mode = 'register'">{{ i18n.t('注册') }}</button>
        </div>
        <div v-if="error" class="alert alert-error text-sm py-2">
          <span>{{ error }}</span>
        </div>
        <form @submit.prevent="submit" class="flex flex-col gap-3">
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('用户名') }}</span>
            <input v-model="username" class="input input-bordered" autocomplete="username" required />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('密码') }}</span>
            <div class="join w-full">
              <input
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                class="input input-bordered join-item min-w-0 flex-1"
                :autocomplete="mode === 'register' ? 'new-password' : 'current-password'"
                required
              />
              <button
                type="button"
                class="btn btn-square join-item"
                :title="showPassword ? i18n.t('隐藏密码') : i18n.t('显示密码')"
                :aria-label="showPassword ? i18n.t('隐藏密码') : i18n.t('显示密码')"
                @click="showPassword = !showPassword"
              >
                <EyeSlashIcon v-if="showPassword" class="h-5 w-5" />
                <EyeIcon v-else class="h-5 w-5" />
              </button>
            </div>
          </label>
          <button class="btn btn-primary mt-2" :disabled="loading">
            <span v-if="loading" class="loading loading-spinner loading-sm"></span>
            {{ mode === 'register' ? i18n.t('注册') : i18n.t('登录') }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>
