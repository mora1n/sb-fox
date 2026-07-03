import type { Envelope } from './types'

// ApiRequestError carries the server-provided error code/message.
export class ApiRequestError extends Error {
  code: string
  status: number
  constructor(message: string, code: string, status: number) {
    super(message)
    this.name = 'ApiRequestError'
    this.code = code
    this.status = status
  }
}

// Callback invoked on 401 responses (wired to the router in main.ts).
let onUnauthorized: (() => void) | null = null
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

const BASE = '/api'

interface RequestOptions {
  method?: string
  body?: unknown
  // when true, skip the 401 redirect (used by auth.me() on app init)
  silent401?: boolean
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {}
  let body: BodyInit | undefined
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  const res = await fetch(BASE + path, {
    method: opts.method ?? 'GET',
    credentials: 'include',
    headers,
    body,
  })

  // 204 or empty body
  const text = await res.text()
  if (res.status === 401 && !text) {
    if (!opts.silent401 && onUnauthorized) onUnauthorized()
    throw new ApiRequestError('未登录或会话已过期', 'unauthorized', 401)
  }
  if (!text) {
    if (!res.ok) throw new ApiRequestError(res.statusText, 'http_error', res.status)
    return undefined as T
  }

  let env: Envelope<T>
  try {
    env = JSON.parse(text) as Envelope<T>
  } catch {
    throw new ApiRequestError('响应解析失败: ' + text.slice(0, 200), 'parse_error', res.status)
  }

  if (env.error) {
    const message = env.error.message || '请求失败'
    if (res.status === 401 && isSessionUnauthorized(message) && !opts.silent401 && onUnauthorized) {
      onUnauthorized()
    }
    throw new ApiRequestError(message, env.error.code || 'error', res.status)
  }
  return env.data as T
}

function isSessionUnauthorized(message: string): boolean {
  return message === 'not authenticated' || message === 'invalid session' || message === 'user not found'
}

export function get<T>(path: string, silent401 = false): Promise<T> {
  return request<T>(path, { method: 'GET', silent401 })
}
export function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, { method: 'POST', body })
}
export function put<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, { method: 'PUT', body })
}
export function del<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'DELETE' })
}

async function downloadError(res: Response): Promise<ApiRequestError> {
  const text = await res.text()
  let code = 'http_error'
  let message = '下载失败 (HTTP ' + res.status + ')'
  if (text) {
    try {
      const env = JSON.parse(text) as Envelope<unknown>
      if (env.error) {
        code = env.error.code || code
        message = env.error.message || message
      }
    } catch {
      message = text.slice(0, 200)
    }
  }
  return new ApiRequestError(message, code, res.status)
}

// downloadPost triggers a browser file download from a POST endpoint that
// returns a raw file (not the {data,error} envelope), e.g. export/template.
export async function downloadPost(path: string, body: unknown, filename: string): Promise<void> {
  const res = await fetch(BASE + path, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const err = await downloadError(res)
    if (res.status === 401 && onUnauthorized) {
      onUnauthorized()
    }
    throw err
  }
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export async function downloadGet(path: string, filename: string): Promise<void> {
  const res = await fetch(BASE + path, {
    method: 'GET',
    credentials: 'include',
  })
  if (!res.ok) {
    const err = await downloadError(res)
    if (res.status === 401 && onUnauthorized) {
      onUnauthorized()
    }
    throw err
  }
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
