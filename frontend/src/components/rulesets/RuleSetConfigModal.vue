<script setup lang="ts">
import type { RuleSet } from '../../api/types'
import { useI18nStore } from '../../stores/i18n'
import JsonViewer from '../JsonViewer.vue'

defineProps<{ item: RuleSet; content: string }>()
const emit = defineEmits<{ close: [] }>()
const i18n = useI18nStore()
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-5xl">
      <h3 class="font-bold text-lg mb-3 truncate" :title="item.name">{{ item.name }}</h3>
      <JsonViewer :content="content" max-height="70vh" />
      <div class="modal-action">
        <button class="btn" @click="emit('close')">{{ i18n.t('关闭') }}</button>
      </div>
    </div>
    <div class="modal-backdrop" @click="emit('close')"></div>
  </div>
</template>
