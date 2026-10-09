import { type MouseEvent, type ReactNode, useState } from 'react'
import { ArrowUp, BellRing, Check, ChevronDown, LoaderCircle, Phone, Smile, Sparkles, Target, Undo2, WandSparkles, X } from 'lucide-react'
import { Button } from '@jaz/ui/button'
import type { DraftRewriteAction } from '@/lib/types'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu'

type Run = (action: DraftRewriteAction, instruction?: string) => void
type Preset = { action: DraftRewriteAction; label: string; icon: ReactNode }

export function DraftRewrite({ pending, disabled, canUndo, onRewrite, onUndo }: {
  pending: boolean
  disabled: boolean
  canUndo: boolean
  onRewrite: Run
  onUndo: () => void
}) {
  return (
    <div className="flex flex-wrap items-center gap-1">
      <Button variant="ghost" size="sm" disabled={disabled || pending} onClick={() => onRewrite('shorten')}>Shorten</Button>
      <Button variant="ghost" size="sm" disabled={disabled || pending} onClick={() => onRewrite('less_salesy')}>Less salesy</Button>
      <AiMenu
        trigger={<Button variant="ghost" size="sm" disabled={disabled || pending}><Sparkles /> Edit with AI…</Button>}
        presets={[{ action: 'one_clear_ask', label: 'One clear ask', icon: <Target /> }, { action: 'warmer', label: 'Warmer', icon: <Smile /> }, { action: 'polish', label: 'Polish', icon: <WandSparkles /> }]}
        custom="custom"
        placeholder="Tell AI what to change…"
        onRun={onRewrite}
      />
      {pending ? <span role="status" className="flex items-center gap-1.5 px-2 text-[12px] text-ink-3"><LoaderCircle className="size-3.5 animate-spin motion-reduce:animate-none" /> Rewriting…</span> : canUndo && <Button variant="ghost" size="sm" disabled={disabled} onClick={onUndo}><Undo2 /> Undo</Button>}
    </div>
  )
}

// Pressing keeps focus where it is: focus would reveal the composer's idle
// rows and move the button from under the pointer before the click lands.
const keepFocus = (event: MouseEvent) => event.preventDefault()

// DraftWrite asks AI for a first draft: whatever the follow-up needs, or a
// chosen kind of reply.
export function DraftWrite({ pending, onWrite }: { pending: boolean; onWrite: Run }) {
  return (
    <div className="m-2 flex shrink-0 items-center">
      <Button variant="ghost" size="sm" disabled={pending} onMouseDown={keepFocus} onClick={() => onWrite('write')} className="rounded-r-none">
        {pending ? <LoaderCircle className="animate-spin motion-reduce:animate-none" /> : <Sparkles />}
        {pending ? 'Drafting…' : 'Draft with AI'}
      </Button>
      <AiMenu
        trigger={<Button variant="ghost" size="sm" disabled={pending} onMouseDown={keepFocus} aria-label="Choose what to draft" className="rounded-l-none px-1"><ChevronDown /></Button>}
        presets={[{ action: 'nudge', label: 'Nudge', icon: <BellRing /> }, { action: 'accept', label: 'Accept', icon: <Check /> }, { action: 'decline', label: 'Decline', icon: <X /> }, { action: 'call', label: 'Suggest a call', icon: <Phone /> }]}
        custom="write"
        placeholder="Tell AI what to write…"
        align="end"
        onRun={onWrite}
      />
    </div>
  )
}

// AiMenu offers preset AI actions, then a field to say what to do instead.
function AiMenu({ trigger, presets, custom, placeholder, align = 'start', onRun }: { trigger: ReactNode; presets: Preset[]; custom: DraftRewriteAction; placeholder: string; align?: 'start' | 'end'; onRun: Run }) {
  const [open, setOpen] = useState(false)
  const [instruction, setInstruction] = useState('')
  const run: Run = (action, instruction) => {
    setOpen(false)
    onRun(action, instruction)
  }
  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
      <DropdownMenuContent align={align} collisionPadding={8} className="w-72 max-w-[calc(100vw-2rem)]">
        {presets.map((preset) => <DropdownMenuItem key={preset.action} onSelect={() => run(preset.action)}>{preset.icon} {preset.label}</DropdownMenuItem>)}
        <DropdownMenuSeparator />
        <form className="flex items-center gap-1 px-2 py-1" onSubmit={(event) => {
          event.preventDefault()
          if (instruction.trim()) {
            run(custom, instruction.trim())
            setInstruction('')
          }
        }}>
          <input aria-label={placeholder.replace('…', '')} placeholder={placeholder} value={instruction} onChange={(event) => setInstruction(event.target.value)} onKeyDown={(event) => {
            event.stopPropagation()
            if (event.key === 'Escape') {
              setOpen(false)
            }
          }} className="min-w-0 flex-1 bg-transparent py-1 text-[13px] text-ink outline-none placeholder:text-ink-3" />
          <Button type="submit" variant="ghost" size="icon-sm" aria-label="Apply instruction" disabled={!instruction.trim()}><ArrowUp /></Button>
        </form>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
