<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import { ApiRequestError } from '../api/client'

const auth = useAuthStore()
const settings = useSettingsStore()
const router = useRouter()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

onMounted(async () => {
  // if already logged in, skip to dashboard
  if (!auth.checked) await auth.me()
  if (auth.isAuthenticated) router.push({ name: 'dashboard' })
})

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    router.push({ name: 'dashboard' })
  } catch (e) {
    error.value = e instanceof ApiRequestError ? e.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-base-200 p-4">
    <div class="card w-full max-w-sm bg-base-100 shadow-xl">
      <div class="card-body">
        <h1 class="text-2xl font-bold text-center">{{ settings.appDisplayName }}</h1>
        <p class="text-center text-sm opacity-60 mb-2">sing-box 配置订阅管理</p>
        <div v-if="error" class="alert alert-error text-sm py-2">
          <span>{{ error }}</span>
        </div>
        <form @submit.prevent="submit" class="flex flex-col gap-3">
          <label class="form-control">
            <span class="label-text mb-1">用户名</span>
            <input v-model="username" class="input input-bordered" autocomplete="username" required />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">密码</span>
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
            登录
          </button>
        </form>
      </div>
    </div>
  </div>
</template>
