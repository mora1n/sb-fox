<script setup lang="ts">
import { computed, ref } from 'vue'
import type { RuleSet, RuleSetPayload, RuleSetSourcePayload } from '../../api/types'
import { ApiRequestError } from '../../api/client'
import { useRuleSetsStore } from '../../stores/ruleSets'
import { useI18nStore } from '../../stores/i18n'
import { useUiStore } from '../../stores/ui'
import { errMsg } from '../../utils/error'
import { ArrowDownIcon, ArrowUpIcon, Bars3Icon, PlusIcon, TrashIcon } from '@heroicons/vue/24/outline'

type EditorMode = 'create' | 'edit' | 'copy'
type EditableSource = RuleSetSourcePayload & { key: number }

const props = defineProps<{ mode: EditorMode; item?: RuleSet }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const store = useRuleSetsStore()
const i18n = useI18nStore()
const ui = useUiStore()

let nextKey = 1
const name = ref(props.mode === 'copy' ? `${props.item?.name || ''}-copy` : props.item?.name || '')
const description = ref(props.item?.description || '')
const sources = ref<EditableSource[]>((props.item?.sources || []).map((source) => ({
  key: nextKey++,
  kind: source.kind,
  format: source.format,
  url: source.url || '',
  content: source.content || '',
})))
const busy = ref(false)
const failedSourceIndex = ref<number | null>(null)
const dragIndex = ref<number | null>(null)

if (!sources.value.length) addManualSource()

const title = computed(() => {
  if (props.mode === 'edit') return i18n.t('编辑规则集')
  if (props.mode === 'copy') return i18n.t('复制规则集')
  return i18n.t('新建规则集')
})

function addManualSource() {
  sources.value.push({ key: nextKey++, kind: 'manual', format: 'source', content: '' })
}

function addRemoteSource() {
  sources.value.push({ key: nextKey++, kind: 'remote', format: 'source', url: '' })
}

function removeSource(index: number) {
  sources.value.splice(index, 1)
  if (!sources.value.length) addManualSource()
}

function moveSource(index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= sources.value.length) return
  const [source] = sources.value.splice(index, 1)
  sources.value.splice(target, 0, source)
}

function dropSource(target: number) {
  if (dragIndex.value === null || dragIndex.value === target) return
  const [source] = sources.value.splice(dragIndex.value, 1)
  sources.value.splice(target, 0, source)
  dragIndex.value = null
}

function onFile(index: number, event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    sources.value[index].content = String(reader.result)
  }
  reader.readAsText(file)
}

function payload(): RuleSetPayload {
  const cleanName = name.value.trim()
  if (!cleanName) throw new Error(i18n.t('请填写规则集名称'))
  if (!sources.value.length) throw new Error(i18n.t('至少添加一个规则源'))
  return {
    name: cleanName,
    description: description.value.trim(),
    sources: sources.value.map((source, index) => {
      if (source.kind === 'manual') {
        const content = source.content?.trim() || ''
        if (!content) throw sourceError(index, i18n.t('请填写规则集 JSON'))
        try {
          JSON.parse(content)
        } catch (error) {
          throw sourceError(index, `${i18n.t('JSON 格式错误')}: ${errMsg(error)}`)
        }
        return { kind: 'manual', format: 'source', content }
      }
      const url = source.url?.trim() || ''
      if (!url) throw sourceError(index, i18n.t('请填写远程 URL'))
      return { kind: 'remote', format: source.format, url }
    }),
  }
}

function sourceError(index: number, message: string) {
  const error = new Error(message) as Error & { sourceIndex?: number }
  error.sourceIndex = index
  return error
}

