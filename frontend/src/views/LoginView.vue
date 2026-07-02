<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import { useI18nStore } from '../stores/i18n'
import { ApiRequestError } from '../api/client'

const auth = useAuthStore()
const settings = useSettingsStore()
const i18n = useI18nStore()
const router = useRouter()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const mode = ref<'login' | 'register'>('login')

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
      if (password.value.length < 8) throw new Error('密码至少 8 位')
      await auth.register(username.value, password.value)
    } else {
      await auth.login(username.value, password.value)
    }
    router.push({ name: 'dashboard' })
  } catch (e) {
    error.value = e instanceof ApiRequestError || e instanceof Error ? e.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-base-200 p-4">
    <button
      class="btn btn-ghost btn-sm min-w-16 fixed right-4 top-4"
      :title="i18n.t('语言')"
      @click="i18n.toggleLocale()"
    >
      <span aria-hidden="true">🌐</span>
      <span>{{ i18n.isEnglish ? '中' : 'EN' }}</span>
    </button>
    <div class="card w-full max-w-sm bg-base-100 shadow-xl">
      <div class="card-body">
        <h1 class="text-2xl font-bold text-center">{{ settings.appDisplayName }}</h1>
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
            <input
              v-model="password"
              type="password"
              class="input input-bordered"
              autocomplete="current-password"
              required
            />
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
