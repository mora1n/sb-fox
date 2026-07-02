import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, downloadGet, get, post, put } from '../api/client'
import type { InspectResult, Template, TemplateSaveResult, TemplateStructure } from '../api/types'

export const useTemplatesStore = defineStore('templates', () => {
  const templates = ref<Template[]>([])
  const loading = ref(false)

  async function fetchAll(): Promise<void> {
    loading.value = true
    try {
      templates.value = (await get<Template[]>('/templates')) ?? []
    } finally {
      loading.value = false
    }
  }

  async function getOne(id: number): Promise<Template> {
    return get<Template>('/templates/' + id)
  }

  async function create(name: string, content: string, description: string): Promise<TemplateSaveResult> {
    const r = await post<TemplateSaveResult>('/templates', { name, content, description })
    await fetchAll()
    return r
  }

  async function update(id: number, content: string, description: string): Promise<{ imported: number }> {
    const r = await put<{ ok: boolean; imported: number }>('/templates/' + id, { content, description })
    await fetchAll()
    return { imported: r.imported ?? 0 }
  }

  async function remove(id: number): Promise<void> {
    await del('/templates/' + id)
    templates.value = templates.value.filter((t) => t.id !== id)
  }

  async function inspect(id: number): Promise<InspectResult> {
    return post<InspectResult>('/templates/' + id + '/inspect')
  }

  async function structure(id: number): Promise<TemplateStructure> {
    return get<TemplateStructure>('/templates/' + id + '/structure')
  }

  async function saveStructure(id: number, payload: TemplateStructure): Promise<TemplateStructure> {
    const r = await put<TemplateStructure>('/templates/' + id + '/structure', payload)
    await fetchAll()
    return r
  }

  async function exportTemplate(id: number, name: string): Promise<void> {
    const filename = name.toLowerCase().endsWith('.json') ? name : name + '.json'
    await downloadGet('/templates/' + id + '/export', filename)
  }

  return {
    templates,
    loading,
    fetchAll,
    getOne,
    create,
    update,
    remove,
    inspect,
    structure,
    saveStructure,
    exportTemplate,
  }
})
