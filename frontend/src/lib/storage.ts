// storage is localStorage with an in-memory fallback: a sandboxed iframe
// with an opaque origin (the MCP App) throws on every access.
const memory = new Map<string, string>()

export function readItem(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return memory.get(key) ?? null
  }
}

export function writeItem(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    memory.set(key, value)
  }
}
