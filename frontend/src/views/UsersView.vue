<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useUsersStore, type UserPayload } from '../stores/users'
import { useUiStore } from '../stores/ui'
import { useI18nStore } from '../stores/i18n'
import { errMsg } from '../utils/error'
import type { User } from '../api/types'
import { PlusIcon, PencilSquareIcon, TrashIcon, KeyIcon, ClipboardDocumentIcon } from '@heroicons/vue/24/outline'

const store = useUsersStore()
const ui = useUiStore()
const i18n = useI18nStore()
const showEdit = ref(false)
const editing = ref<User | null>(null)
const busy = ref(false)
const resetPassword = ref('')
const resetUsername = ref('')
const form = ref<UserPayload>({
  username: '',
  password: '',
  role: 'user',
  node_limit: 0,
  profile_limit: 0,
  template_limit: 0,
})

onMounted(async () => {
  try {
    await store.fetchAll()
  } catch (e) {
    ui.error(errMsg(e))
  }
})

function openCreate() {
  editing.value = null
  form.value = { username: '', password: '', role: 'user', node_limit: 0, profile_limit: 0, template_limit: 0 }
  showEdit.value = true
}

function openEdit(user: User) {
  editing.value = user
  form.value = {
    username: user.username,
    role: user.role,
    node_limit: user.node_limit,
    profile_limit: user.profile_limit,
    template_limit: user.template_limit,
  }
  showEdit.value = true
}

async function save() {
  busy.value = true
  try {
    const payload = { ...form.value }
    if (editing.value) {
      await store.update(editing.value.id, payload)
      ui.success('用户已更新')
    } else {
      if (!payload.password || payload.password.length < 8) throw new Error('密码至少 8 位')
      await store.create(payload)
      ui.success('用户已创建')
    }
    showEdit.value = false
  } catch (e) {
    ui.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

async function remove(user: User) {
  if (!confirm(`删除用户 "${user.username}"？该用户的数据也会删除。`)) return
  try {
    await store.remove(user.id)
    ui.success('用户已删除')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function reset(user: User) {
  if (!confirm(`重置用户 "${user.username}" 的密码？`)) return
  try {
    resetPassword.value = await store.resetPassword(user.id)
    resetUsername.value = user.username
    ui.success('密码已重置')
  } catch (e) {
    ui.error(errMsg(e))
  }
}

async function copyResetPassword() {
  try {
    await navigator.clipboard.writeText(resetPassword.value)
    ui.success(i18n.t('复制成功'))
  } catch {
    ui.error(i18n.t('复制失败'))
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between gap-3">
      <h1 class="text-2xl font-bold">{{ i18n.t('用户') }}</h1>
      <button class="btn btn-sm btn-primary" @click="openCreate">
        <PlusIcon class="h-4 w-4" /> {{ i18n.t('新建用户') }}
      </button>
    </div>

    <div v-if="resetPassword" class="alert alert-info">
      <div class="min-w-0 flex-1">
        <div class="font-semibold">
          {{ i18n.isEnglish ? resetUsername + i18n.t('的新密码') : resetUsername + ' 的新密码' }}
        </div>
        <div class="flex items-center gap-2 min-w-0">
          <div class="mono text-sm break-all min-w-0">{{ resetPassword }}</div>
          <button class="btn btn-xs btn-ghost shrink-0" :title="i18n.t('复制')" @click="copyResetPassword">
            <ClipboardDocumentIcon class="h-4 w-4" />
          </button>
        </div>
      </div>
      <button class="btn btn-sm" @click="resetPassword = ''">{{ i18n.t('关闭') }}</button>
    </div>

    <div v-if="store.loading" class="flex justify-center py-10">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <div v-else class="overflow-x-auto bg-base-100 rounded-box border border-base-300">
      <table class="table table-zebra">
        <thead>
          <tr>
            <th>{{ i18n.t('用户名') }}</th>
            <th>{{ i18n.t('角色') }}</th>
            <th>{{ i18n.t('节点上限') }}</th>
            <th>{{ i18n.t('分组上限') }}</th>
            <th>{{ i18n.t('模板上限') }}</th>
            <th class="text-right">{{ i18n.t('操作') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in store.users" :key="user.id">
            <td class="font-medium">{{ user.username }}</td>
            <td><span class="badge badge-neutral">{{ user.role }}</span></td>
            <td>{{ user.node_limit || i18n.t('不限') }}</td>
            <td>{{ user.profile_limit || i18n.t('不限') }}</td>
            <td>{{ user.template_limit || i18n.t('不限') }}</td>
            <td>
              <div class="flex justify-end gap-1">
                <button class="btn btn-xs btn-ghost" :title="i18n.t('编辑用户')" @click="openEdit(user)">
                  <PencilSquareIcon class="h-4 w-4" />
                </button>
                <button class="btn btn-xs btn-ghost" :title="i18n.t('重置') + i18n.t('密码')" @click="reset(user)">
                  <KeyIcon class="h-4 w-4" />
                </button>
                <button class="btn btn-xs btn-ghost text-error" :title="i18n.t('删除')" @click="remove(user)">
                  <TrashIcon class="h-4 w-4" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showEdit" class="modal modal-open">
      <div class="modal-box max-w-xl">
        <h3 class="font-bold text-lg mb-3">{{ editing ? i18n.t('编辑用户') : i18n.t('新建用户') }}</h3>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label class="form-control sm:col-span-2">
            <span class="label-text mb-1">{{ i18n.t('用户名') }}</span>
            <input v-model="form.username" class="input input-bordered input-sm" />
          </label>
          <label v-if="!editing" class="form-control sm:col-span-2">
            <span class="label-text mb-1">{{ i18n.t('初始密码') }}</span>
            <input v-model="form.password" type="password" class="input input-bordered input-sm" autocomplete="new-password" />
          </label>
          <label class="form-control sm:col-span-2">
            <span class="label-text mb-1">{{ i18n.t('角色') }}</span>
            <select v-model="form.role" class="select select-bordered select-sm">
              <option value="user">user</option>
              <option value="admin">admin</option>
            </select>
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('节点上限') }}</span>
            <input v-model.number="form.node_limit" type="number" min="0" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('分组上限') }}</span>
            <input v-model.number="form.profile_limit" type="number" min="0" class="input input-bordered input-sm" />
          </label>
          <label class="form-control">
            <span class="label-text mb-1">{{ i18n.t('模板上限') }}</span>
            <input v-model.number="form.template_limit" type="number" min="0" class="input input-bordered input-sm" />
          </label>
        </div>
        <div class="modal-action">
          <button class="btn btn-ghost" :disabled="busy" @click="showEdit = false">{{ i18n.t('取消') }}</button>
          <button class="btn btn-primary" :disabled="busy" @click="save">
            <span v-if="busy" class="loading loading-spinner loading-sm"></span>
            {{ i18n.t('保存') }}
          </button>
        </div>
      </div>
      <div class="modal-backdrop" @click="showEdit = false"></div>
    </div>
  </div>
</template>
