import { Button } from '@jaz/ui/button'
import { BookOpen, BriefcaseBusiness, Building2, CalendarDays, ChartNoAxesCombined, Check, CircleCheck, ClipboardList, Code2, Compass, Factory, FileText, Flag, Folder, Globe, Heart, ImagePlus, Layers, Lightbulb, MessageSquare, Package, Rocket, Search, Settings, Shield, SmilePlus, Star, Target, Users, Wrench, Zap } from 'lucide-react'
import { type ReactNode, useRef, useState } from 'react'
import { toast } from 'sonner'
import { pageImage } from '@/lib/page-image'
import { pageIcon } from '@/lib/pages'
import { useAction } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover'

const symbols = {
  'book-open': BookOpen, briefcase: BriefcaseBusiness, building: Building2, calendar: CalendarDays,
  chart: ChartNoAxesCombined, checklist: ClipboardList, code: Code2, compass: Compass, factory: Factory,
  file: FileText, flag: Flag, folder: Folder, globe: Globe, heart: Heart, layers: Layers,
  lightbulb: Lightbulb, message: MessageSquare, package: Package, rocket: Rocket, settings: Settings,
  shield: Shield, star: Star, target: Target, team: Users, wrench: Wrench, zap: Zap, check: CircleCheck,
}
const emojis = [
  ['📄', 'Document'], ['📝', 'Notes'], ['📚', 'Books'], ['📖', 'Reading'], ['📋', 'Checklist'], ['🗂️', 'Files'], ['📁', 'Folder'],
  ['💡', 'Idea'], ['🎯', 'Target'], ['🚀', 'Rocket'], ['⭐', 'Star'], ['✨', 'Sparkles'], ['🔥', 'Fire'], ['⚡', 'Lightning'],
  ['🏭', 'Factory'], ['🔧', 'Wrench'], ['⚙️', 'Settings'], ['🛠️', 'Tools'], ['📦', 'Package'], ['🔬', 'Research'], ['🧪', 'Experiment'],
  ['💼', 'Business'], ['🏢', 'Office'], ['👥', 'Team'], ['🤝', 'Partnership'], ['💬', 'Discussion'], ['📣', 'Marketing'], ['📧', 'Email'],
  ['📊', 'Chart'], ['📈', 'Growth'], ['💰', 'Money'], ['💳', 'Payment'], ['🧾', 'Receipt'], ['📅', 'Calendar'], ['⏰', 'Deadline'],
  ['✅', 'Done'], ['☑️', 'Check'], ['🔒', 'Private'], ['🛡️', 'Security'], ['🚩', 'Flag'], ['🔖', 'Bookmark'], ['📌', 'Pin'],
  ['🌍', 'World'], ['🧭', 'Compass'], ['🗺️', 'Map'], ['🏠', 'Home'], ['🎨', 'Design'], ['🎓', 'Learning'], ['🧠', 'Brain'],
  ['💻', 'Computer'], ['🤖', 'Robot'], ['🔗', 'Link'], ['❤️', 'Heart'], ['🌱', 'Growth seedling'], ['🌳', 'Tree'], ['☀️', 'Sun'],
].map(([value, label]) => ({ value, label }))
const icons = Object.keys(symbols).map((name) => ({ value: `icon:${name}`, label: name.replaceAll('-', ' ') }))
const segmenter = new Intl.Segmenter(undefined, { granularity: 'grapheme' })

export function PageIcon({ value, size = 16, className }: { value?: string; size?: number; className?: string }) {
  const [failed, setFailed] = useState<string>()
  const image = value?.startsWith('image:')
  const name = value?.slice(5) ?? ''
  const Icon = Object.hasOwn(symbols, name) ? symbols[name as keyof typeof symbols] : FileText
  return (
    <span aria-hidden style={{ width: size, height: size, fontSize: size }} className={cn('inline-flex shrink-0 select-none items-center justify-center overflow-hidden leading-none text-ink-2', className)}>
      {image && failed !== value ? <img src={value?.slice(6)} alt="" referrerPolicy="no-referrer" onError={() => setFailed(value)} className="size-full rounded-[20%] object-contain outline outline-1 -outline-offset-1 outline-black/10 dark:outline-white/10" /> : value && !image && !value.startsWith('icon:') ? value : <Icon style={{ width: size, height: size }} />}
    </span>
  )
}

