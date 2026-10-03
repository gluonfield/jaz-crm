import { useState } from 'react'
import { ArrowUp, LoaderCircle, Smile, Sparkles, Target, Undo2, WandSparkles } from 'lucide-react'
import { Button } from '@jaz/ui/button'
import type { DraftRewriteAction } from '@/lib/types'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu'

export function DraftRewrite({ pending, disabled, canUndo, onRewrite, onUndo }: {
  pending: boolean
  disabled: boolean
  canUndo: boolean
  onRewrite: (action: DraftRewriteAction, instruction?: string) => void
  onUndo: () => void
}) {
  const [open, setOpen] = useState(false)
  const [instruction, setInstruction] = useState('')
  const run = (action: DraftRewriteAction, instruction?: string) => {
    setOpen(false)
    onRewrite(action, instruction)
  }
  return (
    <div className="flex flex-wrap items-center gap-1 px-2 pb-1">
      <Button variant="ghost" size="sm" disabled={disabled || pending} onClick={() => run('shorten')}>Shorten</Button>
      <Button variant="ghost" size="sm" disabled={disabled || pending} onClick={() => run('less_salesy')}>Less salesy</Button>
      <DropdownMenu open={open} onOpenChange={setOpen}>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="sm" disabled={disabled || pending}><Sparkles /> Edit with AI…</Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" collisionPadding={8} className="w-72 max-w-[calc(100vw-2rem)]">
          <DropdownMenuItem onSelect={() => run('one_clear_ask')}><Target /> One clear ask</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => run('warmer')}><Smile /> Warmer</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => run('polish')}><WandSparkles /> Polish</DropdownMenuItem>
          <DropdownMenuSeparator />
          <form className="flex items-center gap-1 px-2 py-1" onSubmit={(event) => {
            event.preventDefault()
            if (instruction.trim()) {
              run('custom', instruction.trim())
              setInstruction('')
            }
          }}>
            <input aria-label="Rewrite instruction" placeholder="Tell AI what to change…" value={instruction} onChange={(event) => setInstruction(event.target.value)} onKeyDown={(event) => {
              event.stopPropagation()
              if (event.key === 'Escape') {
                setOpen(false)
              }
            }} className="min-w-0 flex-1 bg-transparent py-1 text-[13px] text-ink outline-none placeholder:text-ink-3" />
            <Button type="submit" variant="ghost" size="icon-sm" aria-label="Apply instruction" disabled={!instruction.trim()}><ArrowUp /></Button>
          </form>
        </DropdownMenuContent>
      </DropdownMenu>
      {pending ? <span role="status" className="flex items-center gap-1.5 px-2 text-[12px] text-ink-3"><LoaderCircle className="size-3.5 animate-spin motion-reduce:animate-none" /> Rewriting…</span> : canUndo && <Button variant="ghost" size="sm" disabled={disabled} onClick={onUndo}><Undo2 /> Undo</Button>}
    </div>
  )
}
