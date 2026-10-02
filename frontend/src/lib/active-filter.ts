import { useMutation } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useCallback, useEffect, useRef, useState } from 'react'
import { call } from './api'
import { useTool } from './queries'
import type { ActiveFilter } from './types'

export function useActiveFilter(object: string, initial?: ActiveFilter) {
  const result = useTool<ActiveFilter>('get_active_filter', { object }, { refetchInterval: 5000 })
  const [draft, setDraft] = useState(initial)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const pending = useRef<(() => void) | undefined>(undefined)
  const navigate = useNavigate()
  const { mutateAsync, error: saveError } = useMutation({
    mutationKey: ['set_active_filter'],
    scope: { id: `active-filter:${object}` },
    mutationFn: (filter: ActiveFilter) => call<ActiveFilter>('set_active_filter', { object, ...filter }),
  })
  const persist = useCallback((filter: ActiveFilter) => mutateAsync(filter).finally(() => {
    setDraft((current) => current === filter ? undefined : current)
  }), [mutateAsync])
  useEffect(() => {
    if (!initial) {
      return
    }
    let current = true
    void persist(initial).then(() => {
      if (current) {
        void navigate({ to: '.', search: (previous) => ({ ...previous, filters: undefined, q: undefined, saved: undefined, where: undefined, category: undefined }), replace: true })
      }
    }).catch(() => {})
    return () => {
      current = false
    }
  }, [initial, persist, navigate])
  useEffect(() => () => clearTimeout(timer.current), [])
  const flush = () => {
    clearTimeout(timer.current)
    pending.current?.()
    pending.current = undefined
  }
  const setFilter = (filter: ActiveFilter, debounce = false) => {
    setDraft(filter)
    clearTimeout(timer.current)
    pending.current = () => {
      void persist(filter).catch(() => {})
    }
    if (debounce) {
      timer.current = setTimeout(flush, 300)
    } else {
      flush()
    }
  }
  return { filter: initial ? undefined : draft ?? result.data, error: saveError ?? result.error, setFilter, flush }
}
