import { Button } from '@jaz/ui/button'
import { type ReactNode, useId, useState } from 'react'
import { cn } from '@/lib/utils'
import { inputClass } from './controls'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'

type Naming = { title: string; initial?: string; placeholder?: string; action: string; onSubmit: (name: string) => void }

// NameDialog asks for a name, such as a new table's.
export function NameDialog({ open, onOpenChange, ...naming }: Naming & { open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false} className="w-[380px] gap-3 rounded-[12px] border-border bg-raised p-4 shadow-[var(--shadow-raised)]">
        <DialogTitle className="text-[14px] font-semibold text-ink">{naming.title}</DialogTitle>
        <NameForm {...naming} onSubmit={(name) => {
          onOpenChange(false)
          naming.onSubmit(name)
        }} />
      </DialogContent>
    </Dialog>
  )
}

// NameForm starts from the current name each time the dialog opens.
function NameForm({ initial = '', placeholder, action, onSubmit }: Naming) {
  const [name, setName] = useState(initial)
  return (
    <form
      className="flex gap-2"
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit(name.trim())
      }}
    >
      <input autoFocus aria-label="Name" value={name} onChange={(e) => setName(e.target.value)} placeholder={placeholder} className={cn(inputClass, 'flex-1')} />
      <Button variant="primary" type="submit" disabled={!name.trim() || name.trim() === initial}>
        {action}
      </Button>
    </form>
  )
}

// ConfirmDialog asks before deleting something for good.
export function ConfirmDialog({ open, onOpenChange, title, children, onConfirm }: { open: boolean; onOpenChange: (open: boolean) => void; title: string; children: ReactNode; onConfirm: () => void }) {
  const description = useId()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        aria-describedby={description}
        showCloseButton={false}
        onOpenAutoFocus={(e) => {
          const dialog = e.currentTarget as HTMLElement
          e.preventDefault()
          dialog.focus()
        }}
        className="bg-raised text-ink sm:max-w-sm"
      >
        <DialogTitle className="text-[14px] leading-snug [overflow-wrap:anywhere]">{title}</DialogTitle>
        <p id={description} className="text-[13px] leading-relaxed text-ink-2">
          {children}
        </p>
        <div className="flex justify-end gap-2">
          <Button onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            variant="danger"
            onClick={() => {
              onOpenChange(false)
              onConfirm()
            }}
          >
            Delete
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
