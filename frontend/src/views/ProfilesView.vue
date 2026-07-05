<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { post } from '../api/client'
import { useProfilesStore } from '../stores/profiles'
import { useTemplatesStore } from '../stores/templates'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { errMsg } from '../utils/error'
import type { Profile } from '../api/types'
import ProfileList from '../components/profiles/ProfileList.vue'
import ProfileConfigModal from '../components/profiles/ProfileConfigModal.vue'
import ProfileEditorModal from '../components/profiles/ProfileEditorModal.vue'

type EditorLaunchMode = 'create' | 'edit' | 'copy'

const store = useProfilesStore()
const templates = useTemplatesStore()
const settings = useSettingsStore()
const ui = useUiStore()

const editorMode = ref<EditorLaunchMode | null>(null)
const editorProfile = ref<Profile | null>(null)
const viewingProfile = ref<Profile | null>(null)
const viewingConfig = ref('')

onMounted(async () => {
  try {
    await Promise.all([
      store.fetchAll(),
      store.fetchSubscriptionToken(),
      templates.fetchAll(),
      settings.fetchAppInfo(),
    ])
  } catch (e) {
    ui.error(errMsg(e))
  }
})

function openCreate() {
  editorMode.value = 'create'
  editorProfile.value = null
}

function openEdit(profile: Profile) {
  editorMode.value = 'edit'
  editorProfile.value = profile
}

function openCopy(profile: Profile) {
  editorMode.value = 'copy'
  editorProfile.value = profile
}

function closeEditor() {
  editorMode.value = null
  editorProfile.value = null
}

async function viewProfileConfig(profile: Profile) {
  viewingProfile.value = null
  viewingConfig.value = ''
  try {
    const r = await post<{ config: string }>('/generate/preview', { profile_id: profile.id })
    viewingProfile.value = profile
    viewingConfig.value = r.config
  } catch (e) {
    ui.error(errMsg(e, '生成失败'))
  }
}

function closeProfileConfigView() {
  viewingProfile.value = null
  viewingConfig.value = ''
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <ProfileList
      @create="openCreate"
      @edit="openEdit"
      @copy="openCopy"
      @view-config="viewProfileConfig"
    />
    <ProfileConfigModal
      v-if="viewingProfile"
      :profile="viewingProfile"
      :config="viewingConfig"
      @close="closeProfileConfigView"
    />
    <ProfileEditorModal
      v-if="editorMode"
      :mode="editorMode"
      :profile="editorProfile"
      @close="closeEditor"
      @saved="closeEditor"
    />
  </div>
</template>
