<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useProfilesStore } from '../stores/profiles'
import { useTemplatesStore } from '../stores/templates'
import { useNodesStore } from '../stores/nodes'
import { useUiStore } from '../stores/ui'
import { errMsg } from '../utils/error'
import type { Profile, ProfileOptions, ProfilePayload } from '../api/types'
import TokenLinkField from '../components/TokenLinkField.vue'
import NodeMultiSelect from '../components/NodeMultiSelect.vue'
import { PlusIcon, PencilSquareIcon, TrashIcon, ArrowPathIcon } from '@heroicons/vue/24/outline'

const store = useProfilesStore()
const templates = useTemplatesStore()
const nodes = useNodesStore()
const ui = useUiStore()

const showForm = ref(false)
const editing = ref<Profile | null>(null)
const busy = ref(false)

const form = ref<{
  name: string
  template_id: number
  node_ids: number[]
  options: ProfileOptions
}>({ name: '', template_id: 0, node_ids: [], options: { autoCountryGroups: true, chainProxy: false } })

onMounted(async () => {
  try {
    await Promise.all([store.fetchAll(), templates.fetchAll(), nodes.fetchAll()])
  } catch (e) {
    ui.error(errMsg(e))
  }
})

function parseOptions(s: string): ProfileOptions {
  try {
    const o = JSON.parse(s || '{}')
    return { autoCountryGroups: !!o.autoCountryGroups, chainProxy: !!o.chainProxy }
  } catch {
    return { autoCountryGroups: false, chainProxy: false }
  }
}

function templateName(id: number) {
  return templates.templates.find((t) => t.id === id)?.name || `#${id}`
}

function openCreate() {
  editing.value = null
  form.value = {
    name: '',
    template_id: templates.templates[0]?.id || 0,
    node_ids: [],
    options: { autoCountryGroups: true, chainProxy: false },
  }
  showForm.value = true
}
function openEdit(p: Profile) {
  editing.value = p
  form.value = {
    name: p.name,
    template_id: p.template_id,
    node_ids: [...p.node_ids],
    options: parseOptions(p.options),
  }
  showForm.value = true
}

async function submit() {
  busy.value = true
  try {
    if (!form.value.name.trim()) throw new Error('请填写名称')
    if (!form.value.template_id) throw new Error('请选择模板')
    const payload: ProfilePayload = { ...form.value }
    if (editing.value) {
      await store.update(editing.value.id, payload)
      ui.success('分组已更新')
    } else {
      await store.create(payload)
      ui.success('分组已创建')
    }
    showForm.value = false
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function rotate(p: Profile) {
  if (!confirm('轮换 token 后旧订阅链接立即失效，确认？')) return
  try {
    await store.rotateToken(p.id)
    ui.success('token 已轮换')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function remove(p: Profile) {
  if (!confirm(`删除分组 "${p.name}"？`)) return
  try {
    await store.remove(p.id)
    ui.success('分组已删除')
  } catch (e) {
    ui.error(errMsg(e))
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold">订阅分组</h1>
      <button class="btn btn-sm btn-primary" @click="openCreate"><PlusIcon class="h-4 w-4" /> 新建分组</button>
    </div>

    <div v-if="store.loading" class="flex justify-center py-10"><span class="loading loading-spinner loading-lg"></span></div>
    <div v-else-if="!store.profiles.length" class="text-center py-10 opacity-60">暂无分组。</div>
    <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div v-for="p in store.profiles" :key="p.id" class="card bg-base-100 shadow-sm">
        <div class="card-body p-4 gap-3">
          <div class="flex items-start justify-between">
            <div>
              <h2 class="card-title text-base">{{ p.name }}</h2>
              <div class="text-xs opacity-70 mt-1 flex flex-wrap gap-2">
                <span>模板: {{ templateName(p.template_id) }}</span>
                <span>{{ p.node_ids.length }} 节点</span>
              </div>
            </div>
            <div class="flex gap-1">
              <button class="btn btn-xs btn-ghost" @click="openEdit(p)"><PencilSquareIcon class="h-4 w-4" /></button>
              <button class="btn btn-xs btn-ghost text-error" @click="remove(p)"><TrashIcon class="h-4 w-4" /></button>
            </div>
          </div>
          <div class="flex flex-wrap gap-1">
            <span v-if="parseOptions(p.options).autoCountryGroups" class="badge badge-sm badge-success">自动国家分组</span>
            <span v-if="parseOptions(p.options).chainProxy" class="badge badge-sm badge-info">链式代理</span>
          </div>
          <TokenLinkField :token="p.token" />
          <div class="flex items-center justify-between">
            <span class="text-xs opacity-60">公开订阅链接</span>
            <button class="btn btn-xs btn-ghost" @click="rotate(p)"><ArrowPathIcon class="h-3 w-3" /> 轮换 token</button>
          </div>
        </div>
      </div>
    </div>

    <!-- create / edit modal -->
    <div v-if="showForm" class="modal modal-open">
      <div class="modal-box max-w-2xl">
        <h3 class="font-bold text-lg mb-3">{{ editing ? '编辑分组' : '新建分组' }}</h3>
        <div class="flex flex-col gap-3">
          <label class="form-control">
            <span class="label-text mb-1">名称</span>
            <input v-model="form.name" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">模板</span>
            <select v-model.number="form.template_id" class="select select-bordered select-sm">
              <option :value="0" disabled>选择模板</option>
              <option v-for="t in templates.templates" :key="t.id" :value="t.id">{{ t.name }} ({{ t.kind }})</option>
            </select>
          </label>
          <div class="flex gap-4">
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="form.options.autoCountryGroups" />
              <span class="label-text">自动国家分组</span>
            </label>
            <label class="label cursor-pointer justify-start gap-2">
              <input type="checkbox" class="toggle toggle-sm" v-model="form.options.chainProxy" />
              <span class="label-text">链式代理</span>
            </label>
          </div>
          <div class="form-control">
            <span class="label-text mb-1">节点</span>
            <NodeMultiSelect :nodes="nodes.nodes" v-model="form.node_ids" />
          </div>
        </div>
        <div class="modal-action">
          <button class="btn btn-ghost" @click="showForm = false" :disabled="busy">取消</button>
          <button class="btn btn-primary" @click="submit" :disabled="busy">
            <span v-if="busy" class="loading loading-spinner loading-sm"></span> 保存
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="showForm = false"></div>
    </div>
  </div>
</template>