export function PageIconPicker({ record, children }: { record: CrmRecord; children?: ReactNode }) {
  const value = pageIcon(record)
  const write = useAction<object>('upsert_record')
  const [open, setOpen] = useState(false)
  const [tab, setTab] = useState<'emojis' | 'icons' | 'image'>('emojis')
  const fileInput = useRef<HTMLInputElement>(null)
  const [preparing, setPreparing] = useState(false)
  const busy = preparing || write.isPending
  const [search, setSearch] = useState('')
  const query = search.trim().toLowerCase()
  const custom = query && [...segmenter.segment(query)].length === 1 && /[\p{Extended_Pictographic}\p{Regional_Indicator}\u20e3]/u.test(query)
  const options = (tab === 'emojis' ? emojis : icons).filter((o) => `${o.label} ${o.value}`.toLowerCase().includes(query))
  if (tab === 'emojis' && custom && !options.some((o) => o.value === query)) {
    options.unshift({ value: query, label: 'Pasted emoji' })
  }
  const pick = (icon?: string) => write.mutate({ object: 'pages', record_id: record.id, ...(icon ? { values: { icon } } : { remove: { icon: [] } }) }, { onSuccess: () => setOpen(false) })
  return (
    <Popover open={open} onOpenChange={(next) => {
      setOpen(next)
      setSearch('')
      if (next) {
        setTab(value?.startsWith('image:') ? 'image' : value?.startsWith('icon:') ? 'icons' : 'emojis')
      }
    }}>
      <PopoverTrigger asChild>
        {children ?? <Button variant="ghost" aria-label={value ? 'Change page icon' : 'Add page icon'} className={cn('-ml-2 mb-3 self-start', value && 'size-16')}>
          {value ? <PageIcon value={value} size={48} /> : <><SmilePlus /> Add icon</>}
        </Button>}
      </PopoverTrigger>
      <PopoverContent align="start" collisionPadding={8} className="w-80 max-w-[calc(100vw-1rem)] overflow-hidden p-0" aria-label="Page icon">
        <div className="flex items-center gap-1 border-b border-border p-2">
          {(['emojis', 'icons', 'image'] as const).map((kind) => <Button key={kind} aria-pressed={tab === kind} disabled={busy} className={tab === kind ? 'bg-list-active' : undefined} onClick={() => {
            setTab(kind)
            setSearch('')
          }}>{kind === 'emojis' ? 'Emojis' : kind === 'icons' ? 'Icons' : 'Image'}</Button>)}
          {value && <Button variant="ghost" className="ml-auto text-ink-3" disabled={busy} onClick={() => pick()}>Remove</Button>}
        </div>
        {tab === 'image' ? <div className="flex flex-col items-center gap-3 p-3">
          {value?.startsWith('image:') && <PageIcon value={value} size={64} />}
          <Button className="h-10 w-full" autoFocus disabled={busy} onClick={() => fileInput.current?.click()}><ImagePlus /> {busy ? 'Uploading…' : value?.startsWith('image:') ? 'Replace image' : 'Upload image'}</Button>
          <input ref={fileInput} className="hidden" type="file" accept="image/*" aria-label="Upload page icon" disabled={busy} onChange={async (event) => {
            const file = event.target.files?.[0]
            event.target.value = ''
            if (!file) {
              return
            }
            setPreparing(true)
            try {
              pick(await pageImage(file))
            } catch (error) {
              toast.error(error instanceof Error ? error.message : 'Could not open this image')
            } finally {
              setPreparing(false)
            }
          }} />
        </div> : <>
          <label className="flex items-center gap-2 px-3 text-ink-3">
            <Search className="size-4 shrink-0" />
            <input autoFocus type="search" aria-label="Search icons or paste an emoji" placeholder={tab === 'emojis' ? 'Search or paste an emoji…' : 'Search icons…'} value={search} onChange={(e) => setSearch(e.target.value)} onKeyDown={(e) => {
              if (e.key === 'ArrowDown') {
                e.preventDefault()
                e.currentTarget.closest('[data-slot=popover-content]')?.querySelector<HTMLButtonElement>('[data-icon-option]')?.focus()
              }
            }} className="h-10 min-w-0 flex-1 bg-transparent text-[13px] text-ink outline-none placeholder:text-ink-3" />
          </label>
          <div className="scrollbar-quiet grid max-h-64 grid-cols-7 overflow-y-auto p-2 pt-0" onKeyDown={(e) => {
            const buttons = [...e.currentTarget.querySelectorAll<HTMLButtonElement>('[data-icon-option]')]
            const at = buttons.indexOf(document.activeElement as HTMLButtonElement)
            const step = { ArrowRight: 1, ArrowLeft: -1, ArrowDown: 7, ArrowUp: -7 }[e.key]
            if (at >= 0 && step !== undefined) {
              e.preventDefault()
              buttons[Math.max(0, Math.min(buttons.length - 1, at + step))]?.focus()
            }
          }}>
            {options.map((option) => <Button key={option.value} variant="ghost" size="icon" data-icon-option aria-label={`Choose ${option.label}`} title={option.label} aria-pressed={value === option.value} disabled={busy} onClick={() => pick(option.value)} className={cn('relative size-10', value === option.value && 'bg-list-active')}>
              <PageIcon value={option.value} size={22} />
              {value === option.value && <Check className="absolute bottom-0.5 right-0.5 size-2.5 text-primary" />}
            </Button>)}
            {options.length === 0 && <p className="col-span-7 py-5 text-center text-[12px] text-ink-3">No icons found</p>}
          </div>
        </>}
      </PopoverContent>
    </Popover>
  )
}
