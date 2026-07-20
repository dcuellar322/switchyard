import { useQuery } from '@tanstack/vue-query'

import { loadHostObservation } from '../api'

export function useHostObservation() {
  return useQuery({
    queryKey: ['host-observation'],
    queryFn: loadHostObservation,
    refetchInterval: 15_000,
  })
}
