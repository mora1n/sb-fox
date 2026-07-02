<script setup lang="ts">
import { ref } from 'vue'
import { useNodesStore } from '../stores/nodes'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'

const emit = defineEmits<{ close: []; imported: [] }>()
const nodesStore = useNodesStore()
const ui = useUiStore()
const i18n = useI18nStore()

type Tab = 'links' | 'subscription' | 'config'
const tab = ref<Tab>('links')
const busy = ref(false)

const links = ref('')
const subName = ref('')
const subUrl = ref('')
const configText = ref('')

async function submit() {
  busy.value = true
  try {
    let count = 0
    if (tab.value === 'links') {
      if (!links.value.trim()) throw new Error('请粘贴分享链接')
      count = (await nodesStore.importLinks(links.value)).imported
    } else if (tab.value === 'subscription') {
      if (!subUrl.value.trim()) throw new Error('请填写订阅 URL')
      count = (await nodesStore.importSubscription(subName.value, subUrl.value)).imported
    } else {
      if (!configText.value.trim()) throw new Error('请粘贴 config JSON')
      count = (await nodesStore.importConfig(configText.value)).imported
    }
    ui.success(`成功导入 ${count} 个节点`)
    emit('imported')
    emit('close')
  } catch (e) {
    ui.error(errMsg(e, '导入失败'))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-2xl">
      <h3 class="font-bold text-lg mb-3">{{ i18n.t('导入节点') }}</h3>
      <div role="tablist" class="tabs tabs-boxed mb-4">
        <a role="tab" class="tab" :class="{ 'tab-active': tab === 'links' }" @click="tab = 'links'">{{ i18n.t('分享链接') }}</a>
        <a role="tab" class="tab" :class="{ 'tab-active': tab === 'subscription' }" @click="tab = 'subscription'">{{ i18n.t('订阅 URL') }}</a>
        <a role="tab" class="tab" :class="{ 'tab-active': tab === 'config' }" @click="tab = 'config'">Config JSON</a>
      </div>

      <div v-show="tab === 'links'">
        <p class="text-sm opacity-70 mb-2">{{ i18n.t('每行一个 ss/vmess/vless/trojan/hysteria2/tuic 链接，或粘贴 base64 订阅内容。') }}</p>
        <textarea v-model="links" class="textarea textarea-bordered w-full h-48 mono text-xs" placeholder="vmess://...&#10;ss://..."></textarea>
      </div>

      <div v-show="tab === 'subscription'" class="flex flex-col gap-3">
        <label class="form-control">
          <span class="label-text mb-1">{{ i18n.t('名称') }}</span>
          <input v-model="subName" class="input input-bordered" :placeholder="i18n.t('我的订阅')" />
        </label>
        <label class="form-control">
          <span class="label-text mb-1">订阅 URL</span>
          <input v-model="subUrl" class="input input-bordered" placeholder="https://example.com/sub" />
        </label>
        <p class="text-xs opacity-60">{{ i18n.t('服务端抓取，默认拒绝私网地址。') }}</p>
      </div>

      <div v-show="tab === 'config'">
        <p class="text-sm opacity-70 mb-2">{{ i18n.t('粘贴完整 config.json，将从 outbounds 中提取代理节点，跳过 selector/direct 等分组。') }}</p>
        <textarea v-model="configText" class="textarea textarea-bordered w-full h-48 mono text-xs" placeholder='{ "outbounds": [ ... ] }'></textarea>
      </div>

      <div class="modal-action">
        <button class="btn btn-ghost" @click="emit('close')" :disabled="busy">{{ i18n.t('取消') }}</button>
        <button class="btn btn-primary" @click="submit" :disabled="busy">
          <span v-if="busy" class="loading loading-spinner loading-sm"></span>
          {{ i18n.t('导入') }}
        </button>
      </div>
    </div>
    <div class="modal-backdrop" @click="emit('close')"></div>
  </div>
</template>
