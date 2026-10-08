import { Button } from '@jaz/ui/button'
import { Check, ChevronDown, FileText, Search, SmilePlus } from 'lucide-react'
import { Tabs } from 'radix-ui'
import { type ReactNode, useState } from 'react'
import { colorValue, type IconColor, iconColors, pageEmojis, pageSymbol, pageSymbolNames, pageSymbols, symbolValue } from '@/lib/page-icons'
import { pageIcon } from '@/lib/pages'
import { useAction } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './ui/dropdown-menu'
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover'

const segmenter = new Intl.Segmenter(undefined, { granularity: 'grapheme' })
const symbols = pageSymbolNames.map((name) => ({ name, value: symbolValue(name, 'default'), label: name.replace(/([a-z0-9])([A-Z])/g, '$1 $2').toLowerCase() }))

export function PageIcon({ value, size = 16, className }: { value?: string; size?: number; className?: string }) {
  const [failed, setFailed] = useState<string>()
  const image = value?.startsWith('image:')
  const symbol = pageSymbol(value)
  const Icon = symbol?.name ? pageSymbols[symbol.name] : FileText
  return (
    <span aria-hidden style={{ width: size, height: size, fontSize: size, ...(symbol && { color: colorValue(symbol.color) }) }} className={cn('inline-flex shrink-0 select-none items-center justify-center overflow-hidden leading-none text-ink-2', className)}>
      {image && failed !== value ? <img src={value?.slice(6)} alt="" referrerPolicy="no-referrer" onError={() => setFailed(value)} className="size-full rounded-[20%] object-contain outline outline-1 -outline-offset-1 outline-black/10 dark:outline-white/10" /> : value && !image && !symbol ? value : <Icon style={{ width: size, height: size }} />}
    </span>
  )
}

