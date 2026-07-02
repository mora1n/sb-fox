<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Node } from '../api/types'
import CountryFlag from './CountryFlag.vue'

const props = defineProps<{ nodes: Node[]; modelValue: number[] }>()
const emit = defineEmits<{ 'update:modelValue': [number[]] }>()

const search = ref('')
const filtered = computed(() => {
  const q = search.value.toLowerCase().trim()
  if (!q) return props.nodes
  return props.nodes.filter(
    (n) => n.tag.toLowerCase().includes(q) || n.server.toLowerCase().includes(q),
  )
})

function toggle(id: number) {
  const set = new Set(props.modelValue)
  if (set.has(id)) set.delete(id)
  else set.add(id)
  emit('update:modelValue', [...set])
}
function selectAllFiltered() {
  const set = new Set(props.modelValue)
  filtered.value.forEach((n) => set.add(n.id))
  emit('update:modelValue', [...set])
}
function clearAll() {
  emit('update:modelValue', [])
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <div class="flex items-center gap-2">
      <input v-model="search" class="input input-bordered input-sm flex-1" placeholder="搜索节点..." />
      <button type="button" class="btn btn-xs" @click="selectAllFiltered">全选</button>
      <button type="button" class="btn btn-xs" @click="clearAll">清空</button>
      <span class="badge badge-primary">{{ modelValue.length }}</span>
    </div>
    <div class="border border-base-300 rounded-box max-h-64 overflow-y-auto divide-y divide-base-200">
      <label
        v-for="n in filtered"
        :key="n.id"
        class="flex items-center gap-2 px-3 py-1.5 cursor-pointer hover:bg-base-200"
      >
        <input
          type="checkbox"
          class="checkbox checkbox-sm"
          :checked="modelValue.includes(n.id)"
          @change="toggle(n.id)"
        />
        <CountryFlag v-if="n.country_code" :code="n.country_code" :show-code="false" />
        <span class="truncate flex-1 text-sm">{{ n.tag }}</span>
        <span class="badge badge-ghost badge-sm">{{ n.type }}</span>
      </label>
      <div v-if="!filtered.length" class="px-3 py-4 text-sm opacity-60 text-center">无匹配节点</div>
    </div>
  </div>
</template>
