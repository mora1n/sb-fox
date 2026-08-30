<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { post } from '../api/client'
import { useProfilesStore } from '../stores/profiles'
import { usePublicTokenStore } from '../stores/publicToken'
import { useTemplatesStore } from '../stores/templates'
import { useNodesStore } from '../stores/nodes'
import { useNodeGroupsStore } from '../stores/nodeGroups'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { errMsg } from '../utils/error'
import type { Profile } from '../api/types'
import ProfileList from '../components/profiles/ProfileList.vue'
import ProfileConfigModal from '../components/profiles/ProfileConfigModal.vue'
import ProfileEditorModal from '../components/profiles/ProfileEditorModal.vue'

type EditorLaunchMode = 'create' | 'edit' | 'copy'

const store = useProfilesStore()
const publicToken = usePublicTokenStore()
const templates = useTemplatesStore()
const nodes = useNodesStore()
const nodeGroups = useNodeGroupsStore()
const settings = useSettingsStore()
const ui = useUiStore()

const editorMode = ref<EditorLaunchMode | null>(null)
const editorProfile = ref<Profile | null>(null)
const viewingProfile = ref<Profile | null>(null)
const viewingConfig = ref('')

onMounted(async () => {
  try {
    await Promise.all([store.fetchAll(), templates.fetchAll()])
    void publicToken.fetchToken().catch((e) => ui.error(errMsg(e)))
    void settings.fetchAppInfo().catch((e) => ui.error(errMsg(e)))
    window.setTimeout(() => {
      prefetchEditorData([], true)
    }, 0)
  } catch (e) {
    ui.error(errMsg(e))
  }
})

function openCreate() {
  prefetchCreateEditor()
  editorMode.value = 'create'
  editorProfile.value = null
}

function openEdit(profile: Profile) {
  prefetchProfileEditor(profile)
  editorMode.value = 'edit'
  editorProfile.value = profile
}

function openCopy(profile: Profile) {
  prefetchProfileEditor(profile)
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

function prefetchEditorData(templateIDs: number[] = [], includeProfileTemplates = false): void {
  const ids = new Set(templateIDs.filter(Boolean))
  if (includeProfileTemplates) {
    for (const profile of store.profiles) ids.add(profile.template_id)
  }
  if (templates.templates[0]?.id) ids.add(templates.templates[0].id)
  void Promise.all([
    templates.prefetchStructures([...ids]),
    nodes.fetchSummary(),
    nodeGroups.fetchAll(),
  ]).catch(() => undefined)
}

function prefetchCreateEditor(): void {
  prefetchEditorData(templates.templates[0]?.id ? [templates.templates[0].id] : [])
}

function prefetchProfileEditor(profile: Profile): void {
  prefetchEditorData([profile.template_id])
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <ProfileList
      @create="openCreate"
      @edit="openEdit"
      @copy="openCopy"
      @prefetch-create="prefetchCreateEditor"
      @prefetch-edit="prefetchProfileEditor"
      @prefetch-copy="prefetchProfileEditor"
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
