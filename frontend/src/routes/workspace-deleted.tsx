import { createFileRoute } from '@tanstack/react-router'
import { embedded } from '@/lib/api'

export const Route = createFileRoute('/workspace-deleted')({ component: WorkspaceDeleted })

function WorkspaceDeleted() {
  return (
    <main className="flex min-h-dvh items-center justify-center bg-bg p-6 text-ink">
      <div className="max-w-sm text-center">
        <h1 className="text-lg font-semibold">Workspace deleted</h1>
        <p className="mt-3 text-[13px] leading-relaxed text-ink-2">
          {embedded() ? 'Reconnect Jaz CRM to access another workspace.' : 'Sign in to access another workspace or start a new one.'}
        </p>
        {!embedded() && <a href="/auth/login?return_to=/" className="mt-5 inline-flex h-8 items-center rounded-full bg-primary px-4 text-[13px] font-medium text-on-primary hover:bg-primary-strong">Sign in</a>}
      </div>
    </main>
  )
}
