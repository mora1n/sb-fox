<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Node } from '../api/types'
import CountryFlag from './CountryFlag.vue'
import { useI18nStore } from '../stores/i18n'
import { useSettingsStore } from '../stores/settings'
import { nodeSourceLabel } from '../utils/nodeSource'
import { emptyNodeFilters, filterNodes, nodeCountries, nodeSources, nodeTypes } from '../utils/nodeFilters'

const props = defineProps<{ nodes: Node[]; modelValue: number[]; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [number[]] }>()
const i18n = useI18nStore()
const settings = useSettingsStore()

const filters = ref(emptyNodeFilters())
const sourceOptions = computed(() => nodeSources(props.nodes))
const countryOptions = computed(() => nodeCountries(props.nodes, settings.countryHeatOrder))
const typeOptions = computed(() => nodeTypes(props.nodes))
const filtered = computed(() => filterNodes(props.nodes, filters.value))

function toggle(id: number) {
  if (props.disabled) return
  const set = new Set(props.modelValue)
  if (set.has(id)) set.delete(id)
  else set.add(id)
  emit('update:modelValue', [...set])
}
function selectAllFiltered() {
  if (props.disabled) return
  const set = new Set(props.modelValue)
  filtered.value.forEach((n) => set.add(n.id))
  emit('update:modelValue', [...set])
}
function clearAll() {
  if (props.disabled) return
  emit('update:modelValue', [])
}
</script>

<template>
  <div class="flex flex-col gap-2" :class="{ 'opacity-60': disabled }">
    <div class="flex items-center gap-2 flex-wrap">
      <input v-model="filters.search" class="input input-bordered input-sm min-w-0 flex-1" :placeholder="i18n.t('搜索节点...')" :disabled="disabled" />
      <button type="button" class="btn btn-xs min-h-7 h-7 shrink-0" @click="selectAllFiltered" :disabled="disabled">{{ i18n.t('全选') }}</button>
      <button type="button" class="btn btn-xs min-h-7 h-7 shrink-0" @click="clearAll" :disabled="disabled">{{ i18n.t('清空') }}</button>
      <span class="badge badge-primary shrink-0">{{ modelValue.length }}</span>
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
      <select v-model="filters.source" class="select select-bordered select-sm min-w-0" :disabled="disabled">
        <option value="">{{ i18n.t('全部来源') }}</option>
        <option v-for="source in sourceOptions" :key="source" :value="source">
          {{ i18n.t(nodeSourceLabel(source)) }}
        </option>
      </select>
      <select v-model="filters.country" class="select select-bordered select-sm min-w-0" :disabled="disabled">
        <option value="">{{ i18n.t('全部国家') }}</option>
        <option v-for="country in countryOptions" :key="country" :value="country">{{ country }}</option>
      </select>
      <select v-model="filters.type" class="select select-bordered select-sm min-w-0" :disabled="disabled">
        <option value="">{{ i18n.t('全部协议') }}</option>
        <option v-for="type in typeOptions" :key="type" :value="type">{{ type }}</option>
      </select>
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
          :disabled="disabled"
          @change="toggle(n.id)"
        />
        <CountryFlag v-if="n.country_code" :code="n.country_code" :show-code="false" />
        <span class="truncate flex-1 text-sm">{{ n.tag }}</span>
        <span class="badge badge-ghost badge-sm">{{ n.type }}</span>
        <span class="badge badge-ghost badge-sm hidden sm:inline-flex">{{ i18n.t(nodeSourceLabel(n.source)) }}</span>
      </label>
      <div v-if="!filtered.length" class="px-3 py-4 text-sm opacity-60 text-center">{{ i18n.t('无匹配节点') }}</div>
    </div>
  </div>
</template>
