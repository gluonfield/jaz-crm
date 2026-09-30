export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
  }
}

// transport replaces HTTP when the app runs as an MCP App, where tools are
// called through the host.
let transport: ((tool: string, args: object) => Promise<unknown>) | null = null

export function setTransport(next: typeof transport) {
  transport = next
}

// embedded reports whether the app runs as an MCP App, without a session.
export function embedded() {
  return transport !== null
}

export function signIn() {
  window.location.assign(`/auth/login?return_to=${encodeURIComponent(window.location.pathname + window.location.search)}`)
}

// call runs one of the server's tools and returns its output.
export async function call<T>(tool: string, args: object = {}): Promise<T> {
  if (transport) {
    return (await transport(tool, args)) as T
  }
  return read<T>(await fetch(`/api/tools/${tool}`, request('POST', args)))
}

// rest calls the session-only account endpoints, which accept only JSON.
export async function rest<T>(method: string, path: string, body?: unknown): Promise<T> {
  if (transport) {
    throw new ApiError('Account settings are available in the Jaz CRM web app', 401)
  }
  return read<T>(await fetch(path, request(method, method === 'GET' ? undefined : (body ?? {}))))
}

function request(method: string, body: unknown): RequestInit {
  return { method, credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) }
}

async function read<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    signIn()
    throw new ApiError('Not signed in', 401)
  }
  const body = res.status === 204 ? undefined : await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new ApiError(body?.error ?? `Request failed (${res.status})`, res.status)
  }
  return body as T
}
