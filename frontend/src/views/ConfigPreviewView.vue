<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useTemplatesStore } from '../stores/templates'
import { useNodesStore } from '../stores/nodes'
import { useSettingsStore } from '../stores/settings'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import { post } from '../api/client'
import type { KernelResult, Node, PreviewPayload, ProfileOptions } from '../api/types'
import NodeMultiSelect from '../components/NodeMultiSelect.vue'
import JsonViewer from '../components/JsonViewer.vue'
import ValidationBadge from '../components/ValidationBadge.vue'

const templates = useTemplatesStore()
const nodes = useNodesStore()
const settings = useSettingsStore()
const ui = useUiStore()
const i18n = useI18nStore()

const templateId = ref(0)
const nodeIds = ref<number[]>([])
const options = ref<ProfileOptions>({ autoCountryGroups: true, chainProxy: false, chainProxyNodeIds: [] })
const allNodes = ref<Node[]>([])
const kernelHint = computed(() => i18n.t('请选择有效 sing-box 内核或联系管理员配置内核'))
const chainProxyNodeIds = computed<number[]>({
  get: () => options.value.chainProxyNodeIds ?? [],
  set: (ids) => {
    options.value.chainProxyNodeIds = ids
  },
})

const config = ref('')
const validation = ref<KernelResult | null>(null)
const busy = ref(false)

onMounted(async () => {
  try {
    const [, loadedNodes] = await Promise.all([templates.fetchAll(), nodes.fetchUnfiltered(), settings.fetchKernelStatus()])
    allNodes.value = loadedNodes
    templateId.value = templates.templates[0]?.id || 0
  } catch (e) {
    ui.error(errMsg(e))
  }
})

watch(nodeIds, () => {
  const selected = new Set(nodeIds.value)
  options.value.chainProxyNodeIds = (options.value.chainProxyNodeIds ?? []).filter((id) => selected.has(id))
})

const chainProxyCandidates = computed(() => {
  const selected = new Set(nodeIds.value)
  return allNodes.value.filter((n) => selected.has(n.id))
})

function validateOptions() {
  if (!options.value.chainProxy) return
  const selected = options.value.chainProxyNodeIds ?? []
  if (!selected.length) throw new Error('请选择链式代理节点')
  if (selected.length >= nodeIds.value.length) throw new Error('链式代理需要至少一个上游节点')
}

async function generate() {
  if (!templateId.value) return ui.info('请选择模板')
  busy.value = true
  validation.value = null
  try {
    validateOptions()
    const payload: PreviewPayload = {
      template_id: templateId.value,
      node_ids: nodeIds.value,
      options: {
        ...options.value,
        chainProxyNodeIds: options.value.chainProxy ? [...(options.value.chainProxyNodeIds ?? [])] : [],
        chainProxyNodeId: 0,
      },
    }
    const r = await post<{ config: string }>('/generate/preview', payload)
    config.value = r.config
    ui.success('已生成配置')
  } catch (e) {
    ui.error(errMsg(e, '生成失败'))
  } finally {
    busy.value = false
  }
}

async function validate() {
  if (!config.value) return ui.info('请先生成配置')
  if (!settings.kernel?.available) return ui.info(kernelHint.value)
  busy.value = true
  try {
    validation.value = await post<KernelResult>('/generate/validate', { config: config.value })
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function format() {
  if (!config.value) return ui.info('请先生成配置')
  if (!settings.kernel?.available) return ui.info(kernelHint.value)
  busy.value = true
  try {
    const r = await post<KernelResult>('/generate/format', { config: config.value })
    if (r.status === 'ok' && r.formatted) {
      config.value = r.formatted
      ui.success('已格式化')
    } else if (r.status === 'unavailable') {
      ui.info('内核不可用，无法格式化')
    } else {
      ui.error('格式化失败: ' + (r.messages || '配置无效'))
    }
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-2xl font-bold">{{ i18n.t('预览生成') }}</h1>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div class="card bg-base-100 shadow-sm">
        <div class="card-body gap-3">
          <h2 class="card-title text-base">{{ i18n.t('输入') }}</h2>
          <div class="grid grid-cols-1 gap-3">
            <label class="form-control">
              <span class="label-text mb-1">{{ i18n.t('模板') }}</span>
              <select v-model.number="templateId" class="select select-bordered select-sm">
                <option :value="0" disabled>{{ i18n.t('选择模板') }}</option>
                <option v-for="t in templates.templates" :key="t.id" :value="t.id">{{ t.name }}</option>
              </select>
            </label>
          </div>
          <div class="flex gap-4">
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="options.autoCountryGroups" />
              <span class="label-text">{{ i18n.t('自动国家分组') }}</span>
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="options.chainProxy" />
              <span class="label-text">{{ i18n.t('链式代理') }}</span>
            </label>
          </div>
          <div class="form-control">
            <span class="label-text mb-1">{{ i18n.t('节点') }}</span>
            <NodeMultiSelect :nodes="allNodes" v-model="nodeIds" />
          </div>
          <div v-if="options.chainProxy" class="form-control">
            <span class="label-text mb-1">{{ i18n.t('链式代理节点') }}</span>
            <NodeMultiSelect :nodes="chainProxyCandidates" v-model="chainProxyNodeIds" />
          </div>
          <div class="flex gap-2">
            <button class="btn btn-primary btn-sm w-24 justify-center" @click="generate" :disabled="busy">
              <span v-if="busy" class="loading loading-spinner loading-sm"></span>
              <span>{{ i18n.t('生成') }}</span>
            </button>
            <button class="btn btn-sm" @click="validate" :disabled="busy" :class="{ 'opacity-50 cursor-not-allowed': !config || !settings.kernel?.available }" :title="settings.kernel?.available ? '' : kernelHint">{{ i18n.t('校验') }}</button>
            <button class="btn btn-sm" @click="format" :disabled="busy" :class="{ 'opacity-50 cursor-not-allowed': !config || !settings.kernel?.available }" :title="settings.kernel?.available ? '' : kernelHint">{{ i18n.t('格式化') }}</button>
          </div>
          <ValidationBadge :status="validation?.status ?? null" :messages="validation?.messages" />
        </div>
      </div>

      <div class="card bg-base-100 shadow-sm">
        <div class="card-body gap-2">
          <h2 class="card-title text-base">{{ i18n.t('配置输出') }}</h2>
          <JsonViewer v-if="config" :content="config" max-height="70vh" />
          <div v-else class="opacity-60 text-sm py-8 text-center">{{ i18n.t('点击「生成」查看配置。') }}</div>
        </div>
      </div>
    </div>
  </div>
</template>
