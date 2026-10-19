import { createContext, type ReactNode, useContext, useEffect, useLayoutEffect, useState, useSyncExternalStore } from 'react'
import { shortcuts, type ShortcutActions, type ShortcutId } from './shortcut-definitions'

export const isMac = () => typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)

export function shortcutLabel(binding: string) {
  return binding.replaceAll('Mod', isMac() ? '⌘' : 'Ctrl').replaceAll('Shift', '⇧').replaceAll('Alt', isMac() ? '⌥' : 'Alt')
    .replaceAll('ArrowUp', '↑').replaceAll('ArrowDown', '↓').replaceAll('ArrowLeft', '←').replaceAll('ArrowRight', '→')
    .replaceAll('Backspace', '⌫').replaceAll('Enter', '↵').replaceAll('Escape', 'Esc').replaceAll('+', ' ').toUpperCase()
}

export function matchesKey(event: KeyboardEvent, binding: string) {
  const parts = binding.split('+')
  const key = parts.pop()!
  const mod = parts.includes('Mod')
  if (event.metaKey !== (mod && isMac()) || event.ctrlKey !== (mod && !isMac()) || event.altKey !== parts.includes('Alt')) {
    return false
  }
  if (key === '?' || key === '#') {
    return event.key === key
  }
  if (event.shiftKey !== parts.includes('Shift')) {
    return false
  }
  const code = key === '.' ? 'Period' : key === ',' ? 'Comma' : /^\d$/.test(key) ? `Digit${key}` : parts.includes('Alt') ? `Key${key.toUpperCase()}` : ''
  return event.key.toLowerCase() === key.toLowerCase() || (!!code && event.code === code)
}

export function isTyping(event: KeyboardEvent) {
  return event.target instanceof HTMLElement && (event.target.isContentEditable || !!event.target.closest('input, textarea, select, [role=textbox]'))
}

function registry() {
  const scopes = new Map<object, { actions: ShortcutActions; priority: number }>()
  const listeners = new Set<() => void>()
  let current: ShortcutActions = {}
  const update = () => {
    current = Object.assign({}, ...Array.from(scopes.values()).sort((a, b) => a.priority - b.priority).map((scope) => scope.actions))
    listeners.forEach((notify) => notify())
  }
  return {
    actions: () => current,
    subscribe: (notify: () => void) => {
      listeners.add(notify)
      return () => { listeners.delete(notify) }
    },
    register: (scope: object, actions?: ShortcutActions, priority = 1) => {
      if (actions) {
        scopes.set(scope, { actions, priority })
      } else {
        scopes.delete(scope)
      }
      update()
    },
  }
}

const Context = createContext<ReturnType<typeof registry> | null>(null)

export function useShortcutActions(actions: ShortcutActions, priority = 1) {
  const store = useContext(Context)
  const [scope] = useState(() => ({}))
  useLayoutEffect(() => {
    store?.register(scope, actions, priority)
  }, [store, scope, actions, priority])
  useLayoutEffect(() => () => store?.register(scope), [store, scope])
}

export function useAvailableShortcuts() {
  const store = useContext(Context)!
  return useSyncExternalStore(store.subscribe, store.actions, store.actions)
}

export function ShortcutProvider({ children }: { children: ReactNode }) {
  const [store] = useState(registry)
  useEffect(() => {
    let sequence = ''
    let timer: ReturnType<typeof setTimeout>
    const reset = () => {
      sequence = ''
      clearTimeout(timer)
    }
    const handle = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.isComposing || event.repeat) {
        reset()
        return
      }
      const actions = store.actions()
      const overlay = document.querySelector('[role=dialog], [role=alertdialog], [role=menu], [data-radix-popper-content-wrapper]')
      const palette = event.target instanceof Node && document.querySelector('[data-command-palette]')?.contains(event.target)
      // Editors own formatting, including Cmd+K; the command menu can toggle itself.
      if (matchesKey(event, shortcuts.palette.keys[0]) && (!isTyping(event) || palette) && (!overlay || palette)) {
        event.preventDefault()
        reset()
        actions.palette?.()
        return
      }
      if (isTyping(event) || overlay || (event.key === 'Enter' && event.target instanceof HTMLElement && event.target.closest('button, a, [role=button], [role=link], summary'))) {
        reset()
        return
      }
      const bindings = (Object.entries(shortcuts) as [ShortcutId, { keys: readonly string[] }][])
        .flatMap(([id, definition]) => actions[id] ? definition.keys.map((keys) => ({ id, keys })) : [])
      const previous = sequence
      reset()
      if (previous) {
        const found = bindings.find(({ keys }) => keys.startsWith(`${previous} `) && matchesKey(event, keys.split(' ')[1]))
        if (found) {
          event.preventDefault()
          actions[found.id]?.()
          return
        }
        if (event.key === 'Escape') {
          event.preventDefault()
          return
        }
      }
      const prefix = bindings.find(({ keys }) => keys.includes(' ') && matchesKey(event, keys.split(' ')[0]))
      if (prefix) {
        sequence = prefix.keys.split(' ')[0]
        event.preventDefault()
        const fallback = bindings.find(({ keys }) => !keys.includes(' ') && matchesKey(event, keys))
        const action = fallback && actions[fallback.id]
        timer = setTimeout(() => {
          reset()
          if (fallback && action === store.actions()[fallback.id]) {
            action?.()
          }
        }, 800)
        return
      }
      const found = bindings.find(({ keys }) => !keys.includes(' ') && matchesKey(event, keys))
      if (found) {
        event.preventDefault()
        actions[found.id]?.()
      }
    }
    window.addEventListener('keydown', handle)
    window.addEventListener('blur', reset)
    document.addEventListener('focusin', reset)
    document.addEventListener('pointerdown', reset)
    return () => {
      reset()
      window.removeEventListener('keydown', handle)
      window.removeEventListener('blur', reset)
      document.removeEventListener('focusin', reset)
      document.removeEventListener('pointerdown', reset)
    }
  }, [store])
  return <Context value={store}>{children}</Context>
}
