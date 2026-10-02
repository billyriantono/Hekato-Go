// Prompt-cache KPI on the overview (see providers' OnCacheUsage callback).
export const en: Record<string, string> = {
  'overview.cacheHitInRange': 'Cache hits ({0})',
  'overview.cacheHint': '{0} read · {1} written',
  'overview.cacheNoData': 'Upstream reports no cache data',
  'logs.cacheRead': 'Cached',
}

export const zh: Record<string, string> = {
  'overview.cacheHitInRange': '缓存命中（{0}）',
  'overview.cacheHint': '{0} 命中 · {1} 写入',
  'overview.cacheNoData': '上游未提供缓存数据',
  'logs.cacheRead': '缓存',
}
