import { useSyncExternalStore } from 'react'
import { readItem, writeItem } from './storage'
import { embedded } from './api'

export type Scheme = 'light' | 'dark'
export type SchemePreference = Scheme | 'system'
// Style is the look over either scheme: the standard one or Correspondence,
// a serif, relationship-first design being tried out (see THEMING.md).
export type Style = 'standard' | 'correspondence'

// ThemeMessage is what a framing page posts to theme the app (see THEMING.md).
export type ThemeMessage = { type: 'jaz:theme'; vars?: Record<string, string>; scheme?: Scheme }

const preferenceKey = 'jaz-crm:scheme'
const styleKey = 'jaz-crm:style'
const listeners = new Set<() => void>()
let hostScheme: Scheme | null = null

const systemDark = () => window.matchMedia('(prefers-color-scheme: dark)').matches

export function schemePreference(): SchemePreference {
  return (readItem(preferenceKey) as SchemePreference | null) ?? 'system'
}

function resolvedScheme(): Scheme {
  const preference = schemePreference()
  return hostScheme ?? (preference === 'system' ? (systemDark() ? 'dark' : 'light') : preference)
}

export function stylePreference(): Style {
  return readItem(styleKey) === 'correspondence' ? 'correspondence' : 'standard'
}

function render() {
  document.documentElement.classList.toggle('dark', resolvedScheme() === 'dark')
  document.documentElement.classList.toggle('correspondence', stylePreference() === 'correspondence')
  listeners.forEach((listener) => listener())
}

export function setSchemePreference(preference: SchemePreference) {
  writeItem(preferenceKey, preference)
  hostScheme = null
  render()
}

export function setStylePreference(style: Style) {
  writeItem(styleKey, style)
  render()
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function useScheme(): Scheme {
  return useSyncExternalStore(subscribe, resolvedScheme, () => 'light')
}

export function useStyle(): Style {
  return useSyncExternalStore(subscribe, stylePreference, () => 'standard')
}

// Only custom properties are accepted, and never values that could load
// resources, so a host can restyle the app but not inject content.
const varName = /^--[a-zA-Z0-9-]+$/

export function applyTheme(message: Omit<ThemeMessage, 'type'>) {
  const root = document.documentElement.style
  for (const [name, value] of Object.entries(message.vars ?? {})) {
    if (varName.test(name) && typeof value === 'string' && !/url\(|expression\(/i.test(value)) {
      root.setProperty(name, value)
    }
  }
  if (message.scheme === 'light' || message.scheme === 'dark') {
    hostScheme = message.scheme
  }
  render()
}

// ?theme= is either a scheme name or base64url JSON of { vars, scheme }.
function themeFromURL(): Omit<ThemeMessage, 'type'> | null {
  const param = new URLSearchParams(window.location.search).get('theme')
  if (!param) {
    return null
  }
  if (param === 'light' || param === 'dark') {
    return { scheme: param }
  }
  try {
    return JSON.parse(atob(param.replace(/-/g, '+').replace(/_/g, '/')))
  } catch {
    return null
  }
}

export function installHostBridge() {
  const fromURL = themeFromURL()
  if (fromURL) {
    applyTheme(fromURL)
  }
  render()
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', render)
  window.addEventListener('message', (event: MessageEvent<ThemeMessage>) => {
    if (event.source === window.parent && window.parent !== window && event.data?.type === 'jaz:theme') {
      applyTheme(event.data)
    }
  })
  if (window.parent !== window && !embedded()) {
    window.parent.postMessage({ type: 'jaz:ready' }, '*')
  }
}
