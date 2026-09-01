<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { RuleSet } from '../api/types'
import { useRuleSetsStore } from '../stores/ruleSets'
import { usePublicTokenStore } from '../stores/publicToken'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { errMsg } from '../utils/error'
import RuleSetList from '../components/rulesets/RuleSetList.vue'
import RuleSetConfigModal from '../components/rulesets/RuleSetConfigModal.vue'
import RuleSetEditorModal from '../components/rulesets/RuleSetEditorModal.vue'

type EditorMode = 'create' | 'edit' | 'copy'

const store = useRuleSetsStore()
const publicToken = usePublicTokenStore()
const settings = useSettingsStore()
const ui = useUiStore()
const editorMode = ref<EditorMode | null>(null)
const editorItem = ref<RuleSet | undefined>()
const viewingItem = ref<RuleSet | undefined>()
const viewingJSON = ref('')

onMounted(async () => {
  try {
    await Promise.all([store.fetchAll(), publicToken.fetchToken(), settings.fetchAppInfo()])
  } catch (error) {
    ui.error(errMsg(error))
  }
})

function openCreate() {
  editorMode.value = 'create'
  editorItem.value = undefined
}

async function openEditor(mode: 'edit' | 'copy', summary: RuleSet) {
  try {
    editorItem.value = await store.getOne(summary.id)
    editorMode.value = mode
  } catch (error) {
    ui.error(errMsg(error))
  }
}

function closeEditor() {
  editorMode.value = null
  editorItem.value = undefined
}

async function viewPublishedJSON(item: RuleSet) {
  viewingItem.value = undefined
  viewingJSON.value = ''
  try {
    viewingJSON.value = await store.publishedJSON(item.id)
    viewingItem.value = item
  } catch (error) {
    ui.error(errMsg(error))
  }
}

function closeJSONView() {
  viewingItem.value = undefined
  viewingJSON.value = ''
}
</script>

<template>
  <RuleSetList @create="openCreate" @view="viewPublishedJSON" @edit="openEditor('edit', $event)" @copy="openEditor('copy', $event)" />
  <RuleSetConfigModal v-if="viewingItem" :item="viewingItem" :content="viewingJSON" @close="closeJSONView" />
  <RuleSetEditorModal v-if="editorMode" :mode="editorMode" :item="editorItem" @close="closeEditor" @saved="closeEditor" />
</template>
