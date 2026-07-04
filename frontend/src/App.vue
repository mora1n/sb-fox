<script setup lang="ts">
import { onMounted, watchEffect } from 'vue'
import { RouterView } from 'vue-router'
import Toasts from './components/Toasts.vue'
import { useSettingsStore } from './stores/settings'
import { useUiStore } from './stores/ui'
import { errMsg } from './utils/error'

const settings = useSettingsStore()
const ui = useUiStore()

onMounted(() => {
  settings.fetchAppInfo().catch((e) => ui.error(errMsg(e)))
})

watchEffect(() => {
  if (!settings.appInfoLoaded) return
  document.title = settings.appDisplayName
})
</script>

<template>
  <RouterView />
  <Toasts />
</template>
