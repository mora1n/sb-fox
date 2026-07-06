const dateTimeFormatter = new Intl.DateTimeFormat(undefined, {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

export function formatDateTime(value?: string | Date | null): string {
  if (!value) return '-'
  const date = value instanceof Date ? value : new Date(value)
  if (!Number.isFinite(date.getTime())) return String(value)
  return dateTimeFormatter.format(date)
}

export function timeSortValue(value?: string | Date | null): number {
  if (!value) return 0
  const date = value instanceof Date ? value : new Date(value)
  const time = date.getTime()
  return Number.isFinite(time) ? time : 0
}
