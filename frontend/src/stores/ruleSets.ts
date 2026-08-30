import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, downloadGet, get, post, put } from '../api/client'
import type { BulkDeleteResult, RuleSet, RuleSetPayload } from '../api/types'

export const useRuleSetsStore = defineStore('ruleSets', () => {
  const ruleSets = ref<RuleSet[]>([])
  const loading = ref(false)
  const loaded = ref(false)
  let inFlight: Promise<void> | null = null

  async function fetchAll(force = false): Promise<void> {
    if (!force && loaded.value) return
    if (!force && inFlight) return inFlight
    loading.value = true
    inFlight = get<RuleSet[]>('/rule-sets').then((items) => {
      ruleSets.value = items ?? []
      loaded.value = true
    }).finally(() => {
      loading.value = false
      inFlight = null
    })
    return inFlight
  }

  function getOne(id: number): Promise<RuleSet> {
    return get<RuleSet>('/rule-sets/' + id)
  }

  async function create(payload: RuleSetPayload): Promise<RuleSet> {
    const item = await post<RuleSet>('/rule-sets', payload)
    await fetchAll(true)
    return item
  }

  async function update(id: number, payload: RuleSetPayload): Promise<RuleSet> {
    const item = await put<RuleSet>('/rule-sets/' + id, payload)
    await fetchAll(true)
    return item
  }

  async function refresh(id: number): Promise<RuleSet> {
    const item = await post<RuleSet>('/rule-sets/' + id + '/refresh')
    await fetchAll(true)
    return item
  }

  async function remove(id: number): Promise<void> {
    await del('/rule-sets/' + id)
    ruleSets.value = ruleSets.value.filter((item) => item.id !== id)
  }

  async function bulkDelete(ids: number[]): Promise<number> {
    const result = await post<BulkDeleteResult>('/rule-sets/bulk-delete', { ids })
    const selected = new Set(ids)
    ruleSets.value = ruleSets.value.filter((item) => !selected.has(item.id))
    return result.deleted
  }

  function exportArtifact(id: number, name: string, format: 'source' | 'binary'): Promise<void> {
    const extension = format === 'source' ? 'json' : 'srs'
    return downloadGet(`/rule-sets/${id}/export/${format}`, `${name}.${extension}`)
  }

  function reset(): void {
    ruleSets.value = []
    loading.value = false
    loaded.value = false
    inFlight = null
  }

  return { ruleSets, loading, fetchAll, getOne, create, update, refresh, remove, bulkDelete, exportArtifact, reset }
})
