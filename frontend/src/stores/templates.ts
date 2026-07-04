import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ApiRequestError, del, downloadGet, get, post, put } from '../api/client'
import type { InspectResult, Template, TemplateSaveResult, TemplateStructure, TemplateSummary } from '../api/types'

export const useTemplatesStore = defineStore('templates', () => {
  const templates = ref<TemplateSummary[]>([])
  const structures = ref<Record<number, TemplateStructure>>({})
  const loading = ref(false)
  const loaded = ref(false)
  let inFlight: Promise<void> | null = null

  async function fetchAll(force = false): Promise<void> {
    if (!force && loaded.value) return
    if (!force && inFlight) return inFlight
    loading.value = true
    inFlight = get<TemplateSummary[]>('/templates?summary=1').then((items) => {
      templates.value = items ?? []
      loaded.value = true
    }).finally(() => {
      loading.value = false
      inFlight = null
    })
    return inFlight
  }

  async function getOne(id: number): Promise<Template> {
    return get<Template>('/templates/' + id)
  }

  async function findByName(name: string): Promise<Template | null> {
    try {
      return await get<Template>('/templates/by-name?name=' + encodeURIComponent(name))
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 404) return null
      throw e
    }
  }

  async function create(name: string, content: string, description: string): Promise<TemplateSaveResult> {
    const r = await post<TemplateSaveResult>('/templates', { name, content, description })
    await fetchAll(true)
    return r
  }

  async function update(id: number, content: string, description: string): Promise<{ imported: number; deduped?: number }> {
    const r = await put<{ ok: boolean; imported: number; deduped?: number }>('/templates/' + id, { content, description })
    delete structures.value[id]
    await fetchAll(true)
    return { imported: r.imported ?? 0, deduped: r.deduped ?? 0 }
  }

  async function remove(id: number): Promise<void> {
    await del('/templates/' + id)
    delete structures.value[id]
    templates.value = templates.value.filter((t) => t.id !== id)
    loaded.value = true
  }

  async function inspect(id: number): Promise<InspectResult> {
    return post<InspectResult>('/templates/' + id + '/inspect')
  }

  async function structure(id: number): Promise<TemplateStructure> {
    if (structures.value[id]) return cloneStructure(structures.value[id])
    const r = await get<TemplateStructure>('/templates/' + id + '/structure')
    structures.value[id] = cloneStructure(r)
    return cloneStructure(r)
  }

  async function saveStructure(id: number, payload: TemplateStructure): Promise<TemplateStructure> {
    const r = await put<TemplateStructure>('/templates/' + id + '/structure', payload)
    structures.value[id] = cloneStructure(r)
    await fetchAll(true)
    return cloneStructure(r)
  }

  async function exportTemplate(id: number, name: string): Promise<void> {
    const filename = name.toLowerCase().endsWith('.json') ? name : name + '.json'
    await downloadGet('/templates/' + id + '/export', filename)
  }

  return {
    templates,
    structures,
    loading,
    fetchAll,
    getOne,
    findByName,
    create,
    update,
    remove,
    inspect,
    structure,
    saveStructure,
    exportTemplate,
  }
})

function cloneStructure(st: TemplateStructure): TemplateStructure {
  return {
    final: st.final,
    available_outbounds: [...st.available_outbounds],
    groups: st.groups.map((g) => ({
      ...g,
      outbounds: [...g.outbounds],
      referenced_by: g.referenced_by ? [...g.referenced_by] : undefined,
    })),
  }
}