async function submit() {
  failedSourceIndex.value = null
  busy.value = true
  try {
    const data = payload()
    if (props.mode === 'edit' && props.item) await store.update(props.item.id, data)
    else await store.create(data)
    ui.success(props.mode === 'edit' ? i18n.t('规则集已更新') : i18n.t('规则集已发布'))
    emit('saved')
  } catch (error) {
    if (error instanceof ApiRequestError && typeof error.details?.source_index === 'number') {
      failedSourceIndex.value = error.details.source_index
    } else if (error instanceof Error && typeof (error as Error & { sourceIndex?: number }).sourceIndex === 'number') {
      failedSourceIndex.value = (error as Error & { sourceIndex: number }).sourceIndex
    }
    ui.error(error instanceof ApiRequestError && error.details?.message ? error.details.message : errMsg(error))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-5xl max-h-[92vh] overflow-y-auto">
      <h3 class="font-bold text-lg mb-4">{{ title }}</h3>
      <div class="grid md:grid-cols-2 gap-3 mb-5">
        <label class="form-control">
          <span class="label-text mb-1">{{ i18n.t('名称') }}</span>
          <input v-model="name" class="input input-bordered" :disabled="busy" />
        </label>
        <label class="form-control">
          <span class="label-text mb-1">{{ i18n.t('描述') }}</span>
          <input v-model="description" class="input input-bordered" :disabled="busy" />
        </label>
      </div>

      <div class="flex items-center justify-between gap-2 mb-3">
        <div>
          <h4 class="font-semibold">{{ i18n.t('规则源') }}</h4>
          <p class="text-xs opacity-60">{{ i18n.t('按列表顺序合并；任一源失败会中止整次发布。') }}</p>
        </div>
        <div class="join">
          <button class="btn btn-sm join-item" type="button" :disabled="busy" @click="addManualSource">
            <PlusIcon class="h-4 w-4" /> {{ i18n.t('手工 JSON') }}
          </button>
          <button class="btn btn-sm join-item" type="button" :disabled="busy" @click="addRemoteSource">
            <PlusIcon class="h-4 w-4" /> {{ i18n.t('远程源') }}
          </button>
        </div>
      </div>

      <div class="space-y-3">
        <section
          v-for="(source, index) in sources"
          :key="source.key"
          draggable="true"
          class="rounded-box border bg-base-100 p-3"
          :class="failedSourceIndex === index ? 'border-error ring-1 ring-error' : 'border-base-300'"
          @dragstart="dragIndex = index"
          @dragover.prevent
          @drop="dropSource(index)"
        >
          <div class="flex items-center gap-2 mb-3">
            <Bars3Icon class="h-5 w-5 opacity-50 cursor-grab" />
            <span class="badge badge-neutral">{{ index + 1 }}</span>
            <span class="font-medium">{{ source.kind === 'manual' ? i18n.t('手工 JSON') : i18n.t('远程源') }}</span>
            <div class="flex-1"></div>
            <button class="btn btn-xs btn-ghost" :disabled="busy || index === 0" @click="moveSource(index, -1)"><ArrowUpIcon class="h-4 w-4" /></button>
            <button class="btn btn-xs btn-ghost" :disabled="busy || index === sources.length - 1" @click="moveSource(index, 1)"><ArrowDownIcon class="h-4 w-4" /></button>
            <button class="btn btn-xs btn-ghost text-error" :disabled="busy" @click="removeSource(index)"><TrashIcon class="h-4 w-4" /></button>
          </div>

          <template v-if="source.kind === 'manual'">
            <div class="flex items-center justify-between mb-2">
              <span class="text-xs opacity-60">sing-box source-format JSON</span>
              <input type="file" accept=".json,application/json" class="file-input file-input-bordered file-input-xs max-w-xs" :disabled="busy" @change="onFile(index, $event)" />
            </div>
            <textarea v-model="source.content" class="textarea textarea-bordered mono text-xs w-full min-h-52" :disabled="busy" spellcheck="false"></textarea>
          </template>
          <template v-else>
            <div class="grid md:grid-cols-[10rem_1fr] gap-3">
              <label class="form-control">
                <span class="label-text mb-1">{{ i18n.t('格式') }}</span>
                <select v-model="source.format" class="select select-bordered" :disabled="busy">
                  <option value="source">Source JSON</option>
                  <option value="binary">Binary SRS</option>
                </select>
              </label>
              <label class="form-control">
                <span class="label-text mb-1">URL</span>
                <input v-model="source.url" class="input input-bordered mono text-sm" placeholder="https://example.com/rules.srs" :disabled="busy" />
              </label>
            </div>
          </template>
        </section>
      </div>

      <div class="modal-action">
        <button class="btn" :disabled="busy" @click="emit('close')">{{ i18n.t('取消') }}</button>
        <button class="btn btn-primary" :disabled="busy" @click="submit">
          <span v-if="busy" class="loading loading-spinner loading-sm"></span>
          {{ i18n.t('保存') }}
        </button>
      </div>
    </div>
    <div class="modal-backdrop" @click="!busy && emit('close')"></div>
  </div>
</template>
