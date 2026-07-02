import { defineStore } from 'pinia'
import { ref } from 'vue'
import { del, get, post, put } from '../api/client'
import type { InspectResult, Template } from '../api/types'

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

  async function create(name: string, content: string, description: string): Promise<Template> {
    const t = await post<Template>('/templates', { name, content, description })
    await fetchAll()
    return t
  }

  async function update(id: number, content: string, description: string): Promise<void> {
    await put('/templates/' + id, { content, description })
    await fetchAll()
  }

  async function remove(id: number): Promise<void> {
    await del('/templates/' + id)
    templates.value = templates.value.filter((t) => t.id !== id)
  }

  async function inspect(id: number): Promise<InspectResult> {
    return post<InspectResult>('/templates/' + id + '/inspect')
  }

  return { templates, loading, fetchAll, getOne, create, update, remove, inspect }
})
