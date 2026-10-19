import { useSyncExternalStore } from 'react'
import { readItem, writeItem } from './storage'

// Shell state shared by components that are not parent and child.
type UI = { paletteOpen: boolean; helpOpen: boolean; sidebarWidth: number; sidebarCollapsed: boolean }

const savedWidth = Number(readItem('jaz-crm:sidebar-width'))
let state: UI = {
  paletteOpen: false,
  helpOpen: false,
  sidebarWidth: Number.isFinite(savedWidth) && savedWidth >= 200 ? Math.min(420, savedWidth) : 232,
  sidebarCollapsed: readItem('jaz-crm:sidebar-collapsed') === 'true',
}
const listeners = new Set<() => void>()

export function getUI() {
  return state
}

export function setUI(next: Partial<UI>) {
  state = { ...state, ...next }
  if (next.sidebarCollapsed !== undefined) {
    writeItem('jaz-crm:sidebar-collapsed', String(next.sidebarCollapsed))
  }
  for (const listener of listeners) {
    listener()
  }
}

export function saveSidebarWidth() {
  writeItem('jaz-crm:sidebar-width', String(state.sidebarWidth))
}

export function useUI<T>(select: (ui: UI) => T) {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    () => select(state),
  )
}