export function PageIconPicker({ record, children }: { record: CrmRecord; children?: ReactNode }) {
  const value = pageIcon(record)
  const saved = pageSymbol(value)
  const write = useAction<object>('upsert_record')
  const [open, setOpen] = useState(false)
  const [tab, setTab] = useState('emojis')
  const [color, setColor] = useState<IconColor>('default')
  const [link, setLink] = useState('')
  const [search, setSearch] = useState('')
  const query = search.trim().toLowerCase()
  const custom = query && [...segmenter.segment(query)].length === 1 && /[\p{Extended_Pictographic}\p{Regional_Indicator}\u20e3]/u.test(query)
  const options = (tab === 'emojis' ? pageEmojis : symbols).filter((option) => (option.label + ' ' + option.value).toLowerCase().includes(query))
  if (tab === 'emojis' && custom && !options.some((option) => option.value === query)) {
    options.unshift({ value: query, label: 'Pasted emoji' })
  }
  const pick = (icon?: string, close = true) => write.mutate({ object: 'pages', record_id: record.id, ...(icon ? { values: { icon } } : { remove: { icon: [] } }) }, { onSuccess: () => {
    if (close) {
      setOpen(false)
    }
  } })
  return (
    <Popover open={open} onOpenChange={(next) => {
      setOpen(next)
      setSearch('')
      if (next) {
        setTab(value?.startsWith('image:') ? 'image' : saved ? 'icons' : 'emojis')
        setColor(saved?.color ?? 'default')
        setLink(value?.startsWith('image:') ? value.slice(6) : '')
      }
    }}>
      <PopoverTrigger asChild>
        {children ?? <Button variant="ghost" aria-label={value ? 'Change page icon' : 'Add page icon'} className={cn('-ml-2 mb-3 self-start', value && 'size-16')}>
          {value ? <PageIcon value={value} size={48} /> : <><SmilePlus /> Add icon</>}
        </Button>}
      </PopoverTrigger>
      <PopoverContent align="start" collisionPadding={8} className="w-80 max-w-[calc(100vw-1rem)] overflow-hidden p-0" aria-label="Page icon">
        <Tabs.Root value={tab} onValueChange={(next) => {
          setTab(next)
          setSearch('')
        }}>
          <div className="flex items-center border-b border-border px-2">
            <Tabs.List aria-label="Icon type" className="flex">
              {(['emojis', 'icons', 'image'] as const).map((kind) => <Tabs.Trigger key={kind} value={kind} asChild>
                <Button variant="ghost" disabled={write.isPending} className="h-10 rounded-none border-0 border-b-2 border-transparent px-3 hover:bg-transparent data-[state=active]:border-ink data-[state=active]:text-ink">{kind === 'emojis' ? 'Emojis' : kind === 'icons' ? 'Icons' : 'Image'}</Button>
              </Tabs.Trigger>)}
            </Tabs.List>
            {value && <Button variant="ghost" className="ml-auto text-ink-3" disabled={write.isPending} onClick={() => pick()}>Remove</Button>}
          </div>
          <Tabs.Content value={tab} className="outline-none">
            {tab === 'image' ? <form className="flex flex-col items-center gap-3 p-3" onSubmit={(event) => {
              event.preventDefault()
              pick('image:' + link.trim())
            }}>
              {value?.startsWith('image:') && <PageIcon value={value} size={64} />}
              <input autoFocus type="url" required pattern="https?://.+" aria-label="Image URL" placeholder="Paste image URL…" value={link} onChange={(event) => setLink(event.target.value)} disabled={write.isPending} className="h-10 w-full min-w-0 rounded-[var(--radius-control)] border border-border bg-bg px-3 text-[13px] text-ink outline-none placeholder:text-ink-3 focus:border-primary" />
              <Button type="submit" className="h-10 w-full" disabled={write.isPending}>Apply</Button>
            </form> : <>
              <div className="flex items-center gap-1 px-3">
                <label className="flex min-w-0 flex-1 items-center gap-2 text-ink-3">
                  <Search className="size-4 shrink-0" />
                  <input autoFocus type="search" aria-label="Search icons or paste an emoji" placeholder={tab === 'emojis' ? 'Search or paste an emoji…' : 'Search icons…'} value={search} onChange={(event) => setSearch(event.target.value)} onKeyDown={(event) => {
                    if (event.key === 'ArrowDown') {
                      event.preventDefault()
                      event.currentTarget.closest('[data-slot=popover-content]')?.querySelector<HTMLButtonElement>('[data-icon-option]')?.focus()
                    }
                  }} className="h-10 min-w-0 flex-1 bg-transparent text-[13px] text-ink outline-none placeholder:text-ink-3" />
                </label>
                {tab === 'icons' && <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" aria-label={'Icon color: ' + color} disabled={write.isPending}><span className="size-3 rounded-full" style={{ backgroundColor: colorValue(color) }} /> Color <ChevronDown /></Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    {iconColors.map((choice) => <DropdownMenuItem key={choice} onSelect={() => {
                      setColor(choice)
                      if (saved?.name) {
                        pick(symbolValue(saved.name, choice), false)
                      }
                    }}>
                      <span className="size-3 shrink-0 rounded-full" style={{ backgroundColor: colorValue(choice) }} />
                      <span className="flex-1 capitalize">{choice}</span>
                      {color === choice && <Check />}
                    </DropdownMenuItem>)}
                  </DropdownMenuContent>
                </DropdownMenu>}
              </div>
              <div className="scrollbar-quiet grid max-h-64 grid-cols-7 overflow-y-auto p-2 pt-0" onKeyDown={(event) => {
                const buttons = [...event.currentTarget.querySelectorAll<HTMLButtonElement>('[data-icon-option]')]
                const at = buttons.indexOf(document.activeElement as HTMLButtonElement)
                const step = { ArrowRight: 1, ArrowLeft: -1, ArrowDown: 7, ArrowUp: -7 }[event.key]
                if (at >= 0 && step !== undefined) {
                  event.preventDefault()
                  buttons[Math.max(0, Math.min(buttons.length - 1, at + step))]?.focus()
                }
              }}>
                {options.map((option) => {
                  const symbol = pageSymbol(option.value)
                  const selected = symbol ? saved?.name === symbol.name : value === option.value
                  const icon = symbol?.name ? symbolValue(symbol.name, color) : option.value
                  return <Button key={option.value} variant="ghost" size="icon" data-icon-option aria-label={'Choose ' + option.label} title={option.label} aria-pressed={selected} disabled={write.isPending} onClick={() => pick(icon)} className={cn('relative size-10 [content-visibility:auto]', selected && 'bg-list-active')}>
                    <PageIcon value={icon} size={22} />
                    {selected && <Check className="absolute bottom-0.5 right-0.5 size-2.5 text-primary" />}
                  </Button>
                })}
                {options.length === 0 && <p className="col-span-7 py-5 text-center text-[12px] text-ink-3">No icons found</p>}
              </div>
            </>}
          </Tabs.Content>
        </Tabs.Root>
      </PopoverContent>
    </Popover>
  )
}
