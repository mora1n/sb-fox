import type { NodeSource } from '../api/types'

const labels: Record<NodeSource, string> = {
  protocol: '协议导入',
  subscription: '订阅导入',
  config: '配置导入',
  manual: '手动创建',
}

export function nodeSourceLabel(source: NodeSource): string {
  return labels[source] ?? source
}
