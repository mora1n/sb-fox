<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18nStore } from '../stores/i18n'
import { useScrollPreserver } from '../utils/scrollPreserver'

const props = defineProps<{
  open: boolean
  title: string
  itemLabel: string
  count: number
  itemName?: string
  loadingPreview?: boolean
  previewError?: string
  affectedNames?: string[]
  deleteNodes?: boolean
  deleteNodeCount?: number
  busy?: boolean
}>()

const emit = defineEmits<{
  close: []
  confirm: []
  'update:deleteNodes': [boolean]
}>()

const i18n = useI18nStore()
const affected = computed(() => props.affectedNames ?? [])
const confirmDisabled = computed(() => props.busy || props.loadingPreview || !!props.previewError)
const modalScroller = ref<HTMLElement | null>(null)
const { preserveScroll } = useScrollPreserver(() => [modalScroller.value])

function setDeleteNodes(value: boolean) {
  preserveScroll(() => emit('update:deleteNodes', value))
}
</script>

<template>
  <div v-if="open" class="modal modal-open">
    <div ref="modalScroller" class="modal-box max-w-lg">
      <h3 class="font-bold text-lg">{{ title }}</h3>
      <p class="py-3 text-sm opacity-80">
        <template v-if="itemName">
          {{ i18n.t('确认删除') }} <span class="font-semibold break-all">「{{ itemName }}」</span>？
        </template>
        <template v-else>
          {{ i18n.t('确认删除') }} <span class="font-semibold">{{ count }}</span> {{ itemLabel }}？
        </template>
      </p>

      <label v-if="deleteNodeCount" class="label cursor-pointer justify-start gap-2 py-1">
        <input
          type="checkbox"
          class="toggle toggle-sm"
          :checked="deleteNodes"
          :disabled="busy"
          @change="setDeleteNodes(($event.target as HTMLInputElement).checked)"
        />
        <span class="label-text">
          {{ i18n.t('同时删除组合中的单节点') }}（{{ deleteNodeCount }} {{ i18n.t('个节点') }}）
        </span>
      </label>
      <div v-if="deleteNodes && deleteNodeCount" class="alert alert-warning text-sm">
        <span>{{ i18n.t('勾选后将删除所选组合中的全部单节点，其他组合和订阅中的相关引用也会自动清理。') }}</span>
      </div>

      <div v-if="loadingPreview" class="alert bg-base-200 border border-base-300">
        <span class="loading loading-spinner loading-sm"></span>
        <span>{{ i18n.t('正在检查引用...') }}</span>
      </div>
      <div v-else-if="previewError" class="alert alert-error">
        <span>{{ previewError }}</span>
      </div>
      <div v-else-if="affected.length" class="alert alert-warning">
        <div>
          <div class="font-medium">{{ i18n.t('节点引用将从组合节点和以下订阅中自动清理') }}</div>
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
