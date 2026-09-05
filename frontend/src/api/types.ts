// API entity types mirroring backend payloads / internal/models.

export interface ApiError {
  code: string
  message: string
  details?: ApiErrorDetails
}

export interface Envelope<T> {
  data: T | null
  error: ApiError | null
}

export type ApiErrorPanel = 'group' | 'country' | 'chain'
export type ApiErrorKind =
  | 'empty_group'
  | 'invalid_final'
  | 'unknown_outbound_ref'
  | 'group_cycle'
  | 'auto_country_empty'
  | 'chain_proxy_empty'

export type RuleSetPublishStage =
  | 'kernel'
  | 'fetch'
  | 'limit'
  | 'decompile'
  | 'format'
  | 'decode'
  | 'merge'
  | 'compile'
  | 'version'

export interface ApiErrorDetails {
  kind: ApiErrorKind | 'rule_set_publish_error'
  panel?: ApiErrorPanel
  groupTag?: string
  outboundTag?: string
  cycle?: string[]
  stage?: RuleSetPublishStage | string
  source_index?: number
  source_kind?: RuleSetSourceKind
  source_format?: RuleSetSourceFormat
  url?: string
  message?: string
}

export type CountrySource = 'auto' | 'manual' | 'override'
export type NodeSource = 'protocol' | 'subscription' | 'config' | 'manual'

export interface Node {
  id: number
  owner_user_id: number
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

export type NodeSummary = Omit<Node, 'raw'>

export type TemplateKind = 'builtin' | 'user'

export interface Template {
  id: number
  owner_user_id: number
  name: string
  kind: TemplateKind
  content: string
  description: string
  created_at: string
  updated_at: string
}

export type TemplateSummary = Omit<Template, 'content'>

export interface TemplateSaveResult {
  template: Template
  imported: number
  deduped?: number
  nodes: Node[]
}

export interface TemplateStructureGroup {
  tag: string
  type: 'selector' | 'urltest'
  outbounds: string[]
  default?: string
  referenced_by?: string[]
  deletable: boolean
  delete_reason?: string
}

export interface TemplateStructure {
  final: string
  groups: TemplateStructureGroup[]
  available_outbounds: string[]
}

export interface SubscriptionSource {
  id: number
  owner_user_id: number
  name: string
  url: string
  last_fetch_at?: string
  last_status: string
  node_count: number
  created_at: string
}

export interface NodeGroup {
  id: number
  owner_user_id: number
  name: string
  description: string
  node_ids: number[]
  created_at: string
  updated_at: string
}

export interface NodeGroupPayload {
  name: string
  description: string
  node_ids: number[]
}

export type RuleSetSourceKind = 'manual' | 'remote'
export type RuleSetSourceFormat = 'source' | 'binary'

export interface RuleSetSource {
  id: number
  rule_set_id: number
  kind: RuleSetSourceKind
  format: RuleSetSourceFormat
  url?: string
  content?: string
  position: number
}

export interface RuleSet {
  id: number
  owner_user_id: number
  name: string
  description: string
  sources?: RuleSetSource[]
  source_count: number
  rule_count: number
  json_size: number
  srs_size: number
  json_sha256: string
  srs_sha256: string
  kernel_version: string
  published_at: string
  last_attempt_at: string
  last_error?: string
  created_at: string
  updated_at: string
}

export interface RuleSetSourcePayload {
  kind: RuleSetSourceKind
  format: RuleSetSourceFormat
  url?: string
  content?: string
}

export interface RuleSetPayload {
  name: string
  description: string
  sources: RuleSetSourcePayload[]
}

export interface ProfileOptions {
  autoCountryGroups: boolean
  chainProxy: boolean
  chainProxyNodeId?: number
  chainProxyNodeIds?: number[]
  groupSelections?: Record<string, NodeSelection>
  autoCountrySelection?: NodeSelection
  chainProxySelection?: NodeSelection
}

export interface NodeSelection {
  nodeIds: number[]
  nodeGroupIds: number[]
  outboundRefs: string[]
  skipCountryGroups: boolean
}

// Profile.Options is a JSON string on the wire (see models.Profile.Options).
export interface Profile {
  id: number
  owner_user_id: number
  name: string
  template_id: number
  options: string
  subscription_enabled: boolean
  node_ids?: number[] | null
  node_group_ids?: number[] | null
  validation?: ProfileValidation | null
  created_at: string
  updated_at: string
}

export interface ProfileValidation {
  valid: boolean
  missing_template: boolean
  missing_node_ids: number[]
  missing_node_group_ids: number[]
}

export interface ProfilePayload {
  name: string
  template_id: number
  node_ids: number[]
  node_group_ids: number[]
  options: ProfileOptions
  subscription_enabled: boolean
}

export interface KernelStatus {
  available: boolean
  active_kernel_id: string
  active?: KernelProbe
  kernels: KernelProbe[]
  path?: string
  version?: string
}

export interface KernelProfile {
  id: string
  name: string
  path: string
}

export interface KernelProbe {
  id: string
  name: string
  path?: string
  available: boolean
  valid: boolean
  active?: boolean
  version?: string
  error?: string
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
  deduped?: number
  nodes: Node[]
  source_id?: number
  warnings?: string[]
  fetches?: ImportFetchResult[]
}

export interface ImportFetchResult {
  url: string
  ok: boolean
  nodes?: number
  error?: string
  from_cache?: boolean
}

export interface ImportPreviewNode {
  tag: string
  type: string
  server: string
  server_port: number
  country_code: string
  country_source: CountrySource
  source: NodeSource
  has_detour: boolean
  detour?: string
}

export interface ImportPreviewResult {
  parsed: number
  importable: number
  deduped: number
  nodes: ImportPreviewNode[]
  warnings?: string[]
  fetches?: ImportFetchResult[]
}

export interface NodeUsage {
  profile_id: number
  profile_name: string
  via_group_id?: number
  via_group_name?: string
}

export interface BulkDeleteResult {
  deleted: number
  deleted_nodes?: number
  deleted_node_ids?: number[]
}

export interface BulkNodeUsage extends NodeUsage {
  node_id: number
}

export interface BulkNodeUsageResult {
  usage: BulkNodeUsage[]
}

export type Settings = Record<string, string>

export interface AppInfo {
  display_name: string
  country_heat_order: string[]
  registration_enabled: boolean
  subscription_host_prefix: string
}

export type UserRole = 'admin' | 'user'

export interface User {
  id: number
  username: string
  role: UserRole
  node_limit: number
  profile_limit: number
  template_limit: number
  active_kernel_id: string
  created_at: string
  updated_at: string
}

export interface PreviewPayload {
  profile_id?: number
  template_id?: number
  template_content?: string
  node_ids?: number[]
  node_group_ids?: number[]
  options?: ProfileOptions
}
