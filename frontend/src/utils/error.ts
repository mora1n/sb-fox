import { ApiRequestError } from '../api/client'

// errMsg extracts a human-readable message from any thrown value.
export function errMsg(e: unknown, fallback = '操作失败'): string {
  if (e instanceof ApiRequestError) return e.message
  if (e instanceof Error) return e.message
  return fallback
}
