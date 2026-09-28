// Auto-route tier health (see tierHealth in proxy/autoroute.go).
export const en: Record<string, string> = {
  'overview.tiers': 'Tiers',
  'overview.tierHealthHint': '{0} candidates · {1} wanted · {2} served · {3} starved',
  'overview.tierStarved': 'Tier "{0}": {1} of {2} requests had to be served by another tier ({3} candidates available).',
  'overview.wantedTier': 'wanted {0}',
}

export const zh: Record<string, string> = {
  'overview.tiers': '档位',
  'overview.tierHealthHint': '{0} 个候选 · {1} 次请求 · {2} 次服务 · {3} 次降级',
  'overview.tierStarved': '档位“{0}”：{2} 次请求中有 {1} 次由其他档位服务（可用候选 {3} 个）。',
  'overview.wantedTier': '期望 {0}',
}
