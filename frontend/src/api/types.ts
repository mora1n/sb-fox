// API entity types mirroring backend payloads / internal/models.

export interface ApiError {
  code: string
  message: string
}

export interface Envelope<T> {
  data: T | null
  error: ApiError | null
}

export type CountrySource = 'auto' | 'manual' | 'override'
export type NodeSource = 'protocol' | 'subscription' | 'config' | 'manual'

export interface Node {
  id: number
  tag: string
  type: string
  server: string
  server_port: number
  country_code: string
  country_source: CountrySource
  source: NodeSource
  source_ref?: number
  has_detour: boolean
  detour?: string
  raw: string
  created_at: string
  updated_at: string
}

export type TemplateKind = 'builtin' | 'user'

export interface Template {
  id: number
  name: string
  kind: TemplateKind
  content: string
  description: string
  created_at: string
  updated_at: string
}

export interface SubscriptionSource {
  id: number
  name: string
  url: string
  last_fetch_at?: string
  last_status: string
  node_count: number
  created_at: string
}

export interface ProfileOptions {
  autoCountryGroups: boolean
  chainProxy: boolean
}

// Profile.Options is a JSON string on the wire (see models.Profile.Options).
export interface Profile {
  id: number
  name: string
  template_id: number
  options: string
  token: string
  node_ids: number[]
  created_at: string
  updated_at: string
}

export interface ProfilePayload {
  name: string
  template_id: number
  node_ids: number[]
  options: ProfileOptions
}

export interface KernelStatus {
  available: boolean
  path: string
  version: string
}

export type ValidateStatus = 'ok' | 'invalid' | 'unavailable'

export interface KernelResult {
  status: ValidateStatus
  messages?: string
  formatted?: string
}

export interface ProxyGroup {
  tag: string
  type: 'selector' | 'urltest'
  outbounds: string[]
}

export interface InspectResult {
  groups: ProxyGroup[]
}

export interface ImportResult {
  imported: number
  nodes: Node[]
  source_id?: number
}

export type Settings = Record<string, string>

export interface AppInfo {
  display_name: string
  country_heat_order: string[]
}

export interface PreviewPayload {
  template_id?: number
  template_content?: string
  node_ids: number[]
  options: ProfileOptions
}
