<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { useSettingsStore } from '../stores/settings'
import { useI18nStore } from '../stores/i18n'
import {
  HomeIcon,
  ServerStackIcon,
  DocumentTextIcon,
  UserGroupIcon,
  UsersIcon,
  Cog6ToothIcon,
  ArrowRightOnRectangleIcon,
  MoonIcon,
  SunIcon,
} from '@heroicons/vue/24/outline'

const auth = useAuthStore()
const ui = useUiStore()
const settings = useSettingsStore()
const i18n = useI18nStore()
const router = useRouter()

const nav = computed(() => [
  { name: 'dashboard', label: i18n.t('仪表盘'), icon: HomeIcon },
  { name: 'nodes', label: i18n.t('节点'), icon: ServerStackIcon },
  { name: 'templates', label: i18n.t('模板'), icon: DocumentTextIcon },
  { name: 'profiles', label: i18n.t('订阅'), icon: UserGroupIcon },
  { name: 'settings', label: i18n.t('设置'), icon: Cog6ToothIcon },
])
const bottomNav = computed(() => (auth.isAdmin ? [{ name: 'users', label: i18n.t('用户'), icon: UsersIcon }] : []))

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
        <div class="flex-1"></div>
        <div class="flex-none gap-2">
          <button class="btn btn-ghost btn-sm min-w-16" :title="i18n.t('语言')" @click="i18n.toggleLocale()">
            <span aria-hidden="true">🌐</span>
            <span>{{ i18n.isEnglish ? '中' : 'EN' }}</span>
          </button>
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
      <aside class="w-60 min-h-full bg-base-100 border-r border-base-300 flex flex-col">
        <div class="p-4 text-xl font-bold flex items-center gap-2 min-w-0">
          <img src="/favicon-32x32.png" :alt="settings.appInfoLoaded ? settings.appDisplayName : ''" class="h-8 w-8 flex-none" />
          <span v-if="settings.appInfoLoaded" class="truncate">{{ settings.appDisplayName }}</span>
          <span v-else class="inline-block h-6 w-24 rounded bg-base-300"></span>
        </div>
        <ul class="menu px-2 gap-1 flex-1">
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
        <ul v-if="bottomNav.length" class="menu px-2 pb-3 gap-1">
          <li v-for="item in bottomNav" :key="item.name">
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
