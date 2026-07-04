import type { NodeSource, NodeSummary } from '../api/types'
import { sortCountryCodes } from './countries'

export interface NodeFilterState {
  search: string
  source: string
  country: string
  type: string
}

export const NODE_SOURCES: NodeSource[] = ['protocol', 'subscription', 'config', 'manual']

export function emptyNodeFilters(): NodeFilterState {
  return { search: '', source: '', country: '', type: '' }
}

export function filterNodes(nodes: NodeSummary[], filters: NodeFilterState): NodeSummary[] {
  const q = filters.search.toLowerCase().trim()
  return nodes.filter((node) => {
    if (filters.source && node.source !== filters.source) return false
    if (filters.country && node.country_code !== filters.country) return false
    if (filters.type && node.type !== filters.type) return false
    if (!q) return true
    return node.tag.toLowerCase().includes(q) || node.server.toLowerCase().includes(q)
  })
}

export function nodeCountries(nodes: NodeSummary[], heatOrder: string[]): string[] {
  return sortCountryCodes([...new Set(nodes.map((node) => node.country_code).filter(Boolean))], heatOrder)
}

export function nodeTypes(nodes: NodeSummary[]): string[] {
  return [...new Set(nodes.map((node) => node.type).filter(Boolean))].sort()
}

export function nodeSources(nodes: NodeSummary[]): NodeSource[] {
  const present = new Set(nodes.map((node) => node.source))
  return NODE_SOURCES.filter((source) => present.has(source))
}
