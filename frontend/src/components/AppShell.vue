<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { useSettingsStore } from '../stores/settings'
import {
  HomeIcon,
  ServerStackIcon,
  DocumentTextIcon,
  UserGroupIcon,
  UsersIcon,
  EyeIcon,
  Cog6ToothIcon,
  ArrowRightOnRectangleIcon,
  MoonIcon,
  SunIcon,
} from '@heroicons/vue/24/outline'

const auth = useAuthStore()
const ui = useUiStore()
const settings = useSettingsStore()
const router = useRouter()

const nav = computed(() => [
  { name: 'dashboard', label: '仪表盘', icon: HomeIcon },
  { name: 'nodes', label: '节点', icon: ServerStackIcon },
  { name: 'templates', label: '模板', icon: DocumentTextIcon },
  { name: 'profiles', label: '订阅分组', icon: UserGroupIcon },
  { name: 'preview', label: '预览生成', icon: EyeIcon },
  ...(auth.isAdmin ? [{ name: 'users', label: '用户', icon: UsersIcon }] : []),
  { name: 'settings', label: '设置', icon: Cog6ToothIcon },
])

async function logout() {
  await auth.logout()
  ui.success('已退出登录')
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="drawer lg:drawer-open min-h-screen">
    <input id="app-drawer" type="checkbox" class="drawer-toggle" />
    <div class="drawer-content flex flex-col">
      <!-- Navbar -->
      <div class="navbar bg-base-100 border-b border-base-300 sticky top-0 z-30">
        <div class="flex-none lg:hidden">
          <label for="app-drawer" class="btn btn-square btn-ghost">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" /></svg>
          </label>
        </div>
        <div class="flex-1">
          <span class="text-lg font-bold px-2">{{ settings.appDisplayName }}</span>
        </div>
        <div class="flex-none gap-2">
          <button class="btn btn-ghost btn-sm" @click="ui.toggleTheme()">
            <MoonIcon v-if="ui.theme === 'light-neutral'" class="h-5 w-5" />
            <SunIcon v-else class="h-5 w-5" />
          </button>
          <span class="text-sm opacity-70 hidden sm:inline">{{ auth.username }}</span>
          <button class="btn btn-ghost btn-sm" @click="logout">
            <ArrowRightOnRectangleIcon class="h-5 w-5" />
          </button>
        </div>
      </div>
      <main class="p-4 md:p-6 flex-1 bg-base-200 min-h-0">
        <RouterView />
      </main>
    </div>
    <div class="drawer-side z-40">
      <label for="app-drawer" class="drawer-overlay"></label>
      <aside class="w-60 min-h-full bg-base-100 border-r border-base-300">
        <div class="p-4 text-xl font-bold">{{ settings.appDisplayName }}</div>
        <ul class="menu px-2 gap-1">
          <li v-for="item in nav" :key="item.name">
            <RouterLink
              :to="{ name: item.name }"
              :class="{ active: router.currentRoute.value.name === item.name }"
            >
              <component :is="item.icon" class="h-5 w-5" />
              {{ item.label }}
            </RouterLink>
          </li>
        </ul>
      </aside>
    </div>
  </div>
</template>
