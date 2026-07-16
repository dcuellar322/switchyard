import { useQuery } from '@tanstack/vue-query'

import { loadSystemInfo } from '../api'

export function useSystemInfo() {
  return useQuery({
    queryKey: ['system-info'],
    queryFn: loadSystemInfo,
    refetchInterval: 30_000,
  })
}
