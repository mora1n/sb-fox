import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ApiRequestError, del, downloadGet, get, post, put } from '../api/client'
import type { InspectResult, Template, TemplateSaveResult, TemplateStructure, TemplateSummary } from '../api/types'

export const useTemplatesStore = defineStore('templates', () => {
  const templates = ref<TemplateSummary[]>([])
  const structures = ref<Record<number, TemplateStructure>>({})
  const loading = ref(false)
  const loaded = ref(false)
  const templateDetails = new Map<number, Template>()
  const templateInFlight = new Map<number, Promise<Template>>()
  const structureInFlight = new Map<number, Promise<TemplateStructure>>()
  let inFlight: Promise<void> | null = null
  let detailVersion = 0
  let structureVersion = 0

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

  async function getOne(id: number, force = false): Promise<Template> {
    if (!force && templateDetails.has(id)) return templateDetails.get(id) as Template
    if (!force && templateInFlight.has(id)) return templateInFlight.get(id) as Promise<Template>
    const version = detailVersion
    const req = get<Template>('/templates/' + id).then((template) => {
      if (version === detailVersion) templateDetails.set(id, template)
      return template
    }).finally(() => {
      templateInFlight.delete(id)
    })
    templateInFlight.set(id, req)
    return req
  }

  async function prefetchOne(id: number): Promise<void> {
    await getOne(id)
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
    detailVersion++
    structureVersion++
    templateDetails.set(r.template.id, r.template)
    await fetchAll(true)
    return r
  }

  async function update(id: number, content: string, description: string): Promise<{ imported: number; deduped?: number }> {
    const r = await put<{ ok: boolean; imported: number; deduped?: number }>('/templates/' + id, { content, description })
    detailVersion++
    structureVersion++
    templateDetails.delete(id)
    templateInFlight.delete(id)
    structureInFlight.delete(id)
    delete structures.value[id]
    await fetchAll(true)
    return { imported: r.imported ?? 0, deduped: r.deduped ?? 0 }
  }

  async function remove(id: number): Promise<void> {
    await del('/templates/' + id)
    detailVersion++
    structureVersion++
    templateDetails.delete(id)
    templateInFlight.delete(id)
    structureInFlight.delete(id)
    delete structures.value[id]
    templates.value = templates.value.filter((t) => t.id !== id)
    loaded.value = true
  }

  async function inspect(id: number): Promise<InspectResult> {
    return post<InspectResult>('/templates/' + id + '/inspect')
  }

  async function structure(id: number): Promise<TemplateStructure> {
    if (structures.value[id]) return cloneStructure(structures.value[id])
    if (structureInFlight.has(id)) return structureInFlight.get(id)!.then(cloneStructure)
    const version = structureVersion
    const req = get<TemplateStructure>('/templates/' + id + '/structure').then((r) => {
      const next = cloneStructure(r)
      if (version === structureVersion) structures.value[id] = cloneStructure(next)
      return next
    }).finally(() => {
      structureInFlight.delete(id)
    })
    structureInFlight.set(id, req)
    return req.then(cloneStructure)
  }

  async function prefetchStructure(id: number): Promise<void> {
    await structure(id)
  }

  async function prefetchStructures(ids: number[]): Promise<void> {
    await Promise.all([...new Set(ids.filter(Boolean))].map((id) => prefetchStructure(id)))
  }

  async function saveStructure(id: number, payload: TemplateStructure): Promise<TemplateStructure> {
    const r = await put<TemplateStructure>('/templates/' + id + '/structure', payload)
    structureVersion++
    structureInFlight.delete(id)
    structures.value[id] = cloneStructure(r)
    await fetchAll(true)
    return cloneStructure(r)
  }

  async function exportTemplate(id: number, name: string): Promise<void> {
    const filename = name.toLowerCase().endsWith('.json') ? name : name + '.json'
    await downloadGet('/templates/' + id + '/export', filename)
  }

  function reset(): void {
    templates.value = []
    structures.value = {}
    loading.value = false
    loaded.value = false
    inFlight = null
    detailVersion++
    structureVersion++
    templateDetails.clear()
    templateInFlight.clear()
    structureInFlight.clear()
  }

  return {
    templates,
    structures,
    loading,
    fetchAll,
    getOne,
    prefetchOne,
    findByName,
    create,
    update,
    remove,
    inspect,
    structure,
    prefetchStructure,
    prefetchStructures,
    saveStructure,
    exportTemplate,
    reset,
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
