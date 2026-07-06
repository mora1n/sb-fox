<script setup lang="ts">
import { computed } from 'vue'
import { useI18nStore } from '../stores/i18n'

const props = defineProps<{
  open: boolean
  title: string
  itemLabel: string
  count: number
  itemName?: string
  loadingPreview?: boolean
  previewError?: string
  affectedNames?: string[]
  busy?: boolean
}>()

const emit = defineEmits<{
  close: []
  confirm: []
}>()

const i18n = useI18nStore()
const affected = computed(() => props.affectedNames ?? [])
const confirmDisabled = computed(() => props.busy || props.loadingPreview || !!props.previewError)
</script>

<template>
  <div v-if="open" class="modal modal-open">
    <div class="modal-box max-w-lg">
      <h3 class="font-bold text-lg">{{ title }}</h3>
      <p class="py-3 text-sm opacity-80">
        <template v-if="itemName">
          {{ i18n.t('确认删除') }} <span class="font-semibold break-all">「{{ itemName }}」</span>？
        </template>
        <template v-else>
          {{ i18n.t('确认删除') }} <span class="font-semibold">{{ count }}</span> {{ itemLabel }}？
        </template>
      </p>

      <div v-if="loadingPreview" class="alert bg-base-200 border border-base-300">
        <span class="loading loading-spinner loading-sm"></span>
        <span>{{ i18n.t('正在检查引用...') }}</span>
      </div>
      <div v-else-if="previewError" class="alert alert-error">
        <span>{{ previewError }}</span>
      </div>
      <div v-else-if="affected.length" class="alert alert-warning">
        <div>
          <div class="font-medium">{{ i18n.t('删除后可能影响以下订阅') }}</div>
          <div class="mt-2 flex flex-wrap gap-1">
            <span v-for="name in affected" :key="name" class="badge badge-sm badge-warning">{{ name }}</span>
          </div>
        </div>
      </div>
      <div v-else class="alert bg-base-200 border border-base-300">
        <span>{{ i18n.t('此操作不可撤销。') }}</span>
      </div>

      <div class="modal-action">
        <button type="button" class="btn" :disabled="busy" @click="emit('close')">{{ i18n.t('取消') }}</button>
        <button type="button" class="btn btn-error" :disabled="confirmDisabled" @click="emit('confirm')">
          <span v-if="busy" class="loading loading-spinner loading-sm"></span>
          {{ i18n.t('删除') }}
        </button>
      </div>
    </div>
    <div class="modal-backdrop" @click="!busy && emit('close')"></div>
  </div>
</template>
