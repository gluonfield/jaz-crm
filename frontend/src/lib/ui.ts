import { useSyncExternalStore } from 'react'

// Shell state shared by components that are not parent and child.
type UI = { paletteOpen: boolean }

let state: UI = { paletteOpen: false }
const listeners = new Set<() => void>()

export function getUI() {
  return state
}

export function setUI(next: Partial<UI>) {
  state = { ...state, ...next }
  for (const listener of listeners) {
    listener()
  }
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
