<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useTemplatesStore } from '../stores/templates'
import { useUiStore } from '../stores/ui'
import { errMsg } from '../utils/error'
import type { ProxyGroup, Template } from '../api/types'
import JsonViewer from '../components/JsonViewer.vue'
import ProxyGroupCard from '../components/ProxyGroupCard.vue'
import { PlusIcon, EyeIcon, RectangleGroupIcon, PencilSquareIcon, TrashIcon } from '@heroicons/vue/24/outline'

const store = useTemplatesStore()
const ui = useUiStore()

const viewing = ref<Template | null>(null)
const groups = ref<ProxyGroup[] | null>(null)
const groupsFor = ref<Template | null>(null)

// import / edit modal
const showForm = ref(false)
const editing = ref<Template | null>(null)
const formName = ref('')
const formDesc = ref('')
const formContent = ref('')
const busy = ref(false)

onMounted(load)
async function load() {
  try {
    await store.fetchAll()
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function view(t: Template) {
  try {
    viewing.value = await store.getOne(t.id)
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function inspect(t: Template) {
  try {
    groups.value = (await store.inspect(t.id)).groups
    groupsFor.value = t
  } catch (e) {
    ui.error(errMsg(e))
  }
}

function openImport() {
  editing.value = null
  formName.value = ''
  formDesc.value = ''
  formContent.value = ''
  showForm.value = true
}
async function openEdit(t: Template) {
  editing.value = t
  try {
    const full = await store.getOne(t.id)
    formName.value = full.name
    formDesc.value = full.description
    formContent.value = full.content
    showForm.value = true
  } catch (e) {
    ui.error(errMsg(e))
  }
}

function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  const reader = new FileReader()
  reader.onload = () => {
    formContent.value = String(reader.result)
    if (!formName.value) formName.value = f.name.replace(/\.json$/i, '')
  }
  reader.readAsText(f)
}

async function submitForm() {
  busy.value = true
  try {
    JSON.parse(formContent.value) // client-side validation
    if (editing.value) {
      await store.update(editing.value.id, formContent.value, formDesc.value)
      ui.success('模板已更新')
    } else {
      if (!formName.value.trim()) throw new Error('请填写模板名称')
      await store.create(formName.value, formContent.value, formDesc.value)
      ui.success('模板已导入')
    }
    showForm.value = false
  } catch (e) {
    ui.error(e instanceof SyntaxError ? 'content 不是合法 JSON' : errMsg(e))
  } finally {
    busy.value = false
  }
}

async function remove(t: Template) {
  if (!confirm(`删除模板 "${t.name}"？`)) return
  try {
    await store.remove(t.id)
    ui.success('模板已删除')
  } catch (e) {
    ui.error(errMsg(e))
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold">模板</h1>
      <button class="btn btn-sm btn-primary" @click="openImport"><PlusIcon class="h-4 w-4" /> 导入模板</button>
    </div>

    <div v-if="store.loading" class="flex justify-center py-10"><span class="loading loading-spinner loading-lg"></span></div>
    <div v-else class="overflow-x-auto card bg-base-100 shadow-sm">
      <table class="table">
        <thead>
          <tr><th>名称</th><th>类型</th><th>描述</th><th class="text-right">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="t in store.templates" :key="t.id">
            <td class="font-semibold">{{ t.name }}</td>
            <td>
              <span class="badge badge-sm" :class="t.kind === 'builtin' ? 'badge-neutral' : 'badge-primary'">{{ t.kind }}</span>
            </td>
            <td class="text-sm opacity-70 max-w-xs truncate">{{ t.description }}</td>
            <td>
              <div class="flex gap-1 justify-end">
                <button class="btn btn-xs btn-ghost" @click="view(t)" title="查看"><EyeIcon class="h-4 w-4" /></button>
                <button class="btn btn-xs btn-ghost" @click="inspect(t)" title="出口分组"><RectangleGroupIcon class="h-4 w-4" /></button>
                <button v-if="t.kind === 'user'" class="btn btn-xs btn-ghost" @click="openEdit(t)" title="编辑"><PencilSquareIcon class="h-4 w-4" /></button>
                <button v-if="t.kind === 'user'" class="btn btn-xs btn-ghost text-error" @click="remove(t)" title="删除"><TrashIcon class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- view content modal -->
    <div v-if="viewing" class="modal modal-open">
      <div class="modal-box max-w-3xl">
        <h3 class="font-bold text-lg mb-3">{{ viewing.name }}</h3>
        <JsonViewer :content="viewing.content" />
        <div class="modal-action"><button class="btn" @click="viewing = null">关闭</button></div>
      </div>
      <div class="modal-backdrop" @click="viewing = null"></div>
    </div>

    <!-- inspect groups modal -->
    <div v-if="groups" class="modal modal-open">
      <div class="modal-box max-w-2xl">
        <h3 class="font-bold text-lg mb-3">出口分组 — {{ groupsFor?.name }}</h3>
        <div v-if="!groups.length" class="opacity-60 text-sm">未检测到 selector/urltest 分组。</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <ProxyGroupCard v-for="(g, i) in groups" :key="i" :group="g" />
        </div>
        <div class="modal-action"><button class="btn" @click="groups = null">关闭</button></div>
      </div>
      <div class="modal-backdrop" @click="groups = null"></div>
    </div>

    <!-- import / edit form modal -->
    <div v-if="showForm" class="modal modal-open">
      <div class="modal-box max-w-2xl">
        <h3 class="font-bold text-lg mb-3">{{ editing ? '编辑模板' : '导入模板' }}</h3>
        <div class="flex flex-col gap-3">
          <label v-if="!editing" class="form-control">
            <span class="label-text mb-1">名称</span>
            <input v-model="formName" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">描述</span>
            <input v-model="formDesc" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">上传 JSON 文件（可选）</span>
            <input type="file" accept=".json,application/json" class="file-input file-input-bordered file-input-sm" @change="onFile" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">模板内容 (JSON)</span>
            <textarea v-model="formContent" class="textarea textarea-bordered h-56 mono text-xs" placeholder='{ "outbounds": [ ... ] }'></textarea>
          </label>
        </div>
        <div class="modal-action">
          <button class="btn btn-ghost" @click="showForm = false" :disabled="busy">取消</button>
          <button class="btn btn-primary" @click="submitForm" :disabled="busy">
            <span v-if="busy" class="loading loading-spinner loading-sm"></span> 保存
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="showForm = false"></div>
    </div>
  </div>
</template>
